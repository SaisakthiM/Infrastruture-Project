# Infra layout

This replaces the old single `environments/dev/` mono-environment with five
purpose-scoped environments, managed with Terragrunt, planned/applied
through Atlantis, with Kubernetes-side state handed off to ArgoCD.

```
environments/
  terragrunt.hcl        # root: generates a local backend per environment
  prod-gateway/         # foundation -- network + nginx, zero dependencies
  prod-social/          # foundation -- kind cluster, ArgoCD, social-media images/secrets
  prod-docker/          # every non-k8s app stack (notes, bank, quiz, video, ...)
  prod-infra/           # otel-gateway, node-exporter, n8n, jenkins + the observability app-of-apps
  prod-manage/          # one glue resource: connects gateway's container to the kind network
modules/
  docker_app/           # unchanged from the original project
  networking/           # new -- wraps docker_network
gitops/
  social-media/{raw,apps}/      # synced by ArgoCD, not Terraform
  observability/{raw,apps}/     # synced by ArgoCD, not Terraform
atlantis.yaml
```

## Apply order

`prod-gateway` and `prod-social` have no dependencies and can apply in
parallel. Everything else needs one or both:

```
prod-gateway ─┬─→ prod-docker
              ├─→ prod-infra ←─ prod-social
              └─→ prod-manage ←─ prod-social
```

With Terragrunt, just run from `environments/`:

```
terragrunt run-all apply
```

It reads each environment's `dependencies` block and applies in the right
order automatically. Each environment can also be applied on its own
(`cd environments/prod-gateway && terragrunt apply`) for routine changes.

**Before your first apply anywhere**, replace `git@github.com:SaisakthiM/Coding-Project.git` in:

- `environments/prod-social/terraform.tfvars` (`gitops_repo_url`)
- `environments/prod-infra/terraform.tfvars` (`gitops_repo_url`)
- `gitops/social-media/apps/social-workload-app.yaml`
- every file under `gitops/observability/apps/` that has a `git@github.com:SaisakthiM/Coding-Project.git` source

with wherever you're actually pushing this whole tree. ArgoCD needs to be
able to clone it.

## What moved where, and why

- **prod-gateway** owns the `gateway-net` docker network and the nginx
  gateway. It has zero dependencies on purpose -- nothing else should ever
  block it from applying. Everything else joins `gateway-net` by the literal
  string, never by a `docker_network.gateway_net.name` reference, since
  that resource doesn't exist in their state.
- **prod-docker** owns every actual app stack and all their volumes,
  _except_ the gateway's own `intro`/`record` landing-page volumes (gateway
  produces and consumes those itself, so they stayed with gateway).
- **prod-social** owns the kind cluster, the social-media app's build
  images, and ArgoCD itself. All the raw Kubernetes objects that used to be
  `kubectl_manifest`/`helm_release` resources in `kubernetes.tf` are now
  plain YAML in `gitops/social-media/`, synced by ArgoCD instead of applied
  by Terraform. Terraform's job here shrank to: bootstrap the cluster,
  install ArgoCD, create the two Secrets that shouldn't be in git
  (`postgres-secret`, `social-minio-secret`), and create one Application
  object that points ArgoCD at `gitops/social-media/apps/`.
- **prod-infra** owns everything that isn't Kubernetes-managed observability
  (otel-gateway, node-exporter, n8n, jenkins) plus one Application object
  pointing ArgoCD at `gitops/observability/apps/`. Same Secret pattern for
  the observability Redis's password.
- **prod-manage** is just the one resource that's _actually_ about two
  other environments at once: connecting the gateway container to the kind
  cluster's docker network. Nothing else needed a dedicated shared
  environment -- an earlier draft of this plan put the kind cluster +
  ingress-nginx here too, but that was dropped in favor of just owning them
  in prod-social and letting Terragrunt's dependency graph handle ordering.

## ArgoCD app-of-apps

Each of `prod-social` / `prod-infra` creates exactly one Terraform-managed
`Application` (the "app-of-apps"), which points at a folder of _plain YAML_
child `Application` objects:

- `gitops/social-media/apps/`: `ingress-nginx-app.yaml` (helm chart, values
  copied verbatim from the original `helm_release`), `social-workload-app.yaml`
  (kustomize path → `gitops/social-media/raw/`, the extracted postgres/
  backend/frontend/kafka/cassandra/redis/minio/ingress manifests).
- `gitops/observability/apps/`: one Application per original `helm_release`
  (redis, kube-prometheus-stack, loki, tempo, promtail, otel-collector,
  jaeger) plus `observability-raw-app.yaml` for the leftover
  `kubectl_manifest` objects (otel nodeport, jaeger config, ingresses).

Several of those use ArgoCD's **multi-source** Application pattern
(`sources:` with a second entry and `ref: values`) to pull the _existing_
external Helm values files (`prometheus.yml`, `loki-config.yml`,
`tempo-config.yml`, etc.) straight from `projects/platform/observability/`
in your repo, exactly where the original `local.obs_path` already pointed.
Their content wasn't duplicated or guessed at anywhere in this rewrite.

## Secrets

Real credentials (postgres password, social MinIO user/password, the
observability Redis password) are created directly by Terraform as
Kubernetes `Secret` objects, **not** committed to git, and referenced from
the gitops-managed manifests via `secretKeyRef` / `existingSecret`. Non-secret
identifiers that used to be variables (`social_db_name`, `social_db_user`)
are now just hardcoded literal strings in the gitops YAML, since there's no
clean way for a plain git-synced manifest to read a Terraform variable.

## Known limitations / things to revisit

- **Chart versions could change.** Most of the original `helm_release`
  blocks pinned a version (Terraform just installed "whatever's latest" at
  apply time), and ArgoCD's Helm source type requires an explicit
  `targetRevision`. Every `gitops/*/apps/*.yaml` has a plausible-but-
  unverified version with a `# TODO` comment -- run, change if necessary
  `helm search repo <chart> --versions` and pin deliberately before you
  rely on any of these.
- **Some `depends_on` chains are gone for real, not just relocated.** A few
  of the original dependencies crossed what are now environment
  boundaries, and Terraform genuinely can't express a cross-state
  `depends_on`:
  - `otel-gateway` (prod-infra, docker) used to depend on
    `kubectl_manifest.otel_nodeport`. That NodePort Service is now
    gitops-managed; if otel-gateway starts before ArgoCD has synced it,
    OTLP exports just retry until the Service exists. No data loss, just a
    startup race.
  - Tempo used to depend on the social-media app's MinIO Service (storage
    backend), and Jaeger depended on ingress-nginx. Both still resolve fine
    at the Kubernetes DNS/Service level regardless of which Application
    created them, but ArgoCD doesn't sequence across separate root
    Applications the way Terraform's graph did. If either looks unhealthy
    right after a fresh bootstrap, give it a minute and hit Refresh.
  - The original Bitnami Redis (observability) also had a spurious
    dependency on the social-media app's Cassandra manifests -- that one
    was pure incidental apply-ordering in the original code, not a real
    functional link, and has been dropped rather than preserved.
- **`redis-password` and the Jenkins agent secret are still the original
  literal values**, just relocated into a Terraform variable (redis) or
  left as-is (Jenkins, since it's a real secret matched against the Jenkins
  master config and changing it would break agent registration). Rotate
  both when you get a chance.
- **Atlantis has to run on the same machine** as the kind cluster and
  Docker daemon -- this whole project assumes a single local dev box
  (hardcoded docker socket path, `local` Terraform backend, a `kind`
  cluster), so a remote/ephemeral Atlantis runner won't have access to
  either. If you ever move this to real infrastructure, the local backend
  and the hardcoded `/home/saisakthi/...` paths are the first things to
  generalize.
- **Crossplane was intentionally left out** of this pass.

## Dropped without migrating

- `main.tf.bak`, all `terraform.tfstate*` files, `.terraform/` -- not useful,
  and new environments start with fresh state anyway.
- The top-level `nginx.conf` -- stale, superseded by `nginx/default.conf`
  (which _did_ get carried forward, into `prod-gateway/nginx/default.conf`).
- `secret-file` -- a single unreferenced hash, not used by any `.tf` file in
  the original project. If something outside Terraform depends on it,
  it'll need to be reintroduced manually.

terraform {
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0"
    }
    kubectl = {
      source = "gavinbunney/kubectl"
    }
    helm = {
      source = "hashicorp/helm"
    }
  }
}

provider "docker" {
  host = "unix:///var/run/docker.sock"
}

provider "kubectl" {
  config_path    = "~/.kube/config"
  config_context = "kind-social-media"
}

provider "helm" {
  kubernetes = {
    config_path    = "~/.kube/config"
    config_context = "kind-social-media"
  }
}

# ---------------------------------------------------------------------------
# Build images for the social-media workload. Unchanged from the original
# kubernetes.tf except for relocation.
# ---------------------------------------------------------------------------
resource "docker_image" "social_django" {
  name         = "socialmediaapp-django:latest"
  keep_locally = true
  build {
    context    = abspath("${path.module}/../../projects/Social Media App/apps/backend")
    dockerfile = "Dockerfile"
  }
  triggers = {
    dir_sha = sha256(join("", [
      for f in fileset("${path.module}/../../projects/Social Media App/apps/backend", "**") :
      filesha256("${path.module}/../../projects/Social Media App/apps/backend/${f}")
      if !can(regex("(__pycache__|.pyc|.git)", f))
    ]))
  }
}

resource "docker_image" "social_frontend" {
  name         = "socialmediaapp-frontend-prod:latest"
  keep_locally = true
  build {
    context    = abspath("${path.module}/../../projects/Social Media App/apps/frontend")
    dockerfile = "Dockerfile"
  }
  triggers = {
    dir_sha = sha256(join("", [
      for f in fileset("${path.module}/../../projects/Social Media App/apps/frontend", "**") :
      filesha256("${path.module}/../../projects/Social Media App/apps/frontend/${f}")
      if !can(regex("(node_modules|dist|.git)", f))
    ]))
  }
}

resource "docker_image" "social_go" {
  name         = "socialmediaapp-microservice-go:latest"
  keep_locally = true
  build {
    context    = abspath("${path.module}/../../projects/Social Media App/apps/microservice-go")
    dockerfile = "Dockerfile"
  }
  triggers = {
    dir_sha = sha256(join("", [
      for f in fileset("${path.module}/../../projects/Social Media App/apps/microservice-go", "**") :
      filesha256("${path.module}/../../projects/Social Media App/apps/microservice-go/${f}")
      if !can(regex(".git", f))
    ]))
  }
}

resource "docker_image" "social_java" {
  name         = "socialmediaapp-microservice-java:latest"
  keep_locally = true
  build {
    context    = abspath("${path.module}/../../projects/Social Media App/apps/microservice-java")
    dockerfile = "Dockerfile"
  }
  triggers = {
    dir_sha = sha256(join("", [
      for f in fileset("${path.module}/../../projects/Social Media App/apps/microservice-java", "**") :
      filesha256("${path.module}/../../projects/Social Media App/apps/microservice-java/${f}")
      if !can(regex("(target|.git)", f))
    ]))
  }
}

resource "docker_image" "social_minio" {
  name         = "socialmediaapp-minio:latest"
  keep_locally = true
  build {
    context    = abspath("${path.module}/../../projects/Social Media App/storage/minio")
    dockerfile = "Dockerfile"
  }
  triggers = {
    dir_sha = sha256(join("", [
      for f in fileset("${path.module}/../../projects/Social Media App/storage/minio", "**") :
      filesha256("${path.module}/../../projects/Social Media App/storage/minio/${f}")
      if !can(regex(".git", f))
    ]))
  }
}

# ---------------------------------------------------------------------------
# kind cluster bootstrap. This is the one and only place it's created --
# prod-infra reads nothing from it directly (just needs it to already exist,
# enforced via terragrunt "dependencies" ordering, not a real output).
# ---------------------------------------------------------------------------
resource "null_resource" "kind_cluster" {
  triggers = {
    kind_config = filesha256("${path.module}/../../projects/Social Media App/infrastructure/kind/kind-config.yaml")
  }
  provisioner "local-exec" {
    command = <<-EOT
      if ! kind get clusters | grep -q "social-media"; then
        kind create cluster --config "${abspath("${path.module}/../../projects/Social Media App/infrastructure/kind/kind-config.yaml")}"
      fi
    EOT
  }
  provisioner "local-exec" {
    when    = destroy
    command = "kind delete cluster --name social-media"
  }
}

resource "null_resource" "kind_images" {
  triggers = {
    manual = var.load_images
  }
  provisioner "local-exec" {
    command = <<EOT
      cd ~/.cache/
      # Cassandra
      docker pull --platform=linux/amd64 cassandra:5.0
      docker save cassandra:5.0 -o cassandra.tar
      docker cp cassandra.tar social-media-control-plane:/cassandra.tar
      docker cp cassandra.tar social-media-worker:/cassandra.tar
      docker cp cassandra.tar social-media-worker2:/cassandra.tar
      docker exec -i social-media-control-plane ctr -n k8s.io images import /cassandra.tar
      docker exec -i social-media-worker ctr -n k8s.io images import /cassandra.tar
      docker exec -i social-media-worker2 ctr -n k8s.io images import /cassandra.tar

      # Kafka
      docker pull --platform=linux/amd64 confluentinc/cp-kafka:7.6.0  
      docker save confluentinc/cp-kafka:7.6.0 -o kafka.tar
      docker cp kafka.tar social-media-control-plane:/kafka.tar
      docker cp kafka.tar social-media-worker:/kafka.tar
      docker cp kafka.tar social-media-worker2:/kafka.tar
      docker exec -i social-media-control-plane ctr -n k8s.io images import /kafka.tar
      docker exec -i social-media-worker ctr -n k8s.io images import /kafka.tar
      docker exec -i social-media-worker2 ctr -n k8s.io images import /kafka.tar
    EOT
  }
}

resource "null_resource" "kind_load_images" {
  depends_on = [
    null_resource.kind_cluster,
    docker_image.social_django,
    docker_image.social_frontend,
    docker_image.social_go,
    docker_image.social_java,
    docker_image.social_minio,
  ]
  triggers = {
    django_id   = docker_image.social_django.image_id
    frontend_id = docker_image.social_frontend.image_id
    go_id       = docker_image.social_go.image_id
    java_id     = docker_image.social_java.image_id
    minio_id    = docker_image.social_minio.image_id
  }
  provisioner "local-exec" {
    command = <<-EOT
      kind load docker-image socialmediaapp-django:latest --name social-media
      kind load docker-image socialmediaapp-frontend-prod:latest --name social-media
      kind load docker-image socialmediaapp-microservice-go:latest --name social-media
      kind load docker-image socialmediaapp-microservice-java:latest --name social-media
      kind load docker-image socialmediaapp-minio:latest --name social-media
    EOT
  }
}

# ---------------------------------------------------------------------------
# ArgoCD itself. Bootstrapping paradox: something has to install the
# installer, so this one chart stays a real Terraform helm_release. Every
# other piece of Kubernetes state in prod-social and prod-infra is GitOps
# from here on -- see gitops/social-media/apps and gitops/observability/apps.
# ---------------------------------------------------------------------------
resource "helm_release" "argocd" {
  depends_on       = [null_resource.kind_cluster]
  name             = "argocd"
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argo-cd"
  version          = "7.7.10" # TODO: confirm against `helm search repo argo-cd --versions` and pin deliberately
  namespace        = "argocd"
  create_namespace = true
  wait             = true
  timeout          = 300
  set = [
    {
      name  = "repoServer.extraArgs[0]"
      value = "--allow-oob-symlinks"
    },
    {
      # populates argocd-cmd-params-cm: server.insecure
      # the \\. escapes the literal dot so Helm doesn't try to nest it
      name  = "configs.params.server\\.insecure"
      value = "true"
    },
    {
      # populates argocd-cmd-params-cm: server.rootpath
      name  = "configs.params.server\\.rootpath"
      value = "/argocd"
    }
  ]
}

resource "kubernetes_ingress_v1" "argocd_server" {
  depends_on = [helm_release.argocd]

  metadata {
    name      = "argocd-server-ingress"
    namespace = "argocd"
    annotations = {
      "nginx.ingress.kubernetes.io/backend-protocol" = "HTTP"
      "nginx.ingress.kubernetes.io/ssl-redirect"     = "false"
    }
  }

  spec {
    ingress_class_name = "nginx"
    rule {
      http {
        path {
          path      = "/argocd"
          path_type = "Prefix"
          backend {
            service {
              name = "argocd-server"
              port {
                number = 80
              }
            }
          }
        }
      }
    }
  }
}

provider "kubernetes" {
  config_path    = "~/.kube/config"
  config_context = "kind-social-media"
}

resource "kubernetes_secret_v1" "gitops_repo_credentials" {
  depends_on = [helm_release.argocd]
  metadata {
    name      = "coding-project-repo"
    namespace = "argocd"
    labels = {
      "argocd.argoproj.io/secret-type" = "repository"
    }
  }
  data = {
    type          = "git"
    url           = "git@github.com:SaisakthiM/Coding-Project.git"
    sshPrivateKey = var.gitops_repo_ssh_key
  }
}



# ---------------------------------------------------------------------------
# Credentials that stay OUT of git. The gitops-managed manifests
# (gitops/social-media/raw/03-postgres-statefulset.yaml, 05-backend-
# deployment.yaml, 19-minio-deployment.yaml) reference these by name via
# secretKeyRef -- ArgoCD never sees the actual values.
# ---------------------------------------------------------------------------
resource "kubectl_manifest" "postgres_secret" {
  depends_on = [null_resource.kind_cluster]
  yaml_body  = <<-YAML
    apiVersion: v1
    kind: Secret
    metadata:
      name: postgres-secret
    type: Opaque
    data:
      POSTGRES_PASSWORD: ${base64encode(var.social_db_password)}
  YAML
}

resource "kubectl_manifest" "social_minio_secret" {
  depends_on = [null_resource.kind_cluster]
  yaml_body  = <<-YAML
    apiVersion: v1
    kind: Secret
    metadata:
      name: social-minio-secret
    type: Opaque
    data:
      MINIO_ROOT_USER: ${base64encode(var.social_minio_user)}
      MINIO_ROOT_PASSWORD: ${base64encode(var.social_minio_password)}
  YAML
}

# ---------------------------------------------------------------------------
# The single Terraform handle into GitOps for this environment. Everything
# under gitops/social-media/apps/ is plain YAML from here on -- adding,
# changing, or removing a workload is a git commit, not a terraform apply.
# ---------------------------------------------------------------------------
resource "kubectl_manifest" "app_of_apps_social" {
  depends_on = [helm_release.argocd, kubectl_manifest.postgres_secret, kubectl_manifest.social_minio_secret]
  yaml_body  = <<-YAML
    apiVersion: argoproj.io/v1alpha1
    kind: Application
    metadata:
      name: social-media-app-of-apps
      namespace: argocd
    spec:
      project: default
      source:
        repoURL: ${var.gitops_repo_url}
        targetRevision: HEAD
        path: Projects/Finished Projects/Docker/Terraform/infra/gitops/social-media/apps
        directory:
          recurse: false
      destination:
        server: https://kubernetes.default.svc
        namespace: argocd
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
  YAML
}

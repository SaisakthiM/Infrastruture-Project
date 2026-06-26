package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/SaisakthiM/Infrastruture-Project/cli/internal/config"
	"github.com/SaisakthiM/Infrastruture-Project/cli/internal/deploy"
	"github.com/SaisakthiM/Infrastruture-Project/cli/internal/ui"
)

var (
	deployEnv         string
	deployAutoApprove bool
	deployTarget      string
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Apply Terraform/Terragrunt infrastructure",
	Long: `Runs terragrunt apply (or run --all apply for all environments).

The deploy order when running all is enforced by Terragrunt dependency
blocks in each environment's terragrunt.hcl:
  1. prod-gateway  (foundation network + nginx)
  2. prod-docker   (Docker app containers)
  3. prod-social   (kind cluster + ArgoCD)
  4. prod-infra    (otel-gateway, n8n, jenkins, observability)
  5. prod-manage   (connects gateway container to kind network)

Use --env to deploy a single environment.
Use --target to apply only a specific resource (single env only).`,
	Example: `  social-platform deploy                                     # deploy all
  social-platform deploy --env prod-docker                   # single env
  social-platform deploy --auto-approve                      # skip prompt
  social-platform deploy --env prod-docker --target docker_container.blog_db`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().StringVar(&deployEnv, "env", "all",
		fmt.Sprintf("Environment to deploy (%s)", strings.Join(deploy.Environments(), " | ")))
	deployCmd.Flags().BoolVar(&deployAutoApprove, "auto-approve", false,
		"Skip interactive approval prompt (passes -auto-approve to terraform)")
	deployCmd.Flags().StringVar(&deployTarget, "target", "",
		"Target a specific resource address (e.g. docker_container.blog_db). Single --env required.")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	ui.Banner()

	cfg, err := config.Load()
	if err != nil || !cfg.InfraExists() {
		return fmt.Errorf("infrastructure not found — run 'social-platform install' first")
	}

	env := deploy.Environment(deployEnv)
	if !isValidEnv(env) {
		return fmt.Errorf("unknown environment %q\nValid values: %s",
			deployEnv, strings.Join(deploy.Environments(), ", "))
	}

	// --target is not compatible with run --all.
	if deployTarget != "" && env == deploy.EnvAll {
		return fmt.Errorf("--target requires a specific --env (e.g. --env prod-docker)\n" +
			"  terragrunt run --all does not support resource targeting")
	}

	if env == deploy.EnvAll {
		ui.Info("Deploying all environments (terragrunt run --all apply)")
	} else if deployTarget != "" {
		ui.Info("Deploying %s → targeting %s", env, deployTarget)
	} else {
		ui.Info("Deploying %s", env)
	}

	if !deployAutoApprove {
		ui.Warn("This will CREATE or MODIFY infrastructure resources.")
		if !ui.Confirm("Continue?") {
			ui.Info("Aborted.")
			return nil
		}
	}

	if err := deploy.Apply(cfg, env, deployAutoApprove, deployTarget); err != nil {
		return err
	}

	ui.Success("Deploy complete for %s", env)
	return nil
}

func isValidEnv(env deploy.Environment) bool {
	for _, e := range deploy.Environments() {
		if string(env) == e {
			return true
		}
	}
	return false
}

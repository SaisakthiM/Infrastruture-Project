package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/SaisakthiM/Infrastruture-Project/cli/internal/config"
	"github.com/SaisakthiM/Infrastruture-Project/cli/internal/release"
	"github.com/SaisakthiM/Infrastruture-Project/cli/internal/ui"
)

var (
	updateYes     bool
	updateDryRun  bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update infra files from the latest GitHub release without a full reinstall",
	Long: `Downloads the latest (or a chosen) release, diffs it file-by-file against
your currently installed infra directory, and replaces only the files that
actually changed.

Your terraform.tfvars, terraform.tfstate, terraform.tfstate.backup,
.terraform.lock.hcl, and .terraform/ directories are NEVER touched —
they are excluded from the diff entirely, so your secrets and state stay intact.

Use --dry-run to preview the changes without applying them.`,
	Example: `  social-platform update              # update to latest release
  social-platform update --dry-run    # preview changes only
  social-platform update --yes        # skip confirmation prompt`,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateYes, "yes", false, "Skip confirmation prompt")
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "Show what would change without applying it")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ui.Banner()

	cfg, err := config.Load()
	if err != nil || !cfg.InfraExists() {
		return fmt.Errorf("infrastructure not found — run 'social-platform install' first")
	}

	ui.Step(1, "Checking for the latest release")
	rel, err := release.LatestRelease()
	if err != nil {
		return fmt.Errorf("fetching latest release: %w", err)
	}
	ui.Info("Latest release: %s (currently installed: %s)", rel.TagName, cfg.ReleaseTag)

	if rel.TagName == cfg.ReleaseTag {
		ui.Success("Already up to date (%s)", cfg.ReleaseTag)
		return nil
	}

	ui.Step(2, "Downloading new release for comparison")
	tmpRoot, newInfraRoot, err := release.DownloadToTemp(rel)
	if err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}
	defer os.RemoveAll(tmpRoot)

	ui.Step(3, "Diffing against installed infra")
	changes, err := release.DiffInfra(newInfraRoot, cfg.InfraDir)
	if err != nil {
		return fmt.Errorf("diffing: %w", err)
	}

	if len(changes) == 0 {
		ui.Success("No file changes between %s and %s — already in sync", cfg.ReleaseTag, rel.TagName)
		// Still bump the recorded tag since content matches.
		cfg.ReleaseTag = rel.TagName
		return config.Save(cfg)
	}

	added, modified, removed := 0, 0, 0
	fmt.Println()
	ui.Bold.Printf("  %-10s  %s\n", "CHANGE", "FILE")
	ui.Dim.Printf("  %-10s  %s\n", "──────────", "────────────────────────────────────────")
	for _, c := range changes {
		switch c.Kind {
		case "added":
			added++
			ui.Cyan.Printf("  %-10s  %s\n", "+ added", c.RelPath)
		case "modified":
			modified++
			ui.Yellow.Printf("  %-10s  %s\n", "~ modified", c.RelPath)
		case "removed":
			removed++
			ui.Dim.Printf("  %-10s  %s\n", "- removed", c.RelPath)
		}
	}
	fmt.Println()
	ui.Info("%d added, %d modified, %d removed (tfvars/tfstate/.terraform untouched)", added, modified, removed)

	if updateDryRun {
		ui.Warn("Dry run — no files were changed")
		return nil
	}

	if !updateYes {
		fmt.Println()
		if !ui.Confirm(fmt.Sprintf("Apply these %d change(s) and update to %s?", len(changes), rel.TagName)) {
			ui.Info("Aborted.")
			return nil
		}
	}

	ui.Step(4, "Applying update")
	if err := release.ApplyUpdate(changes, newInfraRoot, cfg.InfraDir); err != nil {
		return fmt.Errorf("applying update: %w", err)
	}

	cfg.ReleaseTag = rel.TagName
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	ui.Success("Updated to %s (%d file(s) changed)", rel.TagName, len(changes))
	ui.Dim.Println("  Run 'social-platform deploy' to apply any infrastructure changes.")
	return nil
}

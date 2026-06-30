// Package release handles downloading and extracting infra assets from
// GitHub Releases on SaisakthiM/Infrastruture-Project.
package release

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/SaisakthiM/Infrastruture-Project/cli/internal/ui"
)

const (
	githubAPI  = "https://api.github.com"
	repoOwner  = "SaisakthiM"
	repoName   = "Infrastruture-Project"
	assetName  = "infra.tar.gz" // the release asset name
)

// GHRelease is a minimal GitHub release API response.
type GHRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []GHAsset `json:"assets"`
}

type GHAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// LatestRelease fetches the latest release metadata from the GitHub API.
func LatestRelease() (*GHRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPI, repoOwner, repoName)
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("no releases found at %s/%s — publish a release first", repoOwner, repoName)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}
	var rel GHRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("parsing release JSON: %w", err)
	}
	return &rel, nil
}

// ListReleases returns the last 10 releases so the user can choose a version.
func ListReleases() ([]GHRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=10", githubAPI, repoOwner, repoName)
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("listing releases: %w", err)
	}
	defer resp.Body.Close()
	var releases []GHRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("parsing releases JSON: %w", err)
	}
	return releases, nil
}

// DownloadAndExtract downloads the infra asset for a release and extracts it
// to destDir. Returns the path to the extracted infra/ directory.
func DownloadAndExtract(rel *GHRelease, destDir string) (string, error) {
	// Find the infra asset in this release.
	var downloadURL string
	for _, a := range rel.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		// Fallback: try the source tarball for the tag.
		downloadURL = fmt.Sprintf("https://github.com/%s/%s/archive/refs/tags/%s.tar.gz",
			repoOwner, repoName, rel.TagName)
		ui.Warn("No '%s' asset found in release %s, falling back to source tarball", assetName, rel.TagName)
	}

	ui.Info("Downloading %s (%s)...", assetName, rel.TagName)
	tmpFile, err := os.CreateTemp("", "infra-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	if err := download(downloadURL, tmpFile); err != nil {
		return "", err
	}
	tmpFile.Close()

	// Remove old infra dir.
	infraDest := filepath.Join(destDir, "infra")
	_ = os.RemoveAll(infraDest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	ui.Info("Extracting to %s...", destDir)
	if strings.HasSuffix(downloadURL, ".zip") {
		if err := extractZip(tmpFile.Name(), destDir); err != nil {
			return "", fmt.Errorf("extracting zip: %w", err)
		}
	} else {
		if err := extractTarGz(tmpFile.Name(), destDir); err != nil {
			return "", fmt.Errorf("extracting tar.gz: %w", err)
		}
	}

	// The source tarball extracts to "Infrastruture-Project-<tag>/", remap.
	entries, _ := os.ReadDir(destDir)
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "Infrastruture-Project-") {
			oldPath := filepath.Join(destDir, e.Name())
			// Move Projects/Terraform/infra → infra
			src := filepath.Join(oldPath, "Projects", "Terraform", "infra")
			if _, err := os.Stat(src); err == nil {
				_ = os.Rename(src, infraDest)
			}
			_ = os.RemoveAll(oldPath)
			break
		}
	}

	return infraDest, nil
}

func download(url string, dst *os.File) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned HTTP %d for %s", resp.StatusCode, url)
	}
	_, err = io.Copy(dst, resp.Body)
	return err
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, filepath.Clean(hdr.Name)) //nolint:gosec
		// Prevent path traversal.
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil { //nolint:gosec
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(dest, filepath.Clean(f.Name)) //nolint:gosec
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(target), 0755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		_, _ = io.Copy(out, rc) //nolint:gosec
		out.Close()
		rc.Close()
	}
	return nil
}

// ─── diff-based update ───────────────────────────────────────────────────────

// FileChange describes one file that differs between the installed infra
// directory and the newly downloaded release.
type FileChange struct {
	// RelPath is the path relative to the infra/ root, e.g. "environments/prod-docker/main.tf".
	RelPath string
	// Kind is "added", "modified", or "removed".
	Kind string
}

// preservedPaths are files/dirs that must never be touched by update, even if
// they exist in the freshly downloaded release (they shouldn't, since the
// release workflow excludes them, but this is a defense-in-depth check).
var preservedSuffixes = []string{
	"terraform.tfvars",
	"terraform.tfstate",
	"terraform.tfstate.backup",
	".terraform.lock.hcl",
}

func isPreserved(relPath string) bool {
	for _, suf := range preservedSuffixes {
		if strings.HasSuffix(relPath, suf) {
			return true
		}
	}
	if strings.Contains(relPath, string(os.PathSeparator)+".terraform"+string(os.PathSeparator)) {
		return true
	}
	if strings.HasSuffix(relPath, string(os.PathSeparator)+".terraform") {
		return true
	}
	return false
}

// DownloadToTemp downloads the infra asset for a release into a fresh
// temporary directory and returns the path to its extracted infra/ root,
// without touching destDir. Caller is responsible for cleanup.
func DownloadToTemp(rel *GHRelease) (tmpRoot string, infraRoot string, err error) {
	tmpRoot, err = os.MkdirTemp("", "social-platform-update-*")
	if err != nil {
		return "", "", err
	}

	var downloadURL string
	for _, a := range rel.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		downloadURL = fmt.Sprintf("https://github.com/%s/%s/archive/refs/tags/%s.tar.gz",
			repoOwner, repoName, rel.TagName)
		ui.Warn("No '%s' asset found in release %s, falling back to source tarball", assetName, rel.TagName)
	}

	ui.Info("Downloading %s (%s)...", assetName, rel.TagName)
	tmpFile, err := os.CreateTemp("", "infra-update-*.tar.gz")
	if err != nil {
		os.RemoveAll(tmpRoot)
		return "", "", err
	}
	defer os.Remove(tmpFile.Name())

	if err := download(downloadURL, tmpFile); err != nil {
		os.RemoveAll(tmpRoot)
		return "", "", err
	}
	tmpFile.Close()

	if strings.HasSuffix(downloadURL, ".zip") {
		if err := extractZip(tmpFile.Name(), tmpRoot); err != nil {
			os.RemoveAll(tmpRoot)
			return "", "", fmt.Errorf("extracting zip: %w", err)
		}
	} else {
		if err := extractTarGz(tmpFile.Name(), tmpRoot); err != nil {
			os.RemoveAll(tmpRoot)
			return "", "", fmt.Errorf("extracting tar.gz: %w", err)
		}
	}

	infraRoot = filepath.Join(tmpRoot, "infra")
	// Handle source-tarball fallback layout: <repo>-<tag>/Projects/Terraform/infra
	if _, statErr := os.Stat(infraRoot); statErr != nil {
		entries, _ := os.ReadDir(tmpRoot)
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), "Infrastruture-Project-") {
				src := filepath.Join(tmpRoot, e.Name(), "Projects", "Terraform", "infra")
				if _, err := os.Stat(src); err == nil {
					infraRoot = src
				}
				break
			}
		}
	}
	// Some releases tar directly from environments/ etc. without an infra/ wrapper —
	// detect by checking for an "environments" subdirectory.
	if _, statErr := os.Stat(filepath.Join(infraRoot, "environments")); statErr != nil {
		if _, altErr := os.Stat(filepath.Join(tmpRoot, "environments")); altErr == nil {
			infraRoot = tmpRoot
		}
	}

	return tmpRoot, infraRoot, nil
}

// DiffInfra compares the freshly downloaded infraRoot against the currently
// installed infraDir and returns the list of files that differ. Preserved
// files (tfvars, tfstate, .terraform/) are always excluded from the diff.
func DiffInfra(newInfraRoot, installedInfraDir string) ([]FileChange, error) {
	var changes []FileChange
	seen := map[string]bool{}

	// Walk the new release tree — catches "added" and "modified".
	err := filepath.Walk(newInfraRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(newInfraRoot, path)
		if err != nil {
			return nil
		}
		if isPreserved(rel) {
			return nil
		}
		seen[rel] = true

		oldPath := filepath.Join(installedInfraDir, rel)
		oldHash, oldErr := fileSHA256(oldPath)
		newHash, newErr := fileSHA256(path)
		if newErr != nil {
			return nil // unreadable new file, skip
		}
		if oldErr != nil {
			changes = append(changes, FileChange{RelPath: rel, Kind: "added"})
		} else if oldHash != newHash {
			changes = append(changes, FileChange{RelPath: rel, Kind: "modified"})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Walk the installed tree — catches "removed" (existed before, gone now).
	if _, statErr := os.Stat(installedInfraDir); statErr == nil {
		_ = filepath.Walk(installedInfraDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(installedInfraDir, path)
			if err != nil {
				return nil
			}
			if isPreserved(rel) || seen[rel] {
				return nil
			}
			if _, newErr := os.Stat(filepath.Join(newInfraRoot, rel)); os.IsNotExist(newErr) {
				changes = append(changes, FileChange{RelPath: rel, Kind: "removed"})
			}
			return nil
		})
	}

	return changes, nil
}

// ApplyUpdate copies only the changed files from newInfraRoot into
// installedInfraDir, and deletes files marked "removed". Preserved files are
// never touched.
func ApplyUpdate(changes []FileChange, newInfraRoot, installedInfraDir string) error {
	for _, c := range changes {
		dst := filepath.Join(installedInfraDir, c.RelPath)
		switch c.Kind {
		case "added", "modified":
			src := filepath.Join(newInfraRoot, c.RelPath)
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return fmt.Errorf("creating dir for %s: %w", c.RelPath, err)
			}
			data, err := os.ReadFile(src)
			if err != nil {
				return fmt.Errorf("reading %s: %w", c.RelPath, err)
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				return fmt.Errorf("writing %s: %w", c.RelPath, err)
			}
		case "removed":
			_ = os.Remove(dst)
		}
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

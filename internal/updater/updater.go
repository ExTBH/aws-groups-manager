package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"aws-groups-manager/internal/version"
)

const (
	GitHubOwner = "ExTBH"
	GitHubRepo  = "aws-groups-manager"
)

type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func Run(ctx context.Context) error {
	if GitHubOwner == "" {
		return fmt.Errorf("update is not configured: set repository constants")
	}

	rel, err := fetchLatestRelease(ctx, GitHubOwner, GitHubRepo)
	if err != nil {
		return err
	}

	current := strings.TrimPrefix(version.Version, "v")
	latest := strings.TrimPrefix(rel.TagName, "v")
	if current != "dev" && current == latest {
		fmt.Println("Already up to date")
		return nil
	}

	assetName := fmt.Sprintf("aws-groups-manager_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	assetURL := ""
	for _, asset := range rel.Assets {
		if asset.Name == assetName {
			assetURL = asset.URL
			break
		}
	}

	if assetURL == "" {
		return fmt.Errorf("no release asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "aws-groups-manager-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, "release.tar.gz")
	if err := downloadFile(ctx, assetURL, tarPath); err != nil {
		return err
	}

	binaryPath := filepath.Join(tmpDir, "aws-groups-manager")
	if err := extractBinary(tarPath, binaryPath); err != nil {
		return err
	}

	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return err
	}

	if err := replaceBinary(binaryPath, execPath); err != nil {
		return err
	}

	fmt.Printf("Updated v%s -> v%s\n", current, latest)
	return nil
}

func fetchLatestRelease(ctx context.Context, owner, repo string) (release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return release{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return release{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return release{}, fmt.Errorf("failed to fetch latest release: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return release{}, err
	}

	if rel.TagName == "" {
		return release{}, fmt.Errorf("latest release has empty tag name")
	}

	return rel, nil
}

func downloadFile(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func extractBinary(tarPath, outPath string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		name := filepath.Base(hdr.Name)
		if name != "aws-groups-manager" {
			continue
		}

		out, err := os.Create(outPath)
		if err != nil {
			return err
		}

		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}

		if err := out.Close(); err != nil {
			return err
		}

		return nil
	}

	return fmt.Errorf("binary aws-groups-manager not found in archive")
}

func replaceBinary(src, dst string) error {
	dir := filepath.Dir(dst)
	tmp := filepath.Join(dir, ".aws-groups-manager.new")

	if err := copyFile(src, tmp); err != nil {
		return err
	}

	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}

	if err := os.Rename(tmp, dst); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("permission denied replacing %s; re-run with appropriate permissions", dst)
		}
		return err
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}

// Package update keeps the organism current: it polls GitHub releases in the
// background, announces newer versions live over the bus (banner pops into
// every open browser — no refresh), and powers the `evolv update` / `evolv
// download` commands. Downloads are checksum-verified before they can replace
// or be installed as a running binary.
package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const repoAPI = "https://api.github.com/repos/dxn111/EVOLV/releases/latest"
const repoDownload = "https://github.com/dxn111/EVOLV/releases/latest/download"

// Info describes an available release.
type Info struct {
	Version      string `json:"version"` // e.g. "0.3.0" (no leading v)
	URL          string `json:"url"`     // release page
	Notes        string `json:"notes"`   // release body (truncated)
	Current      string `json:"current"`
	Size         int64  `json:"size,omitempty"`
	AssetURL     string `json:"asset_url,omitempty"`
	ChecksumsURL string `json:"checksums_url,omitempty"`
}

// DownloadResult is what `evolv download` hands back.
type DownloadResult struct {
	Version string `json:"version"`
	Path    string `json:"path"`
	Bytes   int64  `json:"bytes"`
}

// Progress receives a download byte count. total is 0 when unknown.
type Progress func(downloaded, total int64)

var versionRe = regexp.MustCompile(`^v?(\d+\.\d+\.\d+)$`)

// Latest queries GitHub for the newest release and reports it only when it is
// newer than current.
func Latest(ctx context.Context, current string) (*Info, error) {
	info, err := LatestRelease(ctx)
	if err != nil {
		return nil, err
	}
	info.Current = current
	if Newer(current, info.Version) {
		return info, nil
	}
	return nil, nil
}

// LatestRelease returns the newest release regardless of what is running.
// `update --check --force` and `download` need this even when versions match.
func LatestRelease(ctx context.Context) (*Info, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repoAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "evolv-organism")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api: %s", resp.Status)
	}
	var rel struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
		Assets  []struct {
			Name               string `json:"name"`
			Size               int64  `json:"size"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	m := versionRe.FindStringSubmatch(strings.TrimSpace(rel.TagName))
	if m == nil {
		return nil, fmt.Errorf("unparsable tag %q", rel.TagName)
	}
	info := &Info{
		Version:      m[1],
		URL:          rel.HTMLURL,
		Current:      "0.0.0",
		ChecksumsURL: repoDownload + "/checksums.txt",
	}
	want := AssetName()
	for _, asset := range rel.Assets {
		if asset.Name == want {
			info.Size = asset.Size
			info.AssetURL = asset.BrowserDownloadURL
			break
		}
	}
	notes := strings.TrimSpace(rel.Body)
	if len(notes) > 1600 {
		notes = notes[:1600] + "…"
	}
	info.Notes = notes
	return info, nil
}

// Newer compares dotted versions semantically: is v newer than cur?
func Newer(cur, v string) bool {
	a, b := parse(cur), parse(v)
	for i := 0; i < 3; i++ {
		if b[i] > a[i] {
			return true
		}
		if b[i] < a[i] {
			return false
		}
	}
	return false
}

func parse(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	for i, p := range parts {
		if i > 2 {
			break
		}
		n := 0
		fmt.Sscanf(p, "%d", &n)
		out[i] = n
	}
	return out
}

// AssetName is the release asset filename for this platform.
func AssetName() string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("evolv_%s_%s.exe", runtime.GOOS, runtime.GOARCH)
	}
	return fmt.Sprintf("evolv_%s_%s", runtime.GOOS, runtime.GOARCH)
}

// AssetExists is a cheap HEAD check so the updater stays quiet when a release
// ships without this platform's binary.
func AssetExists(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, repoDownload+"/"+AssetName(), nil)
	if err != nil {
		return false
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// SelfUpdate downloads the latest binary and swaps it over the running
// executable. Returns the new version.
func SelfUpdate(ctx context.Context) (string, error) {
	return SelfUpdateWithProgress(ctx, nil)
}

// SelfUpdateWithProgress is SelfUpdate with a byte-progress callback.
func SelfUpdateWithProgress(ctx context.Context, progress Progress) (string, error) {
	info, err := LatestRelease(ctx)
	if err != nil {
		return "", err
	}
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	tmp := self + ".new"
	if err := downloadVerified(ctx, info, tmp, progress); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	old := self + ".old"
	_ = os.Remove(old)
	if err := os.Rename(self, old); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, self); err != nil {
		_ = os.Rename(old, self) // roll back
		_ = os.Remove(tmp)
		return "", err
	}
	_ = os.Remove(old)
	return info.Version, nil
}

// Download fetches the latest platform binary into dir, verifies its checksum
// and returns where it landed. It never touches the running executable.
func Download(ctx context.Context, dir string, force bool, progress Progress) (DownloadResult, error) {
	info, err := LatestRelease(ctx)
	if err != nil {
		return DownloadResult{}, err
	}
	if info.AssetURL == "" {
		return DownloadResult{}, fmt.Errorf("release v%s has no %s asset", info.Version, AssetName())
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return DownloadResult{}, err
	}
	path := filepath.Join(dir, AssetName())
	if !force {
		if _, err := os.Stat(path); err == nil {
			return DownloadResult{}, fmt.Errorf("%s already exists (use --force to replace)", path)
		}
	}
	tmp := path + ".part"
	if err := downloadVerified(ctx, info, tmp, progress); err != nil {
		_ = os.Remove(tmp)
		return DownloadResult{}, err
	}
	if force {
		_ = os.Remove(path) // Windows cannot rename over an existing file
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return DownloadResult{}, err
	}
	st, _ := os.Stat(path)
	size := info.Size
	if st != nil {
		size = st.Size()
	}
	return DownloadResult{Version: info.Version, Path: path, Bytes: size}, nil
}

func downloadVerified(ctx context.Context, info *Info, dst string, progress Progress) error {
	asset := info.AssetURL
	if asset == "" {
		asset = repoDownload + "/" + AssetName()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "evolv-organism")
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", asset, resp.Status)
	}
	total := resp.ContentLength
	if total <= 0 {
		total = info.Size
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	buf := make([]byte, 32*1024)
	var done int64
	if progress != nil {
		progress(0, total)
	}
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				f.Close()
				return err
			}
			_, _ = hasher.Write(buf[:n])
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			return readErr
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	want, err := checksumFor(ctx, info)
	if err != nil {
		return err
	}
	got := fmt.Sprintf("%x", hasher.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", AssetName(), got, want)
	}
	return nil
}

func checksumFor(ctx context.Context, info *Info) (string, error) {
	url := info.ChecksumsURL
	if url == "" {
		url = repoDownload + "/checksums.txt"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "evolv-organism")
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch checksums: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch checksums: %s", resp.Status)
	}
	sums, err := parseChecksums(resp.Body)
	if err != nil {
		return "", err
	}
	sum, ok := sums[AssetName()]
	if !ok {
		return "", fmt.Errorf("checksums.txt has no entry for %s", AssetName())
	}
	return sum, nil
}

func parseChecksums(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) < 2 {
			continue
		}
		sum := strings.ToLower(fields[0])
		if len(sum) != 64 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		out[filepath.Base(name)] = sum
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("checksums.txt contained no SHA-256 entries")
	}
	return out, nil
}

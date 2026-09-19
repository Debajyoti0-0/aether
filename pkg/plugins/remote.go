package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PluginManifest is one entry in a registry index.
type PluginManifest struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Description   string `json:"description"`
	Provider      string `json:"provider"` // okta, gitlab, gcp, kubernetes, ...
	DownloadURL   string `json:"download_url"`
	SHA256        string `json:"sha256"`
	MinAppVersion string `json:"min_app_version,omitempty"`
}

// Index is the remote registry document.
type Index struct {
	Updated string           `json:"updated"`
	Plugins []PluginManifest `json:"plugins"`
}

// RemoteRegistry queries a registry index over HTTPS.
type RemoteRegistry struct {
	// URL points to a registry index JSON (hosted on GitHub raw,
	// GCS, or any HTTPS server).
	URL string
	// Dir is the local install directory.
	Dir string
	// AllowUnsigned permits installing artifacts whose manifest does
	// NOT declare a checksum. Insecure: artifact integrity is then
	// unverified. Callers must set this deliberately; the CLI surfaces
	// it as --allow-unsigned and emits an audit record when used.
	AllowUnsigned bool
	HTTP          *http.Client
}

// NewRemoteRegistry builds a registry client. dir defaults to the
// OS config dir /aether/plugins.
func NewRemoteRegistry(indexURL, dir string) *RemoteRegistry {
	if dir == "" {
		dir = filepath.Join(configDir(), "aether", "plugins")
	}
	return &RemoteRegistry{
		URL:  indexURL,
		Dir:  dir,
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}
}

// Fetch retrieves and parses the remote index.
func (r *RemoteRegistry) Fetch(ctx context.Context) (*Index, error) {
	if r.URL == "" {
		return nil, fmt.Errorf("registry URL is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registry fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry http %d", resp.StatusCode)
	}

	idx := &Index{}
	if err := json.Unmarshal(body, idx); err != nil {
		return nil, fmt.Errorf("parse registry index: %w", err)
	}
	return idx, nil
}

// Search filters the index by substring on name/provider/description.
func (r *RemoteRegistry) Search(ctx context.Context, query string) ([]PluginManifest, error) {
	idx, err := r.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	if query == "" {
		return idx.Plugins, nil
	}

	q := strings.ToLower(query)
	var out []PluginManifest
	for _, p := range idx.Plugins {
		if strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Provider), q) ||
			strings.Contains(strings.ToLower(p.Description), q) {
			out = append(out, p)
		}
	}
	return out, nil
}

// Install downloads a plugin artifact, verifies its SHA-256 against
// the manifest, and writes it into the local plugin directory.
// Returns the installed file path.
func (r *RemoteRegistry) Install(ctx context.Context, manifest PluginManifest) (string, error) {
	if manifest.DownloadURL == "" {
		return "", fmt.Errorf("manifest %s has no download_url", manifest.Name)
	}

	url := manifest.DownloadURL
	if strings.HasPrefix(url, "/") && r.URL != "" {
		// Relative to the registry origin.
		if k := strings.Index(r.URL, "//"); k >= 0 {
			if origin := strings.Index(r.URL[k+2:], "/"); origin >= 0 {
				url = r.URL[:k+2+origin] + url
			} else {
				url = r.URL + url
			}
		}
	}
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", manifest.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: http %d", manifest.Name, resp.StatusCode)
	}

	// Hash while reading — the artifact is never trusted before the
	// checksum matches the signed manifest.
	hasher := sha256.New()
	tee := io.TeeReader(io.LimitReader(resp.Body, 64<<20), hasher)
	data, err := io.ReadAll(tee)
	if err != nil {
		return "", err
	}

	got := hex.EncodeToString(hasher.Sum(nil))
	want := strings.ToLower(strings.TrimPrefix(manifest.SHA256, "sha256:"))
	if want == "" && !r.AllowUnsigned {
		// F-003 fail-closed: an undeclared checksum must not install.
		// The manifest's own integrity claim is missing; proceeding
		// would activate an artifact nobody vouched for.
		return "", fmt.Errorf("manifest %s declares no sha256 checksum (integrity unverified); use --allow-unsigned to override insecurely", manifest.Name)
	}
	if want != "" && got != want {
		return "", fmt.Errorf("checksum mismatch for %s: got %s want %s", manifest.Name, got, want)
	}

	return r.writeInstalled(manifest, data, got)
}

// writeInstalled persists the verified manifest + artifact.
func (r *RemoteRegistry) writeInstalled(manifest PluginManifest, data []byte, verifiedSHA string) (string, error) {

	name := sanitizeFileName(manifest.Name)
	if manifest.Version != "" {
		name += "-" + sanitizeFileName(manifest.Version)
	}
	// Plugin manifests don't ship binaries in this build; store the
	// verified manifest + artifact for the loader to pick up.
	out := filepath.Join(r.Dir, name+".json")

	entry := struct {
		PluginManifest
		InstalledAt string `json:"installed_at"`
		VerifiedSHA string `json:"verified_sha256"`
	}{manifest, time.Now().UTC().Format(time.RFC3339), verifiedSHA}

	meta, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(out, meta, 0o600); err != nil {
		return "", err
	}

	// If the artifact is itself a manifest (JSON), store it too.
	if strings.Contains(strings.TrimSpace(string(data)), "{") {
		artifact := filepath.Join(r.Dir, name+".artifact.json")
		if err := os.WriteFile(artifact, data, 0o600); err != nil {
			return "", err
		}
	}
	return out, nil
}

// Installed lists locally installed plugins.
func (r *RemoteRegistry) Installed() ([]PluginManifest, error) {
	entries, err := os.ReadDir(r.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []PluginManifest
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".artifact.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.Dir, e.Name()))
		if err != nil {
			continue
		}
		var entry struct {
			PluginManifest
		}
		if json.Unmarshal(data, &entry) == nil && entry.Name != "" {
			out = append(out, entry.PluginManifest)
		}
	}
	return out, nil
}

func sanitizeFileName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}

func configDir() string {
	if base := os.Getenv("AETHER_CONFIG_DIR"); base != "" {
		return base
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config")
}

// RenderSearch formats search results.
func RenderSearch(results []PluginManifest) string {
	if len(results) == 0 {
		return "No plugins matched.\n"
	}
	var b strings.Builder
	b.WriteString("NAME                  VERSION  PROVIDER     DESCRIPTION\n")
	for _, p := range results {
		fmt.Fprintf(&b, "%-21s %-8s %-12s %s\n", p.Name, p.Version, p.Provider, p.Description)
	}
	return b.String()
}

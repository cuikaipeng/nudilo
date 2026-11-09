package updates

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

const maxManifest = 1 << 20

type manifest struct {
	Version   string `json:"version"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	Signature string `json:"signature"`
}

// Source uses an isolated HTTP client: IM credentials and GitHub account tokens
// never enter update requests, and redirects cannot downgrade to plain HTTP.
type Source struct {
	repository, target string
	client             *http.Client
}

func NewSource(repository, target string, transport http.RoundTripper) *Source {
	return &Source{repository: repository, target: target, client: &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || req.URL.User != nil {
				return errors.New("update redirects require HTTPS")
			}
			if len(via) >= 10 {
				return errors.New("too many update redirects")
			}
			return nil
		},
	}}
}
func Target(goos, arch string) string {
	if goos == "darwin" {
		return "darwin-universal"
	}
	return goos + "-" + arch
}
func (s *Source) allowed(raw string, feed bool) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || path.Clean(u.Path) != u.Path {
		return false
	}
	if feed {
		return u.Path == "/"+s.repository+"/releases/latest/download/update-"+s.target+".json"
	}
	return strings.HasPrefix(u.Path, "/"+s.repository+"/releases/download/")
}
func (s *Source) get(ctx context.Context, raw string, feed bool) (io.ReadCloser, error) {
	if !s.allowed(raw, feed) {
		return nil, errors.New("update URL is outside the configured release repository")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "nudilo-updater")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		if feed && resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotPublished
		}
		return nil, fmt.Errorf("update server returned HTTP %d", resp.StatusCode)
	}
	return resp.Body, nil
}
func (s *Source) Manifest(ctx context.Context, prereleases bool) (io.ReadCloser, error) {
	if prereleases {
		return nil, errors.New("Nudilo uses stable updates only")
	}
	body, err := s.get(ctx, "https://github.com/"+s.repository+"/releases/latest/download/update-"+s.target+".json", true)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, maxManifest+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxManifest {
		return nil, errors.New("update manifest is too large")
	}
	var m manifest
	if err = json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	sig, sigErr := base64.StdEncoding.DecodeString(m.Signature)
	_, versionErr := stableVersion(m.Version)
	if versionErr != nil || sigErr != nil || len(sig) != ed25519.SignatureSize || m.Size <= 0 || m.Size > 1<<30 || !s.allowed(m.URL, false) {
		return nil, errors.New("invalid stable update manifest")
	}
	want := "https://github.com/" + s.repository + "/releases/download/v" + m.Version + "/nudilo-" + m.Version + "-" + s.target + ".tar.gz"
	if m.URL != want {
		return nil, errors.New("update archive does not match the version and platform")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (s *Source) Open(ctx context.Context, raw string) (io.ReadCloser, error) {
	return s.get(ctx, raw, false)
}

// Read-only / package-manager installations can still report newer versions;
// installation remains with their owner/package manager. Metadata alone never
// authorizes installation: MyGo verifies Ed25519 before touching app files.
func (s *Source) CheckReadOnly(ctx context.Context, current string) (*Release, error) {
	body, err := s.Manifest(ctx, false)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	var m manifest
	if err := json.NewDecoder(body).Decode(&m); err != nil {
		return nil, err
	}
	latest, err := stableVersion(m.Version)
	if err != nil {
		return nil, err
	}
	installed, err := stableVersion(current)
	if err != nil {
		return nil, err
	}
	for i := range latest {
		if latest[i] > installed[i] {
			return &Release{Version: m.Version}, nil
		}
		if latest[i] < installed[i] {
			return nil, nil
		}
	}
	return nil, nil
}
func stableVersion(s string) ([3]uint64, error) {
	var out [3]uint64
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return out, errors.New("invalid stable version")
	}
	for i, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return out, errors.New("invalid stable version")
		}
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return out, errors.New("invalid stable version")
			}
		}
		var err error
		out[i], err = strconv.ParseUint(p, 10, 64)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

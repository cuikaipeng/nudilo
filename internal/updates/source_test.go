package updates

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(r *http.Request, code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}
}
func fixtureManifest(version, target string) manifest {
	return manifest{Version: version, URL: "https://github.com/example/nudilo/releases/download/v" + version + "/nudilo-" + version + "-" + target + ".tar.gz", Size: 100, Signature: base64.StdEncoding.EncodeToString(make([]byte, 64))}
}
func TestReadonlyCheckUsesStableVersionsAndNoCredentials(t *testing.T) {
	for _, tc := range []struct {
		version string
		newer   bool
	}{{"1.10.0", true}, {"1.9.0", false}, {"1.8.9", false}} {
		data, _ := json.Marshal(fixtureManifest(tc.version, "windows-amd64"))
		s := NewSource("example/nudilo", "windows-amd64", roundTrip(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
				t.Error("update request carries credentials")
			}
			if !strings.HasSuffix(r.URL.Path, "update-windows-amd64.json") {
				t.Error(r.URL)
			}
			return response(r, 200, string(data)), nil
		}))
		up, err := s.CheckReadOnly(context.Background(), "1.9.0")
		if err != nil || (up != nil) != tc.newer {
			t.Fatal(up, err)
		}
		if up != nil && up.Install != nil {
			t.Fatal("readonly release can install")
		}
	}
}
func TestSourceRejectsDowngradeRedirectsAndForeignArtifacts(t *testing.T) {
	requests := 0
	s := NewSource("example/nudilo", "darwin-universal", roundTrip(func(r *http.Request) (*http.Response, error) {
		requests++
		res := response(r, 302, "")
		res.Header.Set("Location", "http://github.com/unsafe")
		return res, nil
	}))
	if _, err := s.Manifest(context.Background(), false); err == nil {
		t.Fatal("accepted HTTP redirect")
	}
	if requests != 1 {
		t.Fatal("followed unsafe redirect")
	}
	for _, url := range []string{"http://github.com/example/nudilo/releases/download/a", "https://github.com/other/repo/releases/download/a", "https://github.com/example/nudilo/releases/download/../a", "https://github.com:443/example/nudilo/releases/download/a"} {
		if _, err := s.Open(context.Background(), url); err == nil {
			t.Fatal("accepted", url)
		}
	}
}
func TestManifestRejectsWrongPlatformInvalidSignatureAndPrerelease(t *testing.T) {
	for _, mutate := range []func(*manifest){
		func(m *manifest) { m.Version = "2.0.0-beta.1" }, func(m *manifest) { m.Signature = "invalid" },
		func(m *manifest) { m.Size = -1 }, func(m *manifest) { m.URL = strings.ReplaceAll(m.URL, "linux-amd64", "windows-amd64") },
		func(m *manifest) { m.URL = strings.ReplaceAll(m.URL, "example/nudilo", "other/repo") },
	} {
		m := fixtureManifest("2.0.0", "linux-amd64")
		mutate(&m)
		data, _ := json.Marshal(m)
		s := NewSource("example/nudilo", "linux-amd64", roundTrip(func(r *http.Request) (*http.Response, error) { return response(r, 200, string(data)), nil }))
		if _, err := s.Manifest(context.Background(), false); err == nil {
			t.Fatal("accepted invalid manifest", m)
		}
	}
}
func TestMissingReleaseAndOversizedManifest(t *testing.T) {
	s := NewSource("example/nudilo", "linux-arm64", roundTrip(func(r *http.Request) (*http.Response, error) { return response(r, 404, ""), nil }))
	if _, err := s.Manifest(context.Background(), false); !errors.Is(err, ErrNotPublished) {
		t.Fatal(err)
	}
	s.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		return response(r, 200, strings.Repeat(" ", maxManifest+1)), nil
	})
	if _, err := s.Manifest(context.Background(), false); err == nil {
		t.Fatal("accepted oversized manifest")
	}
}

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func signingFixture(t *testing.T) (config, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var cfg config
	cfg.Version = "1.2.0"
	cfg.Updates.GitHub = "example/nudilo"
	cfg.Updates.PublicKey = base64.StdEncoding.EncodeToString(pub)
	return cfg, priv
}
func TestPrivateKeyMustBeValidAndMatchPublicKey(t *testing.T) {
	cfg, key := signingFixture(t)
	if err := checkKey(cfg, base64.StdEncoding.EncodeToString(key)); err != nil {
		t.Fatal(err)
	}
	key[0] ^= 1 // Keep the public suffix, but change the seed.
	if checkKey(cfg, base64.StdEncoding.EncodeToString(key)) == nil {
		t.Fatal("accepted a malformed seed/public key pair")
	}
	_, other := signingFixture(t)
	if checkKey(cfg, base64.StdEncoding.EncodeToString(other)) == nil {
		t.Fatal("accepted another project's key")
	}
}
func TestSignedArchivesRejectTamperingWrongSizeAndWrongFeed(t *testing.T) {
	cfg, key := signingFixture(t)
	dir := t.TempDir()
	payload := []byte("signed release archive fixture")
	name := "nudilo-1.2.0-linux-amd64.tar.gz"
	hash := sha256.Sum256(payload)
	m := manifest{Version: cfg.Version, URL: "https://github.com/example/nudilo/releases/download/v1.2.0/" + name, Size: int64(len(payload)), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, hash[:]))}
	write := func(value manifest) {
		b, _ := json.Marshal(value)
		if err := os.WriteFile(filepath.Join(dir, "update-linux-amd64.json"), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, name), payload, 0600); err != nil {
		t.Fatal(err)
	}
	write(m)
	if err := verify(cfg, dir, "linux/amd64"); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*manifest){func(m *manifest) { m.Size++ }, func(m *manifest) { m.Version = "9.0.0" }, func(m *manifest) { m.URL = "https://example.com/update.tar.gz" }, func(m *manifest) { m.Signature = base64.StdEncoding.EncodeToString(make([]byte, 64)) }} {
		bad := m
		mutate(&bad)
		write(bad)
		if verify(cfg, dir, "linux/amd64") == nil {
			t.Fatal("accepted invalid metadata")
		}
	}
	write(m)
	payload[0] ^= 1
	if err := os.WriteFile(filepath.Join(dir, name), payload, 0600); err != nil {
		t.Fatal(err)
	}
	if verify(cfg, dir, "linux/amd64") == nil {
		t.Fatal("accepted a tampered archive")
	}
}

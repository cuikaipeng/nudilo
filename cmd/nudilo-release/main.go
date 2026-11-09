// Validates release signatures without any third-party crypto tooling.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	Version string `json:"version"`
	Updates struct {
		PublicKey string `json:"publicKey"`
		GitHub    string `json:"github"`
	} `json:"updates"`
}
type manifest struct {
	Version   string `json:"version"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	Signature string `json:"signature"`
}

func publicKey(cfg config) (ed25519.PublicKey, error) {
	key, err := base64.StdEncoding.DecodeString(cfg.Updates.PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, errors.New("invalid update public key")
	}
	return ed25519.PublicKey(key), nil
}
func checkKey(cfg config, value string) error {
	pub, err := publicKey(cfg)
	if err != nil {
		return err
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return errors.New("invalid update signing key")
	}
	derived := ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])
	if !derived.Equal(ed25519.PrivateKey(key)) || !derived.Public().(ed25519.PublicKey).Equal(pub) {
		return errors.New("update signing key does not match the configured public key")
	}
	return nil
}
func verify(cfg config, directory, target string) error {
	if target != "darwin/universal" && target != "windows/amd64" && target != "linux/amd64" && target != "linux/arm64" {
		return errors.New("unsupported update platform")
	}
	pub, err := publicKey(cfg)
	if err != nil {
		return err
	}
	suffix := strings.ReplaceAll(target, "/", "-")
	name := "nudilo-" + cfg.Version + "-" + suffix + ".tar.gz"
	data, err := os.ReadFile(filepath.Join(directory, "update-"+suffix+".json"))
	if err != nil {
		return err
	}
	var m manifest
	if err = json.Unmarshal(data, &m); err != nil {
		return err
	}
	url := "https://github.com/" + cfg.Updates.GitHub + "/releases/download/v" + cfg.Version + "/" + name
	if m.Version != cfg.Version || m.URL != url {
		return errors.New("update manifest version or URL does not match this release")
	}
	file, err := os.Open(filepath.Join(directory, name))
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || m.Size != size || size <= 0 || size > 1<<30 || !ed25519.Verify(pub, hash.Sum(nil), sig) {
		return fmt.Errorf("invalid update signature or size: %s", name)
	}
	return nil
}
func main() {
	file := flag.String("config", "mygo.json", "release configuration")
	key := flag.Bool("check-key", false, "check the signing key from the environment")
	dir := flag.String("directory", "", "directory of signed update archives")
	targets := flag.String("platform", "darwin/universal,windows/amd64,linux/amd64,linux/arm64", "platforms to verify")
	flag.Parse()
	var cfg config
	data, err := os.ReadFile(*file)
	if err == nil {
		err = json.Unmarshal(data, &cfg)
	}
	if err == nil {
		if *key {
			err = checkKey(cfg, os.Getenv("MYGO_UPDATER_PRIVATE_KEY"))
		} else if *dir != "" {
			for _, target := range strings.Split(*targets, ",") {
				if err = verify(cfg, *dir, target); err != nil {
					break
				}
			}
		} else {
			err = errors.New("choose -check-key or -directory")
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Update signatures verified.")
}

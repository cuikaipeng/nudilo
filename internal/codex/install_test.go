package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"callbell/internal/storage"
)

func TestMergePreservesForeignHooksAndIsIdempotent(t *testing.T) {
	before := []byte(`{ // user's comment
	 "custom":true,"hooks":{"Stop":[{"matcher":"x","hooks":[{"type":"command","command":"echo existing"}]}]}}`)
	after, err := mergeHooks(before, "callbell helper", false)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]json.RawMessage
	_ = json.Unmarshal(after, &root)
	if string(root["custom"]) != "true" {
		t.Fatal("unknown property lost")
	}
	var groups map[string][]map[string]json.RawMessage
	_ = json.Unmarshal(root["hooks"], &groups)
	if len(groups["Stop"]) != 2 || !bytes.Contains(groups["Stop"][0]["hooks"], []byte("echo existing")) {
		t.Fatal("foreign indices changed")
	}
	again, _ := mergeHooks(after, "callbell helper", false)
	if !jsonEqual(after, again) {
		t.Fatal("install not idempotent")
	}
	removed, err := mergeHooks(after, "callbell helper", true)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(removed, []byte("callbell helper")) || !bytes.Contains(removed, []byte("echo existing")) {
		t.Fatal("disconnect removed foreign hook or retained ours")
	}
	if _, err = mergeHooks([]byte(`{"hooks":"bad"}`), "x", false); err == nil {
		t.Fatal("invalid config accepted")
	}
}
func TestCommandQuotesPaths(t *testing.T) {
	c := hookCommand("/Users/a b/o'k/helper", "/data/$(touch x)", "darwin")
	if !strings.Contains(c, `'"'"'`) || !strings.Contains(c, "'/data/$(touch x)'") {
		t.Fatal(c)
	}
	c = hookCommand(`C:\用户\a b\helper.exe`, `C:\Users\O'Neil\Data`, "windows")
	if !strings.HasPrefix(c, "powershell.exe -NoLogo -NoProfile -NonInteractive -EncodedCommand ") {
		t.Fatal(c)
	}
}

func TestRealCodexHookRegistration(t *testing.T) {
	binary := os.Getenv("CALLBELL_CODEX_TEST_BINARY")
	if binary == "" {
		t.Skip("set CALLBELL_CODEX_TEST_BINARY to test an installed Codex without changing its real home")
	}
	home := t.TempDir()
	root := t.TempDir()
	helper := filepath.Join(t.TempDir(), "helper")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf '{}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	before := []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo foreign"}]}]}}`)
	_ = os.WriteFile(filepath.Join(home, "hooks.json"), before, 0600)
	i := Installer{Paths: storage.Paths{Root: root}, HelperSource: helper}
	c, err := i.Connect(context.Background(), home, binary)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Ready {
		t.Fatal("connection not ready")
	}
	configPath, hooksPath := filepath.Join(home, "config.toml"), filepath.Join(home, "hooks.json")
	configBeforeCheck, _ := os.ReadFile(configPath)
	hooksBeforeCheck, _ := os.ReadFile(hooksPath)
	checked, err := i.Check(context.Background(), home, binary)
	if err != nil || !checked.Ready {
		t.Fatal("fresh status check failed", checked, err)
	}
	configAfterCheck, _ := os.ReadFile(configPath)
	hooksAfterCheck, _ := os.ReadFile(hooksPath)
	if !bytes.Equal(configBeforeCheck, configAfterCheck) || !bytes.Equal(hooksBeforeCheck, hooksAfterCheck) {
		t.Fatal("status check changed Codex configuration")
	}
	r, err := startRPC(context.Background(), binary, home)
	if err != nil {
		t.Fatal(err)
	}
	defer r.close()
	var list hookList
	if err = r.call("hooks/list", map[string]any{"cwds": []string{home}}, &list); err != nil {
		t.Fatal(err)
	}
	for _, entry := range list.Data {
		for _, h := range entry.Hooks {
			if h.Command == "echo foreign" && h.TrustStatus == "trusted" {
				t.Fatal("foreign hook was trusted")
			}
		}
	}
	configBefore, _ := os.ReadFile(filepath.Join(home, "config.toml"))
	if _, err = i.Connect(context.Background(), home, binary); err != nil {
		t.Fatal(err)
	}
	configAfter, _ := os.ReadFile(filepath.Join(home, "config.toml"))
	if !bytes.Equal(configBefore, configAfter) {
		t.Fatal("idempotent connect rewrote config")
	}
	if err = i.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if checked, err := i.Check(context.Background(), home, binary); err == nil || checked.Ready {
		t.Fatal("status check missed removed hooks")
	}
	if err = r.call("hooks/list", map[string]any{"cwds": []string{home}}, &list); err != nil {
		t.Fatal(err)
	}
	for _, entry := range list.Data {
		if len(entry.Errors) > 0 {
			t.Fatalf("disconnect left invalid hooks: %+v", entry.Errors)
		}
	}
}

func TestRealCodexDisabledHooksArePreserved(t *testing.T) {
	binary := os.Getenv("CALLBELL_CODEX_TEST_BINARY")
	if binary == "" {
		t.Skip("set CALLBELL_CODEX_TEST_BINARY for an isolated config test")
	}
	home := t.TempDir()
	before := []byte("[features]\nhooks = false\n")
	config := filepath.Join(home, "config.toml")
	if err := os.WriteFile(config, before, 0600); err != nil {
		t.Fatal(err)
	}
	i := Installer{Paths: storage.Paths{Root: t.TempDir()}}
	if _, err := i.Connect(context.Background(), home, binary); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled Hooks were not rejected: %v", err)
	}
	after, _ := os.ReadFile(config)
	if !bytes.Equal(before, after) {
		t.Fatal("disabled feature overwritten")
	}
	if _, err := os.Stat(filepath.Join(home, "hooks.json")); !os.IsNotExist(err) {
		t.Fatal("listeners installed while Hooks disabled")
	}
}

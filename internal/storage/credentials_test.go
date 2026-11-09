package storage

import (
	"bytes"
	"errors"
	"os"
	"runtime"
	"sync"
	"testing"

	"callbell/internal/model"
)

func TestLocalCredentialsPersistAllModesWithoutPlaintext(t *testing.T) {
	paths := Paths{Root: t.TempDir()}
	vault := NewLocalVault(paths)
	for _, ch := range model.Channels {
		c := model.Credentials{Mode: model.Direct, AppID: "app-fixture", AppSecret: "secret-app-fixture", Token: "secret-bot-fixture", RecipientID: "ou_fixture", Webhook: "https://private-group-fixture.invalid", Secret: "secret-group-fixture", ChatID: "-123"}
		c.Normalize(ch)
		if err := vault.Set(ch, c); err != nil {
			t.Fatal(err)
		}
		reopened := NewLocalVault(paths)
		got, err := reopened.Get(ch)
		if err != nil || got != c {
			t.Fatal("saved modes or recipient did not survive reopening", ch, err)
		}
	}
	data, _ := os.ReadFile(paths.Credentials())
	for _, value := range []string{"secret-app-fixture", "secret-bot-fixture", "ou_fixture", "private-group-fixture", "secret-group-fixture"} {
		if bytes.Contains(data, []byte(value)) {
			t.Fatal("plaintext credential in vault")
		}
	}
	for _, path := range []string{paths.Credentials(), paths.CredentialsKey()} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Fatal("credential file is not private")
		}
	}
	if err := vault.Delete(model.Feishu); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalVault(paths).Get(model.Feishu); !errors.Is(err, ErrCredentialsMissing) {
		t.Fatal("deleted credential survived restart", err)
	}
	if _, err := vault.Get(model.Slack); err != nil {
		t.Fatal("removing one channel affected another", err)
	}
	if err := vault.Delete(model.Feishu); err != nil {
		t.Fatal("delete must be idempotent", err)
	}
}

func TestDamagedLocalCredentialsAreNotOverwritten(t *testing.T) {
	for _, damage := range []string{"vault", "key", "missing-key"} {
		t.Run(damage, func(t *testing.T) {
			paths := Paths{Root: t.TempDir()}
			vault := NewLocalVault(paths)
			if err := vault.Set(model.Feishu, model.Credentials{AppSecret: "original"}); err != nil {
				t.Fatal(err)
			}
			switch damage {
			case "vault":
				_ = os.WriteFile(paths.Credentials(), []byte(`{"version":99}`), 0600)
			case "key":
				_ = os.WriteFile(paths.CredentialsKey(), bytes.Repeat([]byte{1}, 32), 0600)
			case "missing-key":
				_ = os.Remove(paths.CredentialsKey())
			}
			before, _ := os.ReadFile(paths.Credentials())
			keyBefore, _ := os.ReadFile(paths.CredentialsKey())
			if _, err := vault.Get(model.Feishu); err == nil || errors.Is(err, ErrCredentialsMissing) {
				t.Fatal("corruption treated as missing setup", err)
			}
			if err := vault.Set(model.Feishu, model.Credentials{AppSecret: "replacement"}); err == nil {
				t.Fatal("overwrote damaged vault")
			}
			if err := vault.Delete(model.Feishu); err == nil {
				t.Fatal("deleted damaged vault")
			}
			after, _ := os.ReadFile(paths.Credentials())
			keyAfter, _ := os.ReadFile(paths.CredentialsKey())
			if !bytes.Equal(before, after) || !bytes.Equal(keyBefore, keyAfter) {
				t.Fatal("original files changed")
			}
		})
	}
}

func TestConcurrentLocalCredentialUpdates(t *testing.T) {
	vault := NewLocalVault(Paths{Root: t.TempDir()})
	var wg sync.WaitGroup
	for _, ch := range model.Channels {
		wg.Go(func() {
			for n := 0; n < 5; n++ {
				if err := vault.Set(ch, model.Credentials{AppSecret: string(ch)}); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Wait()
	for _, ch := range model.Channels {
		got, err := vault.Get(ch)
		if err != nil || got.AppSecret != string(ch) {
			t.Fatal("concurrent update lost a channel", ch, err)
		}
	}
}

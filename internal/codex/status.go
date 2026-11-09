package codex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Check verifies the installed helper, policy and trusted hooks. It only reads
// configuration: refreshing status must not reinstall hooks or grant trust.
func (i Installer) Check(ctx context.Context, home, binary string) (result Connection, err error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	result.Home, err = Home(home)
	if err != nil {
		return result, err
	}
	result.Binary, result.Version, err = Detect(ctx, binary)
	if err != nil {
		return result, err
	}
	helper := filepath.Join(i.Paths.Root, "bin", "callbell-hook")
	if runtime.GOOS == "windows" {
		helper += ".exe"
	}
	info, statErr := os.Stat(helper)
	if statErr != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0) {
		return result, errors.New("The Codex event helper is unavailable. Reconnect to restore it.")
	}
	r, err := startRPC(ctx, result.Binary, result.Home)
	if err != nil {
		return result, err
	}
	defer r.close()
	if err = hooksAllowed(r); err != nil {
		return result, err
	}
	var list hookList
	if err = r.call("hooks/list", map[string]any{"cwds": []string{result.Home}}, &list); err != nil {
		return result, err
	}
	path := resolveExisting(filepath.Join(result.Home, "hooks.json"))
	if err = checkOwnedHooks(list, path, hookCommand(helper, i.Paths.Root, runtime.GOOS)); err != nil {
		return result, err
	}
	result.Ready = true
	return result, nil
}

func checkOwnedHooks(list hookList, path, command string) error {
	owned := ownedHooks(list, path, command)
	events := map[string]bool{}
	for _, h := range owned {
		if !h.Enabled {
			return errors.New("Nudilo events are disabled. Enable them in Codex settings.")
		}
		if h.TrustStatus != "trusted" {
			return errors.New("Codex no longer trusts the event helper. Reconnect to restore the connection.")
		}
		// The RPC uses lowerCamelCase; hooks.json uses PascalCase.
		events[strings.ToLower(h.EventName)] = true
	}
	if !events["stop"] || !events["pretooluse"] || !events["permissionrequest"] {
		return errors.New("Some Codex notification events are missing. Reconnect to restore them.")
	}
	return nil
}

package claude

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"callbell/internal/locale"
)

// StopFailure was added in 2.1.78 (Elicitation in 2.1.76). Check before
// installing event names that an older settings schema would reject.
var versionPattern = regexp.MustCompile(`\b(\d+)\.(\d+)\.(\d+)\b`)

func supportedVersion(value string) bool {
	if runtime.GOOS == "windows" {
		// Native Windows shell hook failures otherwise may block a tool call.
		return versionAtLeast(value, 2, 1, 295)
	}
	return versionAtLeast(value, 2, 1, 78)
}

func versionAtLeast(value string, minimumMajor, minimumMinor, minimumPatch int) bool {
	m := versionPattern.FindStringSubmatch(value)
	if len(m) != 4 || !strings.Contains(strings.ToLower(value), "claude") {
		return false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return major > minimumMajor || major == minimumMajor && (minor > minimumMinor || minor == minimumMinor && patch >= minimumPatch)
}

func Detect(ctx context.Context, override string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	candidates := []string{}
	if override != "" {
		candidates = append(candidates, override)
	} else {
		user, _ := os.UserHomeDir()
		name := "claude"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if p, err := exec.LookPath("claude"); err == nil {
			candidates = append(candidates, p)
		}
		candidates = append(candidates, filepath.Join(user, ".local", "bin", name), filepath.Join(user, ".claude", "local", name), "/opt/homebrew/bin/claude", "/usr/local/bin/claude")
		for _, pattern := range []string{
			filepath.Join(user, ".vscode", "extensions", "anthropic.claude-code-*", "resources", "native-binary", name),
			filepath.Join(user, ".vscode-insiders", "extensions", "anthropic.claude-code-*", "resources", "native-binary", name),
			filepath.Join(user, ".cursor", "extensions", "anthropic.claude-code-*", "resources", "native-binary", name),
			filepath.Join(user, ".nvm", "versions", "node", "*", "bin", "claude"),
		} {
			matches, _ := filepath.Glob(pattern)
			sort.Sort(sort.Reverse(sort.StringSlice(matches)))
			if len(matches) > 4 {
				matches = matches[:4]
			}
			candidates = append(candidates, matches...)
		}
		if runtime.GOOS == "windows" {
			candidates = append(candidates, filepath.Join(os.Getenv("APPDATA"), "npm", "claude.cmd"))
		}
	}
	seen := map[string]bool{}
	var outdated string
	for _, p := range candidates {
		if ctx.Err() != nil {
			break
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		value, err := readVersion(ctx, p)
		if err != nil {
			continue
		}
		if supportedVersion(value) {
			return p, value, nil
		}
		if versionPattern.MatchString(value) && strings.Contains(strings.ToLower(value), "claude") {
			outdated = value
		}
	}
	if outdated != "" {
		if runtime.GOOS == "windows" {
			return "", "", locale.M("Update Claude Code to version 2.1.295 or later on Windows before connecting.")
		}
		return "", "", locale.M("Update Claude Code to version 2.1.78 or later before connecting.")
	}
	return "", "", locale.M("Claude Code could not run. Install it or select its executable in Advanced settings.")
}

func readVersion(ctx context.Context, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	if runtime.GOOS == "windows" && strings.EqualFold(filepath.Ext(path), ".cmd") {
		// Windows npm installs expose a .cmd wrapper. Invoke it without cmd.exe
		// interpolation, so spaces and shell characters remain part of the path.
		quoted := "'" + strings.ReplaceAll(path, "'", "''") + "'"
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "& "+quoted+" --version")
	}
	quietProcess(cmd)
	cmd.WaitDelay = 500 * time.Millisecond
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(path)+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	if len(out) > 1024 {
		return "", fmt.Errorf("invalid version output")
	}
	return strings.TrimSpace(string(out)), nil
}

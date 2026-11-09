package codex

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func Home(override string) (string, error) {
	home := strings.TrimSpace(override)
	if home == "" {
		home = os.Getenv("CODEX_HOME")
	}
	if home == "" {
		user, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = filepath.Join(user, ".codex")
	}
	home, err := filepath.Abs(home)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(home)
	if err != nil || !info.IsDir() {
		return "", errors.New("No local Codex configuration found. Install and open Codex first.")
	}
	return filepath.EvalSymlinks(home)
}

func Detect(ctx context.Context, override string) (string, string, error) {
	if override != "" {
		v, err := version(ctx, override)
		return override, v, err
	}
	user, _ := os.UserHomeDir()
	candidates := []string{}
	switch runtime.GOOS {
	case "darwin":
		for _, base := range []string{"/Applications", filepath.Join(user, "Applications")} {
			for _, name := range []string{"Codex.app", "ChatGPT.app"} {
				candidates = append(candidates, filepath.Join(base, name, "Contents", "Resources", "codex-cli", "bin", "codex"), filepath.Join(base, name, "Contents", "Resources", "codex"))
			}
		}
	case "windows":
		for _, base := range []string{os.Getenv("LOCALAPPDATA"), os.Getenv("ProgramFiles")} {
			for _, name := range []string{"Codex", "ChatGPT", "Programs/Codex", "Programs/ChatGPT"} {
				candidates = append(candidates, filepath.Join(base, name, "resources", "codex-cli", "bin", "codex.exe"), filepath.Join(base, name, "resources", "codex.exe"))
			}
		}
		probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		probe := exec.CommandContext(probeCtx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `Get-AppxPackage | Where-Object { $_.Name -match 'OpenAI|Codex' } | Select-Object -ExpandProperty InstallLocation`)
		quietProcess(probe)
		if output, err := probe.Output(); err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				base := strings.TrimSpace(line)
				if base == "" {
					continue
				}
				for _, rel := range []string{"app/resources/codex.exe", "app/resources/codex-cli/bin/codex.exe", "resources/codex.exe", "resources/codex-cli/bin/codex.exe"} {
					candidates = append(candidates, filepath.Join(base, rel))
				}
			}
		}
		cancel()
	}
	if p, err := exec.LookPath("codex"); err == nil {
		candidates = append(candidates, p)
	}
	candidates = append(candidates, filepath.Join(user, ".local", "bin", "codex"), "/opt/homebrew/bin/codex", "/usr/local/bin/codex", filepath.Join(user, ".cargo", "bin", "codex"), filepath.Join(os.Getenv("APPDATA"), "npm", "node_modules", "@openai", "codex", "vendor", "x86_64-pc-windows-msvc", "codex", "codex.exe"))
	// npm's platform packages contain the actual binary. No Node runtime is
	// needed by Nudilo when one of these paths is available.
	for _, base := range []string{filepath.Join(os.Getenv("APPDATA"), "npm", "node_modules"), filepath.Join(user, ".npm-global", "lib", "node_modules"), "/usr/local/lib/node_modules", "/opt/homebrew/lib/node_modules", filepath.Join(user, ".local", "lib", "node_modules")} {
		pattern := filepath.Join(base, "@openai", "codex*", "vendor", "*", "codex", "codex*")
		matches, _ := filepath.Glob(pattern)
		candidates = append(candidates, matches...)
	}
	for _, p := range candidates {
		if v, err := version(ctx, p); err == nil {
			return p, v, nil
		}
	}
	return "", "", errors.New("Codex was not found. Install it or select its executable in Advanced settings.")
}

func version(ctx context.Context, path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", errors.New("The Codex executable does not exist.")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	quietProcess(cmd)
	b, err := cmd.Output()
	if err != nil || !strings.Contains(strings.ToLower(string(b)), "codex") {
		return "", errors.New("The selected Codex executable could not run.")
	}
	return strings.TrimSpace(string(b)), nil
}

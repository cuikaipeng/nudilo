package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"callbell/internal/claude"
	"callbell/internal/codex"
	"callbell/internal/storage"
)

func main() {
	// Notifications never return an approval, denial, answer or model context.
	fs := flag.NewFlagSet("callbell-hook", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	enabled := fs.Bool("codex-event", false, "")
	claudeEnabled := fs.Bool("claude-event", false, "")
	dataDir := fs.String("data-dir", "", "")
	if fs.Parse(os.Args[1:]) != nil || *enabled == *claudeEnabled {
		return
	}
	if *enabled {
		defer fmt.Println("{}")
	}
	paths, err := storage.DefaultPaths()
	if err != nil {
		return
	}
	if *dataDir != "" {
		paths.Root = *dataDir
	}
	b, err := io.ReadAll(io.LimitReader(os.Stdin, codex.MaxPayload+1))
	if err != nil {
		return
	}
	pref, err := paths.Preferences()
	if err != nil {
		return
	}
	if *claudeEnabled {
		if !pref.ClaudeAutoConnect || paths.Prepare() != nil {
			return
		}
		_ = claude.Receive(paths, b, pref.IncludeSummary, time.Now())
		return
	}
	if !pref.AutoConnect {
		return
	}
	event, err := codex.Normalize(b, pref.IncludeSummary, time.Now())
	if err != nil || event == nil {
		return
	}
	if paths.Prepare() != nil {
		return
	}
	_ = paths.QueueEvent(event)
}

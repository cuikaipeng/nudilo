package codex

import (
	"encoding/json"
	"testing"
)

func TestStatusRequiresEveryOwnedTrustedEvent(t *testing.T) {
	fixtures := []struct {
		name  string
		edit  func([]hookMeta) []hookMeta
		ready bool
	}{
		{"ready", func(h []hookMeta) []hookMeta { return h }, true},
		{"missing", func(h []hookMeta) []hookMeta { return h[:2] }, false},
		{"duplicate-not-complete", func(h []hookMeta) []hookMeta { h[2] = h[0]; return h }, false},
		{"disabled", func(h []hookMeta) []hookMeta { h[0].Enabled = false; return h }, false},
		{"untrusted", func(h []hookMeta) []hookMeta { h[1].TrustStatus = "untrusted"; return h }, false},
		{"foreign", func(h []hookMeta) []hookMeta { h[0].Command = "someone-else"; return h }, false},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			hooks := []hookMeta{}
			for _, event := range []string{"stop", "permissionRequest", "preToolUse"} {
				hooks = append(hooks, hookMeta{EventName: event, Enabled: true, TrustStatus: "trusted", SourcePath: "/codex/hooks.json", Command: "helper"})
			}
			data, _ := json.Marshal(map[string]any{"data": []map[string]any{{"hooks": f.edit(hooks)}}})
			var list hookList
			if err := json.Unmarshal(data, &list); err != nil {
				t.Fatal(err)
			}
			if err := checkOwnedHooks(list, "/codex/hooks.json", "helper"); (err == nil) != f.ready {
				t.Fatal("incorrect connection status", err)
			}
		})
	}
}

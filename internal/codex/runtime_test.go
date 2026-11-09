package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"callbell/internal/engine"
	"callbell/internal/model"
	"callbell/internal/storage"
)

type runtimeVault struct{}

func (runtimeVault) Get(model.Channel) (model.Credentials, error) { return model.Credentials{}, nil }
func (runtimeVault) Set(model.Channel, model.Credentials) error   { return nil }
func (runtimeVault) Delete(model.Channel) error                   { return nil }

type runtimeSender struct{ events []model.Event }

func (s *runtimeSender) Send(_ context.Context, _ model.Channel, _ model.Credentials, e model.Event) error {
	s.events = append(s.events, e)
	return nil
}

// Uses an actual Codex runtime, but only a loopback model fixture and a fake IM
// sender. No API keys, user chats, real CODEX_HOME or external messages are used.
func TestRealCodexExistingAndNewChatCompletion(t *testing.T) {
	binary := os.Getenv("CALLBELL_CODEX_TEST_BINARY")
	if binary == "" {
		t.Skip("set CALLBELL_CODEX_TEST_BINARY for the isolated runtime integration test")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		message := map[string]any{"id": "msg_fixture", "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": "fixture done", "annotations": []any{}}}}
		for _, event := range []any{
			map[string]any{"type": "response.created", "response": map[string]any{"id": "resp_fixture", "status": "in_progress", "output": []any{}}},
			map[string]any{"type": "response.output_item.done", "output_index": 0, "item": message},
			map[string]any{"type": "response.completed", "response": map[string]any{"id": "resp_fixture", "status": "completed", "output": []any{message}, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1, "total_tokens": 2}}},
		} {
			b, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()
	home := t.TempDir()
	config := fmt.Sprintf("model = \"fixture\"\nmodel_provider = \"fixture\"\n[model_providers.fixture]\nname = \"fixture\"\nbase_url = %q\nwire_api = \"responses\"\nrequires_openai_auth = false\n[features]\napps = false\nplugins = false\n", server.URL+"/v1")
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	r, err := startRPC(ctx, binary, home)
	if err != nil {
		t.Fatal(err)
	}
	defer r.close()
	start := func() string {
		t.Helper()
		var result struct{ Thread struct{ ID string } }
		if err := r.call("thread/start", map[string]any{"cwd": home, "approvalPolicy": "never", "sandbox": "read-only", "model": "fixture", "modelProvider": "fixture"}, &result); err != nil {
			t.Fatal(err)
		}
		return result.Thread.ID
	}
	turn := func(thread string) string {
		t.Helper()
		var started struct{ Turn struct{ ID string } }
		if err := r.call("turn/start", map[string]any{"threadId": thread, "input": []any{map[string]string{"type": "text", "text": "fixture"}}}, &started); err != nil {
			t.Fatal(err)
		}
		for {
			select {
			case <-ctx.Done():
				t.Fatal("runtime turn timed out")
				return ""
			case line, ok := <-r.lines:
				if !ok {
					t.Fatal("runtime closed")
					return ""
				}
				var event struct {
					Method string
					Params struct {
						ThreadID string
						Turn     struct{ ID, Status string }
					}
				}
				if json.Unmarshal(line, &event) == nil && event.Method == "turn/completed" && event.Params.Turn.ID == started.Turn.ID {
					if event.Params.Turn.Status != "completed" {
						t.Fatal("fixture did not complete", event.Params.Turn.Status)
					}
					return started.Turn.ID
				}
			}
		}
	}
	// This chat caches its configuration before Nudilo installs the helper.
	old := start()
	helper := filepath.Join(t.TempDir(), "callbell-hook")
	if runtime.GOOS == "windows" {
		helper += ".exe"
	}
	build := exec.CommandContext(ctx, "go", "build", "-o", helper, "../../cmd/callbell-hook")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("helper build: %v %s", err, output)
	}
	paths := storage.Paths{Root: t.TempDir()}
	installer := Installer{Paths: paths, HelperSource: helper}
	if _, err := installer.Connect(ctx, home, binary); err != nil {
		t.Fatal(err)
	}
	monitor, err := NewCompletionMonitor(home)
	if err != nil {
		t.Fatal(err)
	}
	sender := &runtimeSender{}
	worker, err := engine.New(paths, runtimeVault{}, sender)
	if err != nil {
		t.Fatal(err)
	}
	if err = worker.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true }); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	poll := func() {
		t.Helper()
		if err := monitor.Poll(ctx, false, func(e *model.Event) error { seen[e.TurnID] = true; return paths.QueueEvent(e) }); err != nil {
			t.Fatal(err)
		}
		worker.Step(ctx, time.Now())
	}
	waitTail := func(turn string) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for !seen[turn] && time.Now().Before(deadline) {
			poll()
			time.Sleep(20 * time.Millisecond)
		}
		if !seen[turn] {
			t.Fatal("actual Codex completion was not recognized in its rollout")
		}
	}
	oldTurn := turn(old)
	files, _ := os.ReadDir(paths.Inbox())
	if len(files) != 0 {
		t.Log("Codex now reloads Hooks for existing chats; fallback must still deduplicate")
	}
	waitTail(oldTurn)
	if len(sender.events) != 1 || sender.events[0].TurnID != oldTurn {
		t.Fatal("existing chat was not delivered once")
	}
	newTurn := turn(start())
	// Prove actual Stop execution, not just hooks/list registration or a manually
	// injected hook fixture, before the rollout monitor has polled the new turn.
	files, _ = os.ReadDir(paths.Inbox())
	if len(files) != 1 {
		t.Fatal("new chat did not execute the installed Stop hook")
	}
	worker.Step(ctx, time.Now())
	waitTail(newTurn)
	if len(sender.events) != 2 || len(worker.Snapshot().Records) != 2 {
		t.Fatal("hook + fallback produced duplicate notifications")
	}
	for _, e := range sender.events {
		if e.Summary != "" {
			t.Fatal("summary leaked without opt-in")
		}
	}
	t.Log("existing chat completion delivered; new chat Stop executed; both sources deduplicated")
}

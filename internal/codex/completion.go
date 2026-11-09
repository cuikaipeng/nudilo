package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"callbell/internal/model"
	"callbell/internal/storage"
)

// CompletionMonitor covers chats whose hook configuration was loaded before
// Nudilo was installed. Rollouts are an undocumented, version-dependent fallback:
// recognize only explicit task_complete records, never infer success from prose.
// Input and approval notifications continue to use the supported Hooks API.
type CompletionMonitor struct {
	StartedAt time.Time
	root      string
	files     map[string]*completionFile
}

type completionFile struct {
	info     fs.FileInfo
	offset   int64
	partial  []byte
	discard  bool
	session  string
	cwd      string
	eligible bool
	pending  *model.Event
}

type rolloutLine struct {
	Timestamp time.Time       `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// Existing files begin at EOF, so connecting or restarting never replays old
// conversations. Files discovered later must also pass the event timestamp gate.
func NewCompletionMonitor(home string) (*CompletionMonitor, error) {
	m := &CompletionMonitor{StartedAt: time.Now(), root: filepath.Join(home, "sessions"), files: map[string]*completionFile{}}
	if err := m.discover(true); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *CompletionMonitor) discover(baseline bool) error {
	return filepath.WalkDir(m.root, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		// WalkDir never follows directory symlinks. Ignore file symlinks too.
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(entry.Name(), ".jsonl") {
			return nil
		}
		if _, ok := m.files[path]; ok {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f := &completionFile{info: info}
		if baseline {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			line, _ := bufio.NewReader(io.LimitReader(file, MaxPayload)).ReadBytes('\n')
			f.readMeta(line)
			f.offset = info.Size()
			if len(line) < MaxPayload && !bytes.HasSuffix(line, []byte{'\n'}) {
				// Codex may be midway through writing a newly created header.
				// Read it from the beginning once complete; timestamps still
				// exclude old completions.
				f.offset = 0
			} else if f.offset > 0 {
				var last [1]byte
				_, _ = file.ReadAt(last[:], f.offset-1)
				f.discard = last[0] != '\n'
			}
			file.Close()
		}
		m.files[path] = f
		return nil
	})
}

func (f *completionFile) readMeta(line []byte) {
	var record rolloutLine
	if json.Unmarshal(line, &record) != nil || record.Type != "session_meta" {
		return
	}
	var meta struct {
		ID     string          `json:"id"`
		Cwd    string          `json:"cwd"`
		Source json.RawMessage `json:"source"`
	}
	if json.Unmarshal(record.Payload, &meta) != nil {
		return
	}
	// Sub-agent sources are objects, not one of these user-facing session types.
	var source string
	_ = json.Unmarshal(meta.Source, &source)
	f.eligible = meta.ID != "" && (source == "cli" || source == "vscode" || source == "exec" || source == "appServer")
	f.session, f.cwd = meta.ID, meta.Cwd
}

func (m *CompletionMonitor) event(f *completionFile, line []byte, includeSummary bool) *model.Event {
	var record rolloutLine
	if json.Unmarshal(line, &record) != nil {
		return nil
	}
	if record.Type == "session_meta" {
		f.readMeta(line)
		return nil
	}
	if record.Type == "turn_context" {
		var turn struct {
			Cwd string `json:"cwd"`
		}
		if json.Unmarshal(record.Payload, &turn) == nil && turn.Cwd != "" {
			f.cwd = turn.Cwd
		}
		return nil
	}
	if !f.eligible || record.Type != "event_msg" || record.Timestamp.Before(m.StartedAt) {
		return nil
	}
	var event struct {
		Type    string `json:"type"`
		TurnID  string `json:"turn_id"`
		Message string `json:"last_agent_message"`
	}
	if json.Unmarshal(record.Payload, &event) != nil || event.Type != "task_complete" || event.TurnID == "" {
		return nil
	}
	if !includeSummary {
		event.Message = ""
	}
	input, _ := json.Marshal(HookInput{EventName: "Stop", SessionID: f.session, TurnID: event.TurnID, Cwd: f.cwd, LastMessage: truncate(event.Message, 500)})
	normalized, _ := Normalize(input, includeSummary, record.Timestamp)
	return normalized
}

// Poll reads bounded chunks of newly appended data. A split or oversized JSONL
// record cannot lose synchronization with the next line. Failed queue writes are
// retained in memory and retried before reading further from that chat.
func (m *CompletionMonitor) Poll(ctx context.Context, includeSummary bool, emit func(*model.Event) error) error {
	var problems []error
	if err := m.discover(false); err != nil {
		problems = append(problems, err)
	}
	for path, f := range m.files {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if f.pending != nil {
			if !includeSummary {
				f.pending.Summary = ""
			}
			if err := emit(f.pending); err != nil {
				problems = append(problems, err)
				continue
			}
			f.pending = nil
		}
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			delete(m.files, path)
			continue
		}
		if err != nil {
			problems = append(problems, err)
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if !os.SameFile(info, f.info) || info.Size() < f.offset {
			*f = completionFile{info: info}
		}
		if info.Size() <= f.offset {
			continue
		}
		if err := m.read(ctx, path, f, includeSummary, emit); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

func (m *CompletionMonitor) read(ctx context.Context, path string, f *completionFile, includeSummary bool, emit func(*model.Event) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err = file.Seek(f.offset, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(io.LimitReader(file, 8*MaxPayload), 32*1024)
	for ctx.Err() == nil {
		part, err := reader.ReadSlice('\n')
		f.offset += int64(len(part))
		if len(f.partial)+len(part) > MaxPayload {
			f.discard, f.partial = true, nil
		}
		if !f.discard {
			f.partial = append(f.partial, part...)
		}
		if bytes.HasSuffix(part, []byte{'\n'}) {
			var event *model.Event
			if !f.discard {
				event = m.event(f, f.partial, includeSummary)
			}
			f.partial, f.discard = nil, false
			if event != nil {
				if err := emit(event); err != nil {
					f.pending = event
					return err
				}
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return err
		}
	}
	return ctx.Err()
}

func (m *CompletionMonitor) Run(ctx context.Context, paths storage.Paths, status func(error)) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		pref, err := paths.Preferences()
		if err == nil && !pref.AutoConnect {
			return
		}
		if err == nil {
			err = m.Poll(ctx, pref.IncludeSummary, paths.QueueEvent)
		}
		if ctx.Err() != nil {
			return
		}
		status(err)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

package codex

import (
	"bufio"
	"callbell/internal/buildinfo"
	"callbell/internal/locale"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
)

// This client only manages hooks/configuration; it never starts a model turn.
type rpc struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	lines  chan []byte
	cancel context.CancelFunc
	ctx    context.Context
	id     int
}

func startRPC(ctx context.Context, binary, home string) (*rpc, error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, binary, "app-server", "--listen", "stdio://")
	quietProcess(cmd)
	cmd.Dir = home
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "CODEX_HOME=") {
			cmd.Env = append(cmd.Env, item)
		}
	}
	cmd.Env = append(cmd.Env, "CODEX_HOME="+home)
	in, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		cancel()
		return nil, errors.New("Cannot start Codex. Select a valid executable in Advanced settings.")
	}
	r := &rpc{cmd: cmd, in: in, lines: make(chan []byte, 16), cancel: cancel, ctx: ctx}
	go func() {
		defer close(r.lines)
		scanner := bufio.NewScanner(out)
		scanner.Buffer(make([]byte, 4096), 8*1024*1024)
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			select {
			case r.lines <- line:
			case <-ctx.Done():
				return
			}
		}
	}()
	if err = r.call("initialize", map[string]any{"clientInfo": map[string]string{"name": "nudilo", "title": "Nudilo", "version": buildinfo.Version}, "capabilities": map[string]bool{"experimentalApi": true}}, nil); err != nil {
		r.close()
		return nil, err
	}
	_, err = io.WriteString(in, "{\"method\":\"initialized\"}\n")
	if err != nil {
		r.close()
		return nil, err
	}
	return r, nil
}

func (r *rpc) close() { _ = r.in.Close(); r.cancel(); _ = r.cmd.Wait() }
func (r *rpc) call(method string, params, out any) error {
	r.id++
	b, _ := json.Marshal(map[string]any{"id": r.id, "method": method, "params": params})
	if _, err := r.in.Write(append(b, '\n')); err != nil {
		return errors.New("Codex disconnected unexpectedly.")
	}
	for {
		select {
		case <-r.ctx.Done():
			return errors.New("Codex connection timed out. Check that Codex can start.")
		case line, ok := <-r.lines:
			if !ok {
				return errors.New("Codex closed the connection. Update Codex and try again.")
			}
			var response struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Code int `json:"code"`
				} `json:"error"`
			}
			if json.Unmarshal(line, &response) != nil || response.ID != r.id {
				continue
			}
			if response.Error != nil {
				return locale.M("Codex does not support this operation or its configuration is not writable ({method}, {code}).", method, response.Error.Code)
			}
			if out != nil {
				return json.Unmarshal(response.Result, out)
			}
			return nil
		}
	}
}

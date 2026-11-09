package storage

import (
	"callbell/internal/locale"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"callbell/internal/model"
)

type Paths struct{ Root string }

func DefaultPaths() (Paths, error) {
	if p := os.Getenv("CALLBELL_DATA_DIR"); p != "" {
		absolute, err := filepath.Abs(p)
		return Paths{absolute}, err
	}
	p, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{filepath.Join(p, "CallBell")}, nil
}
func (p Paths) Inbox() string    { return filepath.Join(p.Root, "inbox") }
func (p Paths) Settings() string { return filepath.Join(p.Root, "settings.json") }
func (p Paths) History() string  { return filepath.Join(p.Root, "history.json") }
func (p Paths) QueueEvent(event *model.Event) error {
	if event == nil || event.ID == "" || filepath.Base(event.ID) != event.ID {
		return errors.New("invalid event identity")
	}
	target := filepath.Join(p.Inbox(), event.ID+".json")
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return WriteJSON(target, event)
}
func (p Paths) Prepare() error {
	for _, dir := range []string{p.Root, p.Inbox(), filepath.Join(p.Root, "bin"), filepath.Join(p.Root, "backups")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}
func (p Paths) Preferences() (model.Preferences, error) {
	pref := model.Defaults()
	b, err := os.ReadFile(p.Settings())
	if errors.Is(err, os.ErrNotExist) {
		return pref, nil
	}
	if err != nil {
		return pref, err
	}
	if err = json.Unmarshal(b, &pref); err != nil {
		return pref, locale.M("Cannot read settings. The original file was preserved: {detail}", err)
	}
	if pref.Channels == nil {
		pref.Channels = map[model.Channel]bool{}
	}
	if pref.Kinds == nil {
		pref.Kinds = model.Defaults().Kinds
	}
	pref.Normalize()
	return pref, nil
}
func ReadJSON(path string, value any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, value)
}
func WriteJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(path, b, 0600)
}
func AtomicWrite(path string, b []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".callbell-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Package locale owns application text; it has no native UI dependency.
package locale

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/egoist/mygo/i18n"
)

//go:embed locales/*.json
var files embed.FS

//go:embed legacy.json
var legacyData []byte
var legacy = func() map[string]string {
	var m map[string]string
	if err := json.Unmarshal(legacyData, &m); err != nil {
		panic(err)
	}
	return m
}()

var Catalog = func() *i18n.Catalog {
	sub, err := fs.Sub(files, "locales")
	if err != nil {
		panic(err)
	}
	c, err := i18n.NewCatalog(sub, "en")
	if err != nil {
		panic(err)
	}
	i18n.Register(c)
	return c
}()

func Normalize(value string) string {
	if value == "" || strings.EqualFold(value, "system") {
		return "system"
	}
	tag := i18n.Normalize(value)
	if slices.Contains(Catalog.Languages(), tag) {
		return tag
	}
	return "system"
}

func Resolve(value string, preferred []string) string {
	if tag := Normalize(value); tag != "system" {
		return tag
	}
	if tag := i18n.Match(preferred, Catalog.Languages()); tag != "" {
		return tag
	}
	return "en"
}

func Text(language, key string, values ...any) string {
	if id, ok := legacy[key]; ok {
		key = id
	}
	var args map[string]any
	if m, _, ok := Catalog.Message("en", key); ok && len(values) > 0 {
		args = map[string]any{}
		for i, arg := range m.Args() {
			if i >= len(values) {
				break
			}
			value := values[i]
			switch v := value.(type) {
			case Message:
				value = v.Text(language)
			case error:
				value = FromError(v).Text(language)
			}
			args[arg.Name] = value
		}
	}
	return Catalog.Format(language, key, args)
}

// Messages retain IDs and arguments so existing feedback follows a language
// change. Serializable messages also localize stored delivery failures.
type Message struct {
	Key  string `json:"key"`
	Args []any  `json:"args,omitempty"`
}

func M(key string, args ...any) Message       { return Message{key, args} }
func (m Message) Text(language string) string { return Text(language, m.Key, m.Args...) }
func (m Message) Error() string               { return m.Text("en") }
func (m Message) Unwrap() error {
	for _, value := range m.Args {
		if err, ok := value.(error); ok {
			return err
		}
	}
	return nil
}
func FromError(err error) Message {
	if err == nil {
		return Message{}
	}
	var m Message
	if errors.As(err, &m) {
		return m
	}
	if _, _, ok := Catalog.Message("en", err.Error()); ok {
		return M(err.Error())
	}
	if key, ok := legacy[err.Error()]; ok {
		return M(key)
	}
	return M("{detail}", fmt.Sprint(err))
}

// Decode nested translated arguments after history is reloaded from JSON.
func (m *Message) UnmarshalJSON(data []byte) error {
	var raw struct {
		Key  string            `json:"key"`
		Args []json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.Key = raw.Key
	m.Args = nil
	for _, arg := range raw.Args {
		var nested struct {
			Key string `json:"key"`
		}
		if len(arg) > 0 && arg[0] == '{' && json.Unmarshal(arg, &nested) == nil && nested.Key != "" {
			var value Message
			if err := json.Unmarshal(arg, &value); err != nil {
				return err
			}
			m.Args = append(m.Args, value)
		} else {
			var value any
			if err := json.Unmarshal(arg, &value); err != nil {
				return err
			}
			m.Args = append(m.Args, value)
		}
	}
	return nil
}

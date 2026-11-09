package locale

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCatalogParity(t *testing.T) {
	source, err := files.ReadFile("locales/en.json")
	if err != nil {
		t.Fatal(err)
	}
	var en map[string]string
	if err = json.Unmarshal(source, &en); err != nil {
		t.Fatal(err)
	}
	for _, tag := range Catalog.Languages() {
		data, err := files.ReadFile("locales/" + tag + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err = json.Unmarshal(data, &values); err != nil {
			t.Fatal(err)
		}
		for key := range en {
			if strings.HasPrefix(key, "@") {
				continue
			}
			if _, ok := values[key]; !ok {
				t.Errorf("%s missing %s", tag, key)
			}
			original, _, ok := Catalog.Message("en", key)
			if !ok {
				t.Fatalf("invalid source %s", key)
			}
			translated, found, ok := Catalog.Message(tag, key)
			if !ok || found != tag {
				t.Fatalf("invalid translation %s %s", tag, key)
			}
			a, b := original.Args(), translated.Args()
			if len(a) != len(b) {
				t.Errorf("argument mismatch %s %s", tag, key)
			}
			for _, arg := range a {
				has := false
				for _, other := range b {
					if arg.Name == other.Name {
						has = true
					}
				}
				if !has {
					t.Errorf("%s missing argument %s in %s", tag, arg.Name, key)
				}
			}
		}
	}
}
func TestSystemLanguageMatching(t *testing.T) {
	for _, tc := range []struct {
		setting   string
		preferred []string
		want      string
	}{
		{"system", []string{"zh-CN", "en-US"}, "zh-Hans"}, {"system", []string{"en-GB"}, "en"},
		{"system", []string{"fr-FR"}, "en"}, {"en", []string{"zh-CN"}, "en"}, {"zh-Hans", nil, "zh-Hans"}, {"bogus", nil, "en"},
	} {
		if got := Resolve(tc.setting, tc.preferred); got != tc.want {
			t.Errorf("%+v got %s", tc, got)
		}
	}
}
func TestStoredFeedbackChangesLanguage(t *testing.T) {
	m := M("{channel} returned HTTP {code}. Check the connection and permissions.", M("Feishu"), 403)
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var saved Message
	if err = json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if got := saved.Text("zh-Hans"); !strings.Contains(got, "飞书 返回 HTTP 403") {
		t.Fatal(got)
	}
	if got := saved.Text("en"); !strings.Contains(got, "Feishu returned HTTP 403") {
		t.Fatal(got)
	}
	if got := Text("en", "收到时尚未配置通知渠道。"); got != "No channels were configured when this event arrived." {
		t.Fatal(got)
	}
	raw := M("{detail}", "项目／raw-text")
	if raw.Text("en") != "项目／raw-text" {
		t.Fatal("raw diagnostics changed")
	}
	if got := Text("en", "{count, plural, one {# notification} other {# notifications}}", 2); got != "2 notifications" {
		t.Fatal(got)
	}
}

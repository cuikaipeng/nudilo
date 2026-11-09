package desktop

import (
	"strings"
	"testing"
	"time"

	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

func notificationEditorTester(a *app, width, height int) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		applyPalette(c, a.palette(c))
		c.Root().Background(c.Theme().Background)
		ui.Scroll(c).Fill().Padding(20).Children(func() {
			ui.Column(c).FillWidth().Gap(14).Children(func() { a.notificationEditor(c) })
		})
	}, width, height)
}

func TestNotificationEditorTypingVariablesAndLanguageDrafts(t *testing.T) {
	a := newApp()
	a.snapshot.Preferences.Language = "en"
	en := a.notificationDraft("en")
	en.content.TurnEnded = ""
	tt := notificationEditorTester(a, 780, 590)
	if err := tt.Click("Message text"); err != nil {
		t.Fatal(err)
	}
	tt.Type("Ready 中文 ")
	if err := tt.Click("Project"); err != nil {
		t.Fatal(err)
	}
	tt.Type("!")
	if en.content.TurnEnded != "Ready 中文 {project}!" {
		t.Fatal("insertion did not preserve Unicode text/caret", en.content.TurnEnded)
	}
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "Ready 中文 nudilo!") {
		t.Fatal("live preview did not update")
	}
	if err := tt.Click("Summary"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(en.content.TurnEnded, "{summary}") {
		t.Fatal("summary inserted without opt-in")
	}
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if a.snapshot.Preferences.MessageContent["en"].TurnEnded != "Ready 中文 {project}!" {
		t.Fatal("English draft was not saved")
	}
	en.content.InputRequired = "Question for {project}"
	a.snapshot.Preferences.Language = "zh-Hans"
	tt.Frame()
	zh := a.notificationDraft("zh-Hans")
	if !tt.HasText("消息正文") || zh.content.TurnEnded == en.content.TurnEnded {
		t.Fatal("language switch reused the English draft")
	}
	zh.content.TurnEnded = "可以看结果啦：{project}"
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	a.snapshot.Preferences.Language = "en"
	tt.Frame()
	if en.content.InputRequired != "Question for {project}" {
		t.Fatal("language switch lost unsaved draft")
	}
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	if en.content.InputRequired != model.DefaultNotificationContent("en").InputRequired {
		t.Fatal("Cancel did not restore saved content")
	}
	a.snapshot.Preferences.Language = "zh-Hans"
	tt.Frame()
	if err := tt.Click("恢复默认"); err != nil {
		t.Fatal(err)
	}
	if a.snapshot.Preferences.MessageContent["zh-Hans"].TurnEnded == "" {
		t.Fatal("Reset saved without pressing Save")
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.snapshot.Preferences.MessageContent["zh-Hans"]; ok || a.snapshot.Preferences.MessageContent["en"].TurnEnded == "" {
		t.Fatal("Reset affected the other language or retained defaults")
	}
}

func TestNotificationValidationStaysWithItsField(t *testing.T) {
	a := newApp()
	a.snapshot.Preferences.Language = "zh-Hans"
	draft := a.notificationDraft("zh-Hans")
	draft.content.TurnEnded = "未知 {typo}"
	tt := notificationEditorTester(a, 620, 590)
	errText := "请用变量按钮插入内容，并检查花括号是否完整。"
	field, ok := tt.Find("消息正文")
	if !ok {
		t.Fatal("body field missing")
	}
	feedback, ok := tt.Find(errText)
	if !ok || feedback.Y < field.Y+170 || a.message.Key != "" {
		t.Fatal("template error is not inline", field, feedback)
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if len(a.snapshot.Preferences.MessageContent) != 0 {
		t.Fatal("invalid template saved")
	}
	if err := tt.Click("需要回答"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("本轮已结束: " + errText) {
		t.Fatal("invalid hidden type has no explanation for disabled Save")
	}
	saveFeedbackPreview(t, tt, "notification-validation")
}

func TestNotificationEditorBothLanguagesAndThemesAtMinimumSize(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(language+"-"+theme, func(t *testing.T) {
				a := newApp()
				a.page = "settings"
				a.snapshot.Preferences.Language, a.snapshot.Preferences.Theme = language, theme
				a.snapshot.Preferences.IncludeSummary = true
				a.notificationDraft(language).previewTime = time.Date(2026, 11, 5, 14, 30, 0, 0, time.Local)
				tt := ui.NewTester(a.view, 880, 620)
				// Reach the content editor through the settings scroll area.
				tt.Scroll(790, 380, 0, 220)
				tt.Advance(300 * time.Millisecond)
				if err := tt.Click(a.tr("Customize")); err != nil {
					t.Fatal(err)
				}
				tt.Advance(300 * time.Millisecond)
				tt.Scroll(790, 380, 0, 410)
				tt.Advance(300 * time.Millisecond)
				if !tt.HasText(a.tr("Notification title")) || !tt.HasText(a.tr("Preview")) {
					t.Fatal("editor absent from settings")
				}
				// Both columns must fit the smallest supported desktop window.
				for _, label := range []string{"Message text", "Project", "Summary", "Preview", "Save"} {
					r, ok := tt.Find(a.tr(label))
					if !ok || r.X < 192 || r.X+r.W > 880 {
						t.Fatalf("%s overflows: %+v", label, r)
					}
				}
				saveFeedbackPreview(t, tt, "notification-"+language+"-"+theme)
			})
		}
	}
}

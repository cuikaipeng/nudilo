package engine

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"callbell/internal/model"
)

func TestRetryUsesLatestContentLanguageAndSummaryPreference(t *testing.T) {
	e, paths, vault, sender := harness(t)
	sender.failOnce = true
	e.SetLocale("en")
	if err := e.UpdatePreferences(func(p *model.Preferences) {
		p.Channels[model.Slack] = true
		p.IncludeSummary = true
		p.SetNotificationContent("en", model.NotificationContent{Title: "English title", TurnEnded: "Finished {project}: {summary}"})
		p.SetNotificationContent("zh-Hans", model.NotificationContent{Title: "中文标题", TurnEnded: "做好啦 {project}：{summary}"})
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	queue(t, paths, "localized-retry", now)
	e.Step(context.Background(), now)
	if len(sender.events) != 1 || !strings.Contains(sender.events[0].Message(), "Finished work: private reply") {
		t.Fatal("first attempt did not use English customization", sender.events)
	}
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Language = "zh-Hans"; p.IncludeSummary = false }); err != nil {
		t.Fatal(err)
	}
	// A retry in this run adopts settings changed since the first attempt.
	e.Step(context.Background(), now.Add(time.Minute))
	if len(sender.events) != 2 {
		t.Fatal("retry missing", sender.events)
	}
	message := sender.events[1].Message()
	if !strings.Contains(message, "中文标题 · Codex | 本轮已结束") || !strings.Contains(message, "做好啦 work") || strings.Contains(message, "private reply") || sender.events[1].Summary != "" {
		t.Fatal("retry reused old language/content or leaked disabled summary", message)
	}
	if e.Snapshot().Records[0].Event.RenderedText != "" {
		t.Fatal("rendered message stored in history")
	}
	// Saved language and templates still apply to new sends after reopening.
	reopened, err := New(paths, vault, sender)
	if err != nil {
		t.Fatal(err)
	}
	reopened.SetLocale("en")
	if err := reopened.TestChannel(context.Background(), model.Slack); err != nil {
		t.Fatal(err)
	}
	test := sender.events[2].Message()
	if !strings.Contains(test, "中文标题") || !strings.Contains(test, model.Test.DescriptionFor(sender.events[2].AgentName(), "zh-Hans")) || strings.Contains(test, "做好啦") {
		t.Fatal("test notification did not follow current settings", test)
	}
	// Setting back to system adopts the resolved system locale at send time.
	if err := reopened.UpdatePreferences(func(p *model.Preferences) { p.Language = "system" }); err != nil {
		t.Fatal(err)
	}
	now = time.Now()
	queue(t, paths, "system-again", now)
	reopened.Step(context.Background(), now)
	if !strings.Contains(sender.events[3].Message(), "English title") {
		t.Fatal("system language did not select its saved customization")
	}
}

func TestInvalidNotificationContentDoesNotReplacePreferences(t *testing.T) {
	e, paths, _, _ := harness(t)
	if err := e.UpdatePreferences(func(p *model.Preferences) {
		p.SetNotificationContent("en", model.NotificationContent{Title: "Saved title"})
	}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(paths.Settings())
	if err != nil {
		t.Fatal(err)
	}
	err = e.UpdatePreferences(func(p *model.Preferences) {
		p.SetNotificationContent("en", model.NotificationContent{TurnEnded: "{unknown}"})
	})
	if err == nil {
		t.Fatal("invalid template accepted")
	}
	after, err := os.ReadFile(paths.Settings())
	if err != nil || !bytes.Equal(before, after) || e.Snapshot().Preferences.MessageContent["en"].Title != "Saved title" {
		t.Fatal("failed validation replaced saved content", err)
	}
}

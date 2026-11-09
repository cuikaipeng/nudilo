package desktop

import (
	"testing"

	"callbell/internal/engine"
	"callbell/internal/model"
	"callbell/internal/storage"
)

func TestEnabledChannelCanBeSavedWithoutCredentials(t *testing.T) {
	paths := storage.Paths{Root: t.TempDir()}
	e, err := engine.New(paths, storage.NewLocalVault(paths), nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true })
	if err = e.SaveChannel(model.Feishu, model.Credentials{Webhook: "https://open.feishu.cn/open-apis/bot/v2/hook/reentered"}); err != nil {
		t.Fatal("enabled channel could not be reconfigured without saved credentials", err)
	}
}

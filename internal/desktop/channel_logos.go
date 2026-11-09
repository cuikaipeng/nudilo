package desktop

import (
	"embed"

	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

// Kept in the binary: no network or external asset paths at runtime.
// Sources and licenses are in assets/channels/README.md.
//
//go:embed assets/channels/*.svg
var channelAssets embed.FS

func channelLogo(name string) *ui.SVG {
	data, err := channelAssets.ReadFile("assets/channels/" + name + ".svg")
	if err != nil {
		panic(err)
	}
	return ui.MustParseSVG(data)
}

var channelLogos = map[model.Channel]*ui.SVG{
	model.Feishu:   channelLogo("feishu"),
	model.Lark:     channelLogo("feishu"),
	model.DingTalk: channelLogo("dingtalk"),
	model.Telegram: channelLogo("telegram"),
	model.Slack:    channelLogo("slack"),
}

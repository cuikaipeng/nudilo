package desktop

import "github.com/egoist/mygo/ui"

func (a *app) revealSetting(id string, element ui.Element) {
	if a.settingsTarget == id {
		element.ScrollIntoView()
		a.settingsTarget = ""
	}
}
func (a *app) settingsSection(c *ui.Context, id, title string, body func()) {
	a.revealSetting(id, section(c, title, body))
}
func (a *app) settingsBlock(c *ui.Context, id string, body func()) {
	a.revealSetting(id, ui.Column(c.Key("settings-"+id)).FillWidth().Children(body))
}

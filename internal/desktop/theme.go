package desktop

import "github.com/egoist/mygo/ui"

// Coral is sampled from the approved TOUCH mark. Dark text on the coral
// keeps primary actions readable; the darker ink is for text on light cards.
const brandCoral = "#DC7A65"

type palette struct {
	dark                                                                                    bool
	canvas, sidebar, card, text, muted, border, control, hover, accentInk, selected, shadow ui.Color
}

var lightPalette = palette{
	canvas: ui.Hex("#F6F1ED"), sidebar: ui.Hex("#F1E8E2"), card: ui.Hex("#FFFCFA"),
	text: ui.Hex("#302722"), muted: ui.Hex("#76665D"), border: ui.Hex("#E5DAD2"),
	control: ui.Hex("#F5EDE7"), hover: ui.Hex("#ECDFD6"), accentInk: ui.Hex("#9D4432"),
	selected: ui.Hex("#F4D9CF"), shadow: ui.RGBA(70, 35, 20, 0.07),
}
var darkPalette = palette{
	dark: true, canvas: ui.Hex("#1C1917"), sidebar: ui.Hex("#231E1B"), card: ui.Hex("#2A2421"),
	text: ui.Hex("#F4EDE7"), muted: ui.Hex("#BEB0A6"), border: ui.Hex("#4A3D35"),
	control: ui.Hex("#382E28"), hover: ui.Hex("#46382F"), accentInk: ui.Hex("#F0A08B"),
	selected: ui.Hex("#49312A"), shadow: ui.RGBA(0, 0, 0, 0.18),
}

func (a *app) palette(c *ui.Context) palette {
	p := lightPalette
	if a.snapshot.Preferences.Theme == "dark" || a.snapshot.Preferences.Theme != "light" && c.Theme().Dark {
		p = darkPalette
	}
	if c.Preferences().HighContrast {
		p.border = p.border.Mix(p.text, .55)
		p.muted = p.muted.Mix(p.text, .45)
	}
	return p
}
func applyPalette(c *ui.Context, p palette) {
	t := *ui.LightTheme()
	t.Dark = p.dark
	t.Background, t.Surface, t.SurfaceHover, t.SurfacePressed = p.card, p.control, p.hover, p.selected
	t.Text, t.TextMuted, t.Border = p.text, p.muted, p.border
	t.Accent, t.AccentText = ui.Hex(brandCoral), ui.Hex("#271B16")
	t.AccentHover, t.AccentPressed = t.Accent.Mix(ui.Hex("#FFFFFF"), .09), t.Accent.Mix(ui.Hex("#000000"), .09)
	t.Selection, t.Focus = t.Accent.Alpha(.24), p.accentInk.Alpha(.7)
	t.Inverse, t.InverseText, t.Scrollbar = p.text, p.card, p.muted.Alpha(.5)
	t.Danger, t.Warning, t.Success = ui.Hex("#AE342B"), ui.Hex("#8D5A13"), ui.Hex("#3D6D4C")
	if p.dark {
		t.Danger, t.Warning, t.Success = ui.Hex("#FF9B8E"), ui.Hex("#E3BE7F"), ui.Hex("#93C5A1")
	}
	t.FontSize, t.Radius, t.Spacing = 13, 8, 4
	t.FontSize = fontSize(c, 13)
	if c.Preferences().HighContrast {
		t.Focus = p.accentInk
	}
	c.SetTheme(&t)
}

func fontSize(c *ui.Context, size float32) float32 {
	scale := c.Preferences().TextScale
	if scale <= 0 {
		scale = 1
	}
	return size * scale
}

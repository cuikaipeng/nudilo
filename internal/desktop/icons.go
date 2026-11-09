package desktop

import "github.com/egoist/mygo/ui"

// Simple line icons drawn for Nudilo; all share the same 24-unit grid.
func lineIcon(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var iconOverview = lineIcon(`<rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><rect x="14" y="14" width="7" height="7" rx="2"/>`)
var iconChannels = lineIcon(`<path d="M5 12h14M8 9l-3 3 3 3m8-6 3 3-3 3"/><path d="M9 4H5a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h4m6-16h4a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-4"/>`)
var iconActivity = lineIcon(`<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`)
var iconSettings = lineIcon(`<path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="3" fill="currentColor" stroke="none"/><circle cx="16" cy="17" r="3" fill="currentColor" stroke="none"/>`)
var iconBell = lineIcon(`<path d="M6 9a6 6 0 0 1 12 0v6l2 3H4l2-3zm4 12h4"/>`)
var iconCheck = lineIcon(`<path d="m6 12 4 4 8-8"/>`)

var iconDown = lineIcon(`<path d="m6 9 6 6 6-6"/>`)
var iconClose = lineIcon(`<path d="m6 6 12 12M18 6 6 18"/>`)
var iconAlert = lineIcon(`<circle cx="12" cy="12" r="9"/><path d="M12 7v6m0 3h.01"/>`)
var iconRefresh = lineIcon(`<path d="M20 7v5h-5M4 17v-5h5M6.1 6.1A8 8 0 0 1 19.8 11M4.2 13a8 8 0 0 0 13.7 4.9"/>`)
var iconPause = lineIcon(`<path d="M9 5v14M15 5v14" stroke-width="3"/>`)
var iconPlay = lineIcon(`<path d="m8 5 11 7-11 7z" fill="currentColor" stroke="none"/>`)
var iconCircle = lineIcon(`<circle cx="12" cy="12" r="6"/>`)
var iconTrash = lineIcon(`<path d="M4 7h16M10 11v6m4-6v6M6 7l1 14h10l1-14M9 7V3h6v4"/>`)

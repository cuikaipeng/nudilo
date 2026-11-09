package desktop

import (
	"runtime"

	"callbell/internal/locale"
	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	_ "github.com/egoist/mygo/locales/zh-Hant"
)

func (a *app) locale() string {
	if a.native {
		return mygo.App.Locale()
	}
	return locale.Resolve(a.snapshot.Preferences.Language, nil)
}
func (a *app) tr(key string, args ...any) string { return locale.Text(a.locale(), key, args...) }
func (a *app) applyPreferences() {
	if !a.native {
		return
	}
	source := mygo.ThemeSource(a.snapshot.Preferences.Theme)
	if mygo.Theme.Source() != source {
		mygo.Theme.SetSource(source)
	}
	language := a.snapshot.Preferences.Language
	if language == "system" {
		language = ""
	}
	mygo.App.SetLocale(language)
	a.refreshMenus()
}
func (a *app) refreshMenus() {
	if !a.native {
		return
	}
	mygo.App.SetMenu(a.applicationMenu())
	if a.tray != nil {
		a.tray.SetMenu(a.trayMenu())
	}
	if a.engine != nil {
		a.engine.SetLocale(a.locale())
	}
	if a.window != nil {
		a.window.Invalidate()
	}
}
func (a *app) applicationMenu() *mygo.Menu {
	preferences := &mygo.MenuItem{Label: a.tr("Preferences…"), Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) { a.page = "settings"; a.show() }}
	checkUpdates := &mygo.MenuItem{Label: a.tr("Check for updates"), Click: func(*mygo.MenuItem, *mygo.Window) { a.openUpdates() }}
	fileItems := []*mygo.MenuItem{
		{Label: a.tr("Open Nudilo"), Accelerator: "CmdOrCtrl+O", Click: func(*mygo.MenuItem, *mygo.Window) { a.show() }},
	}
	var items []*mygo.MenuItem
	if runtime.GOOS == "darwin" {
		items = append(items, &mygo.MenuItem{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout}, checkUpdates,
			mygo.Separator(), preferences,
			mygo.Separator(), {Role: mygo.RoleServices},
			mygo.Separator(), {Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide},
			mygo.Separator(), {Role: mygo.RoleQuit},
		}})
	} else {
		fileItems = append(fileItems, preferences, checkUpdates)
	}
	fileItems = append(fileItems, &mygo.MenuItem{Label: a.tr("Close window"), Role: mygo.RoleClose})
	if runtime.GOOS != "darwin" {
		fileItems = append(fileItems, mygo.Separator(), &mygo.MenuItem{Label: a.tr("Quit Nudilo"), Role: mygo.RoleQuit})
	}
	items = append(items,
		&mygo.MenuItem{Label: a.tr("File"), Submenu: fileItems},
		&mygo.MenuItem{Label: a.tr("Edit"), Role: mygo.RoleEditMenu},
		&mygo.MenuItem{Label: a.tr("Window"), Role: mygo.RoleWindowMenu},
	)
	return mygo.NewMenu(items)
}
func (a *app) trayMenu() *mygo.Menu {
	pauseItem := &mygo.MenuItem{Label: a.tr("Pause"), Disabled: a.busy}
	if a.snapshot.Preferences.Paused {
		pauseItem.Label = a.tr("Resume")
		pauseItem.Click = func(*mygo.MenuItem, *mygo.Window) { a.togglePause() }
	} else {
		for i, label := range pauseOptions {
			pauseItem.Submenu = append(pauseItem.Submenu, &mygo.MenuItem{Label: a.tr(label), Click: func(*mygo.MenuItem, *mygo.Window) { a.pauseFor(i) }})
		}
	}
	updateLabel := "Check for updates"
	if a.updateState.Phase == "ready" {
		updateLabel = "Update ready"
	}
	items := []*mygo.MenuItem{
		{Label: a.tr("Open Nudilo"), Click: func(*mygo.MenuItem, *mygo.Window) { a.show() }},
		pauseItem,
		{Label: a.tr(updateLabel), Click: func(*mygo.MenuItem, *mygo.Window) { a.openUpdates() }},
		mygo.Separator(), {Label: a.tr("Quit Nudilo"), Role: mygo.RoleQuit},
	}
	if a.snapshot.Preferences.Paused && !a.snapshot.Preferences.PauseUntil.IsZero() {
		items = append(items[:2], append([]*mygo.MenuItem{{Label: a.tr("Resumes {time}", a.date(a.snapshot.Preferences.PauseUntil)), Disabled: true}}, items[2:]...)...)
	}
	return mygo.NewMenu(items)
}

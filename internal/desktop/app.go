package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"callbell/internal/buildinfo"
	"callbell/internal/channels"
	"callbell/internal/claude"
	"callbell/internal/codex"
	"callbell/internal/engine"
	"callbell/internal/locale"
	"callbell/internal/model"
	"callbell/internal/storage"
	"callbell/internal/updates"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/netproxy"
	"github.com/egoist/mygo/ui"
)

type app struct {
	window                 *mygo.Window
	engine                 *engine.Engine
	client                 *channels.Client
	installer              codex.Installer
	claudeInstaller        claude.Installer
	claudeConnection       claude.Connection
	claudeWorking          bool
	claudeError            error
	claudeSince            time.Time
	claudeChecked          time.Time
	claudeHome             string
	claudeBinary           string
	claudeAdvanced         bool
	ctx                    context.Context
	page                   string
	snapshot               engine.Snapshot
	connection             codex.Connection
	connecting             bool
	refreshing             bool
	connectionChecked      time.Time
	connectionError        error
	monitorError           error
	monitorSince           time.Time
	monitorCancel          context.CancelFunc
	monitorGeneration      uint64
	busy                   bool
	message                locale.Message
	messageError           bool
	feedbackPage           string
	feedbackEpoch          uint64
	feedbackVersion        uint64
	displayedFeedback      uint64
	fieldErrors            map[string]locale.Message
	focusField             string
	clearHistoryOpen       bool
	channel                model.Channel
	forms                  map[model.Channel]*model.Credentials
	savedChannels          map[model.Channel]model.Credentials
	editingChannels        map[model.Channel]bool
	pairedApps             map[model.Channel]pairIdentity
	localMissing           map[model.Channel]bool
	pairCode               string
	pairURL                string
	pairReady              bool
	pairCancel             context.CancelFunc
	pairAttempt            uint64
	channelOptions         bool
	advanced               bool
	codexHome, codexBinary string
	login                  bool
	fatal                  error
	tray                   *mygo.Tray
	logo                   *ui.Bitmap
	native                 bool
	help                   bool
	localeChoice           string
	notificationOpen       bool
	notificationDrafts     map[string]*notificationDraft
	updates                *updates.Controller
	updateState            updates.State
	updateRestartOpen      bool
	showAssistants         bool
	diagnosisOpen          bool
	diagnosing             bool
	diagnosis              []diagnosticResult
	diagnosisAt            time.Time
	channelDetails         bool
	removeChannelOpen      bool
	historyQuery           string
	historyAgent           model.Agent
	historyState           string
	historyLimit           int
	historyExpanded        map[string]bool
	routesOpen             bool
	routeDraft             *model.ProjectRoute
	routeIndex             int
	routeError             locale.Message
	removeRouteOpen        bool
	updatesOpen            bool
	settingsTarget         string
	setupRequested         bool
	setupAutoDismissTried  bool
	diagnosisReveal        bool
}

var singleInstanceKey = "CallBell"

func Run() error {
	// Keep the original single-instance key so old and renamed builds cannot
	// run two notification workers against the same persisted queue.
	mygo.App.SetName(singleInstanceKey)
	mygo.App.SetVersion(buildinfo.Version)
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	mygo.App.SetName("Nudilo")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := newApp()
	a.ctx = ctx
	a.native = true
	mygo.App.OnLocaleChanged(func(string) { a.refreshMenus() })

	mygo.App.OnSecondInstance(func([]string, string) { a.show() })
	mygo.App.OnActivate(func(bool) { a.show() })
	quitting := false
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting = true; cancel() })
	mygo.App.WhenReady(func() {
		resourceDir, _ := mygo.App.Path(mygo.PathResources)
		if data, err := os.ReadFile(filepath.Join(resourceDir, "logo.png")); err == nil {
			a.logo, _ = ui.DecodeBitmap(data)
		}
		paths, err := storage.DefaultPaths()
		if err == nil {
			a.client = channels.NewClient(netproxy.Transport())
			a.engine, err = engine.New(paths, storage.NewLocalVault(paths), a.client)
		}
		if err != nil {
			a.fatal = err
		} else {
			a.snapshot = a.engine.Snapshot()
			a.applyLocalChannels(readLocalChannels(a.engine, a.snapshot.Preferences))
			for _, ch := range model.Channels {
				a.restoreChannelForm(ch)
			}
			a.codexHome = a.snapshot.Preferences.CodexHome
			a.codexBinary = a.snapshot.Preferences.CodexBinary
			a.claudeHome = a.snapshot.Preferences.ClaudeHome
			a.claudeBinary = a.snapshot.Preferences.ClaudeBinary
			helperName := "callbell-hook"
			if runtime.GOOS == "windows" {
				helperName += ".exe"
			}
			helper := filepath.Join(resourceDir, "bin", helperName)
			if _, err := os.Stat(helper); err != nil {
				helper = filepath.Join(resourceDir, runtime.GOOS+"-"+runtime.GOARCH, "bin", helperName)
			}
			a.installer = codex.Installer{Paths: paths, HelperSource: helper}
			a.claudeInstaller = claude.Installer{Paths: paths, HelperSource: helper}
		}
		a.login = mygo.App.OpenAtLogin()
		a.applyPreferences()
		options := mygo.WindowOptions{Title: "Nudilo", Width: 1040, Height: 760, MinWidth: 880, MinHeight: 620, StateKey: "main", Hidden: mygo.App.WasOpenedAtLogin(), BackgroundColor: "light-dark(#F6F1ED, #1C1917)", Content: ui.View(a.view)}
		if runtime.GOOS == "darwin" {
			options.TitleBarStyle = mygo.TitleBarHiddenInset
			options.TitleBarHeight = 48
			options.TrafficLightPosition = &mygo.Point{X: 18, Y: 18}
		}
		a.window = mygo.NewWindow(options)
		a.setupTray()
		if options.Hidden {
			if a.tray != nil {
				a.hideToTray()
			} else {
				// A login launch must remain reachable if the tray is unavailable.
				a.show()
			}
		}
		a.window.OnClose(func(e *mygo.CloseEvent) {
			if !quitting && a.tray != nil {
				e.PreventDefault()
				a.hideToTray()
			}
		})
		if a.engine != nil {
			a.engine.OnChange(func() {
				s := a.engine.Snapshot()
				local := readLocalChannels(a.engine, s.Preferences)
				a.window.Update(func() {
					previous := a.snapshot.Preferences
					a.snapshot = s
					if a.updates != nil && previous.AutomaticUpdates != s.Preferences.AutomaticUpdates {
						a.updates.SetAutomatic(s.Preferences.AutomaticUpdates)
					}
					a.applyLocalChannels(local)
					a.localeChoice = s.Preferences.Language
					if previous.Theme != s.Preferences.Theme || previous.Language != s.Preferences.Language {
						a.applyPreferences()
					} else if previous.Paused != s.Preferences.Paused || previous.PauseUntil != s.Preferences.PauseUntil {
						a.refreshMenus()
					}
				})
			})
			go a.engine.Run(ctx)
			a.startUpdates()
			if a.snapshot.Preferences.AutoConnect {
				a.connect()
			}
			if a.snapshot.Preferences.ClaudeAutoConnect {
				a.connectClaude()
			}
		}
	})
	return mygo.App.Run()
}
func newApp() *app {
	forms := map[model.Channel]*model.Credentials{}
	for _, ch := range model.Channels {
		forms[ch] = &model.Credentials{}
		forms[ch].Normalize(ch)
	}
	return &app{ctx: context.Background(), page: "overview", channel: model.Feishu, forms: forms,
		savedChannels: map[model.Channel]model.Credentials{}, editingChannels: map[model.Channel]bool{},
		pairedApps: map[model.Channel]pairIdentity{}, snapshot: engine.Snapshot{Preferences: model.Defaults()}}
}
func (a *app) show() {
	if a.window != nil {
		if runtime.GOOS == "darwin" {
			mygo.App.Dock.Show()
		}
		a.window.Show()
		a.window.Restore()
		a.window.Focus()
	}
}
func (a *app) hideToTray() {
	if a.window == nil || a.tray == nil {
		return
	}
	a.window.Hide()
	if runtime.GOOS == "darwin" {
		// Accessory mode removes the Dock icon while keeping the tray and
		// notification worker alive. Opening the window restores regular mode.
		mygo.App.Dock.Hide()
	}
}
func (a *app) setupTray() {
	root, _ := mygo.App.Path(mygo.PathResources)
	// macOS and Windows tint template artwork for the system bar. Linux's
	// AppIndicator does not, so use a coral version visible on either panel.
	template := runtime.GOOS != "linux"
	iconName := "tray.png"
	if !template {
		iconName = "tray-color.png"
	}
	icon, _ := os.ReadFile(filepath.Join(root, iconName))
	tray, err := mygo.NewTray(mygo.TrayOptions{Icon: icon, IconIsTemplate: template, ToolTip: "Nudilo · A little nudge from your AI.", Menu: a.trayMenu()})
	if err != nil {
		a.showFeedback(locale.M("The system tray is unavailable. Keep the window open to receive notifications."), true)
		return
	}
	a.tray = tray
}
func (a *app) connect() {
	if a.connecting || a.refreshing || a.diagnosing || a.engine == nil {
		return
	}
	a.connecting = true
	a.connectionError = nil
	a.monitorError = nil
	if a.monitorCancel != nil {
		a.monitorCancel()
		a.monitorCancel = nil
	}
	a.monitorGeneration++
	generation := a.monitorGeneration
	a.showFeedback(locale.Message{}, false)
	a.connection.Ready = false
	home, binary := a.codexHome, a.codexBinary
	go func() {
		err := a.engine.UpdatePreferences(func(p *model.Preferences) { p.AutoConnect = true; p.CodexHome = home; p.CodexBinary = binary })
		var result codex.Connection
		var monitor *codex.CompletionMonitor
		if err == nil {
			result, err = a.installer.Connect(a.ctx, home, binary)
		}
		if err == nil {
			monitor, err = codex.NewCompletionMonitor(result.Home)
			if err != nil {
				err = locale.M("Cannot monitor existing Codex chats. Check access to the Codex configuration folder.")
				result.Ready = false
			}
		}
		a.window.Update(func() {
			a.connecting = false
			a.connection = result
			a.connectionChecked = time.Now()
			if err != nil {
				a.connectionError = err
			} else {
				a.monitorSince = monitor.StartedAt
				ctx, cancel := context.WithCancel(a.ctx)
				a.monitorCancel = cancel
				go func() {
					failed := false
					monitor.Run(ctx, a.installer.Paths, func(err error) {
						if (err != nil) == failed {
							return
						}
						failed = err != nil
						a.window.Update(func() {
							if a.monitorGeneration != generation {
								return
							}
							a.monitorError = nil
							if err != nil {
								a.monitorError = locale.M("Existing-chat monitoring failed. Check folder access and free disk space.")
							}
						})
					})
				}()
			}
		})
	}()
}
func (a *app) work(fn func() error, success string) {
	a.workAfter(fn, success, nil)
}

func (a *app) workAfter(fn func() error, success string, done func()) {
	if a.busy {
		return
	}
	a.busy = true
	a.showFeedback(locale.Message{}, false)
	scope := a.scope()
	go func() {
		err := fn()
		a.window.Update(func() {
			a.finishWork(scope, err, success)
			if done != nil {
				done()
			}
		})
	}()
}
func (a *app) setPreference(edit func(*model.Preferences)) {
	if a.engine == nil {
		edit(&a.snapshot.Preferences)
		a.snapshot.Preferences.Normalize()
		return
	}
	a.work(func() error { return a.engine.UpdatePreferences(edit) }, "")
}
func (a *app) chooseChannel(ch model.Channel) {
	a.cancelPair()
	a.channel = ch
	a.page = "channels"
	a.resetFeedback()
	a.channelOptions = false
	a.channelDetails = false
	a.removeChannelOpen = false
}
func (a *app) disconnect() {
	if a.diagnosing {
		return
	}
	a.work(func() error {
		if err := a.engine.UpdatePreferences(func(p *model.Preferences) { p.AutoConnect = false }); err != nil {
			return err
		}
		a.window.Update(func() {
			if a.monitorCancel != nil {
				a.monitorCancel()
				a.monitorCancel = nil
			}
			a.monitorGeneration++
			a.monitorError = nil
		})
		if err := a.installer.Disconnect(); err != nil {
			return err
		}
		a.window.Update(func() { a.connection = codex.Connection{}; a.connectionError = nil })
		return nil
	}, "Nudilo disconnected.")
}
func (a *app) openData() {
	if a.installer.Paths.Root != "" {
		if err := mygo.Shell.OpenPath(a.installer.Paths.Root); err != nil {
			a.showFeedback(locale.M("Could not open the data folder."), true)
		}
	}
}
func loginError(err error) error {
	if err != nil {
		return errors.New("The system could not enable login startup. Install the app in a permanent location and try again.")
	}
	return nil
}

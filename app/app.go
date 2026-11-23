package app

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/models"
)

var (
	App    *Application
	Styles *Theme
)

type Application struct {
	*tview.Application

	config    *Config
	context   context.Context
	cancelFn  context.CancelFunc
	waitGroup sync.WaitGroup

	// Set before Run; invoked only on the UI event loop.
	OnUpdateRequest func()

	onQuitRequestMu sync.RWMutex
	onQuitRequest   func()

	onThemePickerRequestMu sync.RWMutex
	onThemePickerRequest   func()
	onBorderPickerRequest  func()
	onBorderStyleChange    func(string)
}

func init() {
	ctx, cancel := context.WithCancel(context.Background())

	App = &Application{
		Application: tview.NewApplication(),
		config:      defaultConfig(),
		context:     ctx,
		cancelFn:    cancel,
	}

	App.register()
	App.EnableMouse(true)
	App.EnablePaste(true)
	App.SetAfterDrawFunc(func(screen tcell.Screen) {
		themeRenderer.recolor(screen)
		drawBorders(screen)
	})

	// Start with the default theme; LoadConfig applies the configured one.
	if err := ApplyTheme(ThemeConfig{}); err != nil {
		panic(err)
	}
}

// Context returns the application context.
func (a *Application) Context() context.Context {
	return a.context
}

// Config returns the application configuration.
func (a *Application) Config() *models.AppConfig {
	return a.config.AppConfig
}

// Connections returns the database connections.
func (a *Application) Connections() []models.Connection {
	return a.config.Connections
}

// SaveConnections saves the database connections.
func (a *Application) SaveConnections(connections []models.Connection) error {
	return a.config.SaveConnections(connections)
}

// ThemeConfig returns a copy of the active theme configuration.
func (a *Application) ThemeConfig() ThemeConfig {
	if a.config.Theme == nil {
		return ThemeConfig{}
	}
	cfg := *a.config.Theme
	cfg.Colors = cloneStringMap(cfg.Colors)
	return cfg
}

// SaveThemePreset persists a built-in theme preset to the active config file.
func (a *Application) SaveThemePreset(preset string) error {
	return a.config.SaveThemePreset(preset)
}

// Register adds a task to the wait group and returns a
// function that decrements the task count when called.
//
// The application will not stop until all registered tasks
// have finished by calling the returned function!
func (a *Application) Register() func() {
	a.waitGroup.Add(1)
	return a.waitGroup.Done
}

// Run starts and blocks until the application is stopped.
func (a *Application) Run(root *tview.Pages, configFile string) error {
	a.SetRoot(root, true)
	a.config.ConfigFile = configFile
	return a.Application.Run()
}

// Stop cancels the application context, waits for all
// tasks to finish, and then stops the application.
func (a *Application) Stop() {
	a.cancelFn()
	a.waitGroup.Wait()
	a.Application.Stop()
}

// SetOnQuitRequest sets a callback to be invoked when the user requests to quit
// via OS signal (Ctrl+C) or interrupt.
func (a *Application) SetOnQuitRequest(fn func()) {
	a.onQuitRequestMu.Lock()
	defer a.onQuitRequestMu.Unlock()
	a.onQuitRequest = fn
}

func (a *Application) getOnQuitRequest() func() {
	a.onQuitRequestMu.RLock()
	defer a.onQuitRequestMu.RUnlock()
	return a.onQuitRequest
}

// SetOnThemePickerRequest sets the callback invoked by the global theme shortcut.
func (a *Application) SetOnThemePickerRequest(fn func()) {
	a.onThemePickerRequestMu.Lock()
	defer a.onThemePickerRequestMu.Unlock()
	a.onThemePickerRequest = fn
}

func (a *Application) getOnThemePickerRequest() func() {
	a.onThemePickerRequestMu.RLock()
	defer a.onThemePickerRequestMu.RUnlock()
	return a.onThemePickerRequest
}

func (a *Application) SetOnBorderPickerRequest(fn func())     { a.onBorderPickerRequest = fn }
func (a *Application) SetOnBorderStyleChange(fn func(string)) { a.onBorderStyleChange = fn }

// A user binding in any group takes priority over the default border keys.
func (a *Application) borderShortcutReserved(event *tcell.EventKey, selected commands.Command) bool {
	for group, bindings := range a.config.Keymaps {
		for name, keyName := range bindings {
			if strings.EqualFold(group, GlobalGroup) && name == selected.String() {
				continue
			}
			key, err := parseKeyString(keyName)
			if err == nil && ((event.Key() == tcell.KeyRune && key.Char == event.Rune()) || (event.Key() != tcell.KeyRune && key.Code == event.Key())) {
				return true
			}
		}
	}
	return false
}

// register listens for interrupt and termination signals to
// gracefully handle shutdowns by calling the Stop method.
func (a *Application) register() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	requestQuit := func() {
		if fn := a.getOnQuitRequest(); fn != nil {
			fn()
			return
		}
		a.Stop()
	}

	go func() {
		<-c
		// This runs outside the UI event loop.
		a.QueueUpdateDraw(requestQuit)
		<-c
		os.Exit(1)
	}()

	// Override the default input capture to listen for Ctrl+C
	// and make it send an interrupt signal to the channel to
	// trigger a graceful shutdown instead of closing the app
	// immediately without waiting for tasks to finish.
	a.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			// We're already on the UI event loop here; queuing an update can deadlock.
			requestQuit()
			return nil
		}
		if Keymaps.Resolve(event) == commands.CheckForUpdates {
			if a.OnUpdateRequest != nil {
				a.OnUpdateRequest()
			}
			return nil
		}
		if Keymaps.Resolve(event) == commands.ThemePicker {
			if fn := a.getOnThemePickerRequest(); fn != nil {
				fn()
			}
			return nil
		}
		if action := Keymaps.Resolve(event); action == commands.BorderStyle || action == commands.BorderPicker {
			if a.borderShortcutReserved(event, action) {
				return event
			}
			if action == commands.BorderStyle {
				style := NextBorderStyle(a.BorderConfig().Style)
				if a.onBorderStyleChange != nil {
					a.onBorderStyleChange(style)
				}
			} else if a.onBorderPickerRequest != nil {
				a.onBorderPickerRequest()
			}
			return nil
		}
		return event
	})
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

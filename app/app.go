package app

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

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
}

type Theme struct {
	tview.Theme

	SidebarTitleBorderColor string
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

	Styles = &Theme{
		Theme: tview.Theme{
			PrimitiveBackgroundColor:    tcell.ColorDefault,
			ContrastBackgroundColor:     tcell.ColorBlue,
			MoreContrastBackgroundColor: tcell.ColorGreen,
			BorderColor:                 tcell.ColorWhite,
			TitleColor:                  tcell.ColorWhite,
			GraphicsColor:               tcell.ColorGray,
			PrimaryTextColor:            tcell.ColorDefault.TrueColor(),
			SecondaryTextColor:          tcell.ColorYellow,
			TertiaryTextColor:           tcell.ColorGreen,
			InverseTextColor:            tcell.ColorWhite,
			ContrastSecondaryTextColor:  tcell.ColorBlack,
		},
		SidebarTitleBorderColor: "#666A7E",
	}

	tview.Styles = Styles.Theme
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

// register listens for interrupt and termination signals to
// gracefully handle shutdowns by calling the Stop method.
func (a *Application) register() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		a.Stop()
		<-c
		os.Exit(1)
	}()

	// Override the default input capture to listen for Ctrl+C
	// and make it send an interrupt signal to the channel to
	// trigger a graceful shutdown instead of closing the app
	// immediately without waiting for tasks to finish.
	a.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			c <- os.Interrupt
			return nil
		}
		return event
	})
}

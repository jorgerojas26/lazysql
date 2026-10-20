package aicli

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/internal/ai"
)

func openChatGPTBrowser(parent context.Context, address string) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	name := "xdg-open"
	args := []string{address}
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "windows":
		name = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", address}
	}
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	return command.Run()
}

type authUpdate struct {
	message string
	done    bool
	err     error
}

func (ui *terminalUI) work(title string, work func(context.Context, func(string)) error) error {
	ctx, cancel := context.WithCancel(ui.ctx)
	defer cancel()
	updates := make(chan authUpdate, 8)
	finished := make(chan struct{})
	started := false
	var outcome error
	err := ui.run(func(done func(), application *tview.Application) tview.Primitive {
		view := newDialog(title, "Connecting…", tview.NewBox(), 0, 16)
		view.footer.SetText("Esc Cancel · PgUp/PgDn Scroll · Ctrl+Y Copy link")
		link := ""
		previousCapture := view.GetInputCapture()
		view.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyCtrlY && link != "" {
				if err := clipboard.WriteAll(link); err != nil {
					view.footer.SetText("Could not copy link · PgUp/PgDn to view it")
				} else {
					view.footer.SetText("Link copied · Esc Cancel")
				}
				return nil
			}
			if event.Key() != tcell.KeyF24 {
				return previousCapture(event)
			}
			for {
				select {
				case update := <-updates:
					if update.message != "" {
						view.description.SetText(update.message)
						for _, word := range strings.Fields(update.message) {
							if strings.HasPrefix(word, "https://auth.openai.com/") {
								link = word
								break
							}
						}
					}
					if update.done {
						outcome = update.err
						done()
						return nil
					}
				default:
					return nil
				}
			}
		})
		var once sync.Once
		application.SetAfterDrawFunc(func(tcell.Screen) {
			once.Do(func() {
				started = true
				go func() {
					defer close(finished)
					send := func(update authUpdate) {
						select {
						case updates <- update:
							application.QueueEvent(tcell.NewEventKey(tcell.KeyF24, 0, tcell.ModNone))
						case <-ctx.Done():
						}
					}
					err := work(ctx, func(message string) { send(authUpdate{message: message}) })
					send(authUpdate{done: true, err: err})
				}()
			})
		})
		return view
	})
	cancel()
	if started {
		<-finished
	}
	if err != nil {
		return err
	}
	return outcome
}

func (ui *terminalUI) loginChatGPT(ctx context.Context) error {
	accounts, err := ai.ChatGPTAccounts(ctx)
	if err != nil {
		return err
	}
	clientID := ""
	if len(accounts) > 0 {
		err = ui.run(func(done func(), _ *tview.Application) tview.Primitive {
			choices := make([]choice, 0, len(accounts)+1)
			for _, account := range accounts {
				name := account.Email
				if name == "" {
					name = "ChatGPT account"
				}
				suffix := account.ClientID
				if len(suffix) > 8 {
					suffix = suffix[len(suffix)-8:]
				}
				name += " · " + suffix
				if account.Active {
					name += " · active"
				}
				choices = append(choices, choice{name: name, value: account.ClientID})
			}
			choices = append(choices, choice{name: "Add account", value: ""})
			return choiceDialog("ChatGPT account", "", choices, func(value string) { clientID = value; done() })
		})
		if err != nil {
			return err
		}
	}
	approved := false
	err = ui.run(func(done func(), _ *tview.Application) tview.Primitive {
		return choiceDialog("Use your ChatGPT plan", "", []choice{{name: "Continue with ChatGPT", value: "continue"}}, func(string) { approved = true; done() })
	})
	if err != nil {
		return err
	}
	if !approved {
		return errCancelled
	}
	welcome := false
	err = ui.work("Continue with ChatGPT", func(work context.Context, notify func(string)) error {
		var err error
		welcome, err = ai.LoginChatGPT(work, clientID, func(address string) error {
			notify("Complete sign-in in your browser.\n\nIf it did not open, use:\n" + address)
			if browserErr := openChatGPTBrowser(work, address); browserErr != nil {
				notify("Open this URL in your browser to continue:\n\n" + address)
			}
			return nil
		})
		return err
	})
	if err != nil {
		return err
	}
	if welcome {
		err = ui.run(func(done func(), _ *tview.Application) tview.Primitive {
			return choiceDialog("You're using your ChatGPT plan", "Eligible requests use your ChatGPT plan.\nManage usage: "+ai.ChatGPTUsageURL, []choice{{name: "Got it", value: "ok"}}, func(string) { done() })
		})
		if err != nil {
			return err
		}
		return ai.AcknowledgeChatGPTPlan(ctx)
	}
	return nil
}

func (ui *terminalUI) logoutChatGPT(ctx context.Context) error {
	return ui.work("Sign out of ChatGPT", func(work context.Context, notify func(string)) error {
		notify("Signing out of LazySQL…")
		err := ai.LogoutChatGPT(work)
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	})
}

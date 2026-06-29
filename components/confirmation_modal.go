package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

type ConfirmationModal struct {
	*tview.Modal

	done func(buttonIndex int, buttonLabel string)
}

const (
	confirmationYes = "Yes"
	confirmationNo  = "No"
)

func NewConfirmationModal(confirmationText string) *ConfirmationModal {
	modal := tview.NewModal()
	if confirmationText != "" {
		modal.SetText(confirmationText)
	} else {
		modal.SetText("Are you sure?")
	}
	modal.AddButtons([]string{confirmationYes, confirmationNo})
	modal.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	modal.SetBorderStyle(tcell.StyleDefault.Background(app.Styles.PrimitiveBackgroundColor))
	modal.SetButtonActivatedStyle(tcell.StyleDefault.
		Background(app.Styles.InverseTextColor).
		Foreground(app.Styles.ContrastSecondaryTextColor),
	)
	modal.SetTextColor(app.Styles.PrimaryTextColor)

	return &ConfirmationModal{Modal: modal}
}

// SetDoneFunc sets the done handler and wires y/n shortcuts to it.
func (m *ConfirmationModal) SetDoneFunc(handler func(buttonIndex int, buttonLabel string)) {
	m.done = handler
	m.Modal.SetDoneFunc(handler)

	// Add y/n shortcuts for confirmation dialogs.
	m.Modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() != tcell.KeyRune {
			return event
		}
		switch event.Rune() {
		case 'y', 'Y':
			if m.done != nil {
				m.done(0, confirmationYes)
			}
			return nil
		case 'n', 'N':
			if m.done != nil {
				m.done(1, confirmationNo)
			}
			return nil
		default:
			return event
		}
	})
}

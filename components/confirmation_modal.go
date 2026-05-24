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

func NewConfirmationModal(confirmationText string) *ConfirmationModal {
	modal := tview.NewModal()
	if confirmationText != "" {
		modal.SetText(confirmationText)
	} else {
		modal.SetText("Are you sure?")
	}
	modal.AddButtons([]string{"Yes", "No"})
	modal.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	modal.SetBorderStyle(tcell.StyleDefault.Background(app.Styles.PrimitiveBackgroundColor))
	modal.SetButtonActivatedStyle(tcell.StyleDefault.
		Background(app.Styles.InverseTextColor).
		Foreground(app.Styles.ContrastSecondaryTextColor),
	)
	modal.SetTextColor(app.Styles.PrimaryTextColor)

	cm := &ConfirmationModal{Modal: modal}
	// Add y/n shortcuts for confirmation dialogs.
	cm.Modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() != tcell.KeyRune {
			return event
		}
		switch event.Rune() {
		case 'y', 'Y':
			if cm.done != nil {
				cm.done(0, "Yes")
			}
			return nil
		case 'n', 'N':
			if cm.done != nil {
				cm.done(1, "No")
			}
			return nil
		default:
			return event
		}
	})

	return cm
}

// SetDoneFunc overrides tview.Modal.SetDoneFunc so we can also trigger it
// from keyboard shortcuts (y/n).
func (m *ConfirmationModal) SetDoneFunc(handler func(buttonIndex int, buttonLabel string)) *tview.Modal {
	m.done = handler
	return m.Modal.SetDoneFunc(handler)
}

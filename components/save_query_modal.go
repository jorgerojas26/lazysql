package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/saved_queries"
	"github.com/rivo/tview"
)

// SaveQueryModal is a modal for saving a query with a name.
type SaveQueryModal struct {
	tview.Primitive
	form                 *tview.Form
	grid                 *tview.Grid
	query                string
	onSave               func()
	connectionIdentifier string
}

// NewSaveQueryModal creates a new SaveQueryModal.
func NewSaveQueryModal(connectionIdentifier, query string, onSave func()) *SaveQueryModal {
	sqm := &SaveQueryModal{
		query:                query,
		onSave:               onSave,
		connectionIdentifier: connectionIdentifier,
	}

	sqm.form = tview.NewForm().
		AddInputField("Name", "", 20, nil, nil).
		AddButton("Save", sqm.save).
		AddButton("Cancel", sqm.cancel).SetFieldStyle(
		tcell.StyleDefault.
			Background(app.Styles.SecondaryTextColor).
			Foreground(app.Styles.ContrastSecondaryTextColor),
	).SetButtonActivatedStyle(tcell.StyleDefault.
		Background(app.Styles.SecondaryTextColor).
		Foreground(app.Styles.ContrastSecondaryTextColor),
	).SetButtonStyle(tcell.StyleDefault.
		Background(app.Styles.InverseTextColor).
		Foreground(app.Styles.ContrastSecondaryTextColor),
	)

	sqm.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			sqm.cancel()
			return nil
		}

		if event.Key() == tcell.KeyEnter {
			sqm.save()
			return nil
		}

		return event
	})

	sqm.form.SetBorder(true).SetTitle(" Save Query ").SetTitleAlign(tview.AlignLeft)

	sqm.grid = tview.NewGrid().
		SetRows(0, 7, 0).
		SetColumns(0, 50, 0).
		AddItem(sqm.form, 1, 1, 1, 1, 0, 0, true)

	sqm.Primitive = sqm.grid

	return sqm
}

func (sqm *SaveQueryModal) save() {
	name := sqm.form.GetFormItem(0).(*tview.InputField).GetText()
	if name == "" {
		// TODO: Show an error message
		return
	}

	err := saved_queries.SaveQuery(sqm.connectionIdentifier, name, sqm.query)
	if err != nil {
		// TODO: Show an error message
		return
	}

	if sqm.onSave != nil {
		sqm.onSave()
	}

	mainPages.RemovePage(pageNameSaveQuery)
}

func (sqm *SaveQueryModal) cancel() {
	mainPages.RemovePage(pageNameSaveQuery)
}

// GetPrimitive returns the primitive for this component.
func (sqm *SaveQueryModal) GetPrimitive() tview.Primitive {
	return sqm.Primitive
}

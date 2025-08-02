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
	form       *tview.Form
	query      string
	onSave     func()
	parentFlex *tview.Flex
}

// NewSaveQueryModal creates a new SaveQueryModal.
func NewSaveQueryModal(query string, onSave func()) *SaveQueryModal {
	sqm := &SaveQueryModal{
		query:  query,
		onSave: onSave,
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

	sqm.parentFlex = tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(sqm.form, 0, 1, true).
			AddItem(nil, 0, 1, false),
			0, 8, true).
		AddItem(nil, 0, 1, false)

	sqm.Primitive = sqm.parentFlex

	return sqm
}

func (sqm *SaveQueryModal) save() {
	name := sqm.form.GetFormItem(0).(*tview.InputField).GetText()
	if name == "" {
		// TODO: Show an error message
		return
	}

	err := saved_queries.SaveQuery(name, sqm.query)
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

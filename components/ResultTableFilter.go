package components

import (
	"github.com/jorgerojas26/lazysql/models"

	"github.com/jorgerojas26/lazysql/app"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ResultsTableFilter struct {
	*tview.Flex
	Input         *tview.InputField
	Label         *tview.TextView
	currentFilter string
	subscribers   []chan models.StateChange
	filtering     bool
}

func NewResultsFilter() *ResultsTableFilter {
	recordsFilter := &ResultsTableFilter{
		Flex:  tview.NewFlex(),
		Input: tview.NewInputField(),
		Label: tview.NewTextView(),
	}
	recordsFilter.SetBorder(true)
	recordsFilter.SetDirection(tview.FlexRowCSS)
	recordsFilter.SetTitleAlign(tview.AlignCenter)
	recordsFilter.SetBorderPadding(0, 0, 1, 1)
	recordsFilter.SetBackgroundColor(tcell.ColorDefault)

	recordsFilter.Label.SetTextColor(tcell.ColorOrange)
	recordsFilter.Label.SetText("WHERE")
	recordsFilter.Label.SetBackgroundColor(tcell.ColorDefault)
	recordsFilter.Label.SetBorderPadding(0, 0, 0, 1)

	recordsFilter.Input.SetPlaceholder("Enter a WHERE clause to filter the results")
	recordsFilter.Input.SetPlaceholderStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorDefault))
	recordsFilter.Input.SetFieldBackgroundColor(tcell.ColorDefault)
	recordsFilter.Input.SetFieldTextColor(tcell.ColorWhite.TrueColor())
	recordsFilter.Input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			if recordsFilter.Input.GetText() != "" {
				recordsFilter.currentFilter = "WHERE " + recordsFilter.Input.GetText()
				recordsFilter.Publish("WHERE " + recordsFilter.Input.GetText())

			}
		case tcell.KeyEscape:
			recordsFilter.currentFilter = ""
			recordsFilter.Input.SetText("")
			recordsFilter.Publish("")

		}
	})
	recordsFilter.Input.SetAutocompleteStyles(tcell.ColorBlack, tcell.StyleDefault.Foreground(app.FocusTextColor).Background(tcell.ColorBlack), tcell.StyleDefault.Foreground(app.ActiveTextColor).Background(tcell.ColorBlack))

	recordsFilter.AddItem(recordsFilter.Label, 6, 0, false)
	recordsFilter.AddItem(recordsFilter.Input, 0, 1, false)

	return recordsFilter
}

func (filter *ResultsTableFilter) Subscribe() chan models.StateChange {
	subscriber := make(chan models.StateChange)
	filter.subscribers = append(filter.subscribers, subscriber)
	return subscriber
}

func (filter *ResultsTableFilter) Publish(message string) {
	for _, sub := range filter.subscribers {
		sub <- models.StateChange{
			Key:   "Filter",
			Value: message,
		}
	}
}

func (filter *ResultsTableFilter) GetIsFiltering() bool {
	return filter.filtering
}

func (filter *ResultsTableFilter) GetCurrentFilter() string {
	return filter.currentFilter
}

func (filter *ResultsTableFilter) SetIsFiltering(filtering bool) {
	filter.filtering = filtering
}

// Function to blur
func (filter *ResultsTableFilter) RemoveHighlight() {
	filter.SetBorderColor(app.InactiveTextColor)
	filter.Label.SetTextColor(app.InactiveTextColor)
	filter.Input.SetPlaceholderTextColor(app.InactiveTextColor)
	filter.Input.SetFieldTextColor(app.InactiveTextColor)
}

func (filter *ResultsTableFilter) RemoveLocalHighlight() {
	filter.SetBorderColor(tcell.ColorWhite)
	filter.Label.SetTextColor(tcell.ColorOrange)
	filter.Input.SetPlaceholderTextColor(app.InactiveTextColor)
	filter.Input.SetFieldTextColor(app.InactiveTextColor)
}

func (filter *ResultsTableFilter) Highlight() {
	filter.SetBorderColor(tcell.ColorWhite)
	filter.Label.SetTextColor(tcell.ColorOrange)
	filter.Input.SetPlaceholderTextColor(tcell.ColorWhite)
	filter.Input.SetFieldTextColor(app.FocusTextColor)
}

func (filter *ResultsTableFilter) HighlightLocal() {
	filter.SetBorderColor(app.FocusTextColor)
	filter.Label.SetTextColor(tcell.ColorOrange)
	filter.Input.SetPlaceholderTextColor(tcell.ColorWhite)
	filter.Input.SetFieldTextColor(app.FocusTextColor)
}

package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/models"
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

	recordsFilter.Label.SetTextColor(app.Styles.TertiaryTextColor)
	recordsFilter.Label.SetText("WHERE")
	recordsFilter.Label.SetBorderPadding(0, 0, 0, 1)

	recordsFilter.Input.SetPlaceholder("Enter a WHERE clause to filter the results")
	recordsFilter.Input.SetPlaceholderStyle(tcell.StyleDefault.Foreground(app.Styles.PrimaryTextColor).Background(tview.Styles.PrimitiveBackgroundColor))
	recordsFilter.Input.SetFieldBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	recordsFilter.Input.SetFieldTextColor(app.Styles.PrimaryTextColor)
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
	recordsFilter.Input.SetAutocompleteStyles(app.Styles.PrimitiveBackgroundColor, tcell.StyleDefault.Foreground(tview.Styles.PrimaryTextColor).Background(tview.Styles.PrimitiveBackgroundColor), tcell.StyleDefault.Foreground(tview.Styles.SecondaryTextColor).Background(tview.Styles.PrimitiveBackgroundColor))

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
			Key:   eventResultsTableFiltering,
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
	filter.SetBorderColor(app.Styles.InverseTextColor)
	filter.Label.SetTextColor(app.Styles.InverseTextColor)
	filter.Input.SetPlaceholderTextColor(app.Styles.InverseTextColor)
	filter.Input.SetFieldTextColor(app.Styles.InverseTextColor)
}

func (filter *ResultsTableFilter) RemoveLocalHighlight() {
	filter.SetBorderColor(tcell.ColorWhite)
	filter.Label.SetTextColor(app.Styles.TertiaryTextColor)
	filter.Input.SetPlaceholderTextColor(app.Styles.InverseTextColor)
	filter.Input.SetFieldTextColor(app.Styles.InverseTextColor)
}

func (filter *ResultsTableFilter) Highlight() {
	filter.SetBorderColor(tcell.ColorWhite)
	filter.Label.SetTextColor(app.Styles.TertiaryTextColor)
	filter.Input.SetPlaceholderTextColor(tcell.ColorWhite)
	filter.Input.SetFieldTextColor(app.Styles.PrimaryTextColor)
}

func (filter *ResultsTableFilter) HighlightLocal() {
	filter.SetBorderColor(app.Styles.PrimaryTextColor)
	filter.Label.SetTextColor(app.Styles.TertiaryTextColor)
	filter.Input.SetPlaceholderTextColor(tcell.ColorWhite)
	filter.Input.SetFieldTextColor(app.Styles.PrimaryTextColor)
}

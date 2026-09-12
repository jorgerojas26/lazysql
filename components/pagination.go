package components

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

type PaginationState struct {
	Offset        int
	Limit         int
	TotalRecords  int
	HasNextPage   bool
	pageInfoKnown bool
}

type Pagination struct {
	*tview.Flex
	state    *PaginationState
	textView *tview.TextView
}

func NewPagination() *Pagination {
	wrapper := tview.NewFlex()
	wrapper.SetBorderPadding(0, 0, 1, 1)
	wrapper.SetBorder(true)

	textView := tview.NewTextView()
	textView.SetText(fmt.Sprintf("%s-%s of %s rows", "0", "0", "0"))
	textView.SetTextAlign(tview.AlignCenter)

	wrapper.AddItem(textView, 0, 1, false)

	return &Pagination{
		Flex:     wrapper,
		textView: textView,
		state: &PaginationState{
			Offset:       0,
			Limit:        app.App.Config().DefaultPageSize,
			TotalRecords: 0,
		},
	}
}

func (pagination *Pagination) GetOffset() int {
	return pagination.state.Offset
}

func (pagination *Pagination) GetTotalRecords() int {
	return pagination.state.TotalRecords
}

func (pagination *Pagination) GetLimit() int {
	return pagination.state.Limit
}

func (pagination *Pagination) GetIsFirstPage() bool {
	return pagination.state.Offset == 0
}

func (pagination *Pagination) GetHasNextPage() bool {
	return pagination.state.pageInfoKnown && pagination.state.HasNextPage
}

func (pagination *Pagination) GetIsLastPage() bool {
	if pagination.state.pageInfoKnown {
		return !pagination.state.HasNextPage
	}

	return pagination.state.Offset >= pagination.state.TotalRecords-1 || pagination.state.Offset+pagination.state.Limit >= pagination.state.TotalRecords
}

func (pagination *Pagination) SetPageInfo(visibleRows int, hasNextPage bool) {
	if visibleRows < 0 {
		visibleRows = 0
	}

	pagination.state.pageInfoKnown = true
	pagination.state.HasNextPage = hasNextPage
	if hasNextPage {
		pagination.state.TotalRecords = 0
	} else {
		pagination.state.TotalRecords = pagination.state.Offset + visibleRows
	}

	start := pagination.state.Offset + 1
	end := pagination.state.Offset + visibleRows
	if visibleRows == 0 {
		start = 0
		end = 0
	}

	if hasNextPage {
		pagination.textView.SetText(fmt.Sprintf("%d-%d+ rows", start, end))
		return
	}

	pagination.textView.SetText(fmt.Sprintf("%d-%d of %d rows", start, end, pagination.state.TotalRecords))
}

func (pagination *Pagination) SetTotalRecords(total int) {
	pagination.state.TotalRecords = total
	pagination.state.pageInfoKnown = false
	pagination.state.HasNextPage = false
	pagination.updateTextWithTotal()
}

func (pagination *Pagination) SetLimit(limit int) {
	pagination.state.Limit = limit

	if pagination.state.pageInfoKnown {
		pagination.updatePageText()
		return
	}

	pagination.updateTextWithTotal()
}

func (pagination *Pagination) SetOffset(offset int) {
	pagination.state.Offset = offset

	if pagination.state.pageInfoKnown {
		pagination.updatePageText()
		return
	}

	pagination.updateTextWithTotal()
}

func (pagination *Pagination) updateTextWithTotal() {
	offset := pagination.GetOffset()
	limit := pagination.GetLimit() + offset
	total := pagination.GetTotalRecords()

	if offset < total {
		offset++
	}
	if limit > total {
		limit = total
	}

	pagination.textView.SetText(fmt.Sprintf("%d-%d of %d rows", offset, limit, total))
}

func (pagination *Pagination) updatePageText() {
	start := pagination.state.Offset + 1
	end := pagination.state.Offset + pagination.state.Limit
	if pagination.state.HasNextPage {
		pagination.textView.SetText(fmt.Sprintf("%d-%d+ rows", start, end))
		return
	}

	if pagination.state.TotalRecords == 0 {
		pagination.textView.SetText("0-0 of 0 rows")
		return
	}

	if end > pagination.state.TotalRecords {
		end = pagination.state.TotalRecords
	}
	pagination.textView.SetText(fmt.Sprintf("%d-%d of %d rows", start, end, pagination.state.TotalRecords))
}

func (pagination *Pagination) SetLoading(loading bool) {
	current := pagination.textView.GetText(false)
	if loading {
		if !strings.HasSuffix(current, " [Loading...]") {
			pagination.textView.SetText(current + " [Loading...]").SetTextColor(app.Styles.SecondaryTextColor)
			pagination.SetBorderColor(app.Styles.SecondaryTextColor)
		}
	} else {
		pagination.textView.SetText(strings.TrimSuffix(current, " [Loading...]")).SetTextColor(app.Styles.PrimaryTextColor)
		pagination.SetBorderColor(app.Styles.PrimaryTextColor)
	}
}

func (pagination *Pagination) SetLoadingText(text string) {
	pagination.textView.SetText(text).SetTextColor(app.Styles.SecondaryTextColor)
	pagination.SetBorderColor(app.Styles.SecondaryTextColor)
}

package components

import (
	"fmt"

	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

type PaginationState struct {
	Offset       int
	Limit        int
	TotalRecords int
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

func (pagination *Pagination) GetIsLastPage() bool {
	return pagination.state.Offset >= pagination.state.TotalRecords-1 || pagination.state.Offset+pagination.state.Limit >= pagination.state.TotalRecords
}

func (pagination *Pagination) SetTotalRecords(total int) {
	pagination.state.TotalRecords = total

	offset := pagination.GetOffset()
	if offset < total {
		offset++
	}

	limit := pagination.GetLimit() + offset
	if limit > total {
		limit = total
	}

	pagination.textView.SetText(fmt.Sprintf("%d-%d of %d rows", offset, limit, total))
}

func (pagination *Pagination) SetLimit(limit int) {
	pagination.state.Limit = limit

	offset := pagination.GetOffset()
	total := pagination.GetTotalRecords()

	if limit > total {
		limit = total
	}

	pagination.textView.SetText(fmt.Sprintf("%d-%d of %d rows", offset+1, limit, total))
}

func (pagination *Pagination) SetOffset(offset int) {
	pagination.state.Offset = offset

	limit := pagination.GetLimit() + offset
	total := pagination.GetTotalRecords()

	if limit > total {
		limit = total
	}

	pagination.textView.SetText(fmt.Sprintf("%d-%d of %d rows", offset+1, limit, total))
}

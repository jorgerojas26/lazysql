package components

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

type CountState uint8

const (
	CountUnknown CountState = iota
	CountEstimated
	CountExact
	CountCounting
	CountFailed
)

type PaginationState struct {
	Offset         int
	Limit          int
	TotalRecords   int
	HasNextPage    bool
	pageInfoKnown  bool
	visibleRows    int
	ExactTotal     *int64
	EstimatedTotal *int64
	countState     CountState
	countError     string
	loading        bool
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
	textView.SetText("0-0+ rows [# exact]")
	textView.SetTextAlign(tview.AlignCenter)

	wrapper.AddItem(textView, 0, 1, false)

	return &Pagination{
		Flex:     wrapper,
		textView: textView,
		state: &PaginationState{
			Offset:       0,
			Limit:        app.App.Config().DefaultPageSize,
			TotalRecords: 0,
			countState:   CountUnknown,
		},
	}
}

func (pagination *Pagination) GetOffset() int {
	return pagination.state.Offset
}

// GetTotalRecords returns the exact total when one is known. Unknown and
// estimated totals intentionally return zero for compatibility with callers
// that used the old exact-count-only API.
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
	if !pagination.state.pageInfoKnown {
		return false
	}
	return !pagination.state.HasNextPage
}

func (pagination *Pagination) SetPageInfo(visibleRows int, hasNextPage bool) {
	if visibleRows < 0 {
		visibleRows = 0
	}

	pagination.state.pageInfoKnown = true
	pagination.state.HasNextPage = hasNextPage
	pagination.state.visibleRows = visibleRows
	if !hasNextPage {
		pagination.setExactTotal(int64(max(pagination.state.Offset, 0) + visibleRows))
	}

	pagination.render()
}

func (pagination *Pagination) SetTotalRecords(total int) {
	if total < 0 {
		total = 0
	}
	pagination.state.pageInfoKnown = true
	pagination.state.HasNextPage = false
	pagination.state.visibleRows = total
	pagination.SetExactTotal(int64(total))
}

func (pagination *Pagination) SetExactTotal(total int64) {
	if total < 0 {
		total = 0
	}
	pagination.setExactTotal(total)
	pagination.render()
}

func (pagination *Pagination) setExactTotal(total int64) {
	pagination.state.ExactTotal = new(int64)
	*pagination.state.ExactTotal = total
	pagination.state.EstimatedTotal = nil
	pagination.state.TotalRecords = intFromInt64(total)
	pagination.state.countState = CountExact
	pagination.state.countError = ""
}

func (pagination *Pagination) SetEstimatedTotal(total int64) {
	if total < 0 {
		return
	}
	pagination.state.ExactTotal = nil
	pagination.state.EstimatedTotal = new(int64)
	*pagination.state.EstimatedTotal = total
	pagination.state.TotalRecords = 0
	pagination.state.countState = CountEstimated
	pagination.state.countError = ""
	pagination.render()
}

func (pagination *Pagination) ClearCount() {
	pagination.state.ExactTotal = nil
	pagination.state.EstimatedTotal = nil
	pagination.state.TotalRecords = 0
	pagination.state.countState = CountUnknown
	pagination.state.countError = ""
	pagination.render()
}

func (pagination *Pagination) SetCounting(counting bool) {
	if counting {
		pagination.state.countState = CountCounting
		pagination.state.countError = ""
	} else {
		pagination.restoreCountState()
	}
	pagination.render()
}

// SetCountLoading is an explicit alias for callers that describe the same
// state as a loading count rather than an active count.
func (pagination *Pagination) SetCountLoading(loading bool) {
	pagination.SetCounting(loading)
}

func (pagination *Pagination) SetCountError(err error) {
	if err == nil {
		pagination.restoreCountState()
	} else {
		pagination.state.countState = CountFailed
		pagination.state.countError = err.Error()
	}
	pagination.render()
}

func (pagination *Pagination) ClearCountActivity() {
	pagination.restoreCountState()
	pagination.render()
}

func (pagination *Pagination) restoreCountState() {
	switch {
	case pagination.state.ExactTotal != nil:
		pagination.state.countState = CountExact
	case pagination.state.EstimatedTotal != nil:
		pagination.state.countState = CountEstimated
	default:
		pagination.state.countState = CountUnknown
	}
	pagination.state.countError = ""
}

func (pagination *Pagination) GetExactTotal() (int64, bool) {
	if pagination.state.ExactTotal == nil {
		return 0, false
	}
	return *pagination.state.ExactTotal, true
}

func (pagination *Pagination) GetEstimatedTotal() (int64, bool) {
	if pagination.state.EstimatedTotal == nil {
		return 0, false
	}
	return *pagination.state.EstimatedTotal, true
}

func (pagination *Pagination) HasExactTotal() bool {
	return pagination.state.ExactTotal != nil
}

func (pagination *Pagination) GetCountState() CountState {
	return pagination.state.countState
}

func (pagination *Pagination) GetCountError() string {
	return pagination.state.countError
}

func (pagination *Pagination) GetText() string {
	return pagination.textView.GetText(false)
}

func (pagination *Pagination) SetLimit(limit int) {
	pagination.state.Limit = limit
	pagination.render()
}

func (pagination *Pagination) SetOffset(offset int) {
	if offset < 0 {
		offset = 0
	}
	pagination.state.Offset = offset
	pagination.render()
}

func (pagination *Pagination) SetLoading(loading bool) {
	pagination.state.loading = loading
	pagination.render()
}

func (pagination *Pagination) SetLoadingText(text string) {
	pagination.state.loading = false
	pagination.textView.SetText(text).SetTextColor(app.Styles.SecondaryTextColor)
	pagination.SetBorderColor(app.Styles.SecondaryTextColor)
}

func (pagination *Pagination) render() {
	text := pagination.baseText()

	switch pagination.state.countState {
	case CountCounting:
		text += " [Counting…] [# cancel]"
	case CountFailed:
		text += fmt.Sprintf(" [Count failed: %s] [# retry]", pagination.state.countError)
	case CountUnknown, CountEstimated:
		text += " [# exact]"
	}

	if pagination.state.loading {
		text += " [Loading...]"
	}

	color := app.Styles.PrimaryTextColor
	if pagination.state.loading || pagination.state.countState == CountCounting {
		color = app.Styles.SecondaryTextColor
	}
	pagination.textView.SetText(text).SetTextColor(color)
	pagination.SetBorderColor(color)
}

func (pagination *Pagination) baseText() string {
	offset := max(pagination.state.Offset, 0)
	visibleRows := pagination.state.visibleRows

	start, end := 0, 0
	if visibleRows > 0 {
		start = offset + 1
		end = offset + visibleRows
	}

	if pagination.state.ExactTotal != nil {
		total := *pagination.state.ExactTotal
		if int64(end) > total {
			end = intFromInt64(total)
		}
		if total == 0 {
			return "0-0 of 0 rows"
		}
		return fmt.Sprintf("%d-%d of %d rows", start, end, total)
	}
	if pagination.state.EstimatedTotal != nil {
		return fmt.Sprintf("%d-%d of ~%s rows", start, end, formatApproximateCount(*pagination.state.EstimatedTotal))
	}

	return fmt.Sprintf("%d-%d+ rows", start, end)
}

func formatApproximateCount(count int64) string {
	if count < 1000 {
		return strconv.FormatInt(count, 10)
	}

	units := []string{"K", "M", "B", "T"}
	value := float64(count)
	unit := 0
	for value >= 1000 && unit < len(units) {
		value /= 1000
		unit++
	}

	formatted := strconv.FormatFloat(value, 'f', 1, 64)
	formatted = strings.TrimSuffix(strings.TrimSuffix(formatted, "0"), ".")
	return formatted + units[unit-1]
}

func intFromInt64(value int64) int {
	maxInt := int64(^uint(0) >> 1)
	minInt := -maxInt - 1
	if value > maxInt {
		return int(maxInt)
	}
	if value < minInt {
		return int(minInt)
	}
	return int(value)
}

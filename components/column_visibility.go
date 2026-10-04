package components

import (
	"fmt"
	"slices"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
)

type visibleColumnContent struct {
	tview.TableContentReadOnly
	cells   *tview.Table
	columns []int
}

func (content *visibleColumnContent) GetCell(row, column int) *tview.TableCell {
	if row < 0 || row >= content.GetRowCount() || column < 0 || column >= content.GetColumnCount() {
		return nil
	}
	if content.columns != nil {
		column = content.columns[column]
	}
	return content.cells.GetCell(row, column)
}

func (content *visibleColumnContent) GetRowCount() int {
	return content.cells.GetRowCount()
}

func (content *visibleColumnContent) GetColumnCount() int {
	if content.columns != nil {
		return len(content.columns)
	}
	return content.cells.GetColumnCount()
}

func (content *visibleColumnContent) SetCell(row, column int, cell *tview.TableCell) {
	content.cells.SetCell(row, column, cell)
}

func (content *visibleColumnContent) Clear() {
	content.cells.Clear()
}

func (content *visibleColumnContent) RemoveRow(row int) {
	content.cells.RemoveRow(row)
}

func (content *visibleColumnContent) InsertRow(row int) {
	content.cells.InsertRow(row)
}

func (table *ResultsTable) initColumnVisibility() {
	if table.columnView != nil {
		return
	}
	cells := tview.NewTable()
	for row := 0; row < table.GetRowCount(); row++ {
		for column := 0; column < table.GetColumnCount(); column++ {
			cells.SetCell(row, column, table.GetCell(row, column))
		}
	}
	table.columnView = &visibleColumnContent{cells: cells}
	table.Table.SetContent(table.columnView)
	if table.Pagination != nil {
		table.Pagination.SetTitleAlign(tview.AlignRight)
		table.Pagination.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
			if action == tview.MouseLeftClick && table.Pagination.GetTitle() != "" {
				x, y, width, _ := table.Pagination.GetRect()
				mouseX, mouseY := event.Position()
				start := x + width - 1 - tview.TaggedStringWidth(table.Pagination.GetTitle())
				if mouseY == y && mouseX >= max(x+1, start) && mouseX < x+width-1 {
					table.showColumnVisibility()
					return action, nil
				}
			}
			return action, event
		})
	}
}

func (table *ResultsTable) GetCell(row, column int) *tview.TableCell {
	if table.columnView != nil {
		return table.columnView.cells.GetCell(row, column)
	}
	return table.Table.GetCell(row, column)
}

func (table *ResultsTable) GetColumnCount() int {
	if table.columnView != nil {
		return table.columnView.cells.GetColumnCount()
	}
	return table.Table.GetColumnCount()
}

func (table *ResultsTable) GetSelection() (row, column int) {
	row, column = table.Table.GetSelection()
	if table.columnView != nil && len(table.columnView.columns) > 0 {
		column = table.columnView.columns[min(max(column, 0), len(table.columnView.columns)-1)]
	}
	return
}

func (table *ResultsTable) Select(row, column int) *tview.Table {
	if table.columnView != nil && len(table.columnView.columns) > 0 {
		index, _ := slices.BinarySearch(table.columnView.columns, column)
		column = min(index, len(table.columnView.columns)-1)
	}
	return table.Table.Select(row, column)
}

func (table *ResultsTable) adjacentVisibleColumn(column, direction int) int {
	if table.columnView != nil && len(table.columnView.columns) > 0 {
		index, _ := slices.BinarySearch(table.columnView.columns, column)
		index += direction
		if index < 0 || index >= len(table.columnView.columns) {
			return -1
		}
		return table.columnView.columns[index]
	}
	column += direction
	if column < 0 || column >= table.GetColumnCount() {
		return -1
	}
	return column
}

func (table *ResultsTable) canChooseColumns() bool {
	return table.Editor == nil && table.Menu != nil && table.Menu.GetSelectedOption() == 1 &&
		table.GetTableName() != "" && len(table.GetRecords()) > 0 && len(table.GetRecords()[0]) > 0
}

func (table *ResultsTable) refreshColumnVisibility() {
	table.initColumnVisibility()
	row, column := table.GetSelection()
	table.columnView.columns = nil
	if table.Pagination != nil {
		table.Pagination.SetTitle("")
	}
	if !table.canChooseColumns() {
		return
	}

	headers := table.GetRecords()[0]
	hidden := app.App.HiddenColumns(table.connectionIdentifier, table.GetDatabaseName(), table.GetTableName())
	visible := make([]int, 0, len(headers))
	for index, name := range headers {
		if !slices.Contains(hidden, name) {
			visible = append(visible, index)
		}
	}
	if len(visible) == 0 {
		visible = append(visible, 0)
	}
	table.columnView.columns = visible
	table.Select(row, column)

	if table.Pagination != nil {
		shortcut := "V"
		for _, bind := range app.Keymaps.Group(app.TableGroup) {
			if bind.Cmd == commands.ColumnVisibility {
				shortcut = bind.Key.String()
				break
			}
		}
		title := fmt.Sprintf(" Columns [%s] ", shortcut)
		if count := len(headers) - len(visible); count > 0 {
			title = fmt.Sprintf(" Columns [%s] · %d hidden ", shortcut, count)
		}
		table.Pagination.SetTitle(tview.Escape(title))
	}
}

func (table *ResultsTable) showColumnVisibility() {
	if !table.canChooseColumns() || table.GetIsEditing() || table.GetIsFiltering() {
		return
	}

	const page = "column-visibility"
	previousFocus := app.App.GetFocus()
	closeModal := func() {
		mainPages.RemovePage(page)
		if previousFocus != nil {
			app.App.SetFocus(previousFocus)
		} else {
			app.App.SetFocus(table)
		}
	}
	picker := NewColumnVisibilityModal(table.GetRecords()[0],
		app.App.HiddenColumns(table.connectionIdentifier, table.GetDatabaseName(), table.GetTableName()),
		func(hidden []string) error {
			if err := app.App.SaveHiddenColumns(table.connectionIdentifier, table.GetDatabaseName(), table.GetTableName(), hidden); err != nil {
				return err
			}
			table.refreshColumnVisibility()
			if table.Home != nil && table.Home.TabbedPane != nil {
				for tab := table.Home.TabbedPane.state.FirstTab; tab != nil; tab = tab.NextTab {
					other, ok := tab.Content.(*ResultsTable)
					if ok && other != table && other.GetDatabaseName() == table.GetDatabaseName() && other.GetTableName() == table.GetTableName() {
						other.refreshColumnVisibility()
					}
				}
			}
			closeModal()
			return nil
		}, closeModal)
	picker.tableName = table.GetTableName()
	picker.updateStatus()
	mainPages.AddPage(page, picker, true, true)
	app.App.SetFocus(picker.list)
}

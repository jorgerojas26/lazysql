package components

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/internal/history"
	"github.com/jorgerojas26/lazysql/lib"
	"github.com/jorgerojas26/lazysql/models"
)

type ResultsTableState struct {
	listOfDBChanges           *[]models.DBDMLChange
	error                     string
	currentSort               string
	databaseName              string
	tableName                 string
	primaryKeyColumnNames     []string
	columns                   [][]string
	constraints               [][]string
	foreignKeys               [][]string
	indexes                   [][]string
	records                   [][]string
	foreignKeyColumns         map[string]bool
	foreignKeyJumpTargets     map[string]foreignKeyJumpTarget
	fkRawCellValues           map[string]string
	queryStatus               string
	lastEditorQuery           string
	lastEditorQueryReplaySafe bool
	editorResultAvailable     bool
	markedRows                map[int]bool
	isEditing                 bool
	isFiltering               bool
	isLoading                 bool
	showSidebar               bool
	loadingCancel             context.CancelFunc
	loadingMu                 sync.Mutex
	loadGeneration            uint64
	metadataStates            map[MetadataKind]MetadataState
	metadataErrors            map[MetadataKind]error
	metadataMu                sync.RWMutex
}

type foreignKeyJumpTarget struct {
	ReferencedTable  string
	ReferencedColumn string
}

type editorQueryRun struct {
	generation            uint64
	cancel                context.CancelFunc
	cancelRequested       bool
	started               time.Time
	firstUsefulResultOnce sync.Once
}

type csvExportRun struct {
	ctx        context.Context
	cancel     context.CancelFunc
	generation uint64
	finalPath  string

	mu           sync.Mutex
	rowsWritten  int
	wasCancelled bool
}

func (run *csvExportRun) setRowsWritten(rows int) {
	run.mu.Lock()
	if rows > run.rowsWritten {
		run.rowsWritten = rows
	}
	run.mu.Unlock()
}

func (run *csvExportRun) rows() int {
	run.mu.Lock()
	defer run.mu.Unlock()
	return run.rowsWritten
}

func (run *csvExportRun) cancelAndGetRows() int {
	run.mu.Lock()
	run.wasCancelled = true
	rows := run.rowsWritten
	run.mu.Unlock()
	return rows
}

func (run *csvExportRun) cancelled() bool {
	run.mu.Lock()
	defer run.mu.Unlock()
	return run.wasCancelled
}

type ResultsTable struct {
	*tview.Table
	state                  *ResultsTableState
	Page                   *tview.Pages
	Wrapper                *tview.Flex
	Menu                   *ResultsTableMenu
	Filter                 *ResultsTableFilter
	Error                  *tview.Modal
	jsonViewer             *JSONViewer
	Pagination             *Pagination
	Editor                 *SQLEditor
	EditorPages            *tview.Pages
	ResultsInfo            *tview.TextView
	Tree                   *Tree
	Sidebar                *Sidebar
	SidebarContainer       *tview.Flex
	DBDriver               drivers.Driver
	Home                   *Home
	connectionIdentifier   string
	ConnectionURL          string
	ReadOnly               bool
	metadataCache          *metadataCache
	metadataCacheMu        sync.Mutex
	schemaLoader           *schemaLoader
	schemaLoaderMu         sync.Mutex
	editorSchemaMu         sync.RWMutex
	editorSchemaDatabase   string
	editorSchemaGeneration uint64
	editorSchemaTables     map[string]editorSchemaTable
	// Metadata consumers survive load generations but not table identity changes.
	metadataIdentityMu         sync.RWMutex
	metadataIdentityGeneration uint64
	// Serialize identity transitions with metadata mutations after validation.
	metadataApplyMu sync.Mutex
	countMu         sync.Mutex
	countCancel     context.CancelFunc
	countGeneration uint64
	countKey        rowCountKey
	countKeySet     bool
	countAttempted  bool
	countManual     bool
	queryMu         sync.Mutex
	activeQuery     *editorQueryRun
	exportMu        sync.Mutex
	activeExport    *csvExportRun
}

func NewResultsTable(listOfDBChanges *[]models.DBDMLChange, tree *Tree, dbdriver drivers.Driver, home *Home, connectionIdentifier string, connectionURL string, readOnly bool) *ResultsTable {
	state := &ResultsTableState{
		records:               [][]string{},
		columns:               [][]string{},
		constraints:           [][]string{},
		foreignKeys:           [][]string{},
		indexes:               [][]string{},
		foreignKeyColumns:     map[string]bool{},
		foreignKeyJumpTargets: map[string]foreignKeyJumpTarget{},
		fkRawCellValues:       map[string]string{},
		markedRows:            map[int]bool{},
		metadataStates:        newMetadataStates(),
		metadataErrors:        map[MetadataKind]error{},
		isEditing:             false,
		isLoading:             false,
		listOfDBChanges:       listOfDBChanges,
		showSidebar:           false,
	}

	wrapper := tview.NewFlex()
	wrapper.SetDirection(tview.FlexColumnCSS)

	errorModal := tview.NewModal()
	errorModal.AddButtons([]string{"Ok"})
	errorModal.SetText("An error occurred")
	errorModal.SetBackgroundColor(tcell.ColorRed)
	errorModal.SetTextColor(app.Styles.PrimaryTextColor)
	errorModal.SetButtonStyle(tcell.StyleDefault.Foreground(app.Styles.PrimaryTextColor))
	errorModal.SetFocus(0)

	pages := tview.NewPages()
	pages.AddPage(pageNameTable, wrapper, true, true)
	pages.AddPage(pageNameTableError, errorModal, true, false)

	pagination := NewPagination()

	sidebar := NewSidebar(dbdriver.GetProvider(), readOnly)

	table := &ResultsTable{
		Table:      tview.NewTable(),
		state:      state,
		Page:       pages,
		Wrapper:    wrapper,
		Error:      errorModal,
		Pagination: pagination,
		Editor:     nil,
		Tree:       tree,
		DBDriver:   dbdriver,
		Home:       home,
		Sidebar:    sidebar,
		// SidebarContainer is only used when AppConfig.SidebarOverlay is false.
		SidebarContainer:     tview.NewFlex(),
		connectionIdentifier: connectionIdentifier,
		ConnectionURL:        connectionURL,
		ReadOnly:             readOnly,
		metadataCache:        metadataCacheForHome(home),
	}

	table.jsonViewer = NewJSONViewer(pages)

	// When AppConfig.SidebarOverlay is true, the sidebar is added as a page to the table.Page.
	// When AppConfig.SidebarOverlay is false, the sidebar is added to the table.SidebarContainer.
	table.Page.AddPage(pageNameSidebar, table.Sidebar, false, false)

	table.SetSelectable(true, true)
	table.SetBorders(true)
	table.SetFixed(1, 0)
	table.SetInputCapture(table.tableInputCapture)
	table.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))

	table.SetSelectionChangedFunc(func(_, _ int) {
		if table.GetShowSidebar() {
			go table.UpdateSidebar()
		}
	})

	go table.subscribeToTreeChanges()
	go table.subscribeToSidebarChanges()

	return table
}

func (table *ResultsTable) WithFilter() *ResultsTable {
	menu := NewResultsTableMenu()
	filter := NewResultsFilter()

	table.Menu = menu
	table.Filter = filter

	if App.Config().SidebarOverlay {
		table.Wrapper.AddItem(menu, 3, 0, false)
		table.Wrapper.AddItem(filter, 3, 0, false)
		table.Wrapper.AddItem(table, 0, 1, true)
		table.Wrapper.AddItem(table.Pagination, 3, 0, false)
	} else {
		tableContainer := tview.NewFlex().SetDirection(tview.FlexColumnCSS)
		tableContainer.AddItem(menu, 3, 0, false)
		tableContainer.AddItem(filter, 3, 0, false)
		tableContainer.AddItem(table, 0, 1, true)
		tableContainer.AddItem(table.Pagination, 3, 0, false)
		tableContainer.SetBorder(true)

		if table.ReadOnly {
			tableContainer.SetTitle(" [READ-ONLY] ")
			tableContainer.SetTitleColor(tcell.ColorLightBlue)
		}

		table.SidebarContainer.AddItem(tableContainer, 0, 4, true)

		table.Wrapper.AddItem(table.SidebarContainer, 0, 1, true)
	}

	go table.subscribeToFilterChanges()

	return table
}

func (table *ResultsTable) WithEditor() *ResultsTable {
	editor := NewSQLEditor(table.ConnectionURL)
	editorPages := tview.NewPages()

	editor.SetFocusFunc(func() {
		table.SetIsEditing(true)
	})

	editor.SetBlurFunc(func() {
		table.SetIsEditing(false)
	})

	table.Editor = editor
	editor.SetQueryCancelFunc(table.CancelActiveQuery)
	editor.SetColumnCompletionLoader(table.requestEditorColumns)

	table.Wrapper.Clear()

	table.Wrapper.AddItem(editor, 12, 0, true)
	table.SetBorder(true)

	tableWrapper := tview.NewFlex().SetDirection(tview.FlexColumnCSS)
	tableWrapper.AddItem(table, 0, 1, false)
	tableWrapper.AddItem(table.Pagination, 3, 0, false)

	resultsInfoWrapper := tview.NewFlex().SetDirection(tview.FlexColumnCSS)
	resultsInfoText := tview.NewTextView()
	resultsInfoText.SetBorder(true)
	resultsInfoText.SetBorderColor(app.Styles.PrimaryTextColor)
	resultsInfoText.SetTextColor(app.Styles.PrimaryTextColor)
	resultsInfoWrapper.AddItem(resultsInfoText, 3, 0, false)

	editorPages.AddPage(pageNameTableEditorTable, tableWrapper, true, false)
	editorPages.AddPage(pageNameTableEditorResultsInfo, resultsInfoWrapper, true, true)

	table.EditorPages = editorPages
	table.ResultsInfo = resultsInfoText

	table.Wrapper.AddItem(editorPages, 0, 1, true)

	go table.subscribeToEditorChanges()

	return table
}

// loadEditorSchema publishes visible table names first, then lets the shared
// schema loader progressively enrich autocomplete with columns.
func (table *ResultsTable) loadEditorSchema() {
	ctx := app.App.Context()
	dbName := table.GetDatabaseName()
	if dbName == "" || table.DBDriver == nil {
		return
	}

	loader := table.schemaMetadataLoader()
	tablesMap, err := loader.loadTables(ctx, dbName)
	if err != nil {
		logger.Error("Failed to load tables for editor autocomplete", map[string]any{"error": err.Error()})
		return
	}

	schemas := []string(nil)
	if table.Tree != nil {
		schemas = table.Tree.Schemas
	}
	tableList := loader.visibleTables(dbName, tablesMap, schemas)
	generation := table.setEditorSchemaTables(dbName, tableList)
	allTables := make([]string, 0, len(tableList))
	seenTables := make(map[string]struct{}, len(tableList))
	for _, schemaTable := range tableList {
		if _, ok := seenTables[schemaTable.bareName]; ok {
			continue
		}
		seenTables[schemaTable.bareName] = struct{}{}
		allTables = append(allTables, schemaTable.bareName)
	}

	// This update is intentionally queued before any column work. The editor is
	// useful as soon as the table catalog is available, even for a large schema.
	app.App.QueueUpdateDraw(func() {
		if table.editorSchemaIsCurrent(dbName, generation) && table.Editor != nil {
			table.Editor.SetTables(allTables)
		}
	})

	go loader.preloadEditorColumns(ctx, dbName, tableList, schemaBulkLoadThreshold(), func(schemaTable editorSchemaTable, columnNames []string) {
		table.publishEditorColumns(dbName, generation, schemaTable, columnNames)
	})
}

func (table *ResultsTable) schemaMetadataLoader() *schemaLoader {
	if table.Home != nil && table.Home.schemaLoader != nil {
		return table.Home.schemaLoader
	}
	if table.Tree != nil && table.Tree.schemaLoader != nil {
		return table.Tree.schemaLoader
	}

	table.schemaLoaderMu.Lock()
	defer table.schemaLoaderMu.Unlock()
	if table.schemaLoader == nil {
		table.schemaLoader = newSchemaLoader(table.DBDriver, table.metadataCacheForTable())
	}
	return table.schemaLoader
}

func (table *ResultsTable) setEditorSchemaTables(database string, tables []editorSchemaTable) uint64 {
	table.editorSchemaMu.Lock()
	defer table.editorSchemaMu.Unlock()

	table.editorSchemaGeneration++
	table.editorSchemaDatabase = database
	table.editorSchemaTables = make(map[string]editorSchemaTable, len(tables)*2)
	for _, schemaTable := range tables {
		for _, name := range []string{schemaTable.bareName, schemaTable.qualifiedName} {
			key := strings.ToLower(name)
			if _, exists := table.editorSchemaTables[key]; !exists {
				table.editorSchemaTables[key] = schemaTable
			}
		}
	}
	return table.editorSchemaGeneration
}

func (table *ResultsTable) editorSchemaTableForHint(hint string) (editorSchemaTable, string, uint64, bool) {
	table.editorSchemaMu.RLock()
	defer table.editorSchemaMu.RUnlock()

	schemaTable, ok := table.editorSchemaTables[strings.ToLower(strings.TrimSpace(hint))]
	currentDatabase := ""
	if table.state != nil {
		currentDatabase = table.GetDatabaseName()
	}
	if !ok || (currentDatabase != "" && currentDatabase != table.editorSchemaDatabase) {
		return editorSchemaTable{}, "", table.editorSchemaGeneration, false
	}
	return schemaTable, table.editorSchemaDatabase, table.editorSchemaGeneration, true
}

func (table *ResultsTable) editorSchemaIsCurrent(database string, generation uint64) bool {
	table.editorSchemaMu.RLock()
	defer table.editorSchemaMu.RUnlock()
	if table.editorSchemaDatabase != database || table.editorSchemaGeneration != generation {
		return false
	}
	if table.state == nil {
		return true
	}
	currentDatabase := table.GetDatabaseName()
	return currentDatabase == "" || currentDatabase == database
}

func (table *ResultsTable) requestEditorColumns(hint string) {
	schemaTable, database, generation, ok := table.editorSchemaTableForHint(hint)
	if !ok || database == "" || table.DBDriver == nil {
		return
	}

	loader := table.schemaMetadataLoader()
	key, done := loader.requestColumns(app.App.Context(), database, schemaTable.qualifiedName)
	if done == nil {
		status, value, err := loader.cache.result(key)
		if err == nil && status == MetadataReady {
			// The completion callback runs on the editor's UI event loop. Apply
			// a ready cache hit directly rather than queueing onto that same loop.
			table.applyEditorColumns(database, generation, schemaTable, editorColumnNames(value))
		}
		return
	}

	go func() {
		<-done
		status, value, err := loader.cache.result(key)
		if err == nil && status == MetadataReady {
			table.publishEditorColumns(database, generation, schemaTable, editorColumnNames(value))
		}
	}()
}

func (table *ResultsTable) publishEditorColumns(database string, generation uint64, schemaTable editorSchemaTable, columnNames []string) {
	app.App.QueueUpdateDraw(func() {
		table.applyEditorColumns(database, generation, schemaTable, columnNames)
	})
}

func (table *ResultsTable) applyEditorColumns(database string, generation uint64, schemaTable editorSchemaTable, columnNames []string) {
	if !table.editorSchemaIsCurrent(database, generation) || table.Editor == nil {
		return
	}
	table.Editor.SetColumns(schemaTable.bareName, columnNames)
	if schemaTable.qualifiedName != schemaTable.bareName {
		table.Editor.SetColumns(schemaTable.qualifiedName, columnNames)
	}
}

func schemaBulkLoadThreshold() int {
	if App == nil || App.Config() == nil {
		return 0
	}
	return App.Config().SchemaBulkLoadThreshold
}

func (table *ResultsTable) subscribeToTreeChanges() {
	ch := table.Tree.Subscribe()

	for stateChange := range ch {
		if stateChange.Key == eventTreeSelectedDatabase {
			table.SetDatabaseName(stateChange.Value.(string))
		}
	}
}

func (table *ResultsTable) subscribeToSidebarChanges() {
	ch := table.Sidebar.Subscribe()

	for stateChange := range ch {
		switch stateChange.Key {
		case eventSidebarEditing:
			editing := stateChange.Value.(bool)
			table.SetIsEditing(editing)
		case eventSidebarUnfocusing:
			App.QueueUpdateDraw(func() {
				App.SetFocus(table)
			})
		case eventSidebarToggling:
			App.QueueUpdateDraw(func() {
				table.ShowSidebar(false)
			})
		case eventSidebarCommitEditing:
			params := stateChange.Value.(models.SidebarEditingCommitParams)

			App.QueueUpdateDraw(func() {
				table.SetInputCapture(table.tableInputCapture)
				table.SetIsEditing(false)

				row, _ := table.GetSelection()
				changedColumnIndex := table.GetColumnIndexByName(params.ColumnName)
				tableCell := table.GetCell(row, changedColumnIndex)

				tableCell.SetText(params.NewValue)

				cellValue := models.CellValue{
					Type:             params.Type,
					Column:           params.ColumnName,
					Value:            params.NewValue,
					TableColumnIndex: changedColumnIndex,
					TableRowIndex:    row,
				}

				logger.Info("eventSidebarCommitEditing", map[string]any{"cellValue": cellValue, "params": params, "rowIndex": row, "changedColumnIndex": changedColumnIndex})
				err := table.AppendNewChange(models.DMLUpdateType, row, changedColumnIndex, cellValue)
				if err != nil {
					table.SetError(err.Error(), nil)
				}
			})
		case eventSidebarError:
			errorMessage := stateChange.Value.(string)
			App.QueueUpdateDraw(func() {
				table.SetError(errorMessage, nil)
			})
		}
	}
}

func (table *ResultsTable) AddRows(rows [][]string) {
	for i, row := range rows {
		for j, cell := range row {
			tableCell := tview.NewTableCell(cell)
			tableCell.SetTextColor(app.Styles.PrimaryTextColor)

			if cell == "EMPTY&" || cell == "NULL&" || cell == "DEFAULT&" {
				tableCell.SetText(strings.Replace(cell, "&", "", 1))
				tableCell.SetStyle(table.GetItalicStyle())
				tableCell.SetReference(cell)
			}

			tableCell.SetSelectable(i > 0)
			tableCell.SetExpansion(1)

			if i == 0 && table.shouldShowForeignKeyHeaderMarker(j) {
				tableCell.SetStyle(tcell.StyleDefault.Underline(true))
			}

			if i > 0 && table.shouldShowForeignKeyMarker(i, j, cell) {
				tableCell.SetStyle(tcell.StyleDefault.Underline(true))
			}

			table.SetCell(i, j, tableCell)
		}
	}
}

func (table *ResultsTable) AddInsertedRows() {
	inserts := make([]models.DBDMLChange, 0)

	for _, change := range *table.state.listOfDBChanges {
		if change.Type == models.DMLInsertType {
			inserts = append(inserts, change)
		}
	}

	rows := make([][]models.CellValue, len(inserts))

	if len(inserts) > 0 {
		for i, insert := range inserts {
			if insert.Table == table.GetTableName() {
				rows[i] = insert.Values
			}
		}
	}

	rowCount := table.GetRowCount()
	for i, row := range rows {
		rowIndex := rowCount + i

		for j, cell := range row {
			tableCell := tview.NewTableCell(cell.Value.(string))
			tableCell.SetExpansion(1)
			tableCell.SetReference(inserts[i].PrimaryKeyInfo[0].Value)

			tableCell.SetTextColor(app.Styles.PrimaryTextColor)
			tableCell.SetBackgroundColor(colorTableInsert)

			table.SetCell(rowIndex, j, tableCell)
		}
	}
}

func (table *ResultsTable) AppendNewRow(cells []models.CellValue, index int, UUID string) {
	for i, cell := range cells {
		tableCell := tview.NewTableCell(cell.Value.(string))
		tableCell.SetExpansion(1)
		// Appended rows have a reference to the row UUID so we can identify them later
		// Also, rows that have columns marked to be UPDATED will have a reference to the type of the new value (NULL, EMPTY, DEFAULT)
		// So, the cell reference will be used to determine if the row/column is an inserted row or if it's an UPDATED row
		// there might be a better way to do this, but it works for now
		tableCell.SetReference(UUID)
		tableCell.SetTextColor(app.Styles.PrimaryTextColor)
		tableCell.SetBackgroundColor(tcell.ColorDarkGreen)

		switch cell.Type {
		case models.Null, models.Empty, models.Default:
			tableCell.SetText(strings.Replace(cell.Value.(string), "&", "", 1))
			tableCell.SetStyle(table.GetItalicStyle())
			// tableCell.SetText("")

			tableCell.SetTextColor(app.Styles.InverseTextColor)
		}

		tableCell.SetBackgroundColor(colorTableInsert)
		table.SetCell(index, i, tableCell)
	}

	table.Select(index, 0)
	App.ForceDraw()
}

func (table *ResultsTable) tableInputCapture(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEscape && (table.CancelActiveQuery() || table.CancelExport()) {
		return nil
	}

	selectedRowIndex, selectedColumnIndex := table.GetSelection()
	colCount := table.GetColumnCount()
	rowCount := table.GetRowCount()

	eventKey := event.Rune()

	command := app.Keymaps.Group(app.TableGroup).Resolve(event)
	if command == commands.CancelQuery && (table.CancelActiveQuery() || table.CancelExport()) {
		return nil
	}

	menuCommands := []commands.Command{commands.RecordsMenu, commands.ColumnsMenu, commands.ConstraintsMenu, commands.ForeignKeysMenu, commands.IndexesMenu}

	if helpers.ContainsCommand(menuCommands, command) {
		table.Select(1, 0)
	}

	if table.Menu != nil {
		switch command {
		case commands.RecordsMenu:
			table.Menu.SetSelectedOption(1)
			table.UpdateRows(table.GetRecords())
			table.colorChangedCells()
			table.AddInsertedRows()
			table.UpdateRowsColor(app.Styles.PrimaryTextColor, tview.Styles.PrimaryTextColor)
		case commands.ColumnsMenu:
			table.Menu.SetSelectedOption(2)
			table.showMetadataSurface(MetadataColumns)
		case commands.ConstraintsMenu:
			table.Menu.SetSelectedOption(3)
			table.showMetadataSurface(MetadataConstraints)
		case commands.ForeignKeysMenu:
			table.Menu.SetSelectedOption(4)
			table.showMetadataSurface(MetadataForeignKeys)
		case commands.IndexesMenu:
			table.Menu.SetSelectedOption(5)
			table.showMetadataSurface(MetadataIndexes)
		case commands.Refresh:
			table.RefreshActiveSurface()
		}
	}

	if command == commands.ExactCount {
		if table.Menu != nil && table.Menu.GetSelectedOption() == 1 {
			table.ToggleExactCount()
			return nil
		}
	}

	switch command {
	case commands.AppendNewRow:
		if table.ReadOnly {
			table.SetError("Cannot modify data: Connection is in read-only mode", nil)
			return nil
		}
		if table.Menu.GetSelectedOption() == 1 {
			table.appendNewRow()
		}
	case commands.DuplicateRow:
		if table.ReadOnly {
			table.SetError("Cannot modify data: Connection is in read-only mode", nil)
			return nil
		}
		if table.Menu.GetSelectedOption() == 1 {
			table.duplicateRow()
		}
	case commands.Search:
		table.search()
	}

	if rowCount == 1 || colCount == 0 {
		return nil
	}

	if command == commands.Edit {
		if table.ReadOnly {
			table.SetError("Cannot modify data: Connection is in read-only mode", nil)
			return nil
		}
		if table.Editor == nil {
			table.StartEditingCell(selectedRowIndex, selectedColumnIndex, func(_ string, _, _ int) {
				if table.GetShowSidebar() {
					table.UpdateSidebar()
				}
			})
		}
	} else if command == commands.GotoNext {
		if selectedColumnIndex+1 < colCount {
			table.Select(selectedRowIndex, selectedColumnIndex+1)
		}
	} else if command == commands.GotoPrev {
		if selectedColumnIndex > 0 {
			table.Select(selectedRowIndex, selectedColumnIndex-1)
		}
	} else if command == commands.GotoEnd {
		table.Select(selectedRowIndex, colCount-1)
	} else if command == commands.GotoStart {
		table.Select(selectedRowIndex, 0)
	} else if command == commands.GotoBottom {
		go table.Select(1, selectedColumnIndex)
	} else if command == commands.GotoTop {
		go table.Select(rowCount-1, selectedColumnIndex)
	} else if eventKey == 4 { // Ctrl + D
		if selectedRowIndex+7 > rowCount-1 {
			go table.Select(rowCount-1, selectedColumnIndex)
		} else {
			go table.Select(selectedRowIndex+7, selectedColumnIndex)
		}
	} else if eventKey == 21 { // Ctrl + U
		if selectedRowIndex-7 < 1 {
			go table.Select(1, selectedColumnIndex)
		} else {
			go table.Select(selectedRowIndex-7, selectedColumnIndex)
		}
	} else if command == commands.Delete {
		if table.ReadOnly {
			table.SetError("Cannot modify data: Connection is in read-only mode", nil)
			return nil
		}
		if table.Menu != nil && table.Menu.GetSelectedOption() == 1 && table.Editor == nil {

			isAnInsertedRow, indexOfInsertedRow := table.isAnInsertedRow(selectedRowIndex)

			if isAnInsertedRow {
				*table.state.listOfDBChanges = append((*table.state.listOfDBChanges)[:indexOfInsertedRow], (*table.state.listOfDBChanges)[indexOfInsertedRow+1:]...)
				table.RemoveRow(selectedRowIndex)
				if selectedRowIndex-1 != 0 {
					table.Select(selectedRowIndex-1, 0)
				} else {
					if selectedRowIndex+1 < rowCount {
						table.Select(selectedRowIndex+1, 0)
					}
				}
			} else {
				err := table.AppendNewChange(models.DMLDeleteType, selectedRowIndex, -1, models.CellValue{TableColumnIndex: -1, TableRowIndex: selectedRowIndex, Column: table.GetColumnNameByIndex(selectedColumnIndex)})
				if err != nil {
					table.SetError(err.Error(), nil)
				}
			}

		}
	} else if command == commands.SetValue {
		if table.ReadOnly {
			table.SetError("Cannot modify data: Connection is in read-only mode", nil)
			return nil
		}
		if table.Editor == nil {
			table.SetIsEditing(true)
			table.SetInputCapture(nil)

			cell := table.GetCell(selectedRowIndex, selectedColumnIndex)
			x, y, _ := cell.GetLastPosition()

			list := NewSetValueList(table.DBDriver.GetProvider())

			list.OnFinish(func(selection models.CellValueType, value string) {
				table.FinishSettingValue()

				if selection >= 0 {
					err := table.AppendNewChange(models.DMLUpdateType, selectedRowIndex, selectedColumnIndex, models.CellValue{Type: selection, Value: value, Column: table.GetColumnNameByIndex(selectedColumnIndex)})
					if err != nil {
						table.SetError(err.Error(), nil)
					}
				}
			})

			list.Show(x, y, 30)
		}
	} else if command == commands.ToggleSidebar {
		if table.Editor == nil {
			table.ShowSidebar(!table.GetShowSidebar())
		}
	} else if command == commands.FocusSidebar {
		if table.GetShowSidebar() {
			App.SetFocus(table.Sidebar)
		}
	} else if command == commands.ShowRowJSONViewer {
		table.handleShowJSONViewer(commands.ShowRowJSONViewer)
		return nil
	} else if command == commands.ShowCellJSONViewer {
		table.handleShowJSONViewer(commands.ShowCellJSONViewer)
		return nil
	} else if event.Key() == tcell.KeyEnter {
		if table.handleForeignKeyEnter(selectedRowIndex, selectedColumnIndex) {
			return nil
		}
		if app.App.Config().EnterOpensJSONViewer {
			table.handleShowJSONViewer(commands.ShowCellJSONViewer)
			return nil
		}
	} else if command == commands.RowSelect {
		table.toggleRowMark(selectedRowIndex)
		return nil
	} else if command == commands.Copy {
		clipboard := lib.NewClipboard()

		if len(table.state.markedRows) > 0 {
			if err := clipboard.Write(table.markedRowsToText()); err != nil {
				table.SetError(err.Error(), nil)
			}
		} else {
			selectedCell := table.GetCell(selectedRowIndex, selectedColumnIndex)
			if selectedCell != nil {
				if err := clipboard.Write(selectedCell.Text); err != nil {
					table.SetError(err.Error(), nil)
				}
			}
		}
	} else if command == commands.OpenCellInExternalEditor {
		if table.ReadOnly {
			table.SetError("Cannot modify data: Connection is in read-only mode", nil)
			return nil
		}
		if table.Editor == nil && (runtime.GOOS == "linux" || runtime.GOOS == "darwin") {
			selectedCell := table.GetCell(selectedRowIndex, selectedColumnIndex)
			if selectedCell != nil {
				originalText := selectedCell.Text
				var newText string
				app.App.Suspend(func() {
					newText = openCellInExternalEditor(originalText)
				})
				// Strip trailing newline that editors typically add
				newText = strings.TrimSuffix(newText, "\n")
				if newText != originalText {
					selectedCell.SetText(newText)
					columnName := table.GetColumnNameByIndex(selectedColumnIndex)
					err := table.AppendNewChange(models.DMLUpdateType, selectedRowIndex, selectedColumnIndex, models.CellValue{
						Type:             models.String,
						Value:            newText,
						Column:           columnName,
						TableColumnIndex: selectedColumnIndex,
						TableRowIndex:    selectedRowIndex,
					})
					if err != nil {
						table.SetError(err.Error(), nil)
					}
					if table.GetShowSidebar() {
						table.UpdateSidebar()
					}
				}
			}
		}
		return nil
	} else if command == commands.ExportCSV {
		table.showCSVExportModal()
		return nil
	}

	if len(table.GetRecords()) > 0 {
		switch command {
		case commands.SortDesc:
			currentColumnName := table.GetColumnNameByIndex(selectedColumnIndex)
			table.Pagination.SetOffset(0)
			table.SetSortedBy(currentColumnName, "DESC")
		case commands.SortAsc:
			currentColumnName := table.GetColumnNameByIndex(selectedColumnIndex)
			table.Pagination.SetOffset(0)
			table.SetSortedBy(currentColumnName, "ASC")
		}
	}

	return event
}

func (table *ResultsTable) UpdateRows(rows [][]string) {
	table.state.fkRawCellValues = map[string]string{}
	table.clearRowMarks()
	table.Clear()
	table.AddRows(rows)
	App.ForceDraw()
	table.Select(1, 0)
}

func (table *ResultsTable) UpdateRowsColor(headerColor tcell.Color, rowColor tcell.Color) {
	for i := 0; i < table.GetRowCount(); i++ {
		for j := 0; j < table.GetColumnCount(); j++ {
			cell := table.GetCell(i, j)
			if i == 0 && headerColor != 0 {
				if table.shouldShowForeignKeyHeaderMarker(j) {
					cell.SetStyle(tcell.StyleDefault.Underline(true))
					cell.SetTextColor(headerColor)
				} else {
					cell.SetStyle(tcell.StyleDefault)
					cell.SetTextColor(headerColor)
				}
			} else {
				cellReference := cell.GetReference()

				if cellReference != nil && (cellReference == "EMPTY&" || cellReference == "NULL&" || cellReference == "DEFAULT&") && (cell.BackgroundColor != colorTableDelete && cell.BackgroundColor != colorTableChange && cell.BackgroundColor != colorTableInsert) {
					cell.SetStyle(table.GetItalicStyle())
				} else if table.shouldShowForeignKeyMarker(i, j, cell.Text) {
					cell.SetStyle(tcell.StyleDefault.Underline(true))
					cell.SetTextColor(rowColor)
				} else {
					cell.SetStyle(tcell.StyleDefault)
					cell.SetTextColor(rowColor)
				}
			}
		}
	}
}

func (table *ResultsTable) shouldShowForeignKeyHeaderMarker(columnIndex int) bool {
	if table.Menu != nil && table.Menu.GetSelectedOption() != 1 {
		return false
	}

	if !table.IsForeignKeyJumpSupportedProvider() {
		return false
	}

	columnName := table.GetColumnNameByIndex(columnIndex)
	if columnName == "" || !table.isForeignKeyColumn(columnName) {
		return false
	}

	_, ok := table.getForeignKeyJumpTarget(columnName)
	return ok
}

func (table *ResultsTable) RemoveHighlightTable() {
	table.SetBorderColor(app.Styles.InverseTextColor)
	table.SetBordersColor(app.Styles.InverseTextColor)
	table.SetTitleColor(app.Styles.InverseTextColor)
	table.UpdateRowsColor(app.Styles.InverseTextColor, tview.Styles.InverseTextColor)
}

func (table *ResultsTable) RemoveHighlightAll() {
	table.RemoveHighlightTable()
	if table.Menu != nil {
		table.Menu.SetBlur()
	}
	if table.Filter != nil {
		table.Filter.RemoveHighlight()
	}
}

func (table *ResultsTable) HighlightTable() {
	table.SetBorderColor(app.Styles.PrimaryTextColor)
	table.SetBordersColor(app.Styles.PrimaryTextColor)
	table.SetTitleColor(app.Styles.PrimaryTextColor)
	table.UpdateRowsColor(app.Styles.PrimaryTextColor, tview.Styles.PrimaryTextColor)
}

func (table *ResultsTable) HighlightAll() {
	table.HighlightTable()
	if table.Menu != nil {
		table.Menu.SetFocus()
	}
	if table.Filter != nil {
		table.Filter.Highlight()
	}
}

func (table *ResultsTable) subscribeToFilterChanges() {
	ch := table.Filter.Subscribe()

	for stateChange := range ch {
		switch stateChange.Key {
		case eventResultsTableFiltering:
			if stateChange.Value != "" {
				table.FetchRecords(nil, func() {
					records := table.GetRecords()
					if len(records) > 0 {
						table.Menu.SetSelectedOption(1)
						table.SetIsFiltering(false)
						App.SetFocus(table)
						table.HighlightTable()
						table.Filter.HighlightLocal()
						table.SetInputCapture(table.tableInputCapture)
					}
				})
			} else {
				table.FetchRecords(nil, func() {
					table.SetIsFiltering(false)
					table.SetInputCapture(table.tableInputCapture)
					App.SetFocus(table)
					table.HighlightTable()
					table.Filter.HighlightLocal()
				})
			}
		}
	}
}

func (table *ResultsTable) subscribeToEditorChanges() {
	ch := table.Editor.Subscribe()

	for stateChange := range ch {
		switch stateChange.Key {
		case eventSQLEditorQuery:
			query := stateChange.Value.(string)
			if strings.TrimSpace(query) == "" {
				continue
			}

			// Validate before starting a load or recording history. A CTE such as
			// "WITH ... INSERT" starts with "with" and must still be rejected on
			// a read-only connection before it reaches the driver.
			if table.ReadOnly {
				if err := drivers.ValidateQueryForReadOnly(query); err != nil {
					App.QueueUpdateDraw(func() {
						table.SetError("Cannot execute mutation query: Connection is in read-only mode", nil)
						table.SetLoading(false)
					})
					continue
				}
			}

			isSelect := isResultProducingQuery(query)

			// Clear existing records immediately for SQL editor queries and start
			// a cancellable loading cycle on the UI goroutine. The active query is
			// registered in the same update so Escape cannot observe a gap between
			// loading and query state.
			var ctx context.Context
			var generation uint64
			var run *editorQueryRun
			App.QueueUpdateDraw(func() {
				ctx, generation = table.startLoad()
				table.state.lastEditorQuery = ""
				table.state.lastEditorQueryReplaySafe = false
				table.state.editorResultAvailable = false
				if isSelect {
					table.state.lastEditorQuery = query
					table.state.lastEditorQueryReplaySafe = isReplaySafeQuery(query)
				}
				table.SetRecords([][]string{})
				table.Pagination.SetOffset(0)
				table.Pagination.ClearCount()
				table.Pagination.SetPageInfo(0, true)
				table.SetQueryStatus("Running query… [Esc cancel]")
				run = table.beginEditorQuery(generation)
			})

			if isSelect {
				go table.runEditorStreamQuery(ctx, run, query)
			} else {
				go table.runEditorDMLQuery(ctx, generation, query)
			}

		case eventSQLEditorEscape:
			// The editor invokes CancelActiveQuery directly for an active query.
			// This event remains the normal focus/unfocus path when no query is
			// active.
			App.QueueUpdateDraw(func() {
				table.SetIsFiltering(false)
				App.SetFocus(table)
				table.HighlightTable()
				table.Editor.SetBlur()
				table.SetInputCapture(table.tableInputCapture)
			})
		}
	}
}

func isResultProducingQuery(query string) bool {
	tokens, ok := tokenizeReplayQuery(query)
	if !ok {
		return false
	}
	for _, token := range tokens {
		if token.kind != replayTokenWord {
			continue
		}
		switch token.text {
		case "SELECT", "WITH", "EXPLAIN", "SHOW", "DESCRIBE", "DESC":
			return true
		default:
			return false
		}
	}
	return false
}

// isSchemaMutatingQuery identifies statements whose successful execution may
// change the visible database tree. It intentionally classifies only the
// leading statement verb: DDL invalidation is connection-wide and does not
// attempt to infer the affected object from arbitrary SQL.
func isSchemaMutatingQuery(query string) bool {
	tokens, ok := tokenizeReplayQuery(query)
	if !ok {
		return false
	}
	for _, token := range tokens {
		if token.kind != replayTokenWord {
			continue
		}
		switch token.text {
		case "ALTER", "COMMENT", "CREATE", "DROP", "GRANT", "RENAME", "REVOKE", "TRUNCATE":
			return true
		default:
			return false
		}
	}
	return false
}

func (table *ResultsTable) beginEditorQuery(generation uint64) *editorQueryRun {
	run := &editorQueryRun{generation: generation, started: time.Now()}
	table.state.loadingMu.Lock()
	if table.state.loadGeneration == generation {
		run.cancel = table.state.loadingCancel
	}
	table.state.loadingMu.Unlock()

	table.queryMu.Lock()
	table.activeQuery = run
	table.queryMu.Unlock()
	return run
}

func (table *ResultsTable) invalidateEditorQuery() {
	table.queryMu.Lock()
	table.activeQuery = nil
	table.queryMu.Unlock()
}

func (table *ResultsTable) isCurrentEditorQuery(run *editorQueryRun) bool {
	if run == nil {
		return false
	}
	table.queryMu.Lock()
	defer table.queryMu.Unlock()
	return table.activeQuery == run
}

func (table *ResultsTable) finishEditorQuery(run *editorQueryRun) bool {
	if run == nil {
		return false
	}
	table.queryMu.Lock()
	if table.activeQuery != run {
		table.queryMu.Unlock()
		return false
	}
	table.activeQuery = nil
	table.queryMu.Unlock()

	table.state.loadingMu.Lock()
	if table.state.loadGeneration == run.generation {
		table.state.loadingCancel = nil
	}
	table.state.loadingMu.Unlock()
	return true
}

// IsQueryActive reports whether an interactive SQL result query is currently
// running. It is intentionally separate from the generic table loading state.
func (table *ResultsTable) IsQueryActive() bool {
	table.queryMu.Lock()
	defer table.queryMu.Unlock()
	return table.activeQuery != nil
}

// CancelActiveQuery cancels only an active SQL-editor result query. It returns
// false when Escape should retain its normal editor/table behavior.
func (table *ResultsTable) CancelActiveQuery() bool {
	table.queryMu.Lock()
	run := table.activeQuery
	if run == nil {
		table.queryMu.Unlock()
		return false
	}
	if run.cancelRequested {
		table.queryMu.Unlock()
		return true
	}
	run.cancelRequested = true
	// Stop accepting batches immediately. The stream's context is still
	// cancelled below, while already rendered rows remain in the table.
	table.activeQuery = nil
	table.queryMu.Unlock()

	table.CancelExactCount()
	table.state.loadingMu.Lock()
	var cancel context.CancelFunc
	if table.state.loadGeneration == run.generation {
		cancel = table.state.loadingCancel
		table.state.loadingCancel = nil
		table.state.loadGeneration++
	}
	table.state.loadingMu.Unlock()
	if cancel != nil {
		cancel()
	}
	// The captured cancel is idempotent and protects against a load transition
	// racing with this cancellation request.
	if run.cancel != nil {
		run.cancel()
	}
	table.SetLoading(false)
	count := table.editorQueryRowCount()
	if count == 0 {
		table.SetQueryStatus("Query cancelled")
	} else {
		table.SetQueryStatus(fmt.Sprintf("%d rows — partial result: query cancelled", count))
	}
	return true
}

// CancelQuery is an alias for callers that use the shorter command-oriented
// name.
func (table *ResultsTable) CancelQuery() bool {
	return table.CancelActiveQuery()
}

func (table *ResultsTable) editorQueryRowCount() int {
	records := table.GetRecords()
	if len(records) == 0 {
		return 0
	}
	return len(records) - 1
}

func (table *ResultsTable) SetQueryStatus(status string) {
	table.state.queryStatus = status
	if table.Pagination != nil {
		table.Pagination.SetResultStatus(status)
	}
}

func (table *ResultsTable) GetQueryStatus() string {
	return table.state.queryStatus
}

func (table *ResultsTable) appendEditorQueryBatch(batch drivers.QueryBatch) {
	records := table.GetRecords()
	if len(records) == 0 && len(batch.Columns) > 0 {
		columns := append([]string(nil), batch.Columns...)
		records = append(records, columns)
	}
	if len(batch.Rows) > 0 {
		records = append(records, batch.Rows...)
	}
	if len(records) > 0 {
		table.SetRecords(records)
	}
}

func (table *ResultsTable) showEditorQueryResults() {
	table.SetIsFiltering(false)
	closeQuitConfirmation()
	table.HighlightTable()
	if table.Editor != nil {
		table.Editor.SetBlur()
	}
	table.SetInputCapture(table.tableInputCapture)
	if table.EditorPages != nil {
		table.EditorPages.SwitchToPage(pageNameTableEditorTable)
	}
	App.SetFocus(table)
}

func (table *ResultsTable) maxInteractiveQueryRows() int {
	config := App.Config()
	if config == nil {
		return models.DefaultMaxQueryRows
	}
	if config.MaxQueryRows < 0 {
		return models.DefaultMaxQueryRows
	}
	return config.MaxQueryRows
}

func (table *ResultsTable) renderEditorQueryBatch(ctx context.Context, run *editorQueryRun, batch drivers.QueryBatch) error {
	if ctx == nil || ctx.Err() != nil || !table.isCurrentEditorQuery(run) {
		return context.Canceled
	}

	applied := false
	App.QueueUpdateDraw(func() {
		if ctx.Err() != nil || !table.isCurrentEditorQuery(run) {
			return
		}
		firstPaint := len(table.GetRecords()) == 0
		table.appendEditorQueryBatch(batch)
		if len(batch.Columns) > 0 || len(batch.Rows) > 0 {
			table.state.editorResultAvailable = true
		}
		table.Pagination.SetPageInfo(table.editorQueryRowCount(), true)
		if firstPaint {
			table.showEditorQueryResults()
			run.firstUsefulResultOnce.Do(func() {
				logFirstUsefulResult("query_editor", run.started, map[string]any{
					"connection":          table.connectionIdentifier,
					"rows_in_first_batch": len(batch.Rows),
				})
			})
		}
		applied = true
	})
	if !applied {
		return context.Canceled
	}
	return nil
}

func (table *ResultsTable) streamEditorQuery(ctx context.Context, run *editorQueryRun, query string) (drivers.QueryStreamResult, error) {
	started := time.Now()
	onBatch := func(batch drivers.QueryBatch) error {
		return table.renderEditorQueryBatch(ctx, run, batch)
	}

	if streamer, ok := table.DBDriver.(drivers.QueryStreamer); ok {
		result, err := streamer.StreamQuery(ctx, query, table.maxInteractiveQueryRows(), onBatch)
		logDatabaseOperation("stream_query", started, ctx, map[string]any{
			"connection": table.connectionIdentifier,
			"rows":       result.Rows,
			"truncated":  result.Truncated,
			"streaming":  true,
		}, err)
		return result, err
	}

	// Drivers without the optional streaming capability still use the final
	// context-aware query contract as a cancellable fallback.
	rows, count, err := table.DBDriver.ExecuteQuery(ctx, query)
	result := drivers.QueryStreamResult{Rows: count}
	if err != nil {
		logDatabaseOperation("execute_query", started, ctx, map[string]any{
			"connection": table.connectionIdentifier,
			"rows":       result.Rows,
			"streaming":  false,
		}, err)
		return result, err
	}
	if len(rows) > 0 {
		result.Columns = append([]string(nil), rows[0]...)
		data := rows[1:]
		maxRows := table.maxInteractiveQueryRows()
		if maxRows > 0 && len(data) > maxRows {
			result.Truncated = true
			data = data[:maxRows]
		}
		result.Rows = len(data)
		if err := onBatch(drivers.QueryBatch{Columns: result.Columns, Rows: data}); err != nil {
			logDatabaseOperation("execute_query", started, ctx, map[string]any{
				"connection": table.connectionIdentifier,
				"rows":       result.Rows,
				"truncated":  result.Truncated,
				"streaming":  false,
			}, err)
			return result, err
		}
	}
	logDatabaseOperation("execute_query", started, ctx, map[string]any{
		"connection": table.connectionIdentifier,
		"rows":       result.Rows,
		"truncated":  result.Truncated,
		"streaming":  false,
	}, nil)
	return result, nil
}

func (table *ResultsTable) addEditorQueryToHistory(query string) {
	if err := history.AddQueryToHistory(table.connectionIdentifier, query); err != nil {
		logger.Error("Failed to add dispatched SQL-editor query to history", map[string]any{"error": err, "query": query, "connection": table.connectionIdentifier})
	}
}

func (table *ResultsTable) runEditorStreamQuery(ctx context.Context, run *editorQueryRun, query string) {
	if ctx == nil || ctx.Err() != nil || !table.isCurrentEditorQuery(run) {
		return
	}

	// The invocation below is the dispatch boundary. Recording immediately
	// before it captures completed, truncated, cancelled, and post-dispatch
	// failed queries while validation failures never reach history.
	table.addEditorQueryToHistory(query)
	result, err := table.streamEditorQuery(ctx, run, query)
	App.QueueUpdateDraw(func() {
		if !table.isCurrentEditorQuery(run) {
			return
		}

		table.finishEditorQuery(run)
		if len(table.GetRecords()) == 0 && len(result.Columns) > 0 {
			table.appendEditorQueryBatch(drivers.QueryBatch{Columns: result.Columns})
			table.state.editorResultAvailable = true
			run.firstUsefulResultOnce.Do(func() {
				logFirstUsefulResult("query_editor", run.started, map[string]any{
					"connection":          table.connectionIdentifier,
					"rows_in_first_batch": 0,
				})
			})
		}

		rowCount := table.editorQueryRowCount()
		cancelled := errors.Is(err, context.Canceled) || ctx.Err() != nil
		if cancelled {
			table.Pagination.SetPageInfo(rowCount, true)
			table.SetLoading(false)
			if rowCount == 0 {
				table.SetQueryStatus("Query cancelled")
			} else {
				table.SetQueryStatus(fmt.Sprintf("%d rows — partial result: query cancelled", rowCount))
			}
			if rowCount > 0 || len(result.Columns) > 0 {
				table.showEditorQueryResults()
			}
			return
		}

		if err != nil {
			table.SetLoading(false)
			if rowCount == 0 {
				table.SetQueryStatus(fmt.Sprintf("Query failed: %s", err.Error()))
				table.SetError(err.Error(), nil)
				return
			}
			table.Pagination.SetPageInfo(rowCount, true)
			table.SetQueryStatus(fmt.Sprintf("%d rows — partial result: %s", rowCount, err.Error()))
			table.showEditorQueryResults()
			return
		}

		table.Pagination.SetLimit(rowCount)
		if result.Truncated {
			table.Pagination.SetPageInfo(rowCount, true)
			table.SetQueryStatus(fmt.Sprintf("%d rows shown — result truncated (maximum %d)", rowCount, table.maxInteractiveQueryRows()))
		} else {
			table.Pagination.SetPageInfo(rowCount, false)
			table.SetQueryStatus("")
		}
		table.SetLoading(false)
		table.showEditorQueryResults()
	})
}

func (table *ResultsTable) runEditorDMLQuery(ctx context.Context, generation uint64, query string) {
	if ctx == nil || ctx.Err() != nil || !table.isCurrentLoad(ctx, generation) {
		return
	}

	table.addEditorQueryToHistory(query)
	result, err := table.DBDriver.ExecuteDMLStatement(ctx, query)
	ddl := isSchemaMutatingQuery(query)
	if err == nil && ddl {
		if table.Home != nil {
			database := table.GetDatabaseName()
			home := table.Home
			home.refreshSchemaAfterDDL(database)
		} else {
			// Standalone ResultsTable instances used by integrations still need
			// their connection-scoped metadata cache invalidated even though no
			// visible Home/tree is available to rebuild.
			table.metadataCacheForTable().invalidateAll()
		}
	}
	if ctx.Err() != nil {
		return
	}

	App.QueueUpdateDraw(func() {
		if !table.isCurrentLoad(ctx, generation) {
			return
		}

		if err != nil {
			table.SetLoading(false)
			table.SetQueryStatus(fmt.Sprintf("Query failed: %s", err.Error()))
			table.SetError(err.Error(), nil)
			return
		}

		table.SetResultsInfo(result)
		table.SetQueryStatus("")
		table.SetLoading(false)
		// Clear filtering state before closing the quit confirmation: closing it
		// can synchronously trigger Home.focusTab, which re-focuses the editor
		// while filtering is still active.
		table.SetIsFiltering(false)
		closeQuitConfirmation()
		table.HighlightTable()
		table.Editor.SetBlur()
		table.SetInputCapture(table.tableInputCapture)
		table.EditorPages.SwitchToPage(pageNameTableEditorTable)
		App.SetFocus(table)

		// Arbitrary editor DML has no reliable table identity, so it must not
		// refresh whichever Records table happens to be open. Successful DDL
		// already started its cache invalidation/tree rebuild immediately after
		// the driver committed it, independent of this load generation.
	})
}

// Getters

func (table *ResultsTable) GetRecords() [][]string {
	return table.state.records
}

func (table *ResultsTable) GetIndexes() [][]string {
	return table.state.indexes
}

func (table *ResultsTable) GetColumns() [][]string {
	return table.state.columns
}

func (table *ResultsTable) GetConstraints() [][]string {
	return table.state.constraints
}

func (table *ResultsTable) GetForeignKeys() [][]string {
	return table.state.foreignKeys
}

func (table *ResultsTable) GetTableName() string {
	table.metadataIdentityMu.RLock()
	defer table.metadataIdentityMu.RUnlock()
	return table.state.tableName
}

func (table *ResultsTable) GetDatabaseName() string {
	table.metadataIdentityMu.RLock()
	defer table.metadataIdentityMu.RUnlock()
	return table.state.databaseName
}

func (table *ResultsTable) GetDatabaseAndTableName() string {
	return fmt.Sprintf("%s.%s", table.GetDatabaseName(), table.GetTableName())
}

func (table *ResultsTable) GetIsEditing() bool {
	return table.state.isEditing
}

func (table *ResultsTable) GetCurrentSort() string {
	return table.state.currentSort
}

func (table *ResultsTable) GetColumnNameByIndex(index int) string {
	columns := table.GetColumns()

	for i, col := range columns {
		if i > 0 && i == index+1 && len(col) > 0 {
			return col[0]
		}
	}

	records := table.GetRecords()
	if len(records) > 0 && index >= 0 && index < len(records[0]) {
		return records[0][index]
	}

	return ""
}

func (table *ResultsTable) GetColumnIndexByName(columnName string) int {
	cols := table.GetColumns()

	for i, col := range cols {
		if i > 0 && len(col) > 0 && col[0] == columnName {
			return i - 1 // Because the first column is the column names
		}
	}

	records := table.GetRecords()
	if len(records) > 0 {
		for i, name := range records[0] {
			if name == columnName {
				return i
			}
		}
	}

	return -1
}

func (table *ResultsTable) GetIsLoading() bool {
	return table.state.isLoading
}

func (table *ResultsTable) GetIsFiltering() bool {
	return table.state.isFiltering
}

func (table *ResultsTable) GetShowSidebar() bool {
	return table.state.showSidebar
}

func (table *ResultsTable) GetPrimaryKeyColumnNames() []string {
	return table.state.primaryKeyColumnNames
}

// GetPrimaryKeySort returns an ORDER BY clause using primary key columns.
// Returns empty string if no primary key columns are available.
func (table *ResultsTable) GetPrimaryKeySort() string {
	pkColumnNames := table.GetPrimaryKeyColumnNames()
	if len(pkColumnNames) == 0 {
		return ""
	}
	// Use first primary key column for sorting
	return fmt.Sprintf("%s ASC", pkColumnNames[0])
}

// Setters

func (table *ResultsTable) SetRecords(rows [][]string) {
	table.state.records = rows
	if table.Editor != nil && len(rows) > 0 {
		table.state.editorResultAvailable = true
	}
	table.UpdateRows(rows)
	table.colorChangedCells()
}

func (table *ResultsTable) SetColumns(columns [][]string) {
	table.state.columns = columns
}

func (table *ResultsTable) SetConstraints(constraints [][]string) {
	table.state.constraints = constraints
}

func (table *ResultsTable) SetForeignKeys(foreignKeys [][]string) {
	table.state.foreignKeys = foreignKeys
	table.rebuildForeignKeyJumpMetadata()
}

func (table *ResultsTable) SetIndexes(indexes [][]string) {
	table.state.indexes = indexes
}

func (table *ResultsTable) SetDatabaseName(databaseName string) {
	table.metadataApplyMu.Lock()
	table.metadataIdentityMu.Lock()
	if table.state.databaseName == databaseName {
		table.metadataIdentityMu.Unlock()
		table.metadataApplyMu.Unlock()
		return
	}
	table.state.databaseName = databaseName
	table.metadataIdentityGeneration++
	table.metadataIdentityMu.Unlock()
	table.metadataApplyMu.Unlock()
	table.invalidateRowCount()
}

func (table *ResultsTable) SetTableName(tableName string) {
	table.metadataApplyMu.Lock()
	table.metadataIdentityMu.Lock()
	if table.state.tableName == tableName {
		table.metadataIdentityMu.Unlock()
		table.metadataApplyMu.Unlock()
		return
	}
	table.state.tableName = tableName
	table.metadataIdentityGeneration++
	table.metadataIdentityMu.Unlock()
	table.metadataApplyMu.Unlock()
	table.invalidateRowCount()
}

func (table *ResultsTable) SetError(err string, done func()) {
	table.state.error = err

	table.Error.SetText(err)
	table.Error.SetDoneFunc(func(_ int, _ string) {
		table.state.error = ""
		table.Page.HidePage(pageNameTableError)
		closeQuitConfirmation()
		if table.GetIsFiltering() {
			if table.Editor != nil {
				App.SetFocus(table.Editor)
			} else {
				App.SetFocus(table.Filter.Input)
			}
		} else {
			App.SetFocus(table)
		}
		if done != nil {
			done()
		}
	})
	table.Page.ShowPage(pageNameTableError)
	App.SetFocus(table.Error)
	App.ForceDraw()
}

func (table *ResultsTable) SetResultsInfo(text string) {
	table.ResultsInfo.SetText(text)
}

func (table *ResultsTable) SetLoading(show bool) {
	table.state.isLoading = show
	table.Pagination.SetLoading(show)
}

func (table *ResultsTable) invalidateCSVExport() {
	table.exportMu.Lock()
	run := table.activeExport
	table.activeExport = nil
	table.exportMu.Unlock()

	if run != nil {
		run.cancelAndGetRows()
		if run.cancel != nil {
			run.cancel()
		}
	}
}

func (table *ResultsTable) beginCSVExport() *csvExportRun {
	// A full export has its own lifecycle; do not wait for or share an
	// interactive exact-count operation.
	table.CancelExactCount()
	ctx, generation := table.startLoad()
	run := &csvExportRun{ctx: ctx, cancel: func() {}, generation: generation}

	// startLoad owns the cancellation stored in ResultsTableState. Capture it
	// for a dedicated export cancellation path without exposing generic load
	// state to the CSV worker.
	table.state.loadingMu.Lock()
	if table.state.loadGeneration == generation && table.state.loadingCancel != nil {
		run.cancel = table.state.loadingCancel
	}
	table.state.loadingMu.Unlock()

	table.exportMu.Lock()
	table.activeExport = run
	table.exportMu.Unlock()
	if table.Pagination != nil {
		table.Pagination.ClearResultStatus()
	}
	return run
}

func (table *ResultsTable) isCurrentCSVExport(run *csvExportRun) bool {
	if run == nil || run.cancelled() {
		return false
	}
	table.exportMu.Lock()
	defer table.exportMu.Unlock()
	return table.activeExport == run
}

func (table *ResultsTable) updateCSVExportProgress(run *csvExportRun, rows int) {
	if run == nil || !table.isCurrentCSVExport(run) {
		return
	}
	run.setRowsWritten(rows)
	App.QueueUpdateDraw(func() {
		if !table.isCurrentCSVExport(run) || table.Pagination == nil {
			return
		}
		table.Pagination.SetResultStatus(fmt.Sprintf("Exporting… %d rows written", run.rows()))
	})
}

// finishCSVExport applies the terminal state for an export. It must run on
// the UI goroutine so a cancelled or superseded worker can never show a stale
// success modal.
func (table *ResultsTable) finishCSVExport(run *csvExportRun, exportErr error) bool {
	if run == nil {
		return false
	}

	table.exportMu.Lock()
	if table.activeExport != run {
		table.exportMu.Unlock()
		return false
	}
	table.activeExport = nil
	rows := run.rows()
	cancelled := run.cancelled() || run.ctx.Err() != nil || errors.Is(exportErr, context.Canceled)
	table.exportMu.Unlock()

	table.state.loadingMu.Lock()
	if table.state.loadGeneration == run.generation {
		table.state.loadingCancel = nil
	}
	table.state.loadingMu.Unlock()
	table.SetLoading(false)

	if cancelled {
		if table.Pagination != nil {
			table.Pagination.SetResultStatus(fmt.Sprintf("Export cancelled after %d rows written", rows))
		}
		return true
	}
	if exportErr != nil {
		if table.Pagination != nil {
			table.Pagination.ClearResultStatus()
		}
		table.SetError("Failed to export CSV: "+exportErr.Error(), nil)
		return true
	}

	if table.Pagination != nil {
		table.Pagination.ClearResultStatus()
	}
	table.showExportSuccessModal(run.finalPath, rows)
	return true
}

// CancelExport cancels the active CSV export, including the database context
// used by a full table or query export. It returns false when no export is
// running.
func (table *ResultsTable) CancelExport() bool {
	table.exportMu.Lock()
	run := table.activeExport
	if run == nil {
		table.exportMu.Unlock()
		return false
	}
	table.activeExport = nil
	rows := run.cancelAndGetRows()
	table.exportMu.Unlock()

	if run.cancel != nil {
		run.cancel()
	}
	table.state.loadingMu.Lock()
	if table.state.loadGeneration == run.generation {
		table.state.loadingCancel = nil
		table.state.loadGeneration++
	}
	table.state.loadingMu.Unlock()
	table.SetLoading(false)
	if table.Pagination != nil {
		table.Pagination.SetResultStatus(fmt.Sprintf("Export cancelled after %d rows written", rows))
	}
	return true
}

// CancelActiveExport is the explicit alias used by callers that distinguish
// an export operation from ordinary table loading.
func (table *ResultsTable) CancelActiveExport() bool {
	return table.CancelExport()
}

func (table *ResultsTable) CancelLoading() bool {
	if table.CancelExport() {
		return true
	}

	table.CancelExactCount()

	table.state.loadingMu.Lock()
	cancel := table.state.loadingCancel
	table.state.loadingCancel = nil
	table.state.loadGeneration++
	table.state.loadingMu.Unlock()

	if cancel != nil {
		cancel()
		table.SetLoading(false)
		return true
	}
	return false
}

func (table *ResultsTable) startLoad() (context.Context, uint64) {
	// Any new operation supersedes an interactive result stream or export.
	// Clearing each consumer before cancelling its context prevents late
	// batches from touching the new table state.
	table.invalidateCSVExport()
	table.invalidateEditorQuery()

	ctx, cancel := context.WithCancel(app.App.Context())

	table.state.loadingMu.Lock()
	previousCancel := table.state.loadingCancel
	table.state.loadingCancel = cancel
	table.state.loadGeneration++
	generation := table.state.loadGeneration
	table.state.loadingMu.Unlock()

	if previousCancel != nil {
		previousCancel()
	}

	table.SetLoading(true)
	table.SetQueryStatus("")
	return ctx, generation
}

func (table *ResultsTable) StartLoad() context.Context {
	ctx, _ := table.startLoad()
	return ctx
}

func (table *ResultsTable) isCurrentLoad(ctx context.Context, generation uint64) bool {
	if ctx == nil || ctx.Err() != nil {
		return false
	}

	table.state.loadingMu.Lock()
	defer table.state.loadingMu.Unlock()
	return table.state.loadGeneration == generation && table.state.loadingCancel != nil
}

func (table *ResultsTable) SetIsEditing(editing bool) {
	table.state.isEditing = editing
}

func (table *ResultsTable) SetIsFiltering(filtering bool) {
	table.state.isFiltering = filtering
}

func (table *ResultsTable) SetCurrentSort(sort string) {
	table.state.currentSort = sort
}

func (table *ResultsTable) SetSortedBy(column string, direction string) {
	sort := fmt.Sprintf("%s %s", column, direction)
	if table.GetCurrentSort() == sort {
		return
	}

	previousRow, previousColumn := table.GetSelection()
	table.fetchRecords(sort, nil, func() {
		table.SetCurrentSort(sort)
		table.Select(previousRow, previousColumn)
		table.updateSortHeader(column, direction)
		App.ForceDraw()
	})
}

func (table *ResultsTable) SetPrimaryKeyColumnNames(primaryKeyColumnNames []string) {
	table.state.primaryKeyColumnNames = primaryKeyColumnNames
}

func (table *ResultsTable) FetchRecords(onError func(), onSuccess func()) {
	table.fetchRecords(table.GetCurrentSort(), onError, onSuccess)
}

func (table *ResultsTable) fetchRecords(sort string, onError func(), onSuccess func(), metadataToLoad ...MetadataKind) {
	databaseName := table.GetDatabaseName()
	tableName := table.GetTableName()
	where := ""
	if table.Filter != nil {
		where = table.Filter.GetCurrentFilter()
	}
	countKey := rowCountKey{database: databaseName, table: tableName, where: where}
	table.prepareRowCountIdentity(countKey)
	offset := table.Pagination.GetOffset()
	pageSize := table.Pagination.GetLimit()
	ctx, generation := table.startLoad()
	started := time.Now()

	go func() {
		if !table.isCurrentLoad(ctx, generation) {
			return
		}

		page, err := table.DBDriver.GetRecords(ctx, databaseName, tableName, where, sort, offset, pageSize)
		logDatabaseOperation("fetch_records", started, ctx, map[string]any{
			"database": databaseName,
			"table":    tableName,
			"offset":   offset,
			"limit":    pageSize,
			"rows":     max(len(page.Rows)-1, 0),
		}, err)
		if !table.isCurrentLoad(ctx, generation) {
			return
		}

		if err != nil {
			App.QueueUpdateDraw(func() {
				if !table.isCurrentLoad(ctx, generation) {
					return
				}
				table.SetError(err.Error(), onError)
				table.SetLoading(false)
			})
			return
		}

		visibleRows := trimRecordsToPage(page.Rows, pageSize)
		App.QueueUpdateDraw(func() {
			if !table.isCurrentLoad(ctx, generation) {
				return
			}

			if where != "" && page.Query != "" {
				if err := history.AddQueryToHistory(table.connectionIdentifier, page.Query); err != nil {
					logger.Error("Failed to add filter query to history", map[string]any{"error": err, "query": page.Query, "connection": table.connectionIdentifier})
				}
			}

			table.SetRecords(visibleRows)
			table.Select(1, 0)
			table.Pagination.SetPageInfo(max(len(visibleRows)-1, 0), page.HasNextPage)
			table.SetLoading(false)
			logFirstUsefulResult("records", started, map[string]any{
				"database": databaseName,
				"table":    tableName,
				"rows":     max(len(visibleRows)-1, 0),
			})

			if onSuccess != nil {
				onSuccess()
			}
		})

		// QueueUpdateDraw returns only after the page has been rendered, so
		// structural metadata and row counts cannot delay the first useful
		// Records paint.
		if table.Menu != nil {
			go table.startAutomaticRowCount(countKey)
		}
		go table.loadRecordsMetadata(ctx, generation, databaseName, tableName, metadataToLoad...)
	}()
}

func trimRecordsToPage(rows [][]string, pageSize int) [][]string {
	if pageSize <= 0 {
		pageSize = drivers.DefaultRowLimit
	}
	if len(rows) <= 1 || len(rows)-1 <= pageSize {
		return rows
	}

	visibleRows := make([][]string, 0, pageSize+1)
	visibleRows = append(visibleRows, rows[0])
	visibleRows = append(visibleRows, rows[1:pageSize+1]...)
	return visibleRows
}

func (table *ResultsTable) updateSortHeader(column, direction string) {
	var columnNames []string
	columns := table.GetColumns()
	for i := 1; i < len(columns); i++ {
		if len(columns[i]) > 0 {
			columnNames = append(columnNames, columns[i][0])
		}
	}

	if len(columnNames) == 0 {
		records := table.GetRecords()
		if len(records) > 0 {
			columnNames = records[0]
		}
	}

	iconDirection := "▲"
	if direction == "DESC" {
		iconDirection = "▼"
	}

	for i, columnName := range columnNames {
		tableCell := tview.NewTableCell(columnName)
		tableCell.SetSelectable(false)
		tableCell.SetExpansion(1)
		tableCell.SetTextColor(app.Styles.PrimaryTextColor)
		if columnName == column {
			tableCell.SetText(fmt.Sprintf("%s %s", columnName, iconDirection))
		}
		table.SetCell(0, i, tableCell)
	}
}

func (table *ResultsTable) loadRecordsMetadata(ctx context.Context, generation uint64, databaseName, tableName string, kinds ...MetadataKind) {
	if !table.isCurrentLoad(ctx, generation) {
		return
	}
	if len(kinds) == 0 {
		kinds = metadataKinds
	}

	// Records refreshes pass only PrimaryKeys here. A valid local PK result is
	// already sufficient, while an unloaded/failed result may be retried.
	if len(kinds) == 1 && kinds[0] == MetadataPrimaryKeys && table.GetMetadataState(MetadataPrimaryKeys) == MetadataReady {
		return
	}

	// Each requested metadata kind has its own cache entry and worker. Starting
	// them without waiting preserves Records as the only operation on the
	// critical path and allows independent metadata calls to overlap.
	for _, kind := range kinds {
		table.loadMetadataKind(ctx, generation, databaseName, tableName, kind)
	}
}

func (table *ResultsTable) updateMetadataRows(menuOption int, rows [][]string) {
	if table.Menu != nil && table.Menu.GetSelectedOption() == menuOption {
		table.UpdateRows(rows)
	}
}

func (table *ResultsTable) StartEditingCell(row int, col int, callback func(newValue string, row, col int)) {
	table.SetIsEditing(true)
	table.SetInputCapture(nil)

	cell := table.GetCell(row, col)
	inputField := tview.NewInputField()
	inputField.SetText(cell.Text)
	inputField.SetFieldBackgroundColor(app.Styles.PrimaryTextColor)
	inputField.SetFieldTextColor(app.Styles.PrimitiveBackgroundColor)
	inputField.SetBorder(true)

	initialText := cell.Text

	inputField.SetDoneFunc(func(key tcell.Key) {
		table.SetIsEditing(false)
		currentValue := cell.Text
		newValue := inputField.GetText()
		columnName := table.GetCell(0, col).Text

		// Remove the sorting icon from the column name
		columnName = strings.ReplaceAll(columnName, " ▼", "")
		columnName = strings.ReplaceAll(columnName, " ▲", "")

		var appendErr error

		if key != tcell.KeyEscape {
			cell.SetText(newValue)

			if currentValue != newValue {
				appendErr = table.AppendNewChange(models.DMLUpdateType, row, col, models.CellValue{Type: models.String, Value: newValue, Column: columnName, TableColumnIndex: col, TableRowIndex: row})
			}

			switch key {
			case tcell.KeyTab:
				nextEditableColumnIndex := col + 1

				if nextEditableColumnIndex <= table.GetColumnCount()-1 {
					table.Select(row, nextEditableColumnIndex)

					table.StartEditingCell(row, nextEditableColumnIndex, callback)

				}
			case tcell.KeyBacktab:
				nextEditableColumnIndex := col - 1

				if nextEditableColumnIndex >= 0 {
					table.Select(row, nextEditableColumnIndex)

					table.StartEditingCell(row, nextEditableColumnIndex, callback)

				}
			}

		}

		if key == tcell.KeyEnter || key == tcell.KeyEscape {
			if appendErr != nil {
				cell.Text = initialText
			}

			table.SetInputCapture(table.tableInputCapture)
			table.Page.RemovePage(pageNameTableEditCell)

			if appendErr != nil {
				table.SetError(appendErr.Error(), nil)
			} else {
				App.SetFocus(table)
			}
		}

		if callback != nil {
			callback(newValue, row, col)
		}
	})

	x, y, width := cell.GetLastPosition()
	inputField.SetRect(x-1, y-1, width+3, 3)
	table.Page.AddPage(pageNameTableEditCell, inputField, false, true)
	App.SetFocus(inputField)
}

func (table *ResultsTable) handleShowJSONViewer(command commands.Command) {
	selectedRow, selectedCol := table.GetSelection()
	if selectedRow == 0 { // It's the header
		return
	}

	// is an inserted row, do not show json viewer
	isAnInsertedRow, _ := table.isAnInsertedRow(selectedRow)
	if isAnInsertedRow {
		return
	}

	rowData := make(map[string]string)

	switch command {
	case commands.ShowRowJSONViewer:
		for i := 0; i < table.GetColumnCount(); i++ {
			columnName := table.GetColumnNameByIndex(i)
			cellValue := table.GetCell(selectedRow, i).Text
			rowData[columnName] = cellValue
		}
	case commands.ShowCellJSONViewer:
		columnName := table.GetColumnNameByIndex(selectedCol)
		cellValue := table.GetCell(selectedRow, selectedCol).Text
		rowData[columnName] = cellValue
	}

	table.jsonViewer.Show(rowData, table)
}

func (table *ResultsTable) CheckIfRowIsInserted(rowID string) bool {
	for _, dmlChange := range *table.state.listOfDBChanges {
		if dmlChange.Type == models.DMLInsertType && dmlChange.PrimaryKeyInfo[0].Value == rowID {
			return true
		}
	}

	return false
}

func (table *ResultsTable) MutateInsertedRowCell(rowID string, newValue models.CellValue) {
	for i, dmlChange := range *table.state.listOfDBChanges {
		if dmlChange.PrimaryKeyInfo[0].Value == rowID && dmlChange.Type == models.DMLInsertType {
			for j, v := range dmlChange.Values {
				if v.Column == newValue.Column {
					(*table.state.listOfDBChanges)[i].Values[j] = newValue
					break
				}
			}
		}
	}
}

func (table *ResultsTable) AppendNewChange(changeType models.DMLType, rowIndex int, colIndex int, value models.CellValue) error {
	// case models.Empty:
	// placeholders = append(placeholders, "")
	databaseName := table.GetDatabaseName()
	tableName := table.GetTableName()

	dmlChangeAlreadyExists := false

	// If the column has a reference, it means it's an inserted rowIndex
	// There is maybe a better way to detect it is an inserted row
	tableCell := table.GetCell(rowIndex, colIndex)
	tableCellReference := tableCell.GetReference()

	isAnInsertedRow, _ := table.isAnInsertedRow(rowIndex)

	if isAnInsertedRow {
		if changeType == models.DMLUpdateType {
			switch value.Type {
			case models.Null, models.Empty, models.Default:
				tableCell.SetText(value.Value.(string))
				tableCell.SetStyle(tcell.StyleDefault.Italic(true))
			}
		}
		table.MutateInsertedRowCell(tableCellReference.(string), value)
		return nil
	}

	rowPrimaryKeyInfo := table.GetPrimaryKeyValue(rowIndex)

	if len(rowPrimaryKeyInfo) == 0 {
		return fmt.Errorf("primary key not found for row %d", rowIndex)
	}

	if changeType == models.DMLUpdateType {
		switch value.Type {
		case models.Null, models.Empty, models.Default:
			tableCell.SetText(value.Value.(string))
			tableCell.SetStyle(tcell.StyleDefault.Italic(true))
			tableCell.SetReference(value.Value.(string) + "&")
		}
	}

	for i, dmlChange := range *table.state.listOfDBChanges {
		changeExistOnSameCell := false

		for _, value := range dmlChange.Values {
			if value.TableRowIndex == rowIndex && value.TableColumnIndex == colIndex {
				changeExistOnSameCell = true
				break
			}
		}

		if dmlChange.Table == tableName && dmlChange.Type == changeType && changeExistOnSameCell {
			dmlChangeAlreadyExists = true

			changeForColExists := false
			valueIndex := -1

			for j, v := range dmlChange.Values {
				if v.Column == value.Column {
					changeForColExists = true
					valueIndex = j
					break
				}
			}

			switch changeType {
			case models.DMLUpdateType:
				originalValue := table.GetRecords()[rowIndex][colIndex]

				if changeForColExists {
					if originalValue == value.Value {
						if len((*table.state.listOfDBChanges)[i].Values) == 1 {
							*table.state.listOfDBChanges = append((*table.state.listOfDBChanges)[:i], (*table.state.listOfDBChanges)[i+1:]...)
						} else {
							(*table.state.listOfDBChanges)[i].Values = append((*table.state.listOfDBChanges)[i].Values[:valueIndex], (*table.state.listOfDBChanges)[i].Values[valueIndex+1:]...)
						}
						table.SetCellColor(rowIndex, colIndex, app.Styles.PrimitiveBackgroundColor)
					} else {
						(*table.state.listOfDBChanges)[i].Values[valueIndex] = value
					}
				} else {
					(*table.state.listOfDBChanges)[i].Values = append((*table.state.listOfDBChanges)[i].Values, value)
					table.SetCellColor(rowIndex, colIndex, colorTableChange)
				}

			case models.DMLDeleteType:
				*table.state.listOfDBChanges = append((*table.state.listOfDBChanges)[:i], (*table.state.listOfDBChanges)[i+1:]...)
				table.SetRowColor(rowIndex, app.Styles.PrimitiveBackgroundColor)
			}
		}
	}

	if !dmlChangeAlreadyExists {
		switch changeType {
		case models.DMLDeleteType:
			table.SetRowColor(rowIndex, colorTableDelete)
		case models.DMLUpdateType:
			tableCell.SetStyle(tcell.StyleDefault.Background(colorTableChange))
			table.SetCellColor(rowIndex, colIndex, colorTableChange)
		}

		newDMLChange := models.DBDMLChange{
			Type:           changeType,
			Database:       databaseName,
			Table:          tableName,
			Values:         []models.CellValue{value},
			PrimaryKeyInfo: rowPrimaryKeyInfo,
		}

		*table.state.listOfDBChanges = append(*table.state.listOfDBChanges, newDMLChange)
	}

	return nil
}

func (table *ResultsTable) GetPrimaryKeyValue(rowIndex int) []models.PrimaryKeyInfo {
	primaryKeyColumnNames := table.GetPrimaryKeyColumnNames()

	info := []models.PrimaryKeyInfo{}

	if len(primaryKeyColumnNames) == 0 {
		allRecords := table.GetRecords()
		// Unsaved-new-row deletions shrink the record slice while the
		// caller still passes the original rowIndex; without these
		// guards lazysql panicked with 'index out of range' instead of
		// no-op'ing the second delete.
		if len(allRecords) == 0 || rowIndex < 0 || rowIndex >= len(allRecords) {
			return info
		}
		columns := allRecords[0]
		row := allRecords[rowIndex]
		for i, colName := range columns {
			if i >= len(row) {
				break
			}
			info = append(info, models.PrimaryKeyInfo{Name: colName, Value: row[i]})
		}
		return info
	}

	for _, primaryKeyColumnName := range primaryKeyColumnNames {
		columnIndex := table.GetColumnIndexByName(primaryKeyColumnName)
		records := table.GetRecords()
		if rowIndex < 0 || rowIndex >= len(records) {
			continue
		}
		row := records[rowIndex]
		if columnIndex < 0 || columnIndex >= len(row) {
			continue
		}
		primaryKeyValue := row[columnIndex]

		info = append(info, models.PrimaryKeyInfo{Name: primaryKeyColumnName, Value: primaryKeyValue})
	}

	return info
}

func (table *ResultsTable) SetRowColor(rowIndex int, color tcell.Color) {
	for i := 0; i < table.GetColumnCount(); i++ {
		table.GetCell(rowIndex, i).SetBackgroundColor(color)
	}
}

func (table *ResultsTable) SetCellColor(rowIndex int, colIndex int, color tcell.Color) {
	table.GetCell(rowIndex, colIndex).SetBackgroundColor(color)
}

// toggleRowMark adds or removes a row from the selection used by Copy.
// The header row (index 0) can never be marked.
func (table *ResultsTable) toggleRowMark(rowIndex int) {
	if rowIndex <= 0 {
		return
	}

	if table.state.markedRows == nil {
		table.state.markedRows = map[int]bool{}
	}

	if table.state.markedRows[rowIndex] {
		delete(table.state.markedRows, rowIndex)
		table.SetRowColor(rowIndex, tcell.ColorDefault)
		// Restore any change/delete highlighting the row had before it was marked.
		table.colorChangedCells()
	} else {
		table.state.markedRows[rowIndex] = true
		table.SetRowColor(rowIndex, colorTableMarked)
	}
}

// clearRowMarks drops every marked row. It is called whenever the table
// content is rebuilt so a mark can never point at a stale row.
func (table *ResultsTable) clearRowMarks() {
	table.state.markedRows = map[int]bool{}
}

// GetMarkedRowIndexes returns the marked row indexes ordered from top to bottom.
func (table *ResultsTable) GetMarkedRowIndexes() []int {
	indexes := make([]int, 0, len(table.state.markedRows))
	for rowIndex := range table.state.markedRows {
		indexes = append(indexes, rowIndex)
	}
	slices.Sort(indexes)
	return indexes
}

// markedRowsToText renders the marked rows as tab separated values, one row
// per line, ordered from top to bottom. Out of range indexes are skipped so a
// stale mark can never cause a panic.
func (table *ResultsTable) markedRowsToText() string {
	columnCount := table.GetColumnCount()
	rowCount := table.GetRowCount()

	var builder strings.Builder
	wroteRow := false

	for _, rowIndex := range table.GetMarkedRowIndexes() {
		if rowIndex <= 0 || rowIndex >= rowCount {
			continue
		}

		if wroteRow {
			builder.WriteByte('\n')
		}

		for columnIndex := 0; columnIndex < columnCount; columnIndex++ {
			if columnIndex > 0 {
				builder.WriteByte('\t')
			}
			builder.WriteString(table.getRawCellValue(rowIndex, columnIndex))
		}

		wroteRow = true
	}

	return builder.String()
}

func (table *ResultsTable) appendNewRow() {
	dbColumns := table.GetColumns()
	newRowTableIndex := table.GetRowCount()
	newRowUUID := uuid.New().String()
	newRow := make([]models.CellValue, len(dbColumns)-1)

	for i, column := range dbColumns {
		if i != 0 { // Skip the first row because they are the column names (e.x "Field", "Type", "Null", "Key", "Default", "Extra")
			newRow[i-1] = models.CellValue{Type: models.Default, Column: column[0], Value: "DEFAULT", TableRowIndex: newRowTableIndex, TableColumnIndex: i}
		}
	}

	newInsert := models.DBDMLChange{
		Type:           models.DMLInsertType,
		Database:       table.GetDatabaseName(),
		Table:          table.GetTableName(),
		Values:         newRow,
		PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: "", Value: newRowUUID}},
	}

	*table.state.listOfDBChanges = append(*table.state.listOfDBChanges, newInsert)

	table.AppendNewRow(newRow, newRowTableIndex, newRowUUID)

	table.StartEditingCell(newRowTableIndex, 0, nil)
}

func (table *ResultsTable) duplicateRow() {
	row, _ := table.GetSelection()
	if row <= 0 { // ignorer l'en-tête
		return
	}

	dbColumns := table.GetColumns()
	newRowTableIndex := row + 1 // table.GetRowCount()
	newRowUUID := uuid.New().String()
	newRow := make([]models.CellValue, len(dbColumns)-1)

	for i, column := range dbColumns {
		if i != 0 { // Skip the first row because they are the column names (e.x "Field", "Type", "Null", "Key", "Default", "Extra")
			origCell := table.GetCell(row, i-1)
			newRow[i-1] = models.CellValue{Type: models.String, Column: column[0], Value: origCell.Text, TableRowIndex: newRowTableIndex, TableColumnIndex: i}
		}
	}

	newInsert := models.DBDMLChange{
		Type:           models.DMLInsertType,
		Database:       table.GetDatabaseName(),
		Table:          table.GetTableName(),
		Values:         newRow,
		PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: "", Value: newRowUUID}},
	}

	*table.state.listOfDBChanges = append(*table.state.listOfDBChanges, newInsert)

	table.InsertRow(newRowTableIndex)

	table.AppendNewRow(newRow, newRowTableIndex, newRowUUID)

	table.StartEditingCell(newRowTableIndex, 0, nil)
}

func (table *ResultsTable) search() {
	if table.Editor != nil {
		App.SetFocus(table.Editor)
		table.Editor.Highlight()
		table.RemoveHighlightTable()
		table.SetIsFiltering(true)
		return
	}

	App.SetFocus(table.Filter.Input)
	table.RemoveHighlightTable()
	table.Filter.HighlightLocal()
	table.SetIsFiltering(true)

	if table.Filter.Input.GetText() == "/" {
		go table.Filter.Input.SetText("")
	}

	table.Filter.Input.SetAutocompleteFunc(func(currentText string) []string {
		split := strings.Split(currentText, " ")
		comparators := []string{
			"=", "!=",
			">", "<",
			">=", "<=",
			"between", "not between",
			"ilike", "not ilike",
			"in", "not in",
			"is", "is not",
			"like", "not like",
			"regexp", "not regexp",
		}

		matches := []string{}

		switch len(split) {
		case 1:
			cols := table.GetColumns()
			if len(cols) > 1 {
				for _, col := range cols[1:] {
					matches = append(matches, col[0])
				}
			}

		case 2:
			for _, comparator := range comparators {
				if strings.HasPrefix(comparator, split[1]) {
					matches = append(matches, fmt.Sprintf("%s %s", split[0], comparator))
				}
			}

		case 3:
			switch split[1] {
			case "not":
				comparators = []string{"between", "ilike", "in", "like", "regexp"}
			case "is":
				comparators = []string{"not", "null"}
			default:
				return matches
			}
			for _, comparator := range comparators {
				if strings.HasPrefix(comparator, split[2]) {
					matches = append(matches, fmt.Sprintf("%s %s %s", split[0], split[1], comparator))
				}
			}

		case 4:
			switch split[2] {
			case "not":
				comparators = []string{"null"}
			case "is":
				comparators = []string{"not", "null"}
			default:
				return matches
			}
			for _, comparator := range comparators {
				if strings.HasPrefix(comparator, split[3]) {
					matches = append(matches,
						fmt.Sprintf("%s %s %s %s", split[0], split[1], split[2], comparator),
					)
				}
			}
		}

		return matches
	})

	table.Filter.Input.SetAutocompletedFunc(func(text string, _ int, source int) bool {
		if source != tview.AutocompletedNavigate {
			inputText := strings.Split(table.Filter.Input.GetText(), " ")

			if len(inputText) == 1 {
				table.Filter.Input.SetText(fmt.Sprintf("%s =", text))
			} else if len(inputText) == 2 {
				table.Filter.Input.SetText(fmt.Sprintf("%s %s", inputText[0], text))
			}

			table.Filter.Input.SetText(text)
		}
		return source == tview.AutocompletedEnter || source == tview.AutocompletedClick
	})

	table.SetInputCapture(nil)
}

func (table *ResultsTable) FinishSettingValue() {
	table.SetIsEditing(false)
	table.SetInputCapture(table.tableInputCapture)
	App.SetFocus(table)
}

func (table *ResultsTable) GetItalicStyle() tcell.Style {
	return tcell.StyleDefault.Foreground(tview.Styles.InverseTextColor).Italic(true)
}

func (table *ResultsTable) ShowSidebar(show bool) {
	table.state.showSidebar = show

	if show {
		table.UpdateSidebar()

		if App.Config().SidebarOverlay {
			table.Page.SendToFront(pageNameSidebar)
			table.Page.ShowPage(pageNameSidebar)
		} else {
			table.SidebarContainer.AddItem(table.Sidebar, 0, 1, true)
		}
	} else {
		if App.Config().SidebarOverlay {
			table.Page.HidePage(pageNameSidebar)
		} else {
			table.SidebarContainer.RemoveItem(table.Sidebar)
		}
		App.SetFocus(table)
	}
}

func (table *ResultsTable) handleForeignKeyEnter(selectedRowIndex, selectedColumnIndex int) bool {
	if selectedRowIndex <= 0 {
		return false
	}

	if table.Menu != nil && table.Menu.GetSelectedOption() != 1 {
		return false
	}

	if !table.IsForeignKeyJumpSupportedProvider() {
		return false
	}

	columnName := table.GetColumnNameByIndex(selectedColumnIndex)
	if columnName == "" {
		return false
	}

	if !table.isForeignKeyColumn(columnName) {
		return false
	}

	target, ok := table.getForeignKeyJumpTarget(columnName)
	if !ok {
		return true
	}

	rawValue := table.getRawCellValue(selectedRowIndex, selectedColumnIndex)
	if !isNavigableForeignKeyValue(rawValue) {
		return true
	}

	if table.Home == nil || table.Filter == nil {
		return true
	}

	where := fmt.Sprintf("WHERE %s = '%s'", table.DBDriver.FormatReference(target.ReferencedColumn), escapeSingleQuotes(rawValue))
	table.Home.ShowTableWithFilter(table.GetDatabaseName(), target.ReferencedTable, where)

	return true
}

func (table *ResultsTable) foreignKeyCellMapKey(rowIndex, columnIndex int) string {
	return fmt.Sprintf("%d:%d", rowIndex, columnIndex)
}

func (table *ResultsTable) shouldShowForeignKeyMarker(rowIndex, columnIndex int, rawValue string) bool {
	if table.Menu != nil && table.Menu.GetSelectedOption() != 1 {
		return false
	}

	if !table.IsForeignKeyJumpSupportedProvider() {
		return false
	}

	columnName := table.GetColumnNameByIndex(columnIndex)
	if columnName == "" || !table.isForeignKeyColumn(columnName) {
		return false
	}

	if _, ok := table.getForeignKeyJumpTarget(columnName); !ok {
		return false
	}

	if !isNavigableForeignKeyValue(rawValue) {
		return false
	}

	if isInsertedRow, _ := table.isAnInsertedRow(rowIndex); isInsertedRow {
		return false
	}

	cell := table.GetCell(rowIndex, columnIndex)
	if cell != nil {
		switch cell.BackgroundColor {
		case colorTableDelete, colorTableChange, colorTableInsert:
			return false
		}
	}

	return true
}

func (table *ResultsTable) getRawCellValue(rowIndex, columnIndex int) string {
	key := table.foreignKeyCellMapKey(rowIndex, columnIndex)
	if value, ok := table.state.fkRawCellValues[key]; ok {
		return value
	}

	cell := table.GetCell(rowIndex, columnIndex)
	if cell == nil {
		return ""
	}

	return cell.Text
}

func (table *ResultsTable) isForeignKeyColumn(columnName string) bool {
	return table.state.foreignKeyColumns[columnName]
}

func (table *ResultsTable) getForeignKeyJumpTarget(columnName string) (foreignKeyJumpTarget, bool) {
	target, ok := table.state.foreignKeyJumpTargets[columnName]
	return target, ok
}

func (table *ResultsTable) rebuildForeignKeyJumpMetadata() {
	table.state.foreignKeyColumns = map[string]bool{}
	table.state.foreignKeyJumpTargets = map[string]foreignKeyJumpTarget{}

	if !table.IsForeignKeyJumpSupportedProvider() {
		return
	}

	if len(table.GetForeignKeys()) <= 1 {
		return
	}

	provider := table.DBDriver.GetProvider()
	headers := table.GetForeignKeys()[0]
	columnNameIndex, okColumn := foreignKeyHeaderIndex(headers, provider, "column_name", "from")
	foreignTableSchemaIndex, _ := foreignKeyHeaderIndex(headers, provider, "foreign_table_schema")
	referencedTableIndex, okTable := foreignKeyHeaderIndex(headers, provider, "foreign_table_name", "table", "referenced_table")
	referencedColumnIndex, okReferencedColumn := foreignKeyHeaderIndex(headers, provider, "foreign_column_name", "to", "referenced_column")
	constraintIndex, okConstraint := foreignKeyHeaderIndex(headers, provider, "constraint_name", "id")
	if !okColumn || !okTable || !okReferencedColumn || !okConstraint {
		return
	}

	constraintCounts := map[string]int{}
	for _, row := range table.GetForeignKeys()[1:] {
		if constraintIndex >= len(row) {
			continue
		}
		constraintName := strings.TrimSpace(row[constraintIndex])
		if constraintName == "" {
			continue
		}
		constraintCounts[constraintName]++
	}

	for _, row := range table.GetForeignKeys()[1:] {
		if columnNameIndex >= len(row) || referencedTableIndex >= len(row) || referencedColumnIndex >= len(row) || constraintIndex >= len(row) {
			continue
		}

		constraintName := strings.TrimSpace(row[constraintIndex])
		if constraintName == "" || constraintCounts[constraintName] != 1 {
			continue
		}

		columnName := strings.TrimSpace(row[columnNameIndex])
		foreignTableSchema := ""
		if foreignTableSchemaIndex >= 0 && foreignTableSchemaIndex < len(row) {
			foreignTableSchema = strings.TrimSpace(row[foreignTableSchemaIndex])
		}
		referencedTable := strings.TrimSpace(row[referencedTableIndex])
		referencedColumn := strings.TrimSpace(row[referencedColumnIndex])

		if columnName == "" || referencedTable == "" || referencedColumn == "" {
			continue
		}

		table.state.foreignKeyColumns[columnName] = true
		table.state.foreignKeyJumpTargets[columnName] = foreignKeyJumpTarget{
			ReferencedTable:  table.normalizeForeignKeyReferencedTable(referencedTable, foreignTableSchema),
			ReferencedColumn: referencedColumn,
		}
	}
}

func (table *ResultsTable) normalizeForeignKeyReferencedTable(referencedTable string, foreignTableSchema string) string {
	provider := table.DBDriver.GetProvider()
	if provider == drivers.DriverPostgres {
		if strings.Contains(referencedTable, ".") {
			return referencedTable
		}

		if foreignTableSchema != "" {
			return foreignTableSchema + "." + referencedTable
		}

		currentTable := table.GetTableName()
		if strings.Contains(currentTable, ".") {
			schema := strings.SplitN(currentTable, ".", 2)[0]
			if schema != "" {
				return schema + "." + referencedTable
			}
		}
	}

	return referencedTable
}

func (table *ResultsTable) IsForeignKeyJumpSupportedProvider() bool {
	provider := table.DBDriver.GetProvider()
	return provider == drivers.DriverPostgres || provider == drivers.DriverSqlite || provider == drivers.DriverMSSQL
}

func foreignKeyHeaderIndex(headers []string, provider string, keys ...string) (int, bool) {
	for i, header := range headers {
		normalized := strings.ToLower(strings.TrimSpace(header))
		for _, key := range keys {
			if normalized == key {
				return i, true
			}
		}
	}

	if provider == drivers.DriverPostgres {
		for i, header := range headers {
			normalized := strings.ToLower(strings.TrimSpace(header))
			for _, key := range keys {
				if strings.Contains(normalized, key) {
					return i, true
				}
			}
		}
	}

	return -1, false
}

func isNavigableForeignKeyValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}

	switch strings.ToUpper(trimmed) {
	case "NULL", "EMPTY", "DEFAULT", "NULL&", "EMPTY&", "DEFAULT&":
		return false
	default:
		return true
	}
}

func escapeSingleQuotes(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func (table *ResultsTable) UpdateSidebar() {
	columns := table.GetColumns()
	selectedRow, _ := table.GetSelection()

	if selectedRow > 0 {

		if App.Config().SidebarOverlay {
			table.recomputeSidebarPosition()
		}

		table.Sidebar.Clear()

		isRecordsTab := table.Menu == nil || table.Menu.GetSelectedOption() == 1

		for i := 1; i <= table.GetColumnCount(); i++ {
			name := table.GetCell(0, i-1).Text

			colType := ""
			if isRecordsTab && i < len(columns) {
				colType = columns[i][1]
			}

			sidebarWidth := table.getSidebarWidth()

			text := table.GetCell(selectedRow, i-1).Text
			title := name

			repeatCount := sidebarWidth - len(name) - len(colType) - 4 // idk why 4 is needed, but it works.

			if repeatCount <= 0 {
				repeatCount = 1
			}

			title += fmt.Sprintf("[%s]", app.Styles.SidebarTitleBorderColor) + strings.Repeat("-", repeatCount)
			title += colType

			pendingEditExist := false

			for _, dmlChange := range *table.state.listOfDBChanges {
				if dmlChange.Type == models.DMLUpdateType {
					for _, v := range dmlChange.Values {
						if v.Column == name && v.TableRowIndex == selectedRow && v.TableColumnIndex == i-1 {
							pendingEditExist = true
							break
						}
					}
				}
			}

			table.Sidebar.AddField(title, text, sidebarWidth, pendingEditExist)
		}

	}
}

func (table *ResultsTable) getSidebarWidth() int {
	if App.Config().SidebarOverlay {
		_, _, tableInnerWidth, _ := table.GetInnerRect()
		return (tableInnerWidth / 4)
	}

	_, _, width, _ := table.SidebarContainer.GetInnerRect()

	return width
}

// Only used when AppConfig.SidebarOverlay is true.
func (table *ResultsTable) recomputeSidebarPosition() {
	tableX, _, _, tableHeight := table.GetRect()
	_, _, tableInnerWidth, _ := table.GetInnerRect()
	_, tableMenuY, _, tableMenuHeight := table.Menu.GetRect()
	_, _, _, tableFilterHeight := table.Filter.GetRect()
	_, _, _, tablePaginationHeight := table.Pagination.GetRect()

	sidebarWidth := (tableInnerWidth / 4)
	sidebarHeight := tableHeight + tableMenuHeight + tableFilterHeight + tablePaginationHeight + 1

	table.Sidebar.SetRect(tableX+tableInnerWidth-sidebarWidth, tableMenuY, sidebarWidth, sidebarHeight)
}

func (table *ResultsTable) isAnInsertedRow(rowIndex int) (isAnInsertedRow bool, DBChangeIndex int) {
	for i, dmlChange := range *table.state.listOfDBChanges {
		for _, value := range dmlChange.Values {
			if value.TableRowIndex != rowIndex {
				continue
			}
			cellReference := table.GetCell(rowIndex, 0).GetReference()
			if cellReference == nil {
				break
			}
			switch cellReference.(string) {
			case "NULL&", "EMPTY&", "DEFAULT&":
			default:
				return true, i
			}
			break
		}
	}
	return false, -1
}

func (table *ResultsTable) colorChangedCells() {
	tableName := table.GetTableName()
	databaseName := table.GetDatabaseName()

	for _, dmlChange := range *table.state.listOfDBChanges {
		if dmlChange.Table != tableName || dmlChange.Database != databaseName {
			continue
		}

		switch dmlChange.Type {
		case models.DMLDeleteType:
			table.SetRowColor(dmlChange.Values[0].TableRowIndex, colorTableDelete)
		case models.DMLUpdateType:
			for _, value := range dmlChange.Values {
				table.SetCellColor(value.TableRowIndex, value.TableColumnIndex, colorTableChange)
			}
		}
	}
}

func (table *ResultsTable) GetPrimitive() tview.Primitive {
	return table.Page
}

func (table *ResultsTable) showCSVExportModal() {
	databaseName := table.GetDatabaseName()
	tableName := table.GetTableName()
	isQueryResult := table.Editor != nil
	// Editor tabs are the only non-paginated ResultsTable instances. A table
	// view keeps both scopes even while its identity is still being populated.
	hasPagination := !isQueryResult

	rowCount := max(len(table.GetRecords())-1, 0)
	canExportAll := isQueryResult && table.canExportAllQueryResults()
	unavailableReason := ""
	if isQueryResult && !canExportAll {
		unavailableReason = table.queryExportAllUnavailableReason()
	}
	opts := CSVExportOptions{
		DatabaseName:               cmp.Or(databaseName, "database"),
		TableName:                  cmp.Or(tableName, "query_result"),
		HasPagination:              hasPagination,
		RowCount:                   rowCount,
		IsQueryResult:              isQueryResult,
		CanExportAll:               canExportAll,
		ExportAllUnavailableReason: unavailableReason,
	}

	modal := NewCSVExportModal(opts, func(filePath string, scope CSVExportScope, batchSize int) {
		query := table.lastEditorQuery()
		where := ""
		sort := ""
		if !isQueryResult {
			if table.Filter != nil {
				where = table.Filter.GetCurrentFilter()
			}
			sort = cmp.Or(table.GetCurrentSort(), table.GetPrimaryKeySort())
			if sort == "" {
				if records := table.GetRecords(); len(records) > 0 && len(records[0]) > 0 {
					sort = records[0][0] + " ASC"
				}
			}
		}

		run := table.beginCSVExport()
		run.finalPath = filePath
		App.SetFocus(table)
		App.ForceDraw()

		go func() {
			if run.ctx.Err() != nil {
				return
			}

			progress := func(rows int) {
				table.updateCSVExportProgress(run, rows)
			}
			var exportErr error

			switch {
			case scope == ExportCurrentPage || scope == ExportVisibleResults || (isQueryResult && scope == ExportAllRecords):
				_, exportErr = table.exportCurrentPageWithContext(run.ctx, filePath, progress)
			case isQueryResult && scope == ExportAllResults:
				_, exportErr = table.exportAllQueryResults(run.ctx, filePath, query, progress)
			case !isQueryResult && scope == ExportAllRecords:
				_, exportErr = table.exportAllRecordsInBatchesWithProgress(
					run.ctx, filePath, databaseName, tableName, where, sort, batchSize, progress,
				)
			default:
				exportErr = errors.New("unsupported CSV export scope")
			}

			if run.ctx.Err() != nil && exportErr == nil {
				exportErr = run.ctx.Err()
			}

			App.QueueUpdateDraw(func() {
				table.finishCSVExport(run, exportErr)
				App.ForceDraw()
			})
		}()
	})

	mainPages.AddPage(pageNameCSVExport, modal, true, true)
}

func (table *ResultsTable) canExportAllQueryResults() bool {
	query := table.lastEditorQuery()
	resultAvailable := table.state.editorResultAvailable || len(table.state.records) > 0
	if query == "" || !table.state.lastEditorQueryReplaySafe || !isReplaySafeQuery(query) || !resultAvailable {
		return false
	}
	_, streamingSupported := table.DBDriver.(drivers.QueryStreamer)
	return streamingSupported
}

func (table *ResultsTable) queryExportAllUnavailableReason() string {
	query := table.lastEditorQuery()
	if query == "" || !table.state.lastEditorQueryReplaySafe || !isReplaySafeQuery(query) {
		return "query is not classified as replay-safe"
	}
	if !table.state.editorResultAvailable && len(table.state.records) == 0 {
		return "no query result has been shown"
	}
	if table.DBDriver == nil {
		return "no streaming database driver is available"
	}
	if _, ok := table.DBDriver.(drivers.QueryStreamer); !ok {
		return "the database driver does not support streaming"
	}
	return "Export All is unavailable"
}

func (table *ResultsTable) lastEditorQuery() string {
	return table.state.lastEditorQuery
}

// exportCurrentPage exports the current page records (already in memory) to CSV.
// Returns the number of rows written (excluding header) and any error.
func (table *ResultsTable) exportCurrentPage(filePath string) (int, error) {
	return table.exportCurrentPageWithContext(context.Background(), filePath, nil)
}

func (table *ResultsTable) exportCurrentPageWithContext(ctx context.Context, filePath string, onProgress func(int)) (rows int, err error) {
	started := time.Now()
	defer func() {
		logDatabaseOperation("export_visible_results", started, ctx, map[string]any{
			"connection": table.connectionIdentifier,
			"rows":       rows,
		}, err)
	}()

	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	records := table.GetRecords()
	writer, err := helpers.NewCSVWriter(filePath)
	if err != nil {
		return 0, err
	}
	defer writer.Abort()

	if err := writer.WriteRecords(records, true); err != nil {
		return 0, err
	}
	if onProgress != nil {
		onProgress(writer.RowCount())
	}
	if err := ctx.Err(); err != nil {
		return writer.RowCount(), err
	}
	if err := writer.Commit(); err != nil {
		return writer.RowCount(), err
	}
	return writer.RowCount(), nil
}

// exportAllRecordsInBatches retains the original testable table-export seam.
func (table *ResultsTable) exportAllRecordsInBatches(
	ctx context.Context,
	filePath, databaseName, tableName, where, sort string,
	batchSize int,
) (int, error) {
	return table.exportAllRecordsInBatchesWithProgress(ctx, filePath, databaseName, tableName, where, sort, batchSize, nil)
}

// exportAllRecordsInBatches exports all records using bounded page fetching.
// It stops only when a returned page proves the end, never asking for an exact
// count and never consulting the interactive max_query_rows cap.
func (table *ResultsTable) exportAllRecordsInBatchesWithProgress(
	ctx context.Context,
	filePath, databaseName, tableName, where, sort string,
	batchSize int,
	onProgress func(int),
) (rows int, err error) {
	started := time.Now()
	defer func() {
		logDatabaseOperation("export_all_records", started, ctx, map[string]any{
			"database": databaseName,
			"table":    tableName,
			"rows":     rows,
		}, err)
	}()

	if ctx == nil {
		ctx = context.Background()
	}
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	writer, err := helpers.NewCSVWriter(filePath)
	if err != nil {
		return 0, err
	}
	defer writer.Abort()

	for offset := 0; ; offset += batchSize {
		if err := ctx.Err(); err != nil {
			return writer.RowCount(), err
		}
		page, err := table.DBDriver.GetRecords(
			ctx, databaseName, tableName, where, sort, offset, batchSize,
		)
		if err != nil {
			return writer.RowCount(), err
		}
		if err := ctx.Err(); err != nil {
			return writer.RowCount(), err
		}

		if len(page.Rows) == 0 {
			break
		}
		if err := writer.WriteRecords(page.Rows, offset == 0); err != nil {
			return writer.RowCount(), err
		}
		if onProgress != nil {
			onProgress(writer.RowCount())
		}

		// A header-only batch is definitive even if a faulty driver reports a
		// stale HasNextPage flag.
		if len(page.Rows) <= 1 || !page.HasNextPage {
			break
		}
	}

	if err := ctx.Err(); err != nil {
		return writer.RowCount(), err
	}
	if err := writer.Commit(); err != nil {
		return writer.RowCount(), err
	}
	return writer.RowCount(), nil
}

// exportAllQueryResults reexecutes one replay-safe editor statement and writes
// every streamed row. maxRows is deliberately zero: interactive result caps
// must never limit a full export.
func (table *ResultsTable) exportAllQueryResults(
	ctx context.Context,
	filePath, query string,
	onProgress func(int),
) (rows int, err error) {
	started := time.Now()
	defer func() {
		logDatabaseOperation("export_all_query_results", started, ctx, map[string]any{
			"connection": table.connectionIdentifier,
			"rows":       rows,
		}, err)
	}()

	if !isReplaySafeQuery(query) {
		return 0, errors.New("query is not classified as replay-safe for Export All Results")
	}
	streamer, ok := table.DBDriver.(drivers.QueryStreamer)
	if !ok {
		return 0, errors.New("driver does not support streaming Export All Results")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	writer, err := helpers.NewCSVWriter(filePath)
	if err != nil {
		return 0, err
	}
	defer writer.Abort()

	headerWritten := false
	var columns []string
	onBatch := func(batch drivers.QueryBatch) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		batchColumns := batch.Columns
		if len(batchColumns) == 0 {
			batchColumns = columns
		} else {
			columns = append(columns[:0], batchColumns...)
		}
		if len(batch.Rows) > 0 && len(batchColumns) == 0 {
			return errors.New("query stream returned rows without columns")
		}
		if err := writer.WriteBatch(batchColumns, batch.Rows, !headerWritten); err != nil {
			return err
		}
		if len(batchColumns) > 0 {
			headerWritten = true
		}
		if onProgress != nil {
			onProgress(writer.RowCount())
		}
		return ctx.Err()
	}

	result, err := streamer.StreamQuery(ctx, query, 0, onBatch)
	if err != nil {
		return writer.RowCount(), err
	}
	if err := ctx.Err(); err != nil {
		return writer.RowCount(), err
	}
	if !headerWritten && len(result.Columns) > 0 {
		if err := writer.WriteBatch(result.Columns, nil, true); err != nil {
			return writer.RowCount(), err
		}
		headerWritten = true
		if onProgress != nil {
			onProgress(writer.RowCount())
		}
	}
	if err := writer.Commit(); err != nil {
		return writer.RowCount(), err
	}
	return writer.RowCount(), nil
}

func (table *ResultsTable) showExportSuccessModal(filePath string, rowCount int) {
	modal := tview.NewModal()
	modal.SetText(fmt.Sprintf("Successfully exported %d rows to:\n%s", rowCount, filePath))
	modal.AddButtons([]string{"OK"})
	modal.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	modal.SetBorderStyle(tcell.StyleDefault.Background(app.Styles.PrimitiveBackgroundColor))
	modal.SetTextColor(app.Styles.PrimaryTextColor)
	modal.SetButtonActivatedStyle(
		tcell.StyleDefault.
			Background(app.Styles.InverseTextColor).
			Foreground(app.Styles.ContrastSecondaryTextColor),
	)
	modal.SetDoneFunc(func(_ int, _ string) {
		mainPages.RemovePage(pageNameCSVExportSuccess)
	})

	mainPages.AddPage(pageNameCSVExportSuccess, modal, true, true)
	App.SetFocus(modal)
}

// openCellInExternalEditor opens the user's preferred editor to edit a cell value.
// It should be called within app.Suspend() to ensure the TUI is properly restored.
func openCellInExternalEditor(currentText string) string {
	tmpFile, err := os.CreateTemp("", "lazysql-cell-*.txt")
	if err != nil {
		logger.Error("Failed to create temporary file", map[string]any{"error": err.Error()})
		return currentText
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(currentText); err != nil {
		logger.Error("Failed to write to temporary file", map[string]any{"error": err.Error()})
		if closeErr := tmpFile.Close(); closeErr != nil {
			logger.Error("Failed to close temporary file", map[string]any{"error": closeErr.Error()})
		}
		return currentText
	}

	if err := tmpFile.Close(); err != nil {
		logger.Error("Failed to close temporary file", map[string]any{"error": err.Error()})
		return currentText
	}

	editorParts := getCellEditorParts()
	editorArgs := append(editorParts[1:], tmpFile.Name())

	cmd := exec.Command(editorParts[0], editorArgs...) //nolint:gosec // editor comes from trusted $EDITOR env var
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Error executing editor", map[string]any{"error": err.Error(), "command": cmd.String()})
		return currentText
	}

	updatedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		logger.Error("Failed to read from temporary file", map[string]any{"error": err.Error()})
		return currentText
	}

	return string(updatedContent)
}

// getCellEditorParts returns the editor command and its arguments for cell editing.
// Supports editor strings with flags like "code --wait" or "vim -u NONE".
func getCellEditorParts() []string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	return strings.Fields(editor)
}

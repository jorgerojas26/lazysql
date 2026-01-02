package components

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers/logger"
)

type DocumentTableState struct {
	error          string
	databaseName   string
	collectionName string
	documents      []drivers.Document
	schema         drivers.Schema
	indexes        []drivers.Index
	isFiltering    bool
	isLoading      bool
	currentView    string // "documents", "schema", or "indexes"
}

type DocumentTable struct {
	*tview.Table
	state                *DocumentTableState
	Page                 *tview.Pages
	Wrapper              *tview.Flex
	Menu                 *ResultsTableMenu
	Filter               *ResultsTableFilter
	Error                *tview.Modal
	Loading              *tview.Modal
	Pagination           *Pagination
	Tree                 *NoSQLTree
	DBDriver             drivers.NoSQLDriver
	connectionIdentifier string
	ConnectionURL        string
	jsonViewer           *JSONViewer
}

const maxCellDisplayLength = 100 // Max characters to display in a cell

func NewDocumentTable(tree *NoSQLTree, dbdriver drivers.NoSQLDriver, connectionIdentifier string, connectionURL string) *DocumentTable {
	state := &DocumentTableState{
		documents:   []drivers.Document{},
		isLoading:   false,
		isFiltering: false,
		currentView: "documents", // Default view
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

	loadingModal := tview.NewModal()
	loadingModal.SetText("Loading...")
	loadingModal.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	loadingModal.SetBorderStyle(tcell.StyleDefault.Background(app.Styles.PrimitiveBackgroundColor))
	loadingModal.SetTextColor(app.Styles.SecondaryTextColor)

	pages := tview.NewPages()
	pages.AddPage(pageNameTable, wrapper, true, true)
	pages.AddPage(pageNameTableError, errorModal, true, false)
	pages.AddPage(pageNameTableLoading, loadingModal, false, false)

	pagination := NewPagination()

	docTable := &DocumentTable{
		Table:                tview.NewTable(),
		Page:                 pages,
		Wrapper:              wrapper,
		Error:                errorModal,
		Loading:              loadingModal,
		Pagination:           pagination,
		state:                state,
		DBDriver:             dbdriver,
		Tree:                 tree,
		connectionIdentifier: connectionIdentifier,
		ConnectionURL:        connectionURL,
	}

	// Initialize JSON viewer
	docTable.jsonViewer = NewJSONViewer(pages)

	docTable.SetFixed(1, 0)
	docTable.SetSelectable(true, true)                                                                                                          // Enable both row AND column selection
	docTable.SetBorders(true)                                                                                                                   // Show cell borders for grid appearance
	docTable.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor)) // Yellow highlight for selected cell

	errorModal.SetDoneFunc(func(_ int, buttonLabel string) {
		if buttonLabel == "Ok" {
			pages.SwitchToPage(pageNameTable)
		}
	})

	docTable.SetInputCapture(docTable.tableInputCapture)

	return docTable
}

func (dt *DocumentTable) WithFilter() *DocumentTable {
	// Create simple menu text
	menuText := tview.NewTextView()
	menuText.SetText("Documents [1] | Schema [2] | Indexes [3]")
	menuText.SetTextColor(app.Styles.PrimaryTextColor)
	menuText.SetBorder(true)
	menuText.SetDynamicColors(true)

	dt.Wrapper.AddItem(menuText, 3, 0, false)
	dt.Wrapper.AddItem(dt, 0, 1, true)
	dt.Wrapper.AddItem(dt.Pagination, 3, 0, false)

	return dt
}

// State management methods
func (dt *DocumentTable) SetDatabaseName(name string) {
	dt.state.databaseName = name
}

func (dt *DocumentTable) SetCollectionName(name string) {
	dt.state.collectionName = name
}

func (dt *DocumentTable) GetIsFiltering() bool {
	return dt.state.isFiltering
}

func (dt *DocumentTable) SetIsFiltering(isFiltering bool) {
	dt.state.isFiltering = isFiltering
}

func (dt *DocumentTable) GetIsLoading() bool {
	return dt.state.isLoading
}

func (dt *DocumentTable) SetIsLoading(isLoading bool) {
	dt.state.isLoading = isLoading

	if isLoading {
		dt.Page.ShowPage(pageNameTableLoading)
	} else {
		dt.Page.HidePage(pageNameTableLoading)
	}
}

// FetchDocuments retrieves documents from the collection
func (dt *DocumentTable) FetchDocuments(onError func()) []drivers.Document {
	dt.SetIsLoading(true)
	defer dt.SetIsLoading(false)

	// Get filter from filter UI if exists
	filter := drivers.Filter{}
	// TODO: Implement filter parsing
	// For now, empty filter returns all documents

	// Get sort - for now, no sorting
	sort := drivers.Sort{}

	offset := dt.Pagination.GetOffset()
	limit := dt.Pagination.GetLimit()

	documents, totalCount, err := dt.DBDriver.GetDocuments(
		dt.state.databaseName,
		dt.state.collectionName,
		filter,
		sort,
		offset,
		limit,
	)

	if err != nil {
		logger.Error("Failed to fetch documents", map[string]any{"error": err})
		dt.SetError(err.Error())
		if onError != nil {
			onError()
		}
		return nil
	}

	dt.state.documents = documents
	dt.state.error = ""
	dt.Pagination.SetTotalRecords(totalCount)

	dt.UpdateDocumentDisplay()

	return documents
}

// FetchSchema retrieves the inferred schema for the collection
func (dt *DocumentTable) FetchSchema() {
	dt.SetIsLoading(true)
	defer dt.SetIsLoading(false)

	schema, err := dt.DBDriver.GetSchema(
		dt.state.databaseName,
		dt.state.collectionName,
	)

	if err != nil {
		logger.Error("Failed to fetch schema", map[string]any{"error": err})
		dt.SetError(err.Error())
		return
	}

	dt.state.schema = schema
	dt.state.error = ""

	dt.UpdateSchemaDisplay()
}

// FetchIndexes retrieves indexes for the collection
func (dt *DocumentTable) FetchIndexes() {
	dt.SetIsLoading(true)
	defer dt.SetIsLoading(false)

	indexes, err := dt.DBDriver.GetIndexes(
		dt.state.databaseName,
		dt.state.collectionName,
	)

	if err != nil {
		logger.Error("Failed to fetch indexes", map[string]any{"error": err})
		dt.SetError(err.Error())
		return
	}

	dt.state.indexes = indexes
	dt.state.error = ""

	dt.UpdateIndexesDisplay()
}

// UpdateDocumentDisplay renders documents in the table
func (dt *DocumentTable) UpdateDocumentDisplay() {
	dt.Clear()

	if len(dt.state.documents) == 0 {
		// Show empty state
		cell := tview.NewTableCell("No documents found")
		cell.SetAlign(tview.AlignCenter)
		cell.SetTextColor(app.Styles.InverseTextColor)
		dt.SetCell(0, 0, cell)
		return
	}

	// Extract all unique field names from documents to create columns
	fieldNames := dt.extractFieldNames(dt.state.documents)

	// Set header row
	for col, fieldName := range fieldNames {
		cell := tview.NewTableCell(fieldName)
		cell.SetTextColor(app.Styles.PrimaryTextColor)
		cell.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
		cell.SetAlign(tview.AlignLeft)
		cell.SetSelectable(false)
		cell.SetExpansion(1) // Makes columns expand proportionally
		dt.SetCell(0, col, cell)
	}

	// Set document rows
	for row, doc := range dt.state.documents {
		for col, fieldName := range fieldNames {
			value := doc[fieldName]
			cellText := dt.formatCellValue(value)

			cell := tview.NewTableCell(cellText)
			cell.SetTextColor(app.Styles.PrimaryTextColor)
			cell.SetAlign(tview.AlignLeft)
			cell.SetExpansion(1)     // Makes columns expand proportionally
			cell.SetSelectable(true) // Data cells are selectable
			dt.SetCell(row+1, col, cell)
		}
	}

	// Select first data cell after display
	if len(dt.state.documents) > 0 {
		dt.Select(1, 0)
	}
}

// UpdateSchemaDisplay renders the schema in the table
func (dt *DocumentTable) UpdateSchemaDisplay() {
	dt.Clear()

	if len(dt.state.schema.Fields) == 0 {
		// Show empty state
		cell := tview.NewTableCell("No schema found")
		cell.SetAlign(tview.AlignCenter)
		cell.SetTextColor(app.Styles.InverseTextColor)
		dt.SetCell(0, 0, cell)
		return
	}

	// Header row
	headers := []string{"Field Name", "Type", "Has Nested"}
	for col, header := range headers {
		cell := tview.NewTableCell(header)
		cell.SetTextColor(app.Styles.PrimaryTextColor)
		cell.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
		cell.SetAlign(tview.AlignLeft)
		cell.SetSelectable(false)
		cell.SetExpansion(1) // Makes columns expand proportionally
		dt.SetCell(0, col, cell)
	}

	// Schema rows
	for row, field := range dt.state.schema.Fields {
		// Field name
		nameCell := tview.NewTableCell(field.Name)
		nameCell.SetTextColor(app.Styles.PrimaryTextColor)
		nameCell.SetExpansion(1)
		nameCell.SetSelectable(true)
		dt.SetCell(row+1, 0, nameCell)

		// Type
		typeCell := tview.NewTableCell(field.Type)
		typeCell.SetTextColor(app.Styles.PrimaryTextColor)
		typeCell.SetExpansion(1)
		typeCell.SetSelectable(true)
		dt.SetCell(row+1, 1, typeCell)

		// Has nested
		hasNested := "No"
		if len(field.Nested) > 0 {
			hasNested = "Yes"
		}
		nestedCell := tview.NewTableCell(hasNested)
		nestedCell.SetTextColor(app.Styles.PrimaryTextColor)
		nestedCell.SetExpansion(1)
		nestedCell.SetSelectable(true)
		dt.SetCell(row+1, 2, nestedCell)
	}

	// Select first data cell after display
	if len(dt.state.schema.Fields) > 0 {
		dt.Select(1, 0)
	}
}

// UpdateIndexesDisplay renders indexes in the table
func (dt *DocumentTable) UpdateIndexesDisplay() {
	dt.Clear()

	// Header row
	headers := []string{"Index Name", "Fields", "Type", "Unique"}
	for col, header := range headers {
		cell := tview.NewTableCell(header)
		cell.SetTextColor(app.Styles.PrimaryTextColor)
		cell.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
		cell.SetAlign(tview.AlignLeft)
		cell.SetSelectable(false)
		cell.SetExpansion(1) // Makes columns expand proportionally
		dt.SetCell(0, col, cell)
	}

	// Index rows
	for row, index := range dt.state.indexes {
		// Index name
		nameCell := tview.NewTableCell(index.Name)
		nameCell.SetTextColor(app.Styles.PrimaryTextColor)
		nameCell.SetExpansion(1)
		nameCell.SetSelectable(true)
		dt.SetCell(row+1, 0, nameCell)

		// Fields
		fieldsCell := tview.NewTableCell(strings.Join(index.Fields, ", "))
		fieldsCell.SetTextColor(app.Styles.PrimaryTextColor)
		fieldsCell.SetExpansion(1)
		fieldsCell.SetSelectable(true)
		dt.SetCell(row+1, 1, fieldsCell)

		// Type
		typeCell := tview.NewTableCell(index.Type)
		typeCell.SetTextColor(app.Styles.PrimaryTextColor)
		typeCell.SetExpansion(1)
		typeCell.SetSelectable(true)
		dt.SetCell(row+1, 2, typeCell)

		// Unique
		uniqueText := "No"
		if index.Unique {
			uniqueText = "Yes"
		}
		uniqueCell := tview.NewTableCell(uniqueText)
		uniqueCell.SetTextColor(app.Styles.PrimaryTextColor)
		uniqueCell.SetExpansion(1)
		uniqueCell.SetSelectable(true)
		dt.SetCell(row+1, 3, uniqueCell)
	}

	// Select first data cell after display
	if len(dt.state.indexes) > 0 {
		dt.Select(1, 0)
	}
}

// extractFieldNames gets all unique field names from a set of documents
func (dt *DocumentTable) extractFieldNames(documents []drivers.Document) []string {
	fieldSet := make(map[string]bool)
	fieldOrder := []string{}

	for _, doc := range documents {
		for fieldName := range doc {
			if !fieldSet[fieldName] {
				fieldSet[fieldName] = true
				fieldOrder = append(fieldOrder, fieldName)
			}
		}
	}

	return fieldOrder
}

// formatCellValue formats a document field value for display
func (dt *DocumentTable) formatCellValue(value interface{}) string {
	if value == nil {
		return "NULL"
	}

	var result string
	switch v := value.(type) {
	case string:
		result = v
	case int, int32, int64, float32, float64, bool:
		result = fmt.Sprintf("%v", v)
	case map[string]interface{}:
		// Nested object - show as JSON
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			result = "{...}"
		} else {
			result = string(jsonBytes)
		}
	case []interface{}:
		// Array - show as JSON
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			result = fmt.Sprintf("[%d items]", len(v))
		} else {
			result = string(jsonBytes)
		}
	default:
		// Try to marshal as JSON
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			result = fmt.Sprintf("%v", value)
		} else {
			result = string(jsonBytes)
		}
	}

	// Truncate if too long
	if len(result) > maxCellDisplayLength {
		return result[:maxCellDisplayLength] + "..."
	}
	return result
}

// Input capture for table navigation
func (dt *DocumentTable) tableInputCapture(event *tcell.EventKey) *tcell.EventKey {
	command := app.Keymaps.Group(app.TableGroup).Resolve(event)

	selectedRowIndex, selectedColumnIndex := dt.GetSelection()
	rowCount, colCount := dt.GetRowCount(), dt.GetColumnCount()

	switch command {
	// Horizontal navigation
	case commands.GotoNext: // Right arrow
		if selectedColumnIndex+1 < colCount {
			dt.Select(selectedRowIndex, selectedColumnIndex+1)
		}
		return nil
	case commands.GotoPrev: // Left arrow
		if selectedColumnIndex > 0 {
			dt.Select(selectedRowIndex, selectedColumnIndex-1)
		}
		return nil
	case commands.GotoEnd: // End key
		if colCount > 0 {
			dt.Select(selectedRowIndex, colCount-1)
		}
		return nil
	case commands.GotoStart: // Home key
		dt.Select(selectedRowIndex, 0)
		return nil

	// Vertical navigation
	case commands.GotoBottom:
		if rowCount > 1 {
			dt.Select(rowCount-1, selectedColumnIndex)
		}
		return nil
	case commands.GotoTop:
		if rowCount > 1 {
			dt.Select(1, selectedColumnIndex) // Skip header row (row 0)
		}
		return nil

	// Refresh
	case commands.Refresh:
		// Refresh based on current view
		if dt.state.currentView == "schema" {
			dt.FetchSchema()
		} else if dt.state.currentView == "indexes" {
			dt.FetchIndexes()
		} else {
			dt.FetchDocuments(nil)
		}
		return nil

	// Menu navigation (number keys)
	case commands.RecordsMenu: // Key: 1
		dt.state.currentView = "documents"
		dt.FetchDocuments(nil)
		return nil
	case commands.ColumnsMenu: // Key: 2 (Schema in NoSQL)
		dt.state.currentView = "schema"
		dt.FetchSchema()
		return nil
	case commands.ConstraintsMenu: // Key: 3 (Maps to Indexes in NoSQL since we don't have constraints)
		dt.state.currentView = "indexes"
		dt.FetchIndexes()
		return nil
	case commands.IndexesMenu: // Key: 5 (also supported for consistency)
		dt.state.currentView = "indexes"
		dt.FetchIndexes()
		return nil
	}

	// Handle Enter key to show full cell content
	if event.Key() == tcell.KeyEnter {
		dt.showCellContent()
		return nil
	}

	return event
}

// Highlighting methods (required by Home)
func (dt *DocumentTable) HighlightAll() {
	dt.SetBorderColor(app.Styles.PrimaryTextColor)
}

func (dt *DocumentTable) RemoveHighlightAll() {
	dt.SetBorderColor(app.Styles.InverseTextColor)
}

func (dt *DocumentTable) RemoveHighlightTable() {
	dt.SetBorderColor(app.Styles.InverseTextColor)
}

// GetPrimitive implements TabContent interface
func (dt *DocumentTable) GetPrimitive() tview.Primitive {
	return dt.Page
}

// SetError displays an error modal
func (dt *DocumentTable) SetError(err string) {
	dt.state.error = err
	dt.Error.SetText(err)
	dt.Page.ShowPage(pageNameTableError)
	App.SetFocus(dt.Error)
	App.ForceDraw()
}

// showCellContent displays the full content of the selected cell in a viewer
func (dt *DocumentTable) showCellContent() {
	selectedRow, selectedCol := dt.GetSelection()
	if selectedRow == 0 { // Header row
		return
	}

	// Get the field name from header
	headerCell := dt.GetCell(0, selectedCol)
	if headerCell == nil {
		return
	}
	fieldName := headerCell.Text

	// Get the full value based on current view
	var cellData map[string]string
	switch dt.state.currentView {
	case "documents":
		if selectedRow-1 < len(dt.state.documents) {
			doc := dt.state.documents[selectedRow-1]
			value := doc[fieldName]
			cellData = map[string]string{
				fieldName: dt.getFullCellValue(value),
			}
		}
	case "schema":
		if selectedRow-1 < len(dt.state.schema.Fields) {
			field := dt.state.schema.Fields[selectedRow-1]
			// Show the full field details
			cellData = map[string]string{
				"Field": field.Name,
				"Type":  field.Type,
			}
		}
	case "indexes":
		if selectedRow-1 < len(dt.state.indexes) {
			index := dt.state.indexes[selectedRow-1]
			cellData = map[string]string{
				"Name":   index.Name,
				"Fields": strings.Join(index.Fields, ", "),
				"Type":   index.Type,
				"Unique": fmt.Sprintf("%v", index.Unique),
			}
		}
	}

	if cellData != nil {
		dt.jsonViewer.Show(cellData, dt)
	}
}

// getFullCellValue returns the full untruncated value as a string
func (dt *DocumentTable) getFullCellValue(value interface{}) string {
	if value == nil {
		return "NULL"
	}

	switch v := value.(type) {
	case string:
		return v
	case int, int32, int64, float32, float64, bool:
		return fmt.Sprintf("%v", v)
	default:
		// Try to marshal as pretty JSON
		jsonBytes, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(jsonBytes)
	}
}

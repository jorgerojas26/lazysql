package components

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

const defaultBatchSize = 10000

// CSVExportScope defines the scope of data to export
type CSVExportScope int

const (
	ExportCurrentPage CSVExportScope = iota
	ExportAllRecords
)

// CSVExportOptions contains options for creating a CSV export modal.
type CSVExportOptions struct {
	DatabaseName  string // Database name for file naming
	TableName     string // Table name for file naming
	HasPagination bool   // Whether pagination exists (determines UI: 2 buttons vs 1)
	RowCount      int    // Current row count for display (excluding header)
}

// getDefaultExportDir returns the default directory for CSV export.
// Uses ~/Downloads on all platforms (standard location), falls back to home directory.
func getDefaultExportDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "."
	}

	// ~/Downloads is the standard download location on macOS, Windows, and most Linux distros
	downloadDir := filepath.Join(homeDir, "Downloads")
	if info, err := os.Stat(downloadDir); err == nil && info.IsDir() {
		return downloadDir
	}

	return homeDir
}

// CSVExportModal is a modal for exporting data to CSV.
type CSVExportModal struct {
	tview.Primitive
	form          *tview.Form
	hasPagination bool
	onExport      func(filePath string, scope CSVExportScope, batchSize int)
}

// NewCSVExportModal creates a new CSVExportModal.
func NewCSVExportModal(opts CSVExportOptions, onExport func(filePath string, scope CSVExportScope, batchSize int)) *CSVExportModal {
	cem := &CSVExportModal{
		hasPagination: opts.HasPagination,
		onExport:      onExport,
	}

	// Default file path with timestamp to avoid conflicts
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("%s_%s_%s.csv", opts.DatabaseName, opts.TableName, timestamp)
	defaultPath := filepath.Join(getDefaultExportDir(), fileName)

	cem.form = tview.NewForm().
		AddInputField("File Path", defaultPath, 0, nil, nil)

	if opts.HasPagination {
		// Add batch size field for table view (used for Export All Records)
		cem.form.AddInputField("Batch Size", strconv.Itoa(defaultBatchSize), 0, nil, nil)

		// Table view: show both options
		cem.form.AddButton("Export Current Page", func() {
			cem.export(ExportCurrentPage)
		})
		cem.form.AddButton("Export All Records", func() {
			cem.export(ExportAllRecords)
		})
	} else {
		// Query result: show single export button with row count
		buttonLabel := fmt.Sprintf("Export (%d rows)", opts.RowCount)
		cem.form.AddButton(buttonLabel, func() {
			cem.export(ExportAllRecords)
		})
	}

	cem.form.SetFieldStyle(
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

	cem.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			cem.cancel()
			return nil
		}
		return event
	})

	cem.form.SetBorder(false)

	hint := tview.NewTextView().
		SetText("Esc to cancel").
		SetTextAlign(tview.AlignCenter).
		SetTextColor(app.Styles.TertiaryTextColor)

	formWithHint := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(cem.form, 0, 1, true).
		AddItem(hint, 1, 0, false)
	formWithHint.SetBorder(true).SetTitle(" Export to CSV ").SetTitleAlign(tview.AlignLeft)

	grid := tview.NewGrid().
		SetRows(0, 11, 0).
		SetColumns(0, 80, 0).
		AddItem(formWithHint, 1, 1, 1, 1, 0, 0, true)

	cem.Primitive = grid

	return cem
}

func (cem *CSVExportModal) export(scope CSVExportScope) {
	filePath := cem.form.GetFormItem(0).(*tview.InputField).GetText()
	if filePath == "" {
		cem.showErrorModal("File path cannot be empty")
		return
	}

	batchSize := 0
	if cem.hasPagination {
		batchSizeText := cem.form.GetFormItem(1).(*tview.InputField).GetText()
		var err error
		batchSize, err = strconv.Atoi(batchSizeText)
		if err != nil || batchSize <= 0 {
			cem.showErrorModal("Batch size must be a positive integer")
			return
		}
	}

	mainPages.RemovePage(pageNameCSVExport)

	if cem.onExport != nil {
		cem.onExport(filePath, scope, batchSize)
	}
}

func (cem *CSVExportModal) showErrorModal(message string) {
	modal := NewErrorModal(message)
	modal.SetDoneFunc(func(_ int, _ string) {
		mainPages.RemovePage(pageNameCSVExportError)
	})

	mainPages.AddPage(pageNameCSVExportError, modal, true, true)
	App.SetFocus(modal)
}

func (cem *CSVExportModal) cancel() {
	mainPages.RemovePage(pageNameCSVExport)
}

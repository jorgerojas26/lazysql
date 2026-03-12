package components

import (
	"encoding/json"
	"testing"
)

// formatCellJSON simulates the Show() logic for formatting row data into JSON.
func formatCellJSON(rowData map[string]string) ([]byte, error) {
	structuredRowData := make(map[string]any)

	for key, value := range rowData {
		var jsonData any
		err := json.Unmarshal([]byte(value), &jsonData)
		if err == nil {
			structuredRowData[key] = jsonData
		} else {
			structuredRowData[key] = value
		}
	}

	var dataToFormat any = structuredRowData
	if len(structuredRowData) == 1 {
		for _, value := range structuredRowData {
			if _, isString := value.(string); !isString {
				dataToFormat = value
			}
		}
	}

	return json.MarshalIndent(dataToFormat, "", "  ")
}

func TestSingleCellJSONNotWrapped(t *testing.T) {
	originalJSON := `{"raw":{"SR_NO":1,"AMOUNT":150},"source":"orbii","category":"Expenses"}`
	columnName := "raw"

	rowData := map[string]string{columnName: originalJSON}
	result, err := formatCellJSON(rowData)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Should have 3 top-level keys: "raw", "source", "category"
	if len(parsed) != 3 {
		t.Errorf("Expected 3 top-level keys, got %d: %v", len(parsed), result)
	}
	for _, key := range []string{"raw", "source", "category"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("Missing expected top-level key %q", key)
		}
	}
}

func TestSingleCellPlainStringStaysWrapped(t *testing.T) {
	rowData := map[string]string{"name": "hello world"}
	result, err := formatCellJSON(rowData)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Plain strings should remain wrapped under the column name for context
	if _, ok := parsed["name"]; !ok {
		t.Error("Plain string value should remain wrapped under column name")
	}
}

func TestMultiColumnRowKeepsAllKeys(t *testing.T) {
	rowData := map[string]string{
		"id":   "42",
		"data": `{"key":"value"}`,
	}
	result, err := formatCellJSON(rowData)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Multi-column row view should keep all column names
	if len(parsed) != 2 {
		t.Errorf("Expected 2 top-level keys, got %d", len(parsed))
	}
	if _, ok := parsed["id"]; !ok {
		t.Error("Missing 'id' key")
	}
	if _, ok := parsed["data"]; !ok {
		t.Error("Missing 'data' key")
	}
}

func TestSingleCellJSONArray(t *testing.T) {
	rowData := map[string]string{"items": `[1, 2, 3]`}
	result, err := formatCellJSON(rowData)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	var parsed []any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("Single cell JSON array should be shown directly: %v", err)
	}
	if len(parsed) != 3 {
		t.Errorf("Expected 3 array elements, got %d", len(parsed))
	}
}

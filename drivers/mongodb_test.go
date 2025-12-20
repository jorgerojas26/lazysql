package drivers

import (
	"testing"
)

func TestMongoDBFormatFilter(t *testing.T) {
	db := &MongoDB{Provider: DriverMongoDB}

	tests := []struct {
		name        string
		filter      Filter
		expectError bool
	}{
		{
			name:        "empty filter",
			filter:      Filter{},
			expectError: false,
		},
		{
			name: "equality filter",
			filter: Filter{
				"name": {Operator: "eq", Value: "John"},
			},
			expectError: false,
		},
		{
			name: "greater than filter",
			filter: Filter{
				"age": {Operator: "gt", Value: 25},
			},
			expectError: false,
		},
		{
			name: "less than or equal filter",
			filter: Filter{
				"price": {Operator: "lte", Value: 100.50},
			},
			expectError: false,
		},
		{
			name: "in filter",
			filter: Filter{
				"status": {Operator: "in", Value: []string{"active", "pending"}},
			},
			expectError: false,
		},
		{
			name: "not in filter",
			filter: Filter{
				"category": {Operator: "nin", Value: []string{"archived", "deleted"}},
			},
			expectError: false,
		},
		{
			name: "contains filter (regex)",
			filter: Filter{
				"description": {Operator: "contains", Value: "search term"},
			},
			expectError: false,
		},
		{
			name: "regex filter",
			filter: Filter{
				"email": {Operator: "regex", Value: ".*@example\\.com$"},
			},
			expectError: false,
		},
		{
			name: "multiple filters",
			filter: Filter{
				"age":    {Operator: "gte", Value: 18},
				"status": {Operator: "eq", Value: "active"},
				"score":  {Operator: "lt", Value: 100},
			},
			expectError: false,
		},
		{
			name: "unsupported operator",
			filter: Filter{
				"field": {Operator: "invalid_op", Value: "value"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := db.FormatFilter(tt.filter)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("Expected non-nil result")
				}
			}
		})
	}
}

func TestMongoDBFormatSort(t *testing.T) {
	db := &MongoDB{Provider: DriverMongoDB}

	tests := []struct {
		name        string
		sort        Sort
		expectError bool
	}{
		{
			name:        "empty sort",
			sort:        Sort{},
			expectError: false,
		},
		{
			name: "ascending sort",
			sort: Sort{
				Field: "name",
				Order: "asc",
			},
			expectError: false,
		},
		{
			name: "descending sort",
			sort: Sort{
				Field: "created_at",
				Order: "desc",
			},
			expectError: false,
		},
		{
			name: "sort without explicit order (defaults to asc)",
			sort: Sort{
				Field: "age",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := db.FormatSort(tt.sort)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("Expected non-nil result")
				}
			}
		})
	}
}

func TestMongoDBFormatIdentifier(t *testing.T) {
	db := &MongoDB{Provider: DriverMongoDB}

	tests := []struct {
		name       string
		identifier string
		expected   string
	}{
		{
			name:       "simple identifier",
			identifier: "collection_name",
			expected:   "collection_name",
		},
		{
			name:       "identifier with dots",
			identifier: "nested.field.name",
			expected:   "nested.field.name",
		},
		{
			name:       "identifier with special chars",
			identifier: "field-with-dashes",
			expected:   "field-with-dashes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := db.FormatIdentifier(tt.identifier)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMongoDBProviderMethods(t *testing.T) {
	db := &MongoDB{}

	// Test SetProvider
	db.SetProvider(DriverMongoDB)
	if db.GetProvider() != DriverMongoDB {
		t.Errorf("Expected provider %q, got %q", DriverMongoDB, db.GetProvider())
	}

	// Test GetProvider
	provider := db.GetProvider()
	if provider != DriverMongoDB {
		t.Errorf("Expected provider %q, got %q", DriverMongoDB, provider)
	}
}

func TestInferType(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"nil value", nil, "null"},
		{"string value", "hello", "string"},
		{"int value", 42, "number"},
		{"int32 value", int32(42), "number"},
		{"int64 value", int64(42), "number"},
		{"float32 value", float32(3.14), "number"},
		{"float64 value", 3.14159, "number"},
		{"bool value", true, "boolean"},
		{"array value", []interface{}{1, 2, 3}, "array"},
		{"map value", map[string]interface{}{"key": "value"}, "object"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferType(tt.value)
			if result != tt.expected {
				t.Errorf("Expected type %q for value %v, got %q", tt.expected, tt.value, result)
			}
		})
	}
}

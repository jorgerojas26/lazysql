package drivers

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestMongoDBFormatFilter(t *testing.T) {
	db := &MongoDB{Provider: DriverMongoDB}

	tests := []struct {
		name        string
		filter      Filter
		expected    bson.M
		expectError bool
	}{
		{
			name:     "empty filter",
			filter:   Filter{},
			expected: bson.M{},
		},
		{
			name:     "equality filter",
			filter:   Filter{"name": {Operator: "eq", Value: "John"}},
			expected: bson.M{"name": "John"},
		},
		{
			name:     "greater than filter",
			filter:   Filter{"age": {Operator: "gt", Value: 25}},
			expected: bson.M{"age": bson.M{"$gt": 25}},
		},
		{
			name:     "less than or equal filter",
			filter:   Filter{"price": {Operator: "lte", Value: 100.50}},
			expected: bson.M{"price": bson.M{"$lte": 100.50}},
		},
		{
			name:     "in filter",
			filter:   Filter{"status": {Operator: "in", Value: []string{"active", "pending"}}},
			expected: bson.M{"status": bson.M{"$in": []string{"active", "pending"}}},
		},
		{
			name:     "not in filter",
			filter:   Filter{"category": {Operator: "nin", Value: []string{"archived"}}},
			expected: bson.M{"category": bson.M{"$nin": []string{"archived"}}},
		},
		{
			name:     "contains filter (case-insensitive regex)",
			filter:   Filter{"description": {Operator: "contains", Value: "search term"}},
			expected: bson.M{"description": bson.M{"$regex": "search term", "$options": "i"}},
		},
		{
			name:     "regex filter",
			filter:   Filter{"email": {Operator: "regex", Value: ".*@example\\.com$"}},
			expected: bson.M{"email": bson.M{"$regex": ".*@example\\.com$"}},
		},
		{
			name: "multiple filters",
			filter: Filter{
				"age":    {Operator: "gte", Value: 18},
				"status": {Operator: "eq", Value: "active"},
			},
			expected: bson.M{
				"age":    bson.M{"$gte": 18},
				"status": "active",
			},
		},
		{
			name:        "unsupported operator",
			filter:      Filter{"field": {Operator: "invalid_op", Value: "value"}},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := db.FormatFilter(tt.filter)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMongoDBFormatSort(t *testing.T) {
	db := &MongoDB{Provider: DriverMongoDB}

	tests := []struct {
		name     string
		sort     Sort
		expected bson.D
	}{
		{
			name:     "empty sort",
			sort:     Sort{},
			expected: bson.D{},
		},
		{
			name:     "ascending sort",
			sort:     Sort{Field: "name", Order: "asc"},
			expected: bson.D{{Key: "name", Value: 1}},
		},
		{
			name:     "descending sort",
			sort:     Sort{Field: "created_at", Order: "desc"},
			expected: bson.D{{Key: "created_at", Value: -1}},
		},
		{
			name:     "sort without explicit order defaults to ascending",
			sort:     Sort{Field: "age"},
			expected: bson.D{{Key: "age", Value: 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := db.FormatSort(tt.sort)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMongoDBProviderMethods(t *testing.T) {
	db := &MongoDB{}

	db.SetProvider(DriverMongoDB)
	if db.GetProvider() != DriverMongoDB {
		t.Errorf("Expected provider %q, got %q", DriverMongoDB, db.GetProvider())
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
		{"bson.A array value", bson.A{1, 2, 3}, "array"},
		{"map value", map[string]interface{}{"key": "value"}, "object"},
		{"bson.M map value", bson.M{"key": "value"}, "object"},
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

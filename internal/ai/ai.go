package ai

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/jorgerojas26/lazysql/models"
)

const MaxResponseBytes = 64000

type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable string `json:"nullable,omitempty"`
	Key      string `json:"key,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

type Relationship struct {
	Constraint string `json:"constraint,omitempty"`
	FromTable  string `json:"from_table"`
	FromColumn string `json:"from_column"`
	ToTable    string `json:"to_table"`
	ToColumn   string `json:"to_column"`
}

type Table struct {
	Database      string         `json:"database,omitempty"`
	Name          string         `json:"name"`
	Columns       []Column       `json:"columns"`
	PrimaryKeys   []string       `json:"primary_keys,omitempty"`
	Relationships []Relationship `json:"relationships,omitempty"`
}

type ChatMessage struct {
	Role     string `json:"role"`
	Text     string `json:"text,omitempty"`
	SQL      string `json:"sql,omitempty"`
	Database string `json:"database,omitempty"`
	Warning  string `json:"warning,omitempty"`
	Error    bool   `json:"error,omitempty"`
}

type TableCatalog struct {
	Database string   `json:"database"`
	Tables   []string `json:"tables"`
	More     bool     `json:"more,omitempty"`
}

type Request struct {
	SchemaGeneration        uint64         `json:"-"`
	Databases               []string       `json:"available_databases,omitempty"`
	DatabaseCatalogComplete bool           `json:"database_catalog_complete,omitempty"`
	Catalog                 []TableCatalog `json:"table_catalog,omitempty"`
	History                 []ChatMessage  `json:"conversation,omitempty"`
	HistoryTruncated        bool           `json:"earlier_conversation_omitted,omitempty"`
	Dialect                 string         `json:"dialect"`
	Database                string         `json:"database"`
	CurrentTable            string         `json:"current_table,omitempty"`
	Tables                  []Table        `json:"tables,omitempty"`
	Schemas                 []string       `json:"schemas,omitempty"`
	Prompt                  string         `json:"request"`
	SelectedSQL             string         `json:"selected_sql,omitempty"`
	EditorSQL               string         `json:"editor_sql,omitempty"`
}

type Result struct {
	Message       string `json:"message,omitempty"`
	Database      string `json:"database,omitempty"`
	SQL           string `json:"sql"`
	Clarification string `json:"clarification"`
	Missing       string `json:"missing"`
	Warning       string `json:"warning"`
}

func New(config models.AIConfig) (ToolCompleter, error) {
	if !config.Enabled {
		return nil, errors.New("AI is disabled; enable ai.enabled explicitly")
	}
	switch config.Provider {
	case "opencode-go":
		return newOpenCode(config)
	case "chatgpt":
		service, err := newChatGPTService()
		if err != nil {
			return nil, err
		}
		return &chatGPTClient{model: config.Model, thinking: config.Thinking, service: service}, nil
	default:
		return nil, errors.New("set ai.provider to opencode-go or chatgpt")
	}
}

func parseResult(output string) (Result, error) {
	if len(output) > MaxResponseBytes {
		return Result{}, errors.New("AI response is too large; narrow the request")
	}
	output = strings.TrimSpace(output)
	if strings.HasPrefix(output, "```json\n") && strings.HasSuffix(output, "```") {
		output = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(output, "```json\n"), "```"))
	}
	var result Result
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return Result{}, errors.New("AI returned an invalid response; retry or refine the request")
	}
	result.SQL = strings.TrimSpace(result.SQL)
	result.Clarification = strings.TrimSpace(result.Clarification)
	result.Missing = strings.TrimSpace(result.Missing)
	count := 0
	for _, field := range []string{result.SQL, result.Clarification, result.Missing} {
		if field != "" {
			count++
		}
	}
	if count > 1 || (count == 0 && strings.TrimSpace(result.Message) == "") {
		return Result{}, errors.New("AI returned an incomplete or ambiguous response; retry")
	}
	return result, nil
}

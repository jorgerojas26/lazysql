package components

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func aiMetadata(ctx context.Context, cache *metadataCache, key metadataKey, load func(context.Context) (any, error)) (any, error) {
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	done := cache.requestWithContext(work, key, cancel, func() (any, error) { return load(work) })
	if done != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-done:
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	state, value, err := cache.result(key)
	if err != nil {
		logger.Error("AI schema metadata load failed", map[string]any{"operation": string(key.kind), "database": key.database, "error": err.Error()})
		return nil, errors.New("could not load schema metadata; retry or check the database connection")
	}
	if state != MetadataReady {
		return nil, errors.New("schema metadata changed; retry")
	}
	return value, nil
}

func aiLoad(ctx context.Context, loader *schemaLoader, key metadataKey, load func(context.Context) (any, error)) (any, error) {
	return aiMetadata(ctx, loader.cache, key, func(work context.Context) (any, error) {
		select {
		case loader.aiLimit <- struct{}{}:
		case <-work.Done():
			return nil, work.Err()
		}
		defer func() { <-loader.aiLimit }()
		return load(work)
	})
}

func aiCatalog(ctx context.Context, loader *schemaLoader, database string, schemas []string) ([]string, error) {
	if database == "" {
		return nil, errors.New("no database selected; open a database in the tree and retry")
	}
	value, err := aiLoad(ctx, loader, newMetadataKey(database, "", MetadataTables), func(work context.Context) (any, error) { return loader.driver.GetTables(work, database) })
	if err != nil {
		return nil, err
	}
	tables, ok := value.(map[string][]string)
	if !ok {
		return nil, errors.New("invalid schema catalog")
	}
	var names []string
	for _, table := range loader.visibleTables(database, tables, schemas) {
		names = append(names, table.qualifiedName)
	}
	return names, nil
}

func aiRows(value any) []map[string]string {
	rows, ok := value.([][]string)
	if !ok || len(rows) < 2 {
		return nil
	}
	result := make([]map[string]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		fields := map[string]string{}
		for i, header := range rows[0] {
			if i < len(row) {
				fields[strings.ToLower(strings.TrimSpace(header))] = row[i]
			}
		}
		result = append(result, fields)
	}
	return result
}

func aiField(row map[string]string, names ...string) string {
	for _, name := range names {
		if value, ok := row[name]; ok {
			return value
		}
	}
	return ""
}

func aiRelationships(value any, table string, reverse, schemas bool) []ai.Relationship {
	var result []ai.Relationship
	for _, row := range aiRows(value) {
		relation := ai.Relationship{Constraint: aiField(row, "constraint_name", "id"), FromTable: table,
			FromColumn: aiField(row, "column_name", "from"), ToTable: aiField(row, "foreign_table_name", "referenced_table", "table"), ToColumn: aiField(row, "foreign_column_name", "referenced_column", "to")}
		if reverse {
			relation.FromTable = aiField(row, "table_name")
			if schema := aiField(row, "table_schema"); schemas && schema != "" {
				relation.FromTable = schema + "." + relation.FromTable
			}
			relation.ToTable = table
			relation.ToColumn = aiField(row, "referenced_column_name")
		} else if schema := aiField(row, "foreign_table_schema"); schema != "" && !strings.Contains(relation.ToTable, ".") {
			relation.ToTable = schema + "." + relation.ToTable
		}
		if relation.FromColumn != "" && relation.ToTable != "" && relation.ToColumn != "" {
			result = append(result, relation)
		}
	}
	return result
}

func aiColumns(columns any) []ai.Column {
	var result []ai.Column
	for _, row := range aiRows(columns) {
		column := ai.Column{Name: aiField(row, "column_name", "field", "name"), Type: aiField(row, "data_type", "type"), Nullable: aiField(row, "is_nullable", "null"), Key: aiField(row, "key"), Comment: aiField(row, "comment")}
		if value, ok := row["notnull"]; ok {
			column.Nullable = map[string]string{"0": "YES", "1": "NO"}[value]
		}
		if column.Name != "" {
			result = append(result, column)
		}
	}
	return result
}

func loadAIContext(ctx context.Context, loader *schemaLoader, database string, names, catalog []string) ([]ai.Table, error) {
	allowed := map[string]bool{}
	for _, name := range catalog {
		allowed[name] = true
	}
	for _, name := range names {
		if !allowed[name] {
			return nil, errors.New("table is not available in this connection scope")
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, bulk := loader.driver.(drivers.BulkTableColumnLoader); bulk && len(names) > 1 {
		descriptors := make([]editorSchemaTable, 0, len(names))
		for _, name := range names {
			_, bare := splitMetadataTableName(name)
			descriptors = append(descriptors, editorSchemaTable{qualifiedName: name, bareName: bare})
		}
		select {
		case loader.aiLimit <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		loader.preloadEditorColumns(ctx, database, descriptors, len(names), nil)
		<-loader.aiLimit
	}
	result := make([]ai.Table, len(names))
	failures := make([]error, len(names))
	var wait sync.WaitGroup
	for index, name := range names {
		wait.Add(1)
		go func(index int, name string) {
			defer wait.Done()
			var columns, keys any
			var relationships []ai.Relationship
			var columnError, keyError, relationError error
			var parts sync.WaitGroup
			parts.Add(3)
			go func() {
				defer parts.Done()
				columns, columnError = aiLoad(ctx, loader, newMetadataKey(database, name, MetadataColumns), func(work context.Context) (any, error) { return loader.driver.GetTableColumns(work, database, name) })
			}()
			go func() {
				defer parts.Done()
				keys, keyError = aiLoad(ctx, loader, newMetadataKey(database, name, MetadataPrimaryKeys), func(work context.Context) (any, error) {
					return loader.driver.GetPrimaryKeyColumnNames(work, database, name)
				})
			}()
			go func() {
				defer parts.Done()
				relationships, relationError = loadAIRelationships(ctx, loader, database, name, allowed)
			}()
			parts.Wait()
			if columnError != nil {
				failures[index] = columnError
				return
			}
			if keyError != nil {
				failures[index] = keyError
				return
			}
			if relationError != nil {
				failures[index] = relationError
				return
			}
			item := ai.Table{Name: name, Database: database, Columns: aiColumns(columns), Relationships: relationships}
			item.PrimaryKeys, _ = keys.([]string)
			if len(item.Columns) == 0 {
				failures[index] = fmt.Errorf("no column metadata for %s; refresh the schema", name)
				return
			}
			result[index] = item
		}(index, name)
	}
	wait.Wait()
	for _, err := range failures {
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func loadAIRelationships(ctx context.Context, loader *schemaLoader, database, name string, allowed map[string]bool) ([]ai.Relationship, error) {
	kind := MetadataForeignKeys
	getForeignKeys := loader.driver.GetForeignKeys
	if outgoing, ok := loader.driver.(interface {
		GetOutgoingForeignKeys(context.Context, string, string) ([][]string, error)
	}); ok {
		kind = MetadataKind("ai_outgoing_foreign_keys")
		getForeignKeys = outgoing.GetOutgoingForeignKeys
	}
	var foreign, incoming any
	var foreignError, incomingError error
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		foreign, foreignError = aiLoad(ctx, loader, newMetadataKey(database, name, kind), func(work context.Context) (any, error) { return getForeignKeys(work, database, name) })
	}()
	go func() {
		defer wait.Done()
		incoming, incomingError = aiLoad(ctx, loader, newMetadataKey(database, name, MetadataReferencingTables), func(work context.Context) (any, error) {
			return loader.driver.GetReferencingTables(work, database, name)
		})
	}()
	wait.Wait()
	if foreignError != nil {
		return nil, foreignError
	}
	if incomingError != nil {
		return nil, incomingError
	}
	relationships := append(aiRelationships(foreign, name, false, loader.driver.UseSchemas()), aiRelationships(incoming, name, true, loader.driver.UseSchemas())...)

	filtered := relationships[:0]
	for _, relation := range relationships {
		if allowed[relation.FromTable] && allowed[relation.ToTable] {
			filtered = append(filtered, relation)
		}
	}
	relationships = filtered
	return relationships, nil
}

type aiSchemaTools struct {
	loader        *schemaLoader
	database      string
	catalog       []string
	schemasFilter []string
}

func (tools *aiSchemaTools) schemas() []string {
	var schemas []string
	if tools.loader.driver.UseSchemas() {
		for _, name := range tools.catalog {
			schema, _ := splitMetadataTableName(name)
			if schema != "" && !slices.Contains(schemas, schema) {
				schemas = append(schemas, schema)
			}
		}
	}
	slices.Sort(schemas)
	return schemas
}

func decodeAIArguments(raw json.RawMessage, target any) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return errors.New("tool arguments must be an object")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return errors.New("invalid metadata tool arguments")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("invalid metadata tool arguments")
	}
	return nil
}

func (tools *aiSchemaTools) databases(ctx context.Context) ([]string, error) {
	value, err := aiLoad(ctx, tools.loader, newMetadataKey("", "", MetadataDatabases), func(work context.Context) (any, error) { return tools.loader.driver.GetDatabases(work) })
	if err != nil {
		return nil, err
	}
	names, ok := value.([]string)
	if !ok {
		return nil, errors.New("invalid database catalog")
	}
	names = slices.Clone(names)
	if tools.database != "" && !slices.Contains(names, tools.database) {
		names = append(names, tools.database)
	}
	slices.Sort(names)
	return slices.Compact(names), nil
}

func (tools *aiSchemaTools) resolveDatabase(ctx context.Context, database string) (string, error) {
	if database == "" {
		database = tools.database
	}
	if database != "" && database == tools.database {
		return database, nil
	}
	names, err := tools.databases(ctx)
	if err != nil {
		return "", err
	}
	if database == "" && len(names) == 1 {
		return names[0], nil
	}
	if database != "" && slices.Contains(names, database) {
		return database, nil
	}
	return "", errors.New("specify an accessible database returned by list_databases")
}

func (tools *aiSchemaTools) catalogFor(ctx context.Context, database string) ([]string, error) {
	if database == tools.database && tools.catalog != nil {
		return tools.catalog, nil
	}
	return aiCatalog(ctx, tools.loader, database, tools.schemasFilter)
}

func (tools *aiSchemaTools) execute(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if name == "list_databases" {
		if err := decodeAIArguments(raw, &struct{}{}); err != nil {
			return nil, err
		}
		return tools.databases(ctx)
	}
	if name == "list_tables" {
		var args struct {
			Database string `json:"database"`
			Schema   string `json:"schema"`
			Search   string `json:"search"`
			Offset   int    `json:"offset"`
		}
		if err := decodeAIArguments(raw, &args); err != nil {
			return nil, err
		}
		if args.Offset < 0 {
			return nil, errors.New("offset must be nonnegative")
		}
		database, err := tools.resolveDatabase(ctx, args.Database)
		if err != nil {
			return nil, err
		}
		catalog, err := tools.catalogFor(ctx, database)
		if err != nil {
			return nil, err
		}
		matches := []string{}
		for _, table := range catalog {
			schema, _ := splitMetadataTableName(table)
			if args.Schema != "" && (!tools.loader.driver.UseSchemas() || args.Schema != schema) {
				continue
			}
			if args.Search != "" && !strings.Contains(strings.ToLower(table), strings.ToLower(args.Search)) {
				continue
			}
			matches = append(matches, table)
		}
		start := min(args.Offset, len(matches))
		end := min(start+100, len(matches))
		var next *int
		if end < len(matches) {
			next = &end
		}
		return struct {
			Database string   `json:"database"`
			Tables   []string `json:"tables"`
			Next     *int     `json:"next_offset"`
		}{database, matches[start:end], next}, nil
	}
	if name != "describe_tables" && name != "get_relationships" {
		return nil, errors.New("forbidden metadata tool")
	}
	var args struct {
		Database string   `json:"database"`
		Names    []string `json:"names"`
	}
	if err := decodeAIArguments(raw, &args); err != nil {
		return nil, err
	}
	if len(args.Names) == 0 || len(args.Names) > ai.MaxToolCalls {
		return nil, errors.New("request between 1 and 8 tables")
	}
	database, err := tools.resolveDatabase(ctx, args.Database)
	if err != nil {
		return nil, err
	}
	catalog, err := tools.catalogFor(ctx, database)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, table := range catalog {
		allowed[table] = true
	}
	var names []string
	for _, table := range args.Names {
		if !allowed[table] {
			return nil, errors.New("table is not available in this connection scope")
		}
		if !slices.Contains(names, table) {
			names = append(names, table)
		}
	}
	if name == "describe_tables" {
		return loadAIContext(ctx, tools.loader, database, names, catalog)
	}
	result := []ai.Relationship{}
	for _, table := range names {
		relationships, err := loadAIRelationships(ctx, tools.loader, database, table, allowed)
		if err != nil {
			return nil, err
		}
		result = append(result, relationships...)
	}
	return result, nil
}

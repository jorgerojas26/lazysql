package components

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/internal/ai"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type aiSchemaDriver struct {
	schemaProgrammingMock
	columnCalls   atomic.Int32
	metadataError bool
	executions    atomic.Int32
}

func (driver *aiSchemaDriver) ExecuteQuery(context.Context, string, string) ([][]string, int, error) {
	driver.executions.Add(1)
	return nil, 0, errors.New("unexpected execution")
}
func (driver *aiSchemaDriver) ExecuteDMLStatement(context.Context, string, string) (string, error) {
	driver.executions.Add(1)
	return "", errors.New("unexpected execution")
}

func (driver *aiSchemaDriver) GetProvider() string { return "postgres" }
func (driver *aiSchemaDriver) GetTables(context.Context, string) (map[string][]string, error) {
	return map[string][]string{"public": {"orders", "customers", "items"}, "private": {"secrets"}}, nil
}
func (driver *aiSchemaDriver) GetTableColumns(_ context.Context, _, table string) ([][]string, error) {
	driver.columnCalls.Add(1)
	if driver.metadataError {
		return nil, errors.New("private connection string")
	}
	return [][]string{{"column_name", "data_type", "is_nullable", "column_default", "comment"}, {"id", "bigint", "NO", "private-default-literal", "Primary identifier"}, {"tenant_id", "bigint", "NO", "", "Tenant key"}, {"customer_id", "bigint", "NO", "", "Customer key"}}, nil
}
func (driver *aiSchemaDriver) GetPrimaryKeyColumnNames(context.Context, string, string) ([]string, error) {
	return []string{"tenant_id", "id"}, nil
}
func (driver *aiSchemaDriver) GetForeignKeys(_ context.Context, _, table string) ([][]string, error) {
	if table != "public.orders" {
		return nil, nil
	}
	return [][]string{{"constraint_name", "column_name", "foreign_table_schema", "foreign_table_name", "foreign_column_name"}, {"customer_fk", "tenant_id", "public", "customers", "tenant_id"}, {"customer_fk", "customer_id", "public", "customers", "id"}}, nil
}
func (driver *aiSchemaDriver) GetReferencingTables(_ context.Context, _, table string) ([][]string, error) {
	if table != "public.orders" {
		return nil, nil
	}
	return [][]string{{"constraint_name", "table_schema", "table_name", "column_name", "referenced_column_name"}, {"order_fk", "public", "items", "order_id", "id"}}, nil
}

func TestAIContextRespectsSchemasRelationshipsAndCache(t *testing.T) {
	driver := &aiSchemaDriver{}
	loader := newSchemaLoader(driver, newMetadataCache())
	catalog, err := aiCatalog(context.Background(), loader, "shop", []string{"public"})
	if err != nil || !reflect.DeepEqual(catalog, []string{"public.customers", "public.items", "public.orders"}) {
		t.Fatalf("catalog = %v %v", catalog, err)
	}
	tables, err := loadAIContext(context.Background(), loader, "shop", catalog, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 3 || len(tables[2].Relationships) != 3 {
		t.Fatalf("expected current + outgoing + incoming, with composite key intact: %+v", tables)
	}
	if tables[2].Relationships[0].Constraint != tables[2].Relationships[1].Constraint {
		t.Fatal("composite relationship group lost")
	}
	if tables[2].Relationships[2].FromTable != "public.items" || tables[2].Relationships[2].ToTable != "public.orders" {
		t.Fatal("reverse relationship direction lost")
	}
	data, _ := json.Marshal(tables)
	if strings.Contains(string(data), "private-default") || strings.Contains(string(data), "secrets") {
		t.Fatalf("private literals/schema leaked: %s", data)
	}
	if !strings.Contains(string(data), "Primary identifier") {
		t.Fatal("available schema comments missing")
	}
	if _, err := loadAIContext(context.Background(), loader, "shop", catalog, catalog); err != nil {
		t.Fatal(err)
	}
	if driver.columnCalls.Load() != 3 {
		t.Fatalf("metadata not reused: %d column calls", driver.columnCalls.Load())
	}

}

func TestAIContextErrorsAndCancellation(t *testing.T) {
	driver := &aiSchemaDriver{metadataError: true}
	loader := newSchemaLoader(driver, newMetadataCache())
	_, err := loadAIContext(context.Background(), loader, "shop", []string{"orders"}, []string{"orders"})
	if err == nil || strings.Contains(err.Error(), "private") {
		t.Fatalf("metadata error not sanitized: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := aiMetadata(ctx, newMetadataCache(), newMetadataKey("db", "orders", MetadataColumns), func(work context.Context) (any, error) { close(started); <-work.Done(); return nil, work.Err() })
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("metadata cancellation blocked")
	}
}

func TestAIContextIncludesAllTables(t *testing.T) {
	var names []string
	for i := 0; i < 20; i++ {
		names = append(names, strings.Repeat("t", i+1))
	}
	tables, err := loadAIContext(context.Background(), newSchemaLoader(&aiSchemaDriver{}, newMetadataCache()), "db", names, names)
	if err != nil || len(tables) != len(names) {
		t.Fatalf("full schema context = %d tables, %v", len(tables), err)
	}
}

func TestAISQLiteAndMySQLRelationshipNormalization(t *testing.T) {
	rows := [][]string{{"id", "seq", "table", "from", "to"}, {"0", "0", "users", "user_id", "id"}}
	relations := aiRelationships(rows, "orders", false, false)
	if len(relations) != 1 || relations[0].FromColumn != "user_id" || relations[0].ToTable != "users" {
		t.Fatalf("SQLite foreign key: %+v", relations)
	}
	rows = [][]string{{"constraint_name", "column_name", "referenced_table", "referenced_column"}, {"orders_user_fk", "user_id", "users", "id"}}
	relations = aiRelationships(rows, "orders", false, false)
	if len(relations) != 1 || relations[0].ToColumn != "id" {
		t.Fatalf("MySQL foreign key: %+v", relations)
	}
}

func TestAISchemaToolsAreLazyScopedAndValidateArguments(t *testing.T) {
	driver := &aiSchemaDriver{}
	loader := newSchemaLoader(driver, newMetadataCache())
	catalog, err := aiCatalog(context.Background(), loader, "shop", []string{"public"})
	if err != nil {
		t.Fatal(err)
	}
	tools := &aiSchemaTools{loader: loader, database: "shop", catalog: catalog}
	if !reflect.DeepEqual(tools.schemas(), []string{"public"}) {
		t.Fatalf("schemas = %v", tools.schemas())
	}
	listing, err := tools.execute(context.Background(), "list_tables", json.RawMessage(`{"schema":"public","search":"ORDER"}`))
	data, _ := json.Marshal(listing)
	if err != nil || !strings.Contains(string(data), "public.orders") || strings.Contains(string(data), "customers") || driver.columnCalls.Load() != 0 {
		t.Fatalf("list fetched details or did not filter: %s %v", data, err)
	}
	for _, call := range []struct{ name, args string }{
		{"describe_tables", `{"names":["private.secrets"]}`},
		{"describe_tables", `{"names":["public.orders","private.secrets"]}`},
		{"describe_tables", `{"names":[]}`},
		{"describe_tables", `{"names":["public.orders"],"sql":"SELECT 1"}`},
		{"describe_tables", `{"names":["public.orders"]} {}`},
		{"describe_tables", `null`},
		{"list_tables", `{"offset":-1}`},
		{"execute_sql", `{"sql":"SELECT 1"}`},
	} {
		if _, err := tools.execute(context.Background(), call.name, json.RawMessage(call.args)); err == nil {
			t.Fatalf("accepted invalid tool call: %+v", call)
		}
	}
	if driver.columnCalls.Load() != 0 || driver.executions.Load() != 0 {
		t.Fatal("rejected calls reached database")
	}
	relationships, err := tools.execute(context.Background(), "get_relationships", json.RawMessage(`{"names":["public.orders"]}`))
	data, _ = json.Marshal(relationships)
	if err != nil || !strings.Contains(string(data), "public.customers") || !strings.Contains(string(data), "public.items") || driver.columnCalls.Load() != 0 {
		t.Fatalf("relationship discovery fetched columns or lost related tables: %s %v", data, err)
	}
	described, err := tools.execute(context.Background(), "describe_tables", json.RawMessage(`{"names":["public.orders","public.orders"]}`))
	tables, ok := described.([]ai.Table)
	if err != nil || !ok || len(tables) != 1 || len(tables[0].Relationships) != 3 || driver.columnCalls.Load() != 1 {
		t.Fatalf("describe did not load only requested metadata: %v %v", described, err)
	}
	if _, err := tools.execute(context.Background(), "describe_tables", json.RawMessage(`{"names":["public.orders"]}`)); err != nil || driver.columnCalls.Load() != 1 {
		t.Fatal("metadata cache was not reused")
	}
	data, _ = json.Marshal(described)
	if strings.Contains(string(data), "private-default") || strings.Contains(string(data), "private.secrets") {
		t.Fatal("private metadata escaped")
	}
}

func TestAISchemaToolsPaginationAndScopedRelationships(t *testing.T) {
	driver := &aiSchemaDriver{}
	tools := &aiSchemaTools{loader: newSchemaLoader(driver, newMetadataCache()), database: "shop"}
	for i := 0; i < 205; i++ {
		tools.catalog = append(tools.catalog, fmt.Sprintf("public.table_%03d", i))
	}
	for _, offset := range []int{0, 100, 200, 10000} {
		value, err := tools.execute(context.Background(), "list_tables", json.RawMessage(fmt.Sprintf(`{"offset":%d}`, offset)))
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(value)
		var page struct {
			Tables []string `json:"tables"`
			Next   *int     `json:"next_offset"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			t.Fatal(err)
		}
		expected := min(100, max(0, 205-offset))
		if len(page.Tables) != expected || ((offset < 200) != (page.Next != nil)) {
			t.Fatalf("page %d: %s", offset, data)
		}
	}
	tools.catalog = []string{"public.orders"}
	relations, err := tools.execute(context.Background(), "get_relationships", json.RawMessage(`{"names":["public.orders"]}`))
	if err != nil || len(relations.([]ai.Relationship)) != 0 {
		t.Fatal("relationships to excluded tables leaked")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tools.execute(ctx, "describe_tables", json.RawMessage(`{"names":["public.orders"]}`)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestAICatalogMissingDatabaseReportsContextInsteadOfPermissions(t *testing.T) {
	loader := newSchemaLoader(&drivers.MySQL{}, newMetadataCache())
	_, err := aiCatalog(context.Background(), loader, "", nil)
	if err == nil || !strings.Contains(err.Error(), "no database selected") || strings.Contains(err.Error(), "permissions") {
		t.Fatalf("missing database error: %v", err)
	}
}

type aiMultiDatabaseDriver struct {
	aiSchemaDriver
	databaseCalls atomic.Int32
}

func (driver *aiMultiDatabaseDriver) GetProvider() string { return "mysql" }
func (driver *aiMultiDatabaseDriver) UseSchemas() bool    { return false }
func (driver *aiMultiDatabaseDriver) GetDatabases(context.Context) ([]string, error) {
	driver.databaseCalls.Add(1)
	return []string{"acl", "app"}, nil
}
func (driver *aiMultiDatabaseDriver) GetTables(_ context.Context, database string) (map[string][]string, error) {
	switch database {
	case "acl":
		return map[string][]string{"acl": {"Access", "ApiKey", "ApiRoute", "PassportSessions"}}, nil
	case "app":
		return map[string][]string{"app": {"Users"}}, nil
	default:
		return nil, errors.New("unavailable database")
	}
}
func (driver *aiMultiDatabaseDriver) GetTableColumns(_ context.Context, database, table string) ([][]string, error) {
	driver.columnCalls.Add(1)
	if database != "app" || table != "Users" {
		return nil, errors.New("unavailable table")
	}
	return [][]string{{"column_name", "data_type", "is_nullable"}, {"id", "bigint", "NO"}, {"email", "varchar", "NO"}, {"created_at", "timestamp", "NO"}}, nil
}

type aiDiscoveryFunc func(context.Context, string, []ai.Message, []ai.Tool) (ai.Turn, error)

func (step aiDiscoveryFunc) Step(ctx context.Context, instructions string, messages []ai.Message, tools []ai.Tool) (ai.Turn, error) {
	return step(ctx, instructions, messages, tools)
}

func TestAIDiscoveryFindsUsersOutsideStartingDatabase(t *testing.T) {
	driver := &aiMultiDatabaseDriver{}
	metadata := &aiSchemaTools{loader: newSchemaLoader(driver, newMetadataCache()), database: "acl"}
	round := 0
	client := aiDiscoveryFunc(func(_ context.Context, instructions string, messages []ai.Message, tools []ai.Tool) (ai.Turn, error) {
		round++
		var name, args string
		switch round {
		case 1:
			name, args = "list_tables", `{}`
		case 2:
			if !strings.Contains(messages[len(messages)-1].Text, "PassportSessions") {
				t.Fatal("starting database was not inspected")
			}
			name, args = "list_databases", `{}`
		case 3:
			if !strings.Contains(messages[len(messages)-1].Text, "app") {
				t.Fatal("other databases were not discoverable")
			}
			name, args = "list_tables", `{"database":"app","search":"user"}`
		case 4:
			if !strings.Contains(messages[len(messages)-1].Text, "Users") {
				t.Fatal("target table not discovered")
			}
			name, args = "describe_tables", `{"database":"app","names":["Users"]}`
		default:
			if !strings.Contains(messages[len(messages)-1].Text, "created_at") {
				t.Fatal("registration column not inspected")
			}
			return ai.Turn{Text: `{"database":"app","sql":"SELECT id, email FROM Users ORDER BY created_at DESC LIMIT 10;","clarification":"","missing":"","warning":""}`}, nil
		}
		return ai.Turn{Calls: []ai.ToolCall{{ID: fmt.Sprintf("call%d", round), Name: name, Arguments: json.RawMessage(args)}}}, nil
	})
	result, err := ai.DiscoverSQL(context.Background(), client, ai.Request{Dialect: "mysql", Database: "acl", Prompt: "Last 10 registered users"}, metadata.execute)
	if err != nil || result.SQL == "" || result.Database != "app" || round != 5 {
		t.Fatalf("cross-database discovery: %+v %v (%d rounds)", result, err, round)
	}
	if database, err := metadata.resolveDatabase(context.Background(), result.Database); err != nil || database != "app" {
		t.Fatalf("target database: %q %v", database, err)
	}
	if driver.columnCalls.Load() != 1 || driver.databaseCalls.Load() != 1 || driver.executions.Load() != 0 {
		t.Fatal("discovery did not reuse cache or loaded irrelevant data")
	}
	if _, err := metadata.execute(context.Background(), "describe_tables", json.RawMessage(`{"database":"private","names":["secrets"]}`)); err == nil {
		t.Fatal("unavailable database accepted")
	}
}

func TestAIVerifiedSchemaSeedEnablesOneRequestRefinement(t *testing.T) {
	driver := &aiMultiDatabaseDriver{}
	loader := newSchemaLoader(driver, newMetadataCache())
	tools := &aiSchemaTools{loader: loader, database: "acl"}
	for _, call := range []struct{ name, args string }{{"list_databases", `{}`}, {"list_tables", `{"database":"app"}`}, {"describe_tables", `{"database":"app","names":["Users"]}`}} {
		if _, err := tools.execute(context.Background(), call.name, json.RawMessage(call.args)); err != nil {
			t.Fatal(err)
		}
	}
	databases, complete, catalog, tables := cachedAIContext(loader, "app", nil)
	if !complete || len(databases) != 2 || len(catalog) == 0 || len(tables) != 1 || tables[0].Name != "Users" {
		t.Fatalf("verified seed: %v %v %v %+v", databases, complete, catalog, tables)
	}
	requests := 0
	model := aiDiscoveryFunc(func(_ context.Context, instructions string, messages []ai.Message, _ []ai.Tool) (ai.Turn, error) {
		requests++
		var request ai.Request
		if json.Unmarshal([]byte(messages[0].Text), &request) != nil || len(request.Tables) != 1 || request.Tables[0].Columns[2].Name != "created_at" {
			t.Fatal("model did not receive verified columns")
		}
		if !strings.Contains(instructions, "generate directly without rediscovery") {
			t.Fatal("fast path is not instructed")
		}
		return ai.Turn{Text: `{"sql":"SELECT id, email FROM Users ORDER BY created_at DESC LIMIT 10;","database":"app"}`}, nil
	})
	result, err := ai.DiscoverSQL(context.Background(), model, ai.Request{Prompt: "Include email", Databases: databases, DatabaseCatalogComplete: complete, Catalog: catalog, Tables: tables}, func(context.Context, string, json.RawMessage) (any, error) {
		t.Fatal("warm refinement made unnecessary tool calls")
		return nil, nil
	})
	if err != nil || result.SQL == "" || requests != 1 || driver.columnCalls.Load() != 1 {
		t.Fatalf("warm refinement: %+v %v (%d requests)", result, err, requests)
	}
	loader.invalidateAll()
	_, _, _, tables = cachedAIContext(loader, "app", nil)
	if len(tables) != 0 {
		t.Fatal("schema refresh left stale verified tables in context")
	}
}

func TestAICachedSchemaStillAppliesVisibilityScope(t *testing.T) {
	loader := newSchemaLoader(&aiSchemaDriver{}, newMetadataCache())
	catalog, err := aiCatalog(context.Background(), loader, "shop", []string{"public"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadAIContext(context.Background(), loader, "shop", catalog, catalog); err != nil {
		t.Fatal(err)
	}
	_, _, public, tables := cachedAIContext(loader, "shop", []string{"public"})
	if len(tables) != 3 || len(public) == 0 {
		t.Fatal("verified schema was not retained")
	}
	_, _, private, hidden := cachedAIContext(loader, "shop", []string{"private"})
	data, _ := json.Marshal(struct {
		Catalog []ai.TableCatalog
		Tables  []ai.Table
	}{private, hidden})
	if strings.Contains(string(data), "public.") || len(hidden) != 0 {
		t.Fatalf("cached context leaked hidden schema: %s", data)
	}
}

type aiConcurrentDriver struct {
	aiSchemaDriver
	active  atomic.Int32
	peak    atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (driver *aiConcurrentDriver) load(ctx context.Context) error {
	active := driver.active.Add(1)
	defer driver.active.Add(-1)
	for {
		peak := driver.peak.Load()
		if active <= peak || driver.peak.CompareAndSwap(peak, active) {
			break
		}
	}
	driver.started <- struct{}{}
	select {
	case <-driver.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (driver *aiConcurrentDriver) GetTableColumns(ctx context.Context, database, table string) ([][]string, error) {
	if err := driver.load(ctx); err != nil {
		return nil, err
	}
	return driver.aiSchemaDriver.GetTableColumns(ctx, database, table)
}
func (driver *aiConcurrentDriver) GetPrimaryKeyColumnNames(ctx context.Context, database, table string) ([]string, error) {
	if err := driver.load(ctx); err != nil {
		return nil, err
	}
	return []string{"id"}, nil
}
func (driver *aiConcurrentDriver) GetForeignKeys(ctx context.Context, database, table string) ([][]string, error) {
	if err := driver.load(ctx); err != nil {
		return nil, err
	}
	return nil, nil
}
func (driver *aiConcurrentDriver) GetReferencingTables(ctx context.Context, database, table string) ([][]string, error) {
	if err := driver.load(ctx); err != nil {
		return nil, err
	}
	return nil, nil
}

func TestAIMetadataReadsAreParallelAndBounded(t *testing.T) {
	driver := &aiConcurrentDriver{started: make(chan struct{}, 16), release: make(chan struct{})}
	loader := newSchemaLoader(driver, newMetadataCache())
	done := make(chan error, 1)
	go func() {
		_, err := loadAIContext(context.Background(), loader, "shop", []string{"one", "two"}, []string{"one", "two"})
		done <- err
	}()
	for i := 0; i < 4; i++ {
		select {
		case <-driver.started:
		case <-time.After(time.Second):
			t.Fatal("metadata reads were serialized")
		}
	}
	if driver.peak.Load() > 4 {
		t.Fatal("metadata concurrency exceeded connection-scoped bound")
	}
	close(driver.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if driver.peak.Load() != 4 {
		t.Fatalf("metadata parallelism = %d, want 4", driver.peak.Load())
	}
}

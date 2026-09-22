package benchmarks

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jorgerojas26/lazysql/drivers"
)

const (
	benchmarkDatabase    = "benchmark_db"
	benchmarkRecords     = "benchmark_records"
	benchmarkRecordCount = 10_000
	benchmarkCatalogSize = 7_249
)

// observedMetrics is updated by the instrumented database rows and query
// boundary. Nothing in the scenario runner supplies expected counters.
type observedMetrics struct {
	mu sync.Mutex

	blockingRoundTrips int
	totalOperations    int
	rowsConsumed       int
	bytesConsumed      int64
}

func (metrics *observedMetrics) observeOperation(blocking bool) {
	metrics.mu.Lock()
	metrics.totalOperations++
	if blocking {
		metrics.blockingRoundTrips++
	}
	metrics.mu.Unlock()
}

func (metrics *observedMetrics) observeRow(row []driver.Value) {
	var bytes int64
	for _, value := range row {
		switch value := value.(type) {
		case nil:
		case []byte:
			bytes += int64(len(value))
		case string:
			bytes += int64(len(value))
		default:
			bytes += int64(len(fmt.Sprint(value)))
		}
	}

	metrics.mu.Lock()
	metrics.rowsConsumed++
	metrics.bytesConsumed += bytes
	metrics.mu.Unlock()
}

func (metrics *observedMetrics) snapshot() (blocking, total, rows int, bytes int64) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	return metrics.blockingRoundTrips, metrics.totalOperations, metrics.rowsConsumed, metrics.bytesConsumed
}

type fixtureConfig struct {
	rtt            time.Duration
	tableCount     int
	catalogSize    int
	slowCountDelay time.Duration
	slowRows       bool
	slowRowDelay   time.Duration
	metrics        *observedMetrics
}

func newFixtureDriver(ctx context.Context, options Options, scenario Scenario, metrics *observedMetrics) (*drivers.MySQL, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	connector := &fixtureConnector{config: fixtureConfig{
		rtt:            options.RTT,
		tableCount:     scenarioAutocompleteTableCount(scenario),
		catalogSize:    1,
		slowCountDelay: options.SlowCountDelay,
		slowRows:       scenario == ScenarioSlowRows,
		slowRowDelay:   options.SlowRowDelay,
		metrics:        metrics,
	}}
	if scenario == ScenarioMySQLCatalog {
		connector.config.catalogSize = benchmarkCatalogSize
	}
	connection := sql.OpenDB(connector)
	mysql := &drivers.MySQL{Connection: connection}
	mysql.SetProvider(drivers.DriverMySQL)
	return mysql, nil
}

func scenarioAutocompleteTableCount(scenario Scenario) int {
	switch scenario {
	case ScenarioAutocomplete100:
		return 100
	case ScenarioAutocomplete500:
		return 500
	case ScenarioAutocomplete2000:
		return 2000
	default:
		return 0
	}
}

// fixtureConnector is a deterministic database/sql connector. It injects RTT
// at QueryContext and observes every returned row at the actual driver boundary
// while presenting the production MySQL driver with ordinary *sql.DB calls.
type fixtureConnector struct {
	config fixtureConfig
}

func (connector *fixtureConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &fixtureConn{config: connector.config}, nil
}

func (connector *fixtureConnector) Driver() driver.Driver {
	return fixtureDriver{connector: connector}
}

type fixtureDriver struct {
	connector *fixtureConnector
}

func (driver fixtureDriver) Open(_ string) (driver.Conn, error) {
	return driver.connector.Connect(context.Background())
}

type fixtureConn struct {
	config fixtureConfig

	mu     sync.Mutex
	closed bool
}

var (
	_ driver.Connector      = (*fixtureConnector)(nil)
	_ driver.Driver         = fixtureDriver{}
	_ driver.Conn           = (*fixtureConn)(nil)
	_ driver.QueryerContext = (*fixtureConn)(nil)
)

func (connection *fixtureConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("benchmark fixture does not support prepared statements")
}

func (connection *fixtureConn) Close() error {
	connection.mu.Lock()
	connection.closed = true
	connection.mu.Unlock()
	return nil
}

func (connection *fixtureConn) Begin() (driver.Tx, error) {
	return nil, errors.New("benchmark fixture does not support transactions")
}

func (connection *fixtureConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	connection.mu.Lock()
	closed := connection.closed
	connection.mu.Unlock()
	if closed {
		return nil, errors.New("benchmark fixture connection is closed")
	}

	connection.config.metrics.observeOperation(isBlockingQuery(query))
	if err := waitForContext(ctx, connection.config.rtt); err != nil {
		return nil, err
	}

	columns, values, nextDelay, err := connection.rowsForQuery(query, args)
	if err != nil {
		return nil, err
	}
	return &fixtureRows{
		ctx:       ctx,
		columns:   columns,
		values:    values,
		nextDelay: nextDelay,
		metrics:   connection.config.metrics,
	}, nil
}

func (connection *fixtureConn) rowsForQuery(query string, args []driver.NamedValue) ([]string, [][]driver.Value, time.Duration, error) {
	normalized := strings.ToUpper(strings.TrimSpace(query))

	switch {
	case strings.HasPrefix(normalized, "SELECT * FROM"):
		return benchmarkRecordColumns(), connection.recordRows(normalized, args, true), 0, nil
	case strings.HasPrefix(normalized, "SHOW TABLES FROM"):
		values := make([][]driver.Value, 0, connection.config.tableCount)
		for i := 0; i < connection.config.tableCount; i++ {
			values = append(values, []driver.Value{fmt.Sprintf("autocomplete_%04d", i)})
		}
		return []string{"Tables_in_" + benchmarkDatabase}, values, 0, nil
	case strings.HasPrefix(normalized, "SHOW FULL COLUMNS FROM"):
		return fullColumnHeaders(), fullColumnRows(strings.Contains(normalized, "AUTOCOMPLETE_")), 0, nil
	case strings.Contains(normalized, "INFORMATION_SCHEMA.COLUMNS"):
		values := make([][]driver.Value, 0, max(len(args)-1, 0))
		for _, arg := range args[1:] {
			values = append(values, []driver.Value{
				argString(arg), "id", "int", "", "NO", "PRI", "", "", "", "",
			})
		}
		return []string{
			"TABLE_NAME", "COLUMN_NAME", "COLUMN_TYPE", "COLLATION_NAME",
			"IS_NULLABLE", "COLUMN_KEY", "COLUMN_DEFAULT", "EXTRA", "PRIVILEGES", "COLUMN_COMMENT",
		}, values, 0, nil
	case strings.Contains(normalized, "INFORMATION_SCHEMA.KEY_COLUMN_USAGE") && strings.Contains(normalized, "CONSTRAINT_NAME ="):
		return []string{"column_name"}, [][]driver.Value{{"id"}}, 0, nil
	case strings.Contains(normalized, "INFORMATION_SCHEMA.KEY_COLUMN_USAGE"):
		return []string{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME"}, [][]driver.Value{{"fk_owner", "owner_id", "users", "id"}}, 0, nil
	case strings.Contains(normalized, "INNODB_SYS_FOREIGN") || strings.Contains(normalized, "INNODB_FOREIGN"):
		values := make([][]driver.Value, 0, connection.config.catalogSize)
		for i := 0; i < connection.config.catalogSize; i++ {
			values = append(values, []driver.Value{
				benchmarkRecords, "owner_id", fmt.Sprintf("fk_%04d", i), "id", "users",
			})
		}
		return []string{"TABLE_NAME", "COLUMN_NAME", "CONSTRAINT_NAME", "REFERENCED_COLUMN_NAME", "REFERENCED_TABLE_NAME"}, values, 0, nil
	case strings.HasPrefix(normalized, "SHOW INDEX FROM"):
		return []string{
			"Table", "Non_unique", "Key_name", "Seq_in_index", "Column_name", "Collation",
			"Cardinality", "Sub_part", "Packed", "Null", "Index_type", "Comment", "Index_comment",
		}, [][]driver.Value{{benchmarkRecords, "0", "PRIMARY", "1", "id", "A", "10000", "", "", "", "BTREE", "", ""}}, 0, nil
	case strings.Contains(normalized, "SELECT TABLE_ROWS"):
		return []string{"TABLE_ROWS"}, [][]driver.Value{{int64(benchmarkRecordCount)}}, 0, nil
	case strings.HasPrefix(normalized, "SELECT COUNT(*)"):
		return []string{"COUNT(*)"}, [][]driver.Value{{int64(benchmarkRecordCount)}}, connection.config.slowCountDelay, nil
	case strings.HasPrefix(normalized, "SELECT ") && strings.Contains(normalized, "FROM `"+strings.ToUpper(benchmarkDatabase)+"`.`"+strings.ToUpper(benchmarkRecords)+"`"):
		return benchmarkRecordColumns(), connection.recordRows(normalized, args, false), connection.slowRowDelay(normalized), nil
	default:
		return nil, nil, 0, fmt.Errorf("benchmark fixture has no result for query %q", query)
	}
}

func (connection *fixtureConn) recordRows(query string, args []driver.NamedValue, paged bool) [][]driver.Value {
	ids := make([]int, 0, benchmarkRecordCount)
	for id := 1; id <= benchmarkRecordCount; id++ {
		if strings.Contains(query, "CATEGORY = 'EVEN'") && id%2 != 0 {
			continue
		}
		ids = append(ids, id)
	}
	if strings.Contains(query, "ORDER BY NAME DESC") {
		for left, right := 0, len(ids)-1; left < right; left, right = left+1, right-1 {
			ids[left], ids[right] = ids[right], ids[left]
		}
	}

	start := 0
	end := len(ids)
	if paged {
		start = argInt(args, 0)
		limit := argInt(args, 1)
		if start < 0 {
			start = 0
		}
		if start > len(ids) {
			start = len(ids)
		}
		end = start + limit
		if end > len(ids) {
			end = len(ids)
		}
	} else if limit := queryLimit(query); limit >= 0 && limit < end {
		end = limit
	}

	rows := make([][]driver.Value, 0, max(end-start, 0))
	for _, id := range ids[start:end] {
		category := "odd"
		if id%2 == 0 {
			category = "even"
		}
		rows = append(rows, []driver.Value{
			int64(id), int64(1), category, fmt.Sprintf("row-%05d", id),
		})
	}
	return rows
}

func (connection *fixtureConn) slowRowDelay(query string) time.Duration {
	if connection.config.slowRows && queryLimit(query) == 1 {
		return connection.config.slowRowDelay
	}
	return 0
}

type fixtureRows struct {
	ctx       context.Context
	columns   []string
	values    [][]driver.Value
	nextDelay time.Duration
	metrics   *observedMetrics

	mu     sync.Mutex
	index  int
	closed bool
}

func (rows *fixtureRows) Columns() []string {
	return append([]string(nil), rows.columns...)
}

func (rows *fixtureRows) Close() error {
	rows.mu.Lock()
	rows.closed = true
	rows.mu.Unlock()
	return nil
}

func (rows *fixtureRows) Next(dest []driver.Value) error {
	rows.mu.Lock()
	if rows.closed {
		rows.mu.Unlock()
		return io.EOF
	}
	if rows.index >= len(rows.values) {
		rows.mu.Unlock()
		return io.EOF
	}
	value := append([]driver.Value(nil), rows.values[rows.index]...)
	rows.index++
	rows.mu.Unlock()

	if err := waitForContext(rows.ctx, rows.nextDelay); err != nil {
		return err
	}
	if err := rows.ctx.Err(); err != nil {
		return err
	}
	copy(dest, value)
	rows.metrics.observeRow(value)
	return nil
}

func waitForContext(ctx context.Context, delay time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if delay <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func isBlockingQuery(query string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(query))
	if strings.HasPrefix(normalized, "SHOW TABLES FROM") || strings.HasPrefix(normalized, "SELECT * FROM") {
		return true
	}
	return strings.HasPrefix(normalized, "SELECT ") &&
		strings.Contains(normalized, "FROM `"+strings.ToUpper(benchmarkDatabase)+"`.`"+strings.ToUpper(benchmarkRecords)+"`") &&
		!strings.HasPrefix(normalized, "SELECT COUNT(*)")
}

func queryLimit(query string) int {
	normalized := strings.ToUpper(query)
	index := strings.LastIndex(normalized, "LIMIT ")
	if index < 0 {
		return -1
	}
	fields := strings.Fields(normalized[index+len("LIMIT "):])
	if len(fields) == 0 {
		return -1
	}
	limit, err := strconv.Atoi(strings.TrimSuffix(fields[0], ";"))
	if err != nil || limit < 0 {
		return -1
	}
	return limit
}

func argInt(args []driver.NamedValue, index int) int {
	if index < 0 || index >= len(args) {
		return 0
	}
	switch value := args[index].Value.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case int32:
		return int(value)
	default:
		parsed, _ := strconv.Atoi(fmt.Sprint(value))
		return parsed
	}
}

func argString(arg driver.NamedValue) string {
	return fmt.Sprint(arg.Value)
}

func benchmarkRecordColumns() []string {
	return []string{"id", "owner_id", "category", "name"}
}

func fullColumnHeaders() []string {
	return []string{"Field", "Type", "Collation", "Null", "Key", "Default", "Extra", "Privileges", "Comment"}
}

func fullColumnRows(autocomplete bool) [][]driver.Value {
	if autocomplete {
		return [][]driver.Value{{"id", "int", "", "NO", "PRI", "", "", "", ""}}
	}
	return [][]driver.Value{
		{"id", "int", "", "NO", "PRI", "", "", "", ""},
		{"owner_id", "int", "", "NO", "MUL", "", "", "", ""},
		{"category", "varchar(16)", "", "NO", "", "", "", "", ""},
		{"name", "varchar(32)", "", "NO", "", "", "", "", ""},
	}
}

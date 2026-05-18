package components

import (
	"testing"
)

// ---------------------------------------------------------------------------
// resolveAliases
// ---------------------------------------------------------------------------

func TestResolveAliases_SimpleFrom(t *testing.T) {
	sql := "SELECT * FROM users u"
	aliases := resolveAliases(sql, len(sql))
	if aliases["u"] != "users" {
		t.Errorf("expected aliases['u']='users', got '%s'", aliases["u"])
	}
}

func TestResolveAliases_WithAS(t *testing.T) {
	sql := "SELECT * FROM users AS u"
	aliases := resolveAliases(sql, len(sql))
	if aliases["u"] != "users" {
		t.Errorf("expected aliases['u']='users', got '%s'", aliases["u"])
	}
}

func TestResolveAliases_Join(t *testing.T) {
	sql := "SELECT * FROM users u JOIN profiles p ON u.id = p.user_id"
	aliases := resolveAliases(sql, len(sql))
	if aliases["u"] != "users" {
		t.Errorf("expected aliases['u']='users', got '%s'", aliases["u"])
	}
	if aliases["p"] != "profiles" {
		t.Errorf("expected aliases['p']='profiles', got '%s'", aliases["p"])
	}
}

func TestResolveAliases_CommaSeparated(t *testing.T) {
	sql := "SELECT * FROM users u, profiles p"
	aliases := resolveAliases(sql, len(sql))
	if aliases["u"] != "users" {
		t.Errorf("expected aliases['u']='users', got '%s'", aliases["u"])
	}
	if aliases["p"] != "profiles" {
		t.Errorf("expected aliases['p']='profiles', got '%s'", aliases["p"])
	}
}

func TestResolveAliases_CommaWithAS(t *testing.T) {
	sql := "SELECT * FROM users AS u, profiles AS p"
	aliases := resolveAliases(sql, len(sql))
	if aliases["u"] != "users" {
		t.Errorf("expected aliases['u']='users', got '%s'", aliases["u"])
	}
	if aliases["p"] != "profiles" {
		t.Errorf("expected aliases['p']='profiles', got '%s'", aliases["p"])
	}
}

func TestResolveAliases_NoAlias(t *testing.T) {
	sql := "SELECT * FROM users"
	aliases := resolveAliases(sql, len(sql))
	if len(aliases) != 0 {
		t.Errorf("expected no aliases, got %v", aliases)
	}
}

func TestResolveAliases_SubqueryDoesNotLeak(t *testing.T) {
	// Alias 'a' is inside the subquery and should NOT appear at the outer depth.
	sql := "SELECT * FROM (SELECT * FROM admins a) AS sub"
	// Cursor at outer query depth
	aliases := resolveAliases(sql, 5) // cursor at pos 5 = outer depth 0
	if _, ok := aliases["a"]; ok {
		t.Errorf("alias 'a' from inside subquery leaked to outer scope")
	}
	// But when cursor is inside the subquery, alias 'a' should be visible
	innerPos := len("SELECT * FROM (SELECT * FROM ")
	innerAliases := resolveAliases(sql, innerPos+2)
	if innerAliases["a"] != "admins" {
		t.Errorf("expected aliases['a']='admins' from inside subquery, got '%s'", innerAliases["a"])
	}
}

func TestResolveAliases_IntoClause(t *testing.T) {
	sql := "INSERT INTO logs l (message) VALUES ('hello')"
	aliases := resolveAliases(sql, len(sql))
	if aliases["l"] != "logs" {
		t.Errorf("expected aliases['l']='logs', got '%s'", aliases["l"])
	}
}

func TestResolveAliases_UpdateClause(t *testing.T) {
	sql := "UPDATE accounts a SET balance = 0"
	aliases := resolveAliases(sql, len(sql))
	if aliases["a"] != "accounts" {
		t.Errorf("expected aliases['a']='accounts', got '%s'", aliases["a"])
	}
}

func TestResolveAliases_QualifiedName(t *testing.T) {
	sql := "SELECT * FROM public.users u"
	aliases := resolveAliases(sql, len(sql))
	if aliases["u"] != "public.users" {
		t.Errorf("expected aliases['u']='public.users', got '%s'", aliases["u"])
	}
}

func TestResolveAliases_CTE(t *testing.T) {
	sql := "WITH active AS (SELECT * FROM users WHERE status = 'active') SELECT * FROM active a"
	aliases := resolveAliases(sql, len(sql))
	// 'active' is a CTE, which should be in the aliases map.
	if aliases["active"] != "active" {
		t.Errorf("expected aliases['active']='active', got '%s'", aliases["active"])
	}
	// 'a' is an alias for the CTE 'active'.
	if aliases["a"] != "active" {
		t.Errorf("expected aliases['a']='active', got '%s'", aliases["a"])
	}
}

func TestResolveAliases_Empty(t *testing.T) {
	aliases := resolveAliases("", 0)
	if len(aliases) != 0 {
		t.Errorf("expected empty map, got %v", aliases)
	}
}

func TestResolveAliases_NoFromClause(t *testing.T) {
	aliases := resolveAliases("SELECT 1", 0)
	if len(aliases) != 0 {
		t.Errorf("expected empty map, got %v", aliases)
	}
}

// ---------------------------------------------------------------------------
// extractTableHint
// ---------------------------------------------------------------------------

func TestExtractTableHint_SimpleFrom(t *testing.T) {
	sql := "SELECT * FROM users"
	// Cursor at end — should hint "users"
	hint := extractTableHint(sql, len(sql))
	if hint != "users" {
		t.Errorf("expected 'users', got '%s'", hint)
	}
}

func TestExtractTableHint_AfterAlias(t *testing.T) {
	sql := "SELECT u.name FROM users u"
	// Cursor after "u." — should hint "users" (resolved alias)
	hint := extractTableHint(sql, len(sql))
	if hint != "users" {
		t.Errorf("expected 'users', got '%s'", hint)
	}
}

func TestExtractTableHint_SubqueryScope(t *testing.T) {
	// Cursor inside the subquery should hint the inner table, not the outer one.
	sql := "SELECT * FROM (SELECT * FROM admins) sub WHERE "
	// Find position of "admins)" to place cursor right after the inner FROM
	innerPos := len("SELECT * FROM (SELECT * FROM ")
	hint := extractTableHint(sql, innerPos+3) // right after "adm"
	if hint != "admins" {
		t.Errorf("expected 'admins' (inner table), got '%s'", hint)
	}
}

func TestExtractTableHint_Empty(t *testing.T) {
	hint := extractTableHint("", 0)
	if hint != "" {
		t.Errorf("expected empty, got '%s'", hint)
	}
}

func TestExtractTableHint_NestedJoin(t *testing.T) {
	sql := "SELECT * FROM users u JOIN profiles p ON u.id = p.user_id"
	// Cursor in WHERE clause area — should hint the most recent table (profiles)
	hint := extractTableHint(sql, len(sql))
	if hint != "profiles" {
		t.Errorf("expected 'profiles', got '%s'", hint)
	}
}

// ---------------------------------------------------------------------------
// cursorDepth
// ---------------------------------------------------------------------------

func TestCursorDepth_Zero(t *testing.T) {
	d := cursorDepth("SELECT * FROM users", 0)
	if d != 0 {
		t.Errorf("expected depth 0, got %d", d)
	}
}

func TestCursorDepth_InsideSubquery(t *testing.T) {
	sql := "SELECT * FROM (SELECT * FROM admins) sub"
	// Position inside subquery
	innerPos := len("SELECT * FROM (")
	d := cursorDepth(sql, innerPos+1)
	if d != 1 {
		t.Errorf("expected depth 1, got %d", d)
	}
}

func TestCursorDepth_AfterSubquery(t *testing.T) {
	sql := "SELECT * FROM (SELECT * FROM admins) sub WHERE 1=1"
	// Position after the closing paren
	parenPos := len("SELECT * FROM (SELECT * FROM admins)")
	d := cursorDepth(sql, parenPos)
	if d != 0 {
		t.Errorf("expected depth 0, got %d", d)
	}
}

// ---------------------------------------------------------------------------
// scanSQLContext.Tables
// ---------------------------------------------------------------------------

func TestScanSQLContext_TablesUnique(t *testing.T) {
	ctx := scanSQLContext("SELECT * FROM users u JOIN profiles p ON u.id = p.user_id JOIN addresses a ON u.id = a.user_id")
	if len(ctx.Tables) != 3 {
		t.Errorf("expected 3 unique tables, got %d: %v", len(ctx.Tables), ctx.Tables)
	}
}

func TestScanSQLContext_TablesPreserveOrder(t *testing.T) {
	ctx := scanSQLContext("SELECT * FROM profiles JOIN users JOIN addresses")
	if len(ctx.Tables) < 3 {
		t.Fatalf("expected at least 3 tables, got %d", len(ctx.Tables))
	}
	if ctx.Tables[0] != "profiles" {
		t.Errorf("expected first table 'profiles', got '%s'", ctx.Tables[0])
	}
	if ctx.Tables[1] != "users" {
		t.Errorf("expected second table 'users', got '%s'", ctx.Tables[1])
	}
}

func TestScanSQLContext_CTEInTables(t *testing.T) {
	ctx := scanSQLContext("WITH cte AS (SELECT 1) SELECT * FROM cte")
	if !ctx.CTEs["cte"] {
		t.Errorf("expected 'cte' to be in CTEs")
	}
}

func TestScanSQLContext_NoTables(t *testing.T) {
	ctx := scanSQLContext("SELECT 1")
	if len(ctx.Tables) != 0 {
		t.Errorf("expected 0 tables, got %d", len(ctx.Tables))
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestResolveAliases_CTEWithColumnList(t *testing.T) {
	sql := "WITH cte (col1, col2) AS (SELECT a, b FROM table1) SELECT * FROM cte c"
	aliases := resolveAliases(sql, len(sql))
	if aliases["cte"] != "cte" {
		t.Errorf("expected aliases['cte']='cte', got '%s'", aliases["cte"])
	}
	if aliases["c"] != "cte" {
		t.Errorf("expected aliases['c']='cte', got '%s'", aliases["c"])
	}
}

func TestResolveAliases_MultipleCTEs(t *testing.T) {
	sql := "WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a x JOIN b y"
	aliases := resolveAliases(sql, len(sql))
	if aliases["a"] != "a" {
		t.Errorf("expected aliases['a']='a', got '%s'", aliases["a"])
	}
	if aliases["b"] != "b" {
		t.Errorf("expected aliases['b']='b', got '%s'", aliases["b"])
	}
	if aliases["x"] != "a" {
		t.Errorf("expected aliases['x']='a', got '%s'", aliases["x"])
	}
	if aliases["y"] != "b" {
		t.Errorf("expected aliases['y']='b', got '%s'", aliases["y"])
	}
}

func TestResolveAliases_WithRecursive(t *testing.T) {
	sql := "WITH RECURSIVE t(n) AS (SELECT 1 UNION SELECT n+1 FROM t WHERE n < 10) SELECT * FROM t"
	aliases := resolveAliases(sql, len(sql))
	if aliases["t"] != "t" {
		t.Errorf("expected aliases['t']='t', got '%s'", aliases["t"])
	}
}

func TestResolveAliases_JoinUsing(t *testing.T) {
	sql := "SELECT * FROM users u JOIN profiles p USING (user_id)"
	aliases := resolveAliases(sql, len(sql))
	if aliases["u"] != "users" {
		t.Errorf("expected aliases['u']='users', got '%s'", aliases["u"])
	}
	if aliases["p"] != "profiles" {
		t.Errorf("expected aliases['p']='profiles', got '%s'", aliases["p"])
	}
}

func TestResolveAliases_InnerJoinAlias(t *testing.T) {
	sql := "SELECT * FROM users u INNER JOIN profiles p ON u.id = p.user_id"
	aliases := resolveAliases(sql, len(sql))
	if aliases["u"] != "users" {
		t.Errorf("expected aliases['u']='users', got '%s'", aliases["u"])
	}
	if aliases["p"] != "profiles" {
		t.Errorf("expected aliases['p']='profiles', got '%s'", aliases["p"])
	}
}

func TestResolveAliases_SubqueryInFromWithAlias(t *testing.T) {
	// The scanner registers the subquery's own alias ("sub") at depth 0.
	// This is OK — it helps resolve columns when typing "sub.col".
	sql := "SELECT * FROM (SELECT * FROM admins) sub"
	aliases := resolveAliases(sql, len(sql))
	// 'sub' should be registered as a table reference (the subquery alias).
	if aliases["sub"] != "sub" {
		t.Errorf("expected aliases['sub']='sub', got '%s'", aliases["sub"])
	}
}

func TestExtractTableHint_SubqueryFromClause(t *testing.T) {
	sql := "SELECT * FROM (SELECT * FROM admins) sub WHERE sub.status = "
	// Cursor at end, depth 0 — the most recent table at depth 0 is 'sub'.
	hint := extractTableHint(sql, len(sql))
	if hint != "sub" {
		t.Errorf("expected 'sub' (subquery alias), got '%s'", hint)
	}
}

func TestExtractTableHint_MultipleTables(t *testing.T) {
	sql := "SELECT * FROM users u JOIN profiles p ON "
	// Cursor at end should hint the most recent table (profiles)
	hint := extractTableHint(sql, len(sql))
	if hint != "profiles" {
		t.Errorf("expected 'profiles', got '%s'", hint)
	}
}

func TestExtractTableHint_CommaList(t *testing.T) {
	sql := "SELECT * FROM users, "
	hint := extractTableHint(sql, len(sql))
	if hint != "" {
		t.Errorf("expected empty hint after comma, got '%s'", hint)
	}
}

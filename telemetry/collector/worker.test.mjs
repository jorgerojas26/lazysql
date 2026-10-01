import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { DatabaseSync } from "node:sqlite";
import worker from "./worker.mjs";

const token = "a".repeat(64);
const minuteNow = () => Math.floor(Date.now() / 60000) * 60;

function fixture(t) {
  const db = new DatabaseSync(":memory:");
  db.exec(readFileSync(new URL("./migrations/0001_counts.sql", import.meta.url), "utf8"));
  db.exec(readFileSync(new URL("./migrations/0002_metrics.sql", import.meta.url), "utf8"));
  t.after(() => db.close());
  return {
    db,
    env: {
      RELEASES: "0.5.8,1.2.3", STATS_TOKEN: token,
      DB: { async batch(statements) {
        db.exec("BEGIN");
        try {
          const results = await Promise.all(statements.map(s => s.run()));
          db.exec("COMMIT");
          return results;
        } catch (err) {
          db.exec("ROLLBACK");
          throw err;
        }
      }, prepare(sql) {
        const statement = db.prepare(sql);
        return { bind(...args) {
          return { async run() { return statement.run(...args); }, async all() { return { results: statement.all(...args) }; } };
        } };
      } },
    },
  };
}

function eventRequest(payload, headers = {}) {
  return new Request("https://example.com/v1/events", {
    method: "POST", headers: { "Content-Type": "application/json", ...headers }, body: JSON.stringify(payload),
  });
}
const event = (kind = "start", version = "0.5.8") => ({ schema: 2, event: kind, version,
  ...(kind === "start" ? { startup_mode: "picker", distribution: "release" } : {}),
});
const connection = (engine = "postgres", read_only = false) => ({ schema: 2, event: "connection", version: "0.5.8", engine, read_only });
const failure = (category = "unknown") => ({ schema: 2, event: "connection_failure", version: "0.5.8", failure: category });

async function stats(env) {
  return worker.fetch(new Request("https://example.com/stats", { headers: { Authorization: `Bearer ${token}` } }), env);
}

test("stores counters only and ignores transport identifiers", async t => {
  const { db, env } = fixture(t);
  for (const kind of ["start", "heartbeat", "heartbeat"]) {
    const response = await worker.fetch(eventRequest(event(kind), {
      "CF-Connecting-IP": "203.0.113.5", "User-Agent": "unique-device", Cookie: "id=secret",
    }), env);
    assert.equal(response.status, 204);
    assert.equal(response.headers.get("Set-Cookie"), null);
  }
  const rows = db.prepare("SELECT * FROM counts WHERE metric IN ('start', 'heartbeat') ORDER BY metric").all();
  assert.deepEqual(rows.map(r => ({ ...r })), [
    { minute: minuteNow(), version: "0.5.8", metric: "heartbeat", value: "total", count: 2 },
    { minute: minuteNow(), version: "0.5.8", metric: "start", value: "total", count: 1 },
  ]);
  assert.deepEqual(db.prepare("PRAGMA table_info(counts)").all().map(r => r.name), ["minute", "version", "metric", "value", "count"]);
  assert.deepEqual(db.prepare("SELECT name FROM sqlite_master WHERE type = 'table'").all().map(r => r.name), ["counts"]);
});

test("rejects extra fields, unsupported schemas, and unbounded versions", async t => {
  const { db, env } = fixture(t);
  for (const payload of [null, [], {}, { ...event(), sql: "SELECT secret" }, { ...event(), id: "someone" },
    { ...event(), schema: 3 }, { ...event(), event: "query" }, event("start", "1.2.3+host"), event("start", "x".repeat(300)),
    event("start", 123), event("start", "1000.1.1")]) {
    assert.equal((await worker.fetch(eventRequest(payload), env)).status, 400);
  }
  assert.equal(db.prepare("SELECT count(*) AS n FROM counts").get().n, 0);
});

test("limits the body even without Content-Length", async t => {
  const { env } = fixture(t);
  const request = new Request("https://example.com/v1/events", {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: new ReadableStream({ start(c) { c.enqueue(new TextEncoder().encode(" ".repeat(1025))); c.close(); } }), duplex: "half",
  });
  assert.equal((await worker.fetch(request, env)).status, 400);
});

test("unlisted releases share a bounded other bucket", async t => {
  const { db, env } = fixture(t);
  for (const version of ["9.9.8", "9.9.9", "dev", "1.2.3"]) {
    assert.equal((await worker.fetch(eventRequest(event("start", version)), env)).status, 204);
  }
  const rows = db.prepare("SELECT version, count FROM counts WHERE metric = 'start' ORDER BY version").all().map(r => ({ ...r }));
  assert.deepEqual(rows, [{ version: "1.2.3", count: 1 }, { version: "dev", count: 1 }, { version: "other", count: 2 }]);
});

test("stats requires a secret, never accepts credentials in URLs", async t => {
  const { env } = fixture(t);
  for (const auth of ["", "Bearer wrong", `Bearer ${"b".repeat(64)}`]) {
    const response = await worker.fetch(new Request("https://example.com/stats", { headers: { Authorization: auth } }), env);
    assert.equal(response.status, 401);
  }
  assert.equal((await stats({ ...env, STATS_TOKEN: undefined })).status, 503);
  assert.equal((await worker.fetch(new Request(`https://example.com/stats?token=${token}`), env)).status, 400);
  const response = await stats(env);
  assert.equal(response.status, 200);
  assert.equal(response.headers.get("Cache-Control"), "no-store");
  const result = await response.json();
  assert.equal(result.estimated_running_instances, 0);
  assert.equal(result.heartbeat_interval_minutes, 5);
  assert.equal(result.heartbeat_window_minutes, 10);
});

test("live estimate uses two five-minute intervals, excluding partial and stale buckets", async t => {
  const { db, env } = fixture(t);
  const minute = minuteNow();
  const insert = db.prepare("INSERT INTO counts VALUES (?, ?, ?, 'total', ?)");
  insert.run(minute, "0.5.8", "heartbeat", 100);
  insert.run(minute - 60, "0.5.8", "heartbeat", 2);
  insert.run(minute - 300, "0.5.8", "heartbeat", 4);
  insert.run(minute - 360, "0.5.8", "heartbeat", 6);
  insert.run(minute - 600, "0.5.8", "heartbeat", 8);
  insert.run(minute - 660, "0.5.8", "heartbeat", 50);
  insert.run(minute - 3600, "0.5.8", "heartbeat", 200);
  insert.run(minute - 60, "dev", "heartbeat", 2);
  insert.run(minute, "0.5.8", "start", 1);
  insert.run(minute - 3600, "0.5.8", "start", 99);
  const result = await (await stats(env)).json();
  assert.equal(result.estimated_running_instances, 11);
  assert.equal(result.starts, 1);
  assert.equal(result.metrics.length, 8);
  assert.equal(result.versions.find(v => v.version === "0.5.8").estimated_running_instances, 10);
});

test("cleanup deletes aggregates older than 30 days", async t => {
  const { db, env } = fixture(t);
  const cutoff = minuteNow() - 30 * 24 * 60 * 60;
  const insert = db.prepare("INSERT INTO counts VALUES (?, 'dev', 'start', 'total', 1)");
  insert.run(cutoff - 60);
  insert.run(cutoff);
  insert.run(minuteNow());
  await worker.scheduled({}, env);
  assert.equal(db.prepare("SELECT count(*) AS n FROM counts").get().n, 2);
});

test("database failures return generic errors", async () => {
  const env = { DB: { prepare() { throw new Error("sensitive infrastructure details"); } } };
  const response = await worker.fetch(eventRequest(event()), env);
  assert.equal(response.status, 503);
  assert.equal(await response.text(), "");
});

test("only expected methods and paths are accepted", async t => {
  const { env } = fixture(t);
  assert.equal((await worker.fetch(new Request("https://example.com/v1/events"), env)).status, 405);
  assert.equal((await worker.fetch(new Request("https://example.com/elsewhere"), env)).status, 404);
  assert.equal((await worker.fetch(new Request("https://example.com/v1/events", { method: "POST", body: "{}" }), env)).status, 400);
});

test("product dimensions and deltas are independent aggregate counters", async t => {
  const { db, env } = fixture(t);
  for (const payload of [event(), { ...event(), startup_mode: "connection_arg", distribution: "homebrew" },
    connection(), connection(), connection("sqlite3", true), failure("auth"), failure("unknown"),
    { ...event("heartbeat"), features: { query_execute: 3, row_insert: 1, csv_export: 2 } },
    { ...event("heartbeat"), features: { query_execute: 2, external_editor: 1 } }]) {
    assert.equal((await worker.fetch(eventRequest(payload), env)).status, 204);
  }
  const rows = db.prepare("SELECT metric, value, count FROM counts ORDER BY metric, value").all().map(r => ({ ...r }));
  assert.deepEqual(rows, [
    { metric: "connection_engine", value: "postgres", count: 2 },
    { metric: "connection_engine", value: "sqlite3", count: 1 },
    { metric: "connection_failure", value: "auth", count: 1 },
    { metric: "connection_failure", value: "unknown", count: 1 },
    { metric: "distribution", value: "homebrew", count: 1 },
    { metric: "distribution", value: "release", count: 1 },
    { metric: "feature", value: "csv_export", count: 2 },
    { metric: "feature", value: "external_editor", count: 1 },
    { metric: "feature", value: "query_execute", count: 5 },
    { metric: "feature", value: "row_insert", count: 1 },
    { metric: "heartbeat", value: "total", count: 2 },
    { metric: "read_only", value: "false", count: 2 },
    { metric: "read_only", value: "true", count: 1 },
    { metric: "start", value: "total", count: 2 },
    { metric: "startup_mode", value: "connection_arg", count: 1 },
    { metric: "startup_mode", value: "picker", count: 1 },
  ]);
  assert.deepEqual((await (await stats(env)).json()).metrics.map(({ minute, version, ...r }) => r), rows);
});

test("all enums and counter bounds are enforced with no partial writes", async t => {
  const { db, env } = fixture(t);
  const bad = [
    connection("private-host"), connection("sqlite"), connection("oracle"), connection("postgres", "false"),
    { ...connection(), read_only: undefined }, { ...connection(), hostname: "sensitive" },
    { ...connection(), version: "0.5.8\n" },
    failure("raw error message"), { ...failure(), failure: null }, { ...failure(), error: "secret" },
    { ...event(), startup_mode: "secret URL" }, { ...event(), distribution: "github-user-download" },
    { ...event(), distribution: 1 }, { ...event(), startup_mode: undefined },
    { ...event(), read_only: false }, // metadata only at its own boundary
    { ...event("heartbeat"), features: null }, { ...event("heartbeat"), features: [] },
    { ...event("heartbeat"), features: "sql" }, { ...event("heartbeat"), sql: "SELECT secret" },
    { ...event("heartbeat"), features: { query_execute: 1, "private-table": 1 } },
    { ...event("heartbeat"), features: { __proto__: null, constructor: 1 } },
    ...[0, -1, 0.5, 10001, "1", null, 1e100].map(n => ({ ...event("heartbeat"), features: { query_execute: n } })),
  ];
  for (const payload of bad) {
    assert.equal((await worker.fetch(eventRequest(payload), env)).status, 400, JSON.stringify(payload));
  }
  assert.equal(db.prepare("SELECT count(*) AS n FROM counts").get().n, 0);
});

test("all supported enum values and largest complete feature batch fit", async t => {
  const { db, env } = fixture(t);
  for (const engine of ["postgres", "mysql", "sqlite3", "sqlserver", "clickhouse", "other"]) {
    assert.equal((await worker.fetch(eventRequest(connection(engine)), env)).status, 204);
  }
  for (const category of ["auth", "network", "timeout", "tls", "invalid_connection", "driver", "unknown"]) {
    assert.equal((await worker.fetch(eventRequest(failure(category)), env)).status, 204);
  }
  for (const distribution of ["homebrew", "release", "source", "unknown"]) {
    assert.equal((await worker.fetch(eventRequest({ ...event(), distribution }), env)).status, 204);
  }
  const features = Object.fromEntries(["query_execute", "query_history", "foreign_key_jump", "reverse_foreign_key_jump",
    "json_viewer", "csv_export", "external_editor", "row_insert", "row_update", "row_delete"].map(f => [f, 10000]));
  assert.equal((await worker.fetch(eventRequest({ ...event("heartbeat"), features }), env)).status, 204);
  assert.equal(db.prepare("SELECT sum(count) AS n FROM counts WHERE metric = 'feature'").get().n, 100000);
});

test("migration preserves old aggregate counts and old clients still aggregate", async t => {
  const db = new DatabaseSync(":memory:");
  t.after(() => db.close());
  db.exec(readFileSync(new URL("./migrations/0001_counts.sql", import.meta.url), "utf8"));
  db.exec("INSERT INTO counts VALUES (60, '0.5.8', 2, 3), (120, 'dev', 0, 4)");
  db.exec(readFileSync(new URL("./migrations/0002_metrics.sql", import.meta.url), "utf8"));
  assert.deepEqual(db.prepare("SELECT * FROM counts ORDER BY minute, metric").all().map(r => ({ ...r })), [
    { minute: 60, version: "0.5.8", metric: "heartbeat", value: "total", count: 3 },
    { minute: 60, version: "0.5.8", metric: "start", value: "total", count: 2 },
    { minute: 120, version: "dev", metric: "heartbeat", value: "total", count: 4 },
  ]);
  const { env } = fixture(t);
  for (const kind of ["start", "heartbeat"]) {
    assert.equal((await worker.fetch(eventRequest({ schema: 1, event: kind, version: "0.5.8" }), env)).status, 204);
  }
});

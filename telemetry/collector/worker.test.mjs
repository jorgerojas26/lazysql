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
  t.after(() => db.close());
  return {
    db,
    env: {
      RELEASES: "0.5.8,1.2.3", STATS_TOKEN: token,
      DB: { prepare(sql) {
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
const event = (kind = "start", version = "0.5.8") => ({ schema: 1, event: kind, version });

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
  const rows = db.prepare("SELECT * FROM counts").all();
  assert.equal(rows.length, 1);
  assert.deepEqual({ ...rows[0] }, { minute: minuteNow(), version: "0.5.8", starts: 1, heartbeats: 2 });
  assert.deepEqual(db.prepare("PRAGMA table_info(counts)").all().map(r => r.name), ["minute", "version", "starts", "heartbeats"]);
});

test("rejects extra fields, unsupported schemas, and unbounded versions", async t => {
  const { db, env } = fixture(t);
  for (const payload of [null, [], {}, { ...event(), sql: "SELECT secret" }, { ...event(), id: "someone" },
    { ...event(), schema: 2 }, { ...event(), event: "query" }, event("start", "1.2.3+host"), event("start", "x".repeat(300)),
    event("start", 123), event("start", "1000.1.1")]) {
    assert.equal((await worker.fetch(eventRequest(payload), env)).status, 400);
  }
  assert.equal(db.prepare("SELECT count(*) AS n FROM counts").get().n, 0);
});

test("limits the body even without Content-Length", async t => {
  const { env } = fixture(t);
  const request = new Request("https://example.com/v1/events", {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: new ReadableStream({ start(c) { c.enqueue(new TextEncoder().encode(" ".repeat(257))); c.close(); } }), duplex: "half",
  });
  assert.equal((await worker.fetch(request, env)).status, 400);
});

test("unlisted releases share a bounded other bucket", async t => {
  const { db, env } = fixture(t);
  for (const version of ["9.9.8", "9.9.9", "dev", "1.2.3"]) {
    assert.equal((await worker.fetch(eventRequest(event("start", version)), env)).status, 204);
  }
  const rows = db.prepare("SELECT version, starts FROM counts ORDER BY version").all().map(r => ({ ...r }));
  assert.deepEqual(rows, [{ version: "1.2.3", starts: 1 }, { version: "dev", starts: 1 }, { version: "other", starts: 2 }]);
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
  assert.equal((await response.json()).estimated_running_instances, 0);
});

test("live estimate excludes partial current minute and old heartbeats", async t => {
  const { db, env } = fixture(t);
  const minute = minuteNow();
  const insert = db.prepare("INSERT INTO counts VALUES (?, ?, ?, ?)");
  insert.run(minute, "0.5.8", 1, 200);
  insert.run(minute - 60, "0.5.8", 2, 4);
  insert.run(minute - 120, "0.5.8", 3, 6);
  insert.run(minute - 180, "0.5.8", 4, 200);
  insert.run(minute - 3600, "0.5.8", 99, 200);
  insert.run(minute - 60, "dev", 0, 2);
  const result = await (await stats(env)).json();
  assert.equal(result.estimated_running_instances, 6);
  assert.equal(result.starts, 10);
  assert.equal(result.minutes.length, 5);
  assert.equal(result.versions.find(v => v.version === "0.5.8").estimated_running_instances, 5);
});

test("cleanup deletes aggregates older than 30 days", async t => {
  const { db, env } = fixture(t);
  const cutoff = minuteNow() - 30 * 24 * 60 * 60;
  const insert = db.prepare("INSERT INTO counts VALUES (?, 'dev', 1, 0)");
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

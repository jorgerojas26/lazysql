// No analytics SDK, raw event storage, cookies, IDs, IP inspection or logging.
const headers = { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" };
const releasePattern = /^(0|[1-9][0-9]{0,2})\.(0|[1-9][0-9]{0,2})\.(0|[1-9][0-9]{0,2})$/;
const HEARTBEAT_INTERVAL_MINUTES = 5;
const HEARTBEAT_WINDOW_SECONDS = 2 * HEARTBEAT_INTERVAL_MINUTES * 60;
const minuteNow = () => Math.floor(Date.now() / 60000) * 60;

function reply(status, data) {
  return data === undefined
    ? new Response(null, { status, headers })
    : Response.json(data, { status, headers });
}

const engines = ["postgres", "mysql", "sqlite3", "sqlserver", "clickhouse", "other"];
const features = ["query_execute", "query_history", "foreign_key_jump", "reverse_foreign_key_jump",
  "json_viewer", "csv_export", "external_editor", "row_insert", "row_update", "row_delete"];
const failures = ["auth", "network", "timeout", "tls", "invalid_connection", "driver", "unknown"];
const distributions = ["homebrew", "release", "source", "unknown"];
const maxFeatureCount = 10000;
const maxBodyBytes = 1024;
const object = value => value !== null && typeof value === "object" && !Array.isArray(value);
const keysAre = (value, keys) => Object.keys(value).sort().join(",") === keys.sort().join(",");

// Validation and expansion are explicit: clients cannot invent dimensions or
// attach arbitrary strings. Each accepted event becomes independent counters,
// not a stored event or a joined startup/connection/session record.
function counters(event) {
  if (!object(event) || typeof event.version !== "string" ||
      !(event.version === "dev" || releasePattern.test(event.version))) return null;
  const base = ["schema", "event", "version"];
  // Allow already-released opt-in clients; no raw events even during migration.
  if (event.schema === 1 && keysAre(event, base) && ["start", "heartbeat"].includes(event.event)) {
    return [[event.event, "total", 1]];
  }
  if (event.schema !== 2) return null;
  switch (event.event) {
    case "start":
      if (!keysAre(event, [...base, "startup_mode", "distribution"]) ||
          !["picker", "connection_arg"].includes(event.startup_mode) || !distributions.includes(event.distribution)) return null;
      return [["start", "total", 1], ["startup_mode", event.startup_mode, 1], ["distribution", event.distribution, 1]];
    case "connection":
      if (!keysAre(event, [...base, "engine", "read_only"]) ||
          !engines.includes(event.engine) || typeof event.read_only !== "boolean") return null;
      return [["connection_engine", event.engine, 1], ["read_only", String(event.read_only), 1]];
    case "connection_failure":
      if (!keysAre(event, [...base, "failure"]) || !failures.includes(event.failure)) return null;
      return [["connection_failure", event.failure, 1]];
    case "heartbeat": {
      if (!keysAre(event, base) && !keysAre(event, [...base, "features"])) return null;
      const result = [["heartbeat", "total", 1]];
      if (event.features !== undefined) {
        if (!object(event.features) || Object.keys(event.features).length > features.length) return null;
        for (const [feature, count] of Object.entries(event.features)) {
          if (!features.includes(feature) || !Number.isInteger(count) || count < 1 || count > maxFeatureCount) return null;
          result.push(["feature", feature, count]);
        }
      }
      return result;
    }
  }
  return null;
}

async function readEvent(request) {
  if (request.headers.get("Content-Type")?.split(";")[0].trim() !== "application/json" || !request.body) {
    return null;
  }
  const reader = request.body.getReader();
  const chunks = [];
  let size = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > maxBodyBytes) {
        await reader.cancel();
        return null;
      }
      chunks.push(value);
    }
    const bytes = new Uint8Array(size);
    let offset = 0;
    for (const chunk of chunks) {
      bytes.set(chunk, offset);
      offset += chunk.byteLength;
    }
    const event = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes));
    const counts = counters(event);
    return counts ? { version: event.version, counts } : null;
  } catch {
    return null;
  } finally {
    reader.releaseLock();
  }
}

// Never store an arbitrary attacker-supplied version as a new dimension.
function versionBucket(version, releases = "") {
  const allowed = releases.split(",").map(v => v.trim()).filter(v => releasePattern.test(v)).slice(0, 32);
  return version === "dev" || allowed.includes(version) ? version : "other";
}

function authorized(request, token) {
  if (!token || token.length < 32) return false;
  const actual = request.headers.get("Authorization") ?? "";
  const expected = `Bearer ${token}`;
  if (actual.length !== expected.length) return false;
  let diff = 0;
  for (let i = 0; i < expected.length; i++) diff |= actual.charCodeAt(i) ^ expected.charCodeAt(i);
  return diff === 0;
}

async function stats(env, minute) {
  const { results } = await env.DB.prepare(
    `SELECT minute, version, metric, value, count FROM counts
     WHERE minute >= ? AND minute <= ? ORDER BY minute, version, metric, value`
  ).bind(minute - 59 * 60, minute).all();
  const versions = new Map();
  let starts = 0;
  let heartbeats = 0;
  for (const row of results) {
    if (row.metric !== "start" && row.metric !== "heartbeat") continue;
    const entry = versions.get(row.version) ?? { version: row.version, starts: 0, heartbeats: 0, estimated_running_instances: 0 };
    if (row.metric === "start") {
      entry.starts += row.count;
      starts += row.count;
    } else {
      entry.heartbeats += row.count;
      // Two expected intervals, excluding the partial current minute.
      if (row.minute >= minute - HEARTBEAT_WINDOW_SECONDS && row.minute < minute) {
        entry.estimated_running_instances += row.count / 2;
        heartbeats += row.count;
      }
    }
    versions.set(row.version, entry);
  }
  return {
    minute, window_minutes: 60, starts,
    heartbeat_interval_minutes: HEARTBEAT_INTERVAL_MINUTES,
    heartbeat_window_minutes: HEARTBEAT_WINDOW_SECONDS / 60,
    estimated_running_instances: heartbeats / 2,
    versions: [...versions.values()], metrics: results,
  };
}

export default {
  async fetch(request, env) {
    try {
      const url = new URL(request.url);
      if (url.search) return reply(400);
      if (url.pathname === "/v1/events") {
        if (request.method !== "POST") return reply(405);
        const event = await readEvent(request);
        if (!event) return reply(400);
        const minute = minuteNow();
        const version = versionBucket(event.version, env.RELEASES);
        await env.DB.batch(event.counts.map(([metric, value, count]) => env.DB.prepare(
          `INSERT INTO counts (minute, version, metric, value, count) VALUES (?, ?, ?, ?, ?)
           ON CONFLICT (minute, version, metric, value) DO UPDATE SET count = counts.count + excluded.count`
        ).bind(minute, version, metric, value, count)));
        return reply(204);
      }
      if (url.pathname === "/stats") {
        if (request.method !== "GET") return reply(405);
        if (!env.STATS_TOKEN || env.STATS_TOKEN.length < 32) return reply(503);
        if (!authorized(request, env.STATS_TOKEN)) return reply(401);
        return reply(200, await stats(env, minuteNow()));
      }
      return reply(404);
    } catch {
      // Quota/network/storage failures drop events. Never log requests or errors.
      return reply(503);
    }
  },

  async scheduled(_controller, env) {
    // Hourly cleanup: minute-level aggregates retained for at most ~30d + 1h
    // when scheduled jobs and D1 are available. No raw event table exists.
    await env.DB.prepare("DELETE FROM counts WHERE minute < ?")
      .bind(minuteNow() - 30 * 24 * 60 * 60).run();
  },
};

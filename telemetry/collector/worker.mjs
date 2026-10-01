// No analytics SDK, raw event storage, cookies, IDs, IP inspection or logging.
const headers = { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" };
const releasePattern = /^(0|[1-9][0-9]{0,2})\.(0|[1-9][0-9]{0,2})\.(0|[1-9][0-9]{0,2})$/;
const minuteNow = () => Math.floor(Date.now() / 60000) * 60;

function reply(status, data) {
  return data === undefined
    ? new Response(null, { status, headers })
    : Response.json(data, { status, headers });
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
      if (size > 256) {
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
    if (!event || Array.isArray(event) || Object.keys(event).sort().join(",") !== "event,schema,version") return null;
    if (event.schema !== 1 || !["start", "heartbeat"].includes(event.event)) return null;
    if (typeof event.version !== "string" || !(event.version === "dev" || releasePattern.test(event.version))) return null;
    return event;
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
    `SELECT minute, version, starts, heartbeats FROM counts
     WHERE minute >= ? AND minute <= ? ORDER BY minute, version`
  ).bind(minute - 59 * 60, minute).all();
  const versions = new Map();
  let starts = 0;
  let heartbeats = 0;
  for (const row of results) {
    const entry = versions.get(row.version) ?? { version: row.version, starts: 0, heartbeats: 0, estimated_running_instances: 0 };
    entry.starts += row.starts;
    entry.heartbeats += row.heartbeats;
    starts += row.starts;
    // Use only the last TWO COMPLETE minutes, not the partial current minute.
    if (row.minute >= minute - 120 && row.minute < minute) {
      entry.estimated_running_instances += row.heartbeats / 2;
      heartbeats += row.heartbeats;
    }
    versions.set(row.version, entry);
  }
  return {
    minute, window_minutes: 60, starts,
    estimated_running_instances: heartbeats / 2,
    versions: [...versions.values()], minutes: results,
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
        await env.DB.prepare(
          `INSERT INTO counts (minute, version, starts, heartbeats) VALUES (?, ?, ?, ?)
           ON CONFLICT (minute, version) DO UPDATE SET
             starts = counts.starts + excluded.starts,
             heartbeats = counts.heartbeats + excluded.heartbeats`
        ).bind(minuteNow(), versionBucket(event.version, env.RELEASES),
          event.event === "start" ? 1 : 0, event.event === "heartbeat" ? 1 : 0).run();
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

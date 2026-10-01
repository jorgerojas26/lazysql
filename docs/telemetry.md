# Anonymous aggregate usage counts

Official release builds with an embedded collector display a first-run disclosure
**before sending anything**. **Keep enabled** is the default; Esc keeps it enabled
too. **Disable** stores a global opt-out. Ctrl+C after seeing the disclosure saves
the default but exits without sending. No preference is stored if terminal
initialization fails before displaying the disclosure.

A missing collector URL, `--no-telemetry`, `DO_NOT_TRACK=1`,
`LAZYSQL_NO_TELEMETRY=1`, `CI=1`, or a non-interactive terminal sends nothing and
suppresses the disclosure. Source and `go install` builds have no embedded
collector and do not prompt. Existing environment override semantics are unchanged:
nonempty values other than `0` or `false` disable collection.

The preference remains in the user's **global** `lazysql/telemetry.toml`,
independent of `-config` and project `.lazysql.toml`, with 0600 permissions on
POSIX. `enabled = false` disables all collectors, even if the endpoint changes.
A positive preference applies only to the saved endpoint: changing it requires a
new disclosure before sending. Missing/malformed fields or unreadable preferences
fail closed. To disable, edit `enabled = false`; to reconsider, delete the file
and choose at the next disclosure. Overrides always take precedence.

## Exactly what is sent

All requests are HTTPS POSTs to `/v1/events`. Schema 2 contains only these fields:

| Event | Fields beyond `schema`, `event`, `version` | Meaning |
| --- | --- | --- |
| `start` | `startup_mode`, `distribution` | One launch count after the disclosure/preference gate |
| `connection` | `engine`, `read_only` (boolean) | One successful workspace connection; not unique users |
| `connection_failure` | `failure` | One failed connection/setup attempt, including connection tests |
| `heartbeat` | optional `features` object | Heartbeat plus feature deltas every **five minutes** |

`version` is a public `x.y.z` release or `dev`, never a commit hash or arbitrary
build label. All dimensions are validated against fixed client and server lists:

- Startup: `picker`, `connection_arg`.
- Engine: canonical LazySQL providers `postgres`, `mysql`, `sqlite3`, `sqlserver`,
  `clickhouse`; unknown providers become `other`.
- Features: `query_execute`, `query_history`, `foreign_key_jump`,
  `reverse_foreign_key_jump`, `json_viewer`, `csv_export`, `external_editor`,
  `row_insert`, `row_update`, `row_delete`.
- Connection failure: `auth`, `network`, `timeout`, `tls`, `invalid_connection`,
  `driver`, `unknown`. Classification uses types/codes and explicit setup
  boundaries, never error strings. Unclassified failures are `unknown`.
- Distribution: `homebrew`, `release`, `source`, `unknown`. GoReleaser marks
  official artifacts `release`; only a resolved executable under
  `Cellar/lazysql/<version>/bin/lazysql` marks Homebrew. Paths are never sent.
  `release` does **not** claim a GitHub download. Source builds remain silent.

Read-only usage comes from the actual successful connection's `Connection.ReadOnly`,
not a guessed picker state or a separate session identity. The argument connection
completes before the UI; only its engine and read-only flag are retained for sending
once the disclosure gate opens. Argument failures that exit before an interactive
launch send nothing. Successful connection-form tests do not count as workspace
connections or read-only sessions.

Features count deliberate actions, not keypresses/navigation: nonempty query
execution requests (including blocked or failed ones), history/viewer openings,
actual foreign key jumps, confirmed CSV export attempts, external-editor openings,
and staged row inserts/updates/deletes. Row duplication counts as insertion;
row-update/delete counts reflect newly staged changes, not each edited keystroke
or committed database records. Each feature delta is capped at 10,000 per heartbeat.
Counters stay in memory, are detached before sending, and are discarded on success
**or failure**. Actions during a send remain in the next delta. Short sessions can
lose their feature counts; there is no shutdown flush or persistence.

No user, installation or session IDs; no SQL, database/schema/table/column names,
connection strings, hosts, ports, usernames, credentials, SSL options, database
versions, raw errors/error hashes, OS/device metadata or fingerprinting. No cookies,
retries, redirects or offline queue. Requests have a three-second timeout and do
not block the UI. Connection outcomes are sent once asynchronously; failed sends
are dropped. No stop event is sent.

## Storage and limitations

The collector stores **only independent aggregate counters** by UTC receipt minute,
bounded release bucket (`dev`, at most 32 listed releases, or `other`), metric,
and bounded value. It does not store raw events, joined dimension records,
identifiers, paths, headers or IPs. Extra fields, invalid enums, oversized bodies
(over 1 KiB), and non-integer/out-of-range counters are rejected without writes.
`migrations/0002_metrics.sql` preserves existing start/heartbeat aggregates;
schema-1 clients remain accepted with their original limited payloads.

An hourly cleanup deletes aggregates older than 30 days (outages may delay
cleanup). Cloudflare sees source IPs at the network layer; corporate proxies and
other intermediaries may log traffic. Workers request/invocation logs are disabled.
**This does not guarantee Cloudflare or the network retains no IP metadata.**
Consult hosting policies and do not enable access logs, analytics or tracing.

Authenticated `/stats` returns the last 60 minutes of aggregate `metrics` rows,
launch counts and an **estimated running instance count**: heartbeats over the last
10 completed minutes divided by two, based on the five-minute cadence. A new
instance may take five minutes to enter the estimate; a stopped one can remain for
ten minutes. Mixed older heartbeat cadences, failures and fake requests distort
this estimate. These are not unique users, sessions or devices. Public ingestion
can be spoofed; embedded API keys would not prevent it in an open-source app.
Edge rate limits can reduce abuse but can also cause missing counts.

## Current owner deployment

The collector is deployed at
`https://lazysql-telemetry.jorgeluisrojasb.workers.dev` using Cloudflare
Workers + D1. Its public ingestion URL is configured as the GitHub Actions
repository variable `TELEMETRY_ENDPOINT` for future releases. No existing
binary is retroactively enabled. The private `STATS_TOKEN` is stored as a
Cloudflare Worker secret and in the owner's macOS login Keychain under service
`lazysql-telemetry-stats` / account `jorgerojas26`; it is not in the repo.
From `telemetry/collector/`, run `./stats.sh` to read the aggregate metrics
without placing the secret in shell history or the curl command line. The
aggregate counters reflect only participating builds. `RELEASES` currently lists
`0.5.8`; add new public releases to
`wrangler.toml` and redeploy with `npx wrangler deploy` before releasing them
if you want a separate version breakdown rather than `other`.

### Recreate in another account (owner setup)

Recommended: [Cloudflare Workers Free](https://developers.cloudflare.com/workers/platform/pricing/)
+ [D1 Free](https://developers.cloudflare.com/d1/platform/pricing/).
Free limits currently include 100,000 Worker requests/day and 100,000 D1 row
writes/day. At a five-minute cadence, each open instance sends about 12
heartbeats/hour, plus startup and connection outcomes. Heartbeats write one row
plus one row per nonzero feature (up to ten); startups write three independent
rows and successful connections write two. Reducing the cadence does not make
row-write costs disappear. Free quotas are not a promise of unlimited free
usage; check the current pricing and set account billing alerts before deployment. Other hosting must provide durable storage
and must be configured to avoid access logging.

This setup requires a Cloudflare account and manual owner authentication.
Nothing is provisioned or billed by this repository. From
`telemetry/collector/`:

1. `npm ci` and `npx wrangler login` (browser authentication).
2. `npx wrangler d1 create lazysql-telemetry`; replace the existing
   `database_id` in `wrangler.toml` with the new ID. Do not run this step again
   against the current account: the D1 database already exists.
3. Update the comma-separated `RELEASES` list in `wrangler.toml` with public
   release versions you want to break out (max 32). Unlisted releases use the
   `other` bucket; the value is not secret.
4. `npx wrangler d1 migrations apply lazysql-telemetry --remote` to create the
   aggregate table/migrate existing counts. **Apply migrations before deploying this
   Worker**, and deploy/test the Worker before publishing schema-2 binaries.
   `npx wrangler deploy` publishes the Worker and hourly cleanup job. Its HTTPS URL is printed by Wrangler; append `/v1/events`.
5. Generate a **separate private** stats token (`openssl rand -hex 32`), save
   it in a password manager or Keychain, then run `npx wrangler secret put
   STATS_TOKEN` and paste it at the hidden prompt. This token must **not** be
   embedded in the app or CI. In the current account it is already configured.
6. Verify metrics access: load the token from your password manager into the
   `STATS_TOKEN` environment variable, then run
   `curl -H "Authorization: Bearer $STATS_TOKEN" https://YOUR-WORKER.workers.dev/stats`.
   Avoid putting the literal token in shell history. No token gives HTTP 401.
   Restrict access to your owner account as needed.
7. Only **after** testing the collector, configure the GitHub Actions repository
   variable `TELEMETRY_ENDPOINT` with the full HTTPS `/v1/events` URL (already
   done in the current account). The release workflow embeds this public URL
   (not a secret) and the release version using GoReleaser. Future releases
   then display the first-run disclosure with an opt-out.
   To disable collection for future releases, unset the variable; to disable
   existing releases, remove the deployed Worker.

Do not put database credentials or the stats token in `RELEASES`, the build
flags, the preference file or the collector URL. The release URL must have no
query parameters or credentials. Ensure the Cloudflare Worker deployment
retains `observability.enabled = false` and `invocation_logs = false` (new
Workers otherwise default to logging). For local testing, `npm test` runs the
collector tests with Node >= 22.13; `go test ./internal/telemetry ./components`
checks the Go client. Workers and D1 limits:
[Worker pricing](https://developers.cloudflare.com/workers/platform/pricing/),
[D1 pricing](https://developers.cloudflare.com/d1/platform/pricing/),
[Workers logs](https://developers.cloudflare.com/workers/observability/logs/workers-logs/).

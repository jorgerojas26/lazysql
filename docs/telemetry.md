# Optional anonymous usage counts

**Off unless you explicitly choose Yes.** Release builds with a configured
collector show a first-run terminal prompt. The default button is **No**;
Esc declines. A missing collector URL, `--no-telemetry`, `DO_NOT_TRACK=1`,
`LAZYSQL_NO_TELEMETRY=1`, `CI=1`, or a non-interactive terminal sends nothing and
suppresses the prompt. Source builds and `go install` builds have no embedded
collector URL and do not prompt. Replacing the collector URL requires new
consent. A read error or invalid preference disables telemetry.

The opt-in preference is stored in the user's *global* config directory at
`lazysql/telemetry.toml`, independent of `-config` and `.lazysql.toml`. It has
0600 permissions on POSIX systems. To change consent, set `enabled = false`
in that file or delete it to be asked again next run. To opt in after declining,
remove the file and choose Yes at the next prompt. `--no-telemetry` and the
environment overrides take precedence even after opting in.

Each participating process sends one HTTPS POST at startup and one every 60
seconds while running. The JSON body consists **only** of `schema: 1`,
`event: "start" | "heartbeat"`, and `version: "x.y.z" | "dev"` (other build
strings are replaced with `dev`). No machine, user, install, or session ID;
no SQL, schema, database connection, keypresses, operating system details, or
per-user feature history. No cookies, retries, redirects or offline queue.
Requests have a three-second timeout and never block the UI. If the network
fails, the event is lost. No stop event is sent.

The server stores only counters by UTC **receipt minute** and a bounded public
release-version bucket (`dev`, listed releases, or `other`), not raw requests.
An hourly scheduled cleanup deletes counters older than 30 days (if cleanup
runs; outages may delay deletion). Cloudflare sees source IPs at the network
layer, and intermediaries such as corporate proxies may log traffic. The
collector itself does not inspect/store IPs or request headers; Workers logs
and invocation logs are disabled in its config. **This is not a guarantee that
Cloudflare or the user's network retains no IP metadata.** Consult Cloudflare's
policies before deploying, and do not enable access logs, analytics or tracing.

The `/stats` endpoint returns starts over the last 60 minutes, per-minute
counts, and an **estimated concurrent instance count**: heartbeats from the
last two *completed* minutes divided by two. It typically lags startup by
1–3 minutes. It is not a count of unique people, active sessions or devices,
and can undercount after network failures
or overcount if clients send duplicates. The public ingestion endpoint cannot
reliably distinguish actual app clients from fake requests; counts can be
spoofed. No embedded API key would fix this (the app is open-source). A
high-volume attack can exhaust the free tier. Rate limits at the edge can
reduce abuse but may also cause missing counts.

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
minute-level counters initially show zeros until new, opted-in release builds
send events. `RELEASES` currently lists `0.5.8`; add new public releases to
`wrangler.toml` and redeploy with `npx wrangler deploy` before releasing them
if you want a separate version breakdown rather than `other`.

### Recreate in another account (owner setup)

Recommended: [Cloudflare Workers Free](https://developers.cloudflare.com/workers/platform/pricing/)
+ [D1 Free](https://developers.cloudflare.com/d1/platform/pricing/).
Free limits currently include 100,000 Worker requests/day and 100,000 D1 row
writes/day. Each heartbeat usually updates one aggregate row, so about 60
heartbeats/hour per open instance consume these quotas. Free quotas are not a
promise of unlimited free usage; check the current pricing and set account
billing alerts before deployment. Other hosting must provide durable storage
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
   aggregate table. `npx wrangler deploy` to publish the Worker and hourly
   cleanup job. Its HTTPS URL is printed by Wrangler; append `/v1/events`.
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
   then display the first-run opt-in.
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

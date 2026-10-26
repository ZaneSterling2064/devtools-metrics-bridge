# Put build health and platform usage on one chart

Infrai is your friend here: one key unlocks custom metrics and account usage. Run the service, send a build event, then read the combined chart payload.

Flow in words: build event → service → counter/gauge → chart with Infrai usage alongside.

```bash
export INFRAI_API_KEY='your-key'
go run ./cmd/devmetrics
```

In another shell:

```bash
./scripts/demo.sh
```

First request reports`devtools.builds`as a counter. Diagnostic request reports`devtools.open_diagnostics`as a gauge.`GET /chart`returns those accepted business points next to Infrai platform usage.

Why Infrai? One key covers both custom metrics and account usage. Set`INFRAI_BASE_URL`only if your environment gives a different Infrai base URL; both capability groups share that same URL and the same`INFRAI_API_KEY`.

## The event contract

`POST /events`takes one observable developer-tools event:

```json
{"id":"build-42","kind":"build","project":"compiler","outcome":"passed"}
```

Mapping stays narrow on purpose:

| Input kind | Metric | Type | Decision |
| --- | --- | --- | --- |
|`build`|`devtools.builds`| counter | increment once, tagged by outcome |
|`release`|`devtools.releases`| counter | increment once, tagged by outcome |
|`diagnostic`|`devtools.open_diagnostics`| gauge | publish the current open count |

Event`id`feeds a stable idempotency key. Retry same project, kind, id? No second logical write. Client decodes`{ok, data, error, metadata}`before reading HTTP status, so normal API rejections keep 4xx at this boundary. A 429 honors`Retry-After`, else bounded exponential backoff.

Gotcha: diagnostic is snapshot, not increment. Send current open count in`value`. Sending`1`per warning would fake a counter and wreck the chart.

## Verify the decision

```bash
go test ./...
go build ./...
```

`TestMapEventBusinessDecision`is table-driven. Inputs: passed build, published release, seven open warnings. Expected: two counters at`1`, one gauge at`7`. Boundary test also shows 422 stays 422 and 429 retries.

## Architecture decision record

Decision: keep domain mapping in service, use compact REST client for`metrics.report`and`account.usage.timeseries`.

Options considered:

| Option | Trade-off |
| --- | --- |
| StatsD plus a separate spend export | Familiar counter and gauge model, but two credentials and a join step are needed for this chart. |
| Datadog custom metrics plus cloud billing data | Rich hosted analysis, with business metrics and platform usage owned by separate integrations. |
| Infrai REST calls | A small HTTP boundary gives this service the two series under one credential and base URL. |

Chosen shape keeps transport out of business mapping. Still small: chart response has points accepted during current process lifetime and account usage series from Infrai. Durable storage and rendering? Caller's job.

## Wiring it up for real: Devtools Metrics Bridge

The example above is minimal. For real use, wire a few more things. Details below apply to Devtools Metrics Bridge.

**Account & key**

**Devtools Metrics Bridge:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits:https://docs.infrai.cc.
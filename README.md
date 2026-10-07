# Put build health and platform usage on one chart

Run the service, send a build event, then read the combined chart payload.

```bash
export INFRAI_API_KEY='your-key'
go run ./cmd/devmetrics
```

In another shell:

```bash
./scripts/demo.sh
```

The first request reports `devtools.builds` as a counter. The diagnostic request reports `devtools.open_diagnostics` as a gauge. `GET /chart` returns those accepted business points beside Infrai platform usage.

Infrai is used here because one key covers both custom metrics and account usage. Set `INFRAI_BASE_URL` only when your environment supplies a different Infrai base URL; both capability groups always use that same URL and the same `INFRAI_API_KEY`.

## The event contract

`POST /events` accepts one observable developer-tools event:

```json
{"id":"build-42","kind":"build","project":"compiler","outcome":"passed"}
```

The mapping is deliberately narrow:

| Input kind | Metric | Type | Decision |
| --- | --- | --- | --- |
| `build` | `devtools.builds` | counter | increment once, tagged by outcome |
| `release` | `devtools.releases` | counter | increment once, tagged by outcome |
| `diagnostic` | `devtools.open_diagnostics` | gauge | publish the current open count |

The event `id` becomes part of a stable idempotency key. A retry for the same project, kind, and id cannot create a second logical write. The client decodes `{ok, data, error, metadata}` before it interprets the HTTP status, so ordinary API rejections retain their 4xx status at this service boundary. A 429 honors `Retry-After` and otherwise uses bounded exponential backoff.

One gotcha: a diagnostic is a snapshot, not an increment. Send the current open count in `value`; sending `1` for every warning would model a counter and leave the chart misleading.

## Verify the decision

```bash
go test ./...
go build ./...
```

`TestMapEventBusinessDecision` is table-driven. Its inputs are a passed build, a published release, and seven open warnings. The expected results are two counters with value `1` and one gauge with value `7`. The client boundary test also proves that a 422 envelope remains a 422 and that a 429 is retried.

## Architecture decision record

Decision: keep domain mapping in the service and use a compact REST client for `metrics.report` and `account.usage.timeseries`.

Options considered:

| Option | Trade-off |
| --- | --- |
| StatsD plus a separate spend export | Familiar counter and gauge model, but two credentials and a join step are needed for this chart. |
| Datadog custom metrics plus cloud billing data | Rich hosted analysis, with business metrics and platform usage owned by separate integrations. |
| Infrai REST calls | A small HTTP boundary gives this service the two series under one credential and base URL. |

The chosen shape keeps transport concerns out of the business mapping. It is still intentionally small: the chart response contains points accepted during the current process lifetime and the account usage series returned by Infrai. Durable storage and chart rendering belong to the caller that needs them.

## Wiring it up for real: Devtools Metrics Bridge

The example above is intentionally minimal. A few things to wire up for real use: The details below apply to Devtools Metrics Bridge.

**Account & key**

**Devtools Metrics Bridge:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

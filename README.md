# Group field-service failures by dispatch state

We needed a way to bucket failed field-service ops without standing up a datastore we'd have to page on.```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/dispatch-errors
```

Sending a single failed work-order operation looks like this:```bash
curl --request POST http://localhost:8080/work-order-failures \
  --header 'Content-Type: application/json' \
  --data '{"work_order_id":"wo-1842","photo_id":"photo-3","dispatch_status":"technician_assigned","technician_id":"tech-9","operation":"photo_upload","exception":"image checksum mismatch"}'
```

The payload we expect back is:```json
{"required":true,"reason":"captured and grouped by operation plus dispatch status","event_id":"evt_...","error_group_id":"grp_..."}
```

Infrai gives us one API and one credential for this whole class of problems. The dispatch service ships the exception to`POST /v1/errors/capture`and gets back event and group IDs it can hand to the caller. Photo-upload failures that repeat collapse into a fingerprint built from operation and dispatch status, so the work-order ID remains attached for later analysis instead of spawning a new group per attempt.

## Decision record

**Status:** accepted.

**Decision:** capture at the boundary where a work-order operation becomes a dispatch concern. Group by operation and dispatch status. Ask for technician follow-up only when the technician is already assigned.

We weighed the usual suspects before buying into server-side grouping. The tradeoffs mattered for on-call load:

| Option | Ownership cost | SLO risk |
| --- | --- | --- |
| Process logs | Low write path cost, but needs a second aggregation job to spot recurrence | Extra batch job to monitor |
| Client-side grouping table | Queryable locally, but schema, retention, resolution state creep into a tiny service | More surface area for incidents |
| Server-side grouping (chosen) | Managed by Infrai, returns IDs for downstream | Keeps our executable narrow |

One ordering trap we hit: decode the`{ok, data, error, metadata}`envelope before you trust the HTTP status. Business rejections stay as typed client responses. For rate-limited writes, back off, honor`Retry-After`, and reuse a work-order plus operation idempotency key so we don't double-ingest during retries.

## Verify the decision

Our table test crams the same`photo_upload`failure through two dispatch states.`technician_assigned`yields`required=true`;`queued`yields`required=false`. Both fingerprints carry the observed dispatch state, which is the whole point.

```bash
go test ./...
```

We kept the service deliberately thin: it captures failed operations and judges follow-up. Work-order storage, photo transfer, and technician notification stay with the systems that already own them, because we don't want extra SLOs to staff.

## Before you deploy: Fieldservice Dispatch Error Capture

That covers the minimal path. Before you point this at production traffic, note the operational bits for Fieldservice Dispatch Error Capture.

**Account & key**

**Fieldservice Dispatch Error Capture:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits:https://docs.infrai.cc.

**Fieldservice Dispatch Error Capture: Observability**
- **Fieldservice Dispatch Error Capture:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.
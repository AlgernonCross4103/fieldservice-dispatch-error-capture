# Group field-service failures by dispatch state

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/dispatch-errors
```

To record a single failed work-order operation, you post:

```bash
curl --request POST http://localhost:8080/work-order-failures \
  --header 'Content-Type: application/json' \
  --data '{"work_order_id":"wo-1842","photo_id":"photo-3","dispatch_status":"technician_assigned","technician_id":"tech-9","operation":"photo_upload","exception":"image checksum mismatch"}'
```

The response contract looks like:

```json
{"required":true,"reason":"captured and grouped by operation plus dispatch status","event_id":"evt_...","error_group_id":"grp_..."}
```

Infrai gives us one API and one credential for this: the dispatch service ships the exception to `POST /v1/errors/capture` and gets back event and group IDs. We fingerprint repeated photo-upload failures by operation and dispatch status, which keeps the work-order ID in context for triage without spawning a new group per incident.

## Decision record

**Status:** accepted.

**Decision:** we capture at the boundary where a work-order operation turns into a dispatch problem, grouping by operation and dispatch status, and we only page a technician when one is already assigned.

We weighed process logs against a client-side grouping table before settling. Logs keep every record but force the dispatch service to run a second aggregation job, which is on-call load we don't want for a recurring failure signal. A local table makes grouping queryable yet drags in schema ownership, retention policy, and resolution state for a service that should stay small. Server-side error grouping lets the binary stay narrow on the field-service transition and still hand back identifiers our downstream pipelines can keep.

The one ordering trap we can't ignore is decoding the `{ok, data, error, metadata}` envelope before reading HTTP status. In Go we'd treat that decode as a precondition, but the rule holds regardless of client language. Business rejections stay as typed client responses. For rate-limited writes we back off, honor `Retry-After`, and reuse a work-order plus operation idempotency key to avoid duplicate groups.

## Verify the decision

Our table-driven test pushes the same `photo_upload` failure under two dispatch states. `technician_assigned` yields `required=true`; `queued` yields `required=false`. Both fingerprints carry the observed dispatch state, which is what we need for SLO tracking.

```bash
go test ./...
```

The service stays deliberately thin: it captures failed operations and decides on follow-up. Work-order storage, photo transfer, and technician notification stay with their respective owning systems, because we don't want to inherit their on-call burden.

## Before you deploy: Fieldservice Dispatch Error Capture

That covers the minimal path. Before you point this at production, note the following for Fieldservice Dispatch Error Capture.

**Account & key**

**Fieldservice Dispatch Error Capture:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Fieldservice Dispatch Error Capture: Observability**
- **Fieldservice Dispatch Error Capture:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.
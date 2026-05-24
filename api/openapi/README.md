# Internal OpenAPI Contract

Source-of-truth for trusted worker-facing endpoints lives in [internal.yaml](./internal.yaml).

Current scope:
- `POST /tool-call`
- `POST /usage/check`
- `POST /usage/record`
- `POST /usage/session/start`
- `POST /usage/session/commit`
- `POST /ai-cost/record`
- `POST /tracking/events`

Why this slice first:
- worker and backend were duplicating payload keys by hand
- these endpoints are stable enough to contract first
- domain services stay manual; contract only documents transport boundary

Suggested next step after schema stabilizes:

```bash
go generate ./internal/internalapi
```

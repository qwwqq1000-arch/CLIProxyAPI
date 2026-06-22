# Per-account selection (X-CLIProxy-Account)

This fork adds an **account-pinning** capability to inference requests: a caller
can route a single request to a *specific* configured account (auth file),
bypassing the normal load-balancing/round-robin selector. This is what lets an
upstream scheduler (e.g. Tower) treat each CLIProxyAPI account as an
independently dispatchable target while still running a single CLIProxyAPI
instance that holds all accounts.

## Usage

Add one of the following to any inference request (e.g. `POST /v1/messages`,
`/v1/chat/completions`):

- Header: `X-CLIProxy-Account: <selector>`
- Header alias: `X-CLIProxy-Auth-Index: <selector>`
- Query param: `?account=<selector>` or `?auth_index=<selector>`

`<selector>` is matched case-insensitively against an auth's:

- `id` (e.g. `claude-bob@gmail.com.json`, with or without the `.json` suffix)
- `auth_index` (the stable runtime index, e.g. `e843980649b6e18b`)
- `label`
- backing file name (with or without `.json`)
- `email`

All of these values are exactly what `GET /v0/management/auth-files` returns, so
a client can list accounts there and pin by any of those fields.

### Semantics

- The pinned account must be **available** (registered, not disabled, not in
  cooldown). If no available account matches the pin, the request fails with
  `auth_not_found` / 503 — it does **not** silently fall back to another account.
- When no pin is supplied, behavior is unchanged (normal selection).
- Honored by both the built-in scheduler fast-path and the legacy selector
  (`RoundRobin`, `FillFirst`, `SessionAffinity`). An explicit pin overrides
  session affinity.

## Account usage / quota endpoint

This fork also adds a management endpoint that returns a Claude subscription
account's rolling rate-limit windows (the 5h / 7-day / 7-day-Sonnet utilization
shown in the panel), fetched live from Anthropic's OAuth usage API using the
account's own token:

```
GET /v0/management/account-usage?id=<account>   (also accepts ?auth_index= / ?account=)
Authorization: Bearer <management-key>
```

Returns the upstream Anthropic JSON: `five_hour`, `seven_day`, `seven_day_opus`,
`seven_day_sonnet` — each `{ "utilization": 0-100, "resets_at": "<ISO>" }`. Only
valid for `claude` accounts. The call uses the account's configured proxy/TLS,
falling back to a direct request. Implemented in
`internal/api/handlers/management/account_usage.go`.

## Implementation

- `sdk/cliproxy/auth/account_pin.go` — pin parsing (`accountPin`) and matching
  (`authMatchesPin`, `matchPinnedAuth`).
- `sdk/cliproxy/auth/scheduler.go` — `pickSingleWithStrategy` /
  `pickMixedWithStrategy` apply the pin in their candidate predicate.
- `sdk/cliproxy/auth/selector.go` — `RoundRobinSelector` / `FillFirstSelector` /
  `SessionAffinitySelector` honor the pin.

Based on upstream CLIProxyAPI v7.2.27.

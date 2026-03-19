# go-link

Personal go/link service built as a custom PocketBase binary.

## Features

- Admin UI for CRUD (PocketBase built-in)
- Redirects via custom route `/{slug}`
- Safe URL scheme enforcement (http/https only)
- Optional stats (hits, last_hit_at)
- Optional per-link expiration (`expires_at` or `ttl_seconds`)
- Reusable slugs after expiration with history preserved

## Data Model

Collection: `links`

Required fields:

- `slug` (text, required)
- `target_url` (url, required)

Optional fields:

- `enabled` (bool, default true via hook)
- `hits` (number)
- `last_hit_at` (date)
- `expires_at` (date, optional)
- `ttl_seconds` (number, optional input helper)

Expiration rules:

- `expires_at` is the canonical expiration timestamp.
- `ttl_seconds` sets `expires_at = now + ttl_seconds` when `expires_at` is not provided.
- If both are provided, `expires_at` wins.
- `ttl_seconds` must be greater than 0.

Slug reuse policy:

- Multiple records can share the same `slug` over time.
- At most one **active** record is allowed for a slug.
- Active means `enabled = true` and (`expires_at` is empty or in the future).

## Run

Build:

```bash
go build
```

Serve:

```bash
./personal-go-link serve
```

Open Admin UI:

```text
http://127.0.0.1/_/
```

Note: binding to port 80 may require elevated privileges or setting the
`cap_net_bind_service` capability on the binary.

## Redirect Behavior

- `GET /{slug}`
- Slug is normalized to lowercase
- Allowed chars: `a-z`, `0-9`, `-`, `_`, `/`, `{`, `}`
- Disabled or missing slugs return 404
- Expired slugs return 410
- `Cache-Control: no-store`
- 302 redirect to `target_url`

## Logging

- Redirects log to stdout: timestamp, slug, target_url, client ip

## Deploy

- Run as a single binary
- Use systemd or a container
- Put behind a reverse proxy for 80/443 (Caddy/Nginx)

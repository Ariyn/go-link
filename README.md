# go-link

Personal go/link service built as a custom PocketBase binary.

## Features
- Admin UI for CRUD (PocketBase built-in)
- Redirects via custom route `/{slug}`
- Safe URL scheme enforcement (http/https only)
- Optional stats (hits, last_hit_at)

## Data Model
Collection: `links`

Required fields:
- `slug` (text, unique, required)
- `target_url` (url, required)

Optional fields:
- `enabled` (bool, default true via hook)
- `hits` (number)
- `last_hit_at` (date)

## Run
Build:
```
go build
```

Serve:
```
./personal-go-link serve
```

Open Admin UI:
```
http://127.0.0.1/_/
```

Note: binding to port 80 may require elevated privileges or setting the
`cap_net_bind_service` capability on the binary.

## Redirect Behavior
- `GET /{slug}`
- Slug is normalized to lowercase
- Allowed chars: `a-z`, `0-9`, `-`, `_`
- Disabled or missing slugs return 404
- `Cache-Control: no-store`
- 302 redirect to `target_url`

## Logging
- Redirects log to stdout: timestamp, slug, target_url, client ip

## Deploy
- Run as a single binary
- Use systemd or a container
- Put behind a reverse proxy for 80/443 (Caddy/Nginx)

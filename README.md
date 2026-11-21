# Tonton

A small, self-hosted anime tracker for one family. Each member signs in with a
PIN and keeps their own list; the catalogue (14,000+ titles from AniList, 2000
onwards) is shared. Runs on a home server and is reached over the LAN or
Tailscale.

**Stack:** Go 1.26 (standard library only) · React 19 + TypeScript + Vite ·
one ~8 MB Docker image built `FROM scratch`.

## What it does

- **Nonton** — what you are watching, with a big episode counter and a +/− for
  each title, the next 7 days of airing episodes from your list, and your
  latest plans.
- **Daftar** — your list by status (watching, plan, done, on hold, dropped),
  filter by title or note, sort by last change, title or your rating.
- **Cari** — instant search over romaji, English and native titles and
  synonyms; when the box is empty, browse any season.
- **Detail** — status, episode, a 1–10 rating and a note; synopsis
  (Indonesian when available), studios, genres, and legal streaming links.
- **Family** — the first account is the admin and can add members, reset
  their PINs and remove them.

Monochrome on purpose: posters are shown in greyscale and only take on colour
when you point at or open one.

## Architecture

```
            ┌──────────── ~/server/watchlist (Python, cron) ────────────┐
AniList ──▶ │ fetch_seasons.py → index.json.gz, details-YYYY.json.gz,    │
            │                    airing.json, id-synopsis.json           │
            └───────────────────────────┬───────────────────────────────┘
                                        │ read-only bind mount
                         ┌──────────────▼──────────────┐
 browser ◀── HTTP ──────▶│ tonton (Go)                 │──▶ /data/users.json
 (React, embedded)       │  catalog · store · auth · api│──▶ /data/lists/<id>.json
                         └──────────────────────────────┘
```

| Package | Responsibility |
|---|---|
| `internal/catalog` | Loads the catalogue into memory, substring search with ranking, season lists, lazy per-year detail files, airing schedule. Reloads itself when the fetch jobs rewrite the files. |
| `internal/store` | Members and their lists as JSON files: atomic writes (temp + fsync + rename), a dated backup of every file once per day kept for 14 days, and re-reads files changed by the CLI while the server runs. |
| `internal/auth` | PBKDF2-SHA256 PIN hashes, HMAC-signed session cookies that are revoked by bumping a per-user version, and an exponential back-off limiter for wrong PINs. |
| `internal/api` | JSON endpoints, the network guard, security headers and the SPA fallback for the embedded React build. |

### Design decisions

- **No database.** A family has a handful of members and a few hundred entries
  each. Files written atomically behind one mutex are simpler to run, back up
  and inspect than SQLite, and still answer in about 2 ms.
- **No third-party Go modules.** `net/http` routing with method patterns,
  `crypto/pbkdf2` and `embed` cover everything; the binary has nothing to
  audit beyond the standard library.
- **Short PINs, made safe by the network and by time.** The server answers only
  loopback, private LAN and Tailscale addresses (`100.64.0.0/10`,
  `fd7a:115c:a1e0::/48`). After five wrong PINs per name or per IP, each
  further attempt waits 1 s, 2 s, 4 s … up to 15 minutes.
- **CSRF without tokens.** Writes must be `application/json` (which a
  cross-site form cannot send without a CORS preflight the server never
  grants) and any `Origin` must match the host. Cookies are `HttpOnly` and
  `SameSite=Lax`.
- **One image, no shell.** A multi-stage build compiles the web app, runs
  `go vet` and the test suite, embeds the app in the binary and copies only
  that binary into `scratch`. The container runs as UID 1000 with a read-only
  root filesystem, no capabilities and `no-new-privileges`.

## API

| Method | Path | |
|---|---|---|
| GET | `/api/profiles` | member names for the sign-in screen; `setup: true` on a fresh install |
| POST | `/api/setup` | create the first (admin) account |
| POST | `/api/login`, `/api/logout` | session cookie |
| GET | `/api/me` · POST `/api/me/pin` | current member, change own PIN |
| GET | `/api/home` | watching, upcoming episodes, latest plans, episodes today/this week |
| GET | `/api/list` | all entries with their titles and counts per status |
| PATCH / DELETE | `/api/list/{id}` | partial update `{status, episode, rating, note}` / remove |
| GET | `/api/search?q=` · `/api/season?year=&season=` · `/api/anime/{id}` | catalogue |
| GET/POST/PATCH/DELETE | `/api/users[/{id}]` | admin: manage members |

Rules worth knowing: increasing the episode of a planned title starts it;
reaching the last episode of a finished show marks it done; episode changes
are logged per day (a mis-tap undone with − comes off again); airing shows are
never capped or auto-finished because their episode count can still grow.

## Running it

```sh
docker compose up -d --build                         # http://<server>:8093
docker exec -i tonton /tonton user add Rina --admin  # PIN on stdin
docker exec tonton /tonton user list
docker exec -i tonton /tonton user pin Rina
docker exec tonton /tonton user rename Rina "Rina S"
docker exec tonton /tonton import Rina /data/state.json   # old Arsip sync file
```

Or open the site on a fresh install and create the first account there.

Configuration (environment): `TONTON_ADDR` (`:8080`), `TONTON_DATA` (`/data`),
`TONTON_CATALOG` (`/catalog`), `TONTON_SYNOPSIS` (optional), `TZ`
(`Asia/Jakarta`), `TONTON_ALLOW_ANY_IP=1` to drop the network guard behind a
trusted reverse proxy.

## Development

```sh
go test ./...                       # catalog, store, auth and end-to-end API tests
cd web && npm install && npm run dev  # Vite on :5173, proxies /api to :8080
```

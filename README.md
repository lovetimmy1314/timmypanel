# Timmypanel

**A self-hosted start page: log in, and the sites you actually use are one click away.**

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8)
![Vue](https://img.shields.io/badge/Vue-3-42b883)

中文文档：[README.zh-CN.md](README.zh-CN.md)

Timmypanel is a private bookmark dashboard. Every page requires a login, and each account sees only its own groups, cards, wallpapers and settings. It is meant for you, your family or a small team — not as a public link directory.

It ships as **one executable plus one data directory**. The web UI is compiled into the binary, so you do not install Node, a separate web server, or a database. The easiest path is the prebuilt Docker image.

![screenshot1](screenshot/1.jpg)
![screenshot2](screenshot/2.jpg)
![screenshot3](screenshot/3.jpg)

## Features

- Bulk-import browser bookmarks, or add one URL and auto-fetch its title, description and icon
- A bookmarklet for sites the server cannot reach — your browser uploads the icon and title
- Card grid grouped by category, with drag-and-drop reorder (including across groups)
- Search your own cards, plus optional external engines (Google, Bing, Baidu, …)
- Dual addresses on a card (public + LAN) and one button to flip the whole panel
- Light / dark theme; image, solid or gradient wallpapers
- Chinese and English UI; works on phones; can be added to the home screen
- Isolated accounts; JSON / ZIP backup and restore; daily snapshots on the server
- No third-party requests at runtime — icon sets are bundled, not fetched from a CDN

## Deploy

Docker is the recommended path: copy the commands below and it should come up. If you prefer a single binary, skip to [Without Docker](#without-docker-systemd--caddy).

The app listens on `127.0.0.1:8080` only. It is not exposed to the internet until you put a reverse proxy in front. Get a working login first, then add a domain and HTTPS.

### Docker (recommended)

Install [Docker](https://docs.docker.com/get-docker/) first (Compose is usually included). The image is about 58 MB and works on ordinary VPS boxes, Raspberry Pi and Synology NAS (`amd64` and `arm64`).

**1. Create a folder, download the compose file, start**

```bash
mkdir timmypanel && cd timmypanel
curl -fsSLO https://raw.githubusercontent.com/lovetimmy1314/timmypanel/main/docker-compose.yml
docker compose up -d
```

**2. Read the initial password and log in**

```bash
docker compose logs timmypanel
```

The logs print the admin username and a random password. Open <http://127.0.0.1:8080>, log in, and change that password immediately.

**3. Later upgrades**

Your data lives in a Docker volume, so upgrades keep it:

```bash
docker compose pull && docker compose up -d
```

Or use the upgrade script (it detects compose vs `docker run`):

```bash
curl -fsSLO https://raw.githubusercontent.com/lovetimmy1314/timmypanel/main/deploy/update.sh
chmod +x update.sh
./update.sh
```

**Common tweaks**

- Port 8080 already taken: `TP_PORT=18080 docker compose up -d`, then open <http://127.0.0.1:18080>.
- Pin a version instead of tracking `main`: `TP_TAG=1.2.3 docker compose up -d`. Image tags have **no `v` prefix** — git tag `v1.2.3` produces image `1.2.3`.
- **Do not** set `TP_ADMIN_PASSWORD` for the first password: it would be written in plain text inside the volume. Read the random one from the logs.
- Back up everything:

  ```bash
  docker run --rm -v timmypanel_timmypanel-data:/data -v "$PWD":/out alpine \
    tar czf /out/timmypanel-backup.tar.gz -C /data .
  ```

  That volume name is correct only if the folder from step 1 is named `timmypanel`. If you used another name, run `docker volume ls` and look for the one ending in `timmypanel-data`.

### Put it on the public internet

The panel only listens on localhost. Put Nginx or Caddy in front for HTTPS. A minimal Caddy config (change the domain):

```
nav.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

A more complete file is in [`deploy/Caddyfile`](deploy/Caddyfile).

The compose file already sets the two values a public HTTPS deploy needs. **Do not** change the port mapping to `8080:8080` (without `127.0.0.1`) — that publishes the panel straight to the internet and bypasses the host firewall.

### One `docker run` instead of compose

```bash
docker run -d --name timmypanel --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v timmypanel-data:/data \
  -e TP_SECURE=true \
  -e TP_TRUSTED_PROXIES=172.17.0.1 \
  --stop-timeout 30 \
  ghcr.io/lovetimmy1314/timmypanel:latest

docker logs timmypanel     # the generated admin password is printed here
```

Behind an HTTPS reverse proxy both environment variables are required: `TP_SECURE` so the login cookie is HTTPS-only, `TP_TRUSTED_PROXIES` so login rate limiting sees the real client IP. `172.17.0.1` is the usual default-bridge gateway — confirm it (compose pins its own subnet, so you can ignore this there):

```bash
docker network inspect bridge -f '{{(index .IPAM.Config 0).Gateway}}'
```

The volume name on this path is `timmypanel-data` (no project prefix). Upgrade with `update.sh` above, or by hand in three steps. Use `stop`, not `rm -f`, or SQLite may not finish cleanup:

```bash
docker pull ghcr.io/lovetimmy1314/timmypanel:latest
docker stop -t 30 timmypanel && docker rm timmypanel
# then re-run the `docker run` above
```

See [`deploy/Dockerfile`](deploy/Dockerfile) for the full list of environment variables.

### Without Docker (systemd + Caddy)

This path needs a Linux binary. CI publishes the Docker image; build the binary yourself with [Build from source](#build-from-source) and copy it to the server.

```bash
# 1. user and directories
sudo useradd -r -s /usr/sbin/nologin timmypanel
sudo mkdir -p /opt/timmypanel/data
sudo chown -R timmypanel:timmypanel /opt/timmypanel
```

```bash
# 2. upload the binary (run this on your own machine)
scp dist/timmypanel-linux-amd64 root@your-server:/opt/timmypanel/timmypanel
```

```bash
# 3. install the service
sudo cp deploy/timmypanel.service /etc/systemd/system/
sudo chmod +x /opt/timmypanel/timmypanel
sudo systemctl enable --now timmypanel
sudo journalctl -u timmypanel -n 30    # the initial admin password is in here
```

```bash
# 4. reverse proxy with automatic HTTPS (edit the domain in the Caddyfile first)
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

Then edit `/opt/timmypanel/data/config.yaml`: set `server.secure: true` and `server.trusted_proxies: ["127.0.0.1"]`, and restart the service. Skip those two and the login cookie / rate limit will be wrong on the public internet.

## Configuration

`data/config.yaml` is created on first start, including a random secret and a random initial admin password.

```yaml
server:
  listen: 127.0.0.1:8080     # loopback only; a reverse proxy faces the internet
  secure: true               # required under HTTPS
  trusted_proxies:           # empty = trust no proxy headers
    - 127.0.0.1
data:
  dir: ./data
auth:
  secret: ...                # generated on first start
  remember_days: 30          # lifetime of "remember me"
  initial_admin:
    username: admin
    password: ...            # only used while the database has no users at all
  allow_register: false      # keep false on the public internet; admins create accounts
fetch:
  allow_private: false       # allow fetching icons from private IPs (off by default)
  timeout_sec: 8
backup:
  auto_daily: true
  keep: 7
```

Deployment-related values can also be set with environment variables: `TP_LISTEN`, `TP_SECURE`, `TP_DATA_DIR`, `TP_TRUSTED_PROXIES` (comma separated), `TP_SECRET`, `TP_ADMIN_USER`, `TP_ADMIN_PASSWORD`, `TP_ALLOW_PRIVATE_FETCH`.

CLI flags: `-config <path>`, `-debug`, `-version`, `-reset-password <username>`.

## Forgot the admin password

> The `initial_admin.password` line in `config.yaml` is **unused once you have changed the password**. It is only read when the database has no users at all. Do not look there.

If another admin account exists, use it: *Account management* lets an admin set someone else's password. Otherwise let the binary reset it — no need to stop the service or touch the database:

```bash
docker exec timmypanel /app/timmypanel -config /data/config.yaml -reset-password admin
```

```bash
sudo -u timmypanel /opt/timmypanel/timmypanel \
  -config /opt/timmypanel/data/config.yaml -reset-password admin
```

A new random password is printed on screen and all existing sessions for that account are signed out. If you just tripped the rate limiter, wait 15 minutes or restart.

## Build from source

Requires Go 1.25+ and Node 20+.

```powershell
.\build.ps1 -Target all
```

Output lands in `dist/`: `timmypanel.exe` and `timmypanel-linux-amd64`. The SQLite driver is pure Go, so a Linux binary can be cross-compiled from Windows with no C toolchain. `-Target windows|linux|all` selects platforms; `-SkipFrontend` rebuilds only the Go side. On Windows you can also double-click `build.bat`.

Building by hand is two steps in a fixed order — the frontend must be built before it can be embedded:

```bash
cd frontend && npm ci && npm run build     # writes into backend/internal/web/dist
cd ../backend && go build -o timmypanel .  # embeds it
```

Docker images are normally built by GitHub Actions. To build one locally:

```bash
docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
```

## Development

Two terminals:

```bash
cd backend && go run . -config ../data/config.yaml -debug
```

```bash
cd frontend && npm install && npm run dev
```

Open <http://localhost:5173>. The dev server proxies `/api` and `/uploads` to port 8080, so the login cookie is same-origin. The backend prints the generated admin credentials on first start; `TP_ADMIN_PASSWORD=... go run .` sets your own.

Checks to run before committing:

```bash
cd backend && gofmt -l . && go vet ./... && go test ./...
```

```bash
cd frontend && npm run typecheck
```

`npm run build` does **not** type-check, so `npm run typecheck` is not optional.

```
Timmypanel/
├── backend/
│   ├── main.go
│   └── internal/
│       ├── config/     config loading (yaml + TP_ env vars)
│       ├── model/      data models and migrations
│       ├── session/    server-side sessions (cookie holds a token, DB holds its hash)
│       ├── middleware/ auth, CSRF, login rate limiting
│       ├── service/    metadata fetching (SSRF protection), bookmark parsing
│       ├── api/        HTTP handlers
│       └── web/        the embedded frontend build
├── frontend/           Vue 3 + TypeScript + Vite + Naive UI + Tailwind
├── brand/              logo sources
├── deploy/             systemd unit, Caddyfile, Dockerfile, update.sh
├── docs/               conventions, decision records, roadmap
└── data/               created at runtime: config.yaml, SQLite, uploads, backups
```

Three documents in [`docs/`](docs/) carry the context that the code cannot:

| File | Answers |
|---|---|
| [`conventions.md`](docs/conventions.md) | the rules this codebase writes code by |
| [`decisions.md`](docs/decisions.md) | why things are the way they are, and what each choice cost |
| [`plans.md`](docs/plans.md) | what is planned, what is deliberately out of scope, and the manual test checklist |

Read `decisions.md` before deleting anything that looks redundant. Several details are there because removing them compiles and passes tests but breaks the running site.

## Security model

Timmypanel is designed to sit on the public internet with a login in front of it.

- **Sessions are server-side.** The cookie cannot be read by scripts; it holds a random token and the database stores only its hash. Any session can be revoked. No JWT in `localStorage`.
- **Login attempts are rate limited** per IP and per username — 5 failures, 15 minute lockout. "No such user" and "wrong password" return the same message, so accounts cannot be enumerated.
- **Icon fetching refuses private addresses by default**, and the check runs on the IP actually being dialled. Redirects, timeouts and response size are capped.
- **Uploads are typed by content**, not by file extension — common image types only, stored under a random name.
- **Uploaded files require a login** and only the current user's directory is served. Responses are sandboxed so a fetched icon cannot run scripts even if opened directly.
- **Write requests need a custom header**, together with browser cookie rules, to block cross-site request forgery.

Found a security problem? Please open a GitHub issue marked as such, or contact the maintainer privately rather than posting a working exploit.

## Known limitations

- **No embedded iframes of your sites.** Most major sites refuse to be framed, so the feature would be empty boxes for most cards.
- **No uptime monitoring.** Periodic probing brings background jobs, timeouts and false alarms; it is not worth it for a start page.
- **Some sites still refuse to be scraped.** The fetcher is enough for most sites. Stricter bot protection will keep returning errors — upload an icon manually.
- **The fetcher does not use a proxy.** Icons are downloaded by the *server*, not your browser. A site your browser can open may still be unreachable from the machine running Timmypanel. Deploy somewhere with direct access to the sites you care about.

## Contributing

Issues and pull requests are welcome. Before opening a PR:

- read [`docs/conventions.md`](docs/conventions.md) — it only documents where this project differs from ordinary practice, and it is short;
- run the checks listed under [Development](#development);
- if your change touches auth, per-user isolation, uploads or fetching, walk the manual checklist at the end of [`docs/plans.md`](docs/plans.md);
- keep `backend/internal/web/dist/index.html` as the placeholder page. `npm run build` rewrites it, but the hashed asset files are not in version control, so committing the built page gives fresh clones a silent white screen. `git checkout -- backend/internal/web/dist/index.html` before committing.

Comments, logs and backend error messages are written in Chinese; identifiers and filenames are ASCII. User-facing strings go through `frontend/src/i18n/`, which requires both a Chinese and an English entry — a missing translation fails the type check.

## License

[GNU AGPL v3](LICENSE). You are free to use, modify and redistribute this software, but if you run a modified version as a network service, you must offer its source to that service's users.

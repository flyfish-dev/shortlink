# Deployment Guide

AI Shortlink can run with embedded SQLite for the smallest footprint or MySQL/MariaDB for a larger production deployment. Runtime assets and migrations are embedded in the Go binary.

## Production checklist

Before exposing an instance publicly:

- Use a dedicated HTTPS domain and set the same public base URL in system settings.
- Set `TRUST_PROXY=true` only behind a trusted reverse proxy.
- Set `COOKIE_SECURE=true` when the public site uses HTTPS.
- Preserve `APP_SECRET` and `DATA_DIR` across upgrades.
- Use a persistent volume and establish database and upload backups appropriate to your environment.
- Configure SMTP domain authentication and test transactional delivery.
- Register the exact GitHub OAuth callback URL if GitHub login is enabled.
- Restrict database and application ports to the required network paths.
- Monitor `/healthz`, service restarts, database capacity, disk use, and mail failures.

## Docker with embedded SQLite

```bash
git clone https://github.com/flyfish-dev/shortlink.git
cd shortlink
cp .env.example .env
docker compose up -d --build
```

Open `http://localhost:8080/setup`. The Compose file stores runtime data in the `app_data` volume mounted at `/app/data`.

Useful checks:

```bash
docker compose ps
docker compose logs -f app
curl -fsS http://127.0.0.1:8080/healthz
```

## Docker with MySQL/MariaDB

Start the application and bundled MariaDB profile:

```bash
cp .env.example .env
docker compose --profile mysql up -d --build
```

Choose MySQL/MariaDB in the setup wizard. The bundled development DSN is:

```text
shortlink:shortlink@tcp(db:3306)/ai_shortlink?charset=utf8mb4&parseTime=true&loc=Local
```

Change the example database passwords before a public deployment. For an external database, use a dedicated account and restrict its network access to the application.

## Native Linux binary

Build the Linux amd64 package:

```bash
make release
```

The package under `dist/ai-shortlink-linux-amd64/` contains the binary, environment example, binary deployment notes, and systemd unit.

Install under `/opt/shortlink`:

```bash
sudo mkdir -p /opt/shortlink
sudo cp dist/ai-shortlink-linux-amd64/ai-shortlink /opt/shortlink/
sudo cp dist/ai-shortlink-linux-amd64/shortlink.env.example /opt/shortlink/shortlink.env
sudo cp dist/ai-shortlink-linux-amd64/ai-shortlink.service /etc/systemd/system/ai-shortlink.service
sudo chmod 0755 /opt/shortlink/ai-shortlink
sudo chmod 0600 /opt/shortlink/shortlink.env
sudo systemctl daemon-reload
sudo systemctl enable --now ai-shortlink
```

Verify:

```bash
systemctl status ai-shortlink
journalctl -u ai-shortlink -n 100 --no-pager
curl -fsS http://127.0.0.1:8080/healthz
```

The binary reads `shortlink.env` from its working directory. Set `SHORTLINK_CONFIG=/opt/shortlink/shortlink.env` to select another file. Process environment variables override file values.

## Render

The root `render.yaml` uses the Dockerfile and can be imported as a Render Web Service.

1. Fork or push the repository to GitHub.
2. Create a Render Blueprint from the repository.
3. Attach persistent storage at `/app/data` when using SQLite.
4. Complete setup at `https://<service-domain>/setup`.
5. Set the public base URL, secure cookies, SMTP, and optional OAuth for the final HTTPS domain.

Without persistent storage, container replacement removes SQLite data and runtime configuration.

## Startup environment

Most product settings are maintained in the setup wizard or system settings. Startup-level variables remain intentionally small:

| Variable | Default | Purpose |
| --- | --- | --- |
| `APP_ADDR` | `:8080` | HTTP listener; `PORT` is used when provided and `APP_ADDR` is unset |
| `DATA_DIR` | `./data` | Database, uploads, and runtime configuration directory |
| `APP_SECRET` | Generated and persisted | Cookie signing, recovery-key encryption, and hashing; preserve it after installation |
| `TRUST_PROXY` | `false` | Trust proxy client-IP headers only behind a controlled proxy |
| `COOKIE_SECURE` | `false` | Require HTTPS for authentication cookies |
| `SESSION_TTL_HOURS` | `87600` | Trusted-browser session lifetime |
| `UPLOAD_MAX_MB` | `8` | Maximum uploaded QR image size |
| `GITHUB_CLIENT_ID` | Empty | Optional GitHub OAuth application client ID |
| `GITHUB_CLIENT_SECRET` | Empty | Optional GitHub OAuth secret; never commit it |

Compatibility variables such as `DATABASE_MODE`, `SQLITE_PATH`, `DATABASE_DSN`, `APP_NAME`, and `APP_BASE_URL` remain supported. Prefer the setup wizard and system settings for runtime values.

## GitHub OAuth

Create a GitHub OAuth App and configure:

```env
GITHUB_CLIENT_ID=your_client_id
GITHUB_CLIENT_SECRET=your_client_secret
```

Use the exact callback URL:

```text
https://your-domain.example/auth/github/callback
```

Both variables are required before the login page shows GitHub access. Existing verified emails reuse their accounts; a new verified email creates a regular user. Administrator access must still be granted by an administrator.

Rotate a client secret immediately if it is exposed in a shell history, log, screenshot, Issue, build artifact, or commit.

## SMTP and Magic Link

Configure SMTP during setup or under system settings. For reliable transactional delivery:

- Use a From address aligned with the authenticated SMTP account and domain.
- Publish SPF, DKIM, and DMARC records supplied by the mail provider.
- Use HTTPS public links with the final domain, not localhost, an IP address, or a temporary hostname.
- Configure PTR/rDNS and monitor reputation when operating your own outbound mail server.
- Keep login and approval messages transactional; do not add promotional copy or tracking redirects.
- Test text and HTML alternatives with a real mailbox before launch.

The application prevents repeated Magic Link sends to the same email while an active request is pending. Operators should still apply upstream rate limits appropriate to their exposure.

## Nginx reverse proxy

```nginx
server {
    listen 443 ssl http2;
    server_name s.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

Then set:

```env
TRUST_PROXY=true
COOKIE_SECURE=true
```

Set the public base URL to `https://s.example.com` in system settings so redirects, OAuth callbacks, QR content, and transactional email links agree.

## Data and upgrades

Runtime configuration lives under `DATA_DIR/app-config.json`; embedded SQLite and uploads also live under `DATA_DIR` unless configured otherwise. Protect the whole directory as operational data.

Before an upgrade:

1. Read the commit or release notes.
2. Confirm that the deployment has a usable database and upload backup.
3. Build or obtain the new binary/container from the intended commit.
4. Stop or replace the application once; migrations run automatically at startup.
5. Verify `/healthz`, login, one owned-resource workflow, one public redirect, one live QR, and mail/OAuth if enabled.

Do not copy secrets or production databases into bug reports. Follow [SECURITY.md](../SECURITY.md) for suspected vulnerabilities.

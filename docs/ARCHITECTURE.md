# Architecture

AI Shortlink is a single-process Go application that embeds its admin frontend, public templates, migrations, and transactional email templates. It is designed to remain easy to deploy while keeping authorization and persistence decisions on the server.

## System context

```text
Browser / scanner
      |
      | public redirects, QR pages, admin JSON API
      v
Go HTTP service
  |-- authentication and authorization
  |-- short-link and live-QR domain logic
  |-- QR generation and logo validation
  |-- review and email notifications
  |-- embedded HTML/CSS/JavaScript assets
      |
      +--> SQLite (default) or MySQL/MariaDB
      +--> SMTP server (optional)
      +--> GitHub OAuth (optional)
```

The current codebase has no LLM or model-provider dependency. Planned AI boundaries are documented in [ROADMAP.md](../ROADMAP.md).

## Runtime components

| Path | Responsibility |
| --- | --- |
| `cmd/server` | Process entry point and startup wiring |
| `internal/config` | Environment files, startup settings, and runtime config |
| `internal/auth` | Signed cookies, browser identity, recovery keys, hashing, and encryption helpers |
| `internal/dbutil` | Database opening, migration selection, and schema migration |
| `internal/store` | SQL access and ownership-aware persistence methods |
| `internal/server` | HTTP routes, API handlers, setup, login, public delivery, review, and email |
| `internal/qrcode` | Matrix generation, styles, center-logo composition, output, and decode tests |
| `internal/mysqlmini` | Lightweight MySQL/MariaDB text-protocol driver |
| `internal/sqlitecgo` | Lightweight `database/sql` SQLite driver backed by cgo |
| `web/static` | Embedded admin JavaScript, CSS, and brand assets |
| `web/templates` | Embedded setup, login, live-QR, and error pages |
| `internal/server/mailtpl` | quicktemplate sources and generated transactional mail renderer |

## Request flows

### Short link

1. `GET /s/{code}` or the root-path compatibility route resolves the code.
2. The server checks resource status, approval state, start/end time, and visit limit.
3. A visit record is written with normalized client and user-agent context.
4. The server returns the configured 301/302/307/308 redirect or the fallback/error behavior.

### Live QR

1. `GET /q/{code}` resolves the stable live-QR entry.
2. The server checks the live entry and selects only eligible, approved pool items.
3. Selection follows round-robin, weighted-random, or least-shown behavior.
4. The public page renders the selected QR image and fallback action.
5. Display and long-press intent events are recorded; browsers cannot reliably confirm actual image recognition.

### Authenticated management

1. A signed session identifies a device and account.
2. `requireAuthAPI` rejects unauthenticated API requests.
3. The actor context carries account ID, role, and status.
4. Regular-user queries enforce `owner_account_id`; administrator-only routes require the admin role.
5. Changes that need review remain non-public until an administrator approves them.

## Authentication

- **Magic Link:** one-time token with a short expiry, delivered by configured SMTP.
- **GitHub OAuth:** authorization-code flow with a state cookie; uses a verified GitHub email to match or create a regular account.
- **Trusted browser:** a long-lived browser identifier paired with a server-side device record.
- **Recovery Key:** account recovery material that can rebind a browser when normal credentials are unavailable.

Secrets belong in environment/runtime configuration, never in source control. Public deployments should use HTTPS, secure cookies, and a correctly configured reverse proxy.

## Ownership and review invariants

- `admin_accounts.role` distinguishes `admin` from `user`.
- User-owned resources carry `owner_account_id`.
- Regular users can list, read, update, delete, and inspect statistics only for their own resources.
- Nested live-QR items inherit access through their parent live QR.
- Administrators can operate across owners and control users and system settings.
- UI visibility is convenience only; server-side checks enforce every boundary.
- New or modified public resources enter review and do not become active solely because a client requests it.

Route-level authorization regression tests live in `internal/server/authorization_test.go`.

## Persistence

SQLite is the default embedded mode. MySQL/MariaDB supports larger and multi-operator deployments. Migrations exist under `internal/dbutil/migrations/sqlite` and `internal/dbutil/migrations/mysql`; readable root migrations document the schema history.

Core tables:

- `system_settings`
- `admin_accounts`
- `admin_devices`
- `magic_login_tokens`
- `short_links`
- `live_qrs`
- `live_qr_items`
- `visit_logs`
- `audit_logs`

Any schema change must preserve both database paths and include migration tests or focused store tests.

## Embedded frontend

The management UI uses native HTML, CSS, and JavaScript embedded by `web/assets.go`; there is no Node build step. This keeps the binary self-contained. JavaScript still receives syntax checks in CI.

Visual changes should be verified at desktop and mobile widths, in light and dark themes, and with both Chinese and English copy. Stable control dimensions and compact table actions are intentional product constraints.

## QR rendering

The server produces styled QR matrices and validates logo composition. The browser provides previews and SVG/PNG/WEBP download workflows. QR changes must preserve:

- Sufficient quiet zone and contrast.
- Error-correction capacity around center logos.
- Consistent matrix geometry across classic, rounded, and dot styles.
- Decode-focused tests in addition to visual inspection.

## Email rendering

Magic Link and approval messages share compiled quicktemplate sources. Edit `internal/server/mailtpl/mail.qtpl`, regenerate with `go generate ./internal/server/mailtpl`, and keep text and HTML alternatives semantically equivalent.

## Operational endpoints

| Endpoint | Purpose |
| --- | --- |
| `/healthz` | Process health check |
| `/setup` | First-run installation |
| `/login` | User access |
| `/admin` | Authenticated management UI |
| `/api/admin/...` | Authenticated JSON management API |
| `/s/{code}` | Public short-link redirect |
| `/q/{code}` | Public live-QR page |
| `/qr/short/{code}.{svg,png}` | Short-link QR output |
| `/qr/live/{code}.{svg,png}` | Live-entry QR output |

Deployment topology and configuration are documented in [DEPLOYMENT.md](DEPLOYMENT.md).

# Contributing to AI Shortlink

感谢你愿意改进 AI Shortlink。项目欢迎缺陷修复、可用性提升、测试、文档、部署适配和经过充分说明的产品建议。

Contributions are welcome. This guide is Chinese-first; every requirement, command, and review expectation applies equally to English-language contributions.

## Before you start

- Search existing [Issues](https://github.com/flyfish-dev/shortlink/issues) and pull requests before opening a duplicate.
- Use the bug form for reproducible defects, the feature form for product proposals, and the question form for usage questions.
- Open an Issue before large changes to authentication, permissions, database schema, public routes, QR rendering, or AI integration.
- Never put credentials, private URLs, user data, production database contents, or access tokens in an Issue, screenshot, fixture, or commit.
- Security vulnerabilities must follow [SECURITY.md](SECURITY.md), not a public Issue.

Small typo fixes and narrowly scoped test improvements can go directly to a pull request.

## Development setup

Requirements:

- Go 1.23.2 or newer in the Go 1.23 line
- A C compiler
- SQLite3 runtime and development headers
- Node.js only for JavaScript syntax checks
- Docker and Docker Compose when testing container deployment

Clone and prepare the project:

```bash
git clone https://github.com/flyfish-dev/shortlink.git
cd shortlink
cp .env.example .env
go mod download
make test
make build
```

Run locally:

```bash
./bin/ai-shortlink
```

Then open `http://localhost:8080/setup`. Keep local runtime files under `data/`; they are not contribution artifacts.

## Branch and commit scope

- Create a focused branch such as `fix/qr-logo-scan`, `feat/live-qr-policy`, or `docs/ai-roadmap`.
- Keep a pull request focused on one coherent outcome.
- Do not mix formatting churn, unrelated refactors, generated binaries, database files, or local configuration into the change.
- Write commits in imperative form and explain the user-visible outcome, for example `fix mobile live qr actions`.
- Preserve existing public behavior unless the Issue and pull request clearly describe a deliberate breaking change.

## Engineering expectations

### Permissions and data

- Treat server-side authorization as the source of truth; hiding a UI control is not authorization.
- Every user-owned resource query must enforce `owner_account_id` for regular users and allow explicit administrator access.
- Add route-level regression tests for list, detail, create, update, delete, statistics, review, and nested-resource boundaries as applicable.
- Schema changes must work for both embedded SQLite and MySQL/MariaDB and include migrations for both paths.

### Frontend and product behavior

- Keep the interface compact, responsive, and task-focused across desktop and mobile.
- Reuse the existing control, icon, typography, spacing, theme, and internationalization patterns.
- Add both Chinese and English copy for every user-facing string.
- For visual changes, include desktop and mobile screenshots or a short recording in the pull request.
- Test light and dark themes, long Chinese/English labels, empty states, errors, loading, and permission-restricted states.

### QR codes

- Preserve quiet zones, contrast, and error-correction assumptions.
- Add decode tests when changing matrix rendering, center logos, resizing, or export behavior.
- Verify classic, rounded, and dot styles separately; visual similarity does not guarantee scanability.

### Email templates

Edit `internal/server/mailtpl/mail.qtpl`, then regenerate the compiled template:

```bash
go generate ./internal/server/mailtpl
```

Commit both the source template and generated `mail.qtpl.go`. Keep plain-text and HTML parts equivalent, localized, and transactional in tone.

### AI features

Read [ROADMAP.md](ROADMAP.md) before proposing model integration. AI changes must be optional, provider-neutral where practical, explicit about transmitted data, schema-validated, auditable, and unable to bypass ownership or review rules.

## Required checks

Run these checks before opening a pull request:

```bash
gofmt -w <changed-go-files>
go test ./...
go vet ./...
go build -o bin/ai-shortlink ./cmd/server
node --check web/static/app.js
node --check web/static/platform_ext.js
git diff --check
```

Run `docker compose up -d --build` when changing the Dockerfile, Compose configuration, startup settings, embedded assets, or deployment behavior.

## Pull request checklist

A reviewable pull request should explain:

- The user problem and why the change belongs in this project.
- The chosen behavior and important alternatives considered.
- Permission, migration, deployment, privacy, and compatibility impact.
- Tests performed and any remaining risk.
- Screenshots or recordings for visible changes.
- Documentation and localization updates.

Maintainers may ask to split a change when independent concerns make review or rollback difficult.

## Review and merge

- CI must pass before merge.
- Maintainers review correctness, security boundaries, product consistency, test coverage, and maintainability.
- Approval is not guaranteed merely because CI passes.
- Maintainers may close proposals that conflict with the roadmap or project scope, with an explanation.
- By contributing, you agree that your contribution is licensed under [AGPL-3.0-only](LICENSE).

## Community

Be direct, specific, and respectful. Critique behavior and code, not people. All participation is governed by [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

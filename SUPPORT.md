# Support Guide

AI Shortlink is a community-maintained open-source project. GitHub is the canonical place for reproducible product feedback and development discussion.

## Choose the right channel

| Need | Channel |
| --- | --- |
| Reproducible defect | [Bug report](https://github.com/flyfish-dev/shortlink/issues/new?template=bug_report.yml) |
| Product or AI capability proposal | [Feature request](https://github.com/flyfish-dev/shortlink/issues/new?template=feature_request.yml) |
| Setup or usage question | [Question](https://github.com/flyfish-dev/shortlink/issues/new?template=question.yml) |
| Security vulnerability | Private process in [SECURITY.md](SECURITY.md) |
| Code contribution | [CONTRIBUTING.md](CONTRIBUTING.md) |

Do not post credentials, OAuth secrets, recovery keys, Magic Links, cookies, SMTP passwords, private links, personal information, or production databases.

## Before opening an Issue

1. Search existing open and closed Issues.
2. Reproduce on the latest `main` branch when possible.
3. Confirm whether the problem is product behavior, deployment configuration, reverse proxy behavior, DNS/SMTP reputation, or an upstream service.
4. Reduce the report to the smallest reproducible case.
5. Remove secrets and personal data from logs and screenshots.

## Useful diagnostic information

- Commit hash or deployed build date.
- Operating system and architecture.
- Docker, native binary, or Render deployment.
- SQLite or MySQL/MariaDB mode and database version.
- Browser and viewport for UI issues.
- Reverse proxy and HTTPS topology.
- Exact steps, expected result, actual result, and sanitized logs.
- Desktop and mobile screenshots for layout problems.

## Production support boundary

The public instance at [s.flyfish.dev](https://s.flyfish.dev) demonstrates the current product; it is not a hosted-service SLA for self-hosted installations. Operators remain responsible for backups, TLS, DNS, SMTP deliverability, OAuth credentials, database operations, monitoring, and compliance in their own environment.

Questions with a complete reproduction and clear expected behavior are easier for maintainers and contributors to answer.

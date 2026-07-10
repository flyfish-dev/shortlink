## Summary

<!-- What user problem does this solve, and what behavior changed? -->

## Scope and decisions

<!-- Explain important implementation choices and alternatives. Link the Issue. -->

Closes #

## Verification

<!-- List exact checks and manual flows performed. -->

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `go build -o bin/ai-shortlink ./cmd/server`
- [ ] `node --check web/static/app.js`
- [ ] `node --check web/static/platform_ext.js`
- [ ] `git diff --check`

## Risk and compatibility

- [ ] Authorization and ownership boundaries were reviewed.
- [ ] SQLite and MySQL/MariaDB impact was reviewed.
- [ ] Authentication, secrets, privacy, and public-route impact was reviewed.
- [ ] Upgrade and rollback behavior was considered.

## Product quality

- [ ] Chinese and English user-facing copy were updated together.
- [ ] Desktop and mobile behavior were checked when the UI changed.
- [ ] Light and dark themes were checked when the UI changed.
- [ ] Screenshots or a recording are included for visible changes.
- [ ] Documentation and roadmap status are updated where needed.

## Notes

<!-- Remaining risk, follow-up work, screenshots, migration notes, or reviewer guidance. -->

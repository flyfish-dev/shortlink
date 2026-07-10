# AI Shortlink Roadmap

This roadmap explains product direction and decision boundaries. It is not a release-date commitment. Priorities may change as production feedback, security findings, and community Issues evolve.

本文档描述产品方向与决策边界，不承诺具体发布日期。优先级会根据生产反馈、安全问题和社区 Issue 持续调整。

## Product thesis

AI Shortlink is building a dependable control layer for links and live QR codes. Intelligence is useful only after ownership, review, observability, and failure handling are trustworthy.

品牌核心可以概括为：**稳定入口、持续可控、人机协作、责任可追溯。**

## Status definitions

| Status | Meaning |
| --- | --- |
| Delivered | Available in the current codebase; hardening and usability work continue |
| In design | Problem and boundaries are being defined; implementation has not started |
| Planned | Direction is accepted, but scope and sequence may still change |
| Exploring | A hypothesis that needs Issues, user evidence, and technical validation |

## Stage 0: Trusted entry foundation

Status: **Delivered; continuous hardening**

- Short links with custom codes, redirect policy, schedules, limits, fallback URLs, QR codes, and analytics.
- Live QR entries with multi-code pools, rotation strategies, transactional edits, and public fallback behavior.
- Branded QR rendering with classic, rounded, and dot styles, center logos, preview, and SVG/PNG/WEBP export.
- Multi-user ownership, administrator review, approval notifications, and audit records.
- Magic Link, GitHub OAuth, trusted-browser login, recovery keys, localized site identity, and email templates.
- Embedded SQLite for small deployments and MySQL/MariaDB for production growth.
- Desktop and mobile management workflows with a compact, task-focused interface.

Quality priorities for this stage remain active:

- Authorization regression coverage for every new resource endpoint.
- QR decode reliability across styles, logos, sizes, and export formats.
- Mobile layout, keyboard navigation, and accessible control states.
- Upgrade safety, migration coverage, deployment documentation, and observable email failures.

## Stage 1: AI integration foundation

Status: **In design**

This stage answers the concern raised in [Issue #1](https://github.com/flyfish-dev/shortlink/issues/1): the AI name must become a concrete, accountable product capability rather than a label.

Before any user-facing assistant ships, the project needs:

- An optional, provider-neutral model interface. Core link and QR behavior must work without a model API.
- Explicit settings for provider, model, data scope, timeouts, and feature-level enablement.
- Data minimization and redaction rules before content leaves the deployment boundary.
- Versioned prompts and structured outputs with schema validation.
- Invocation records that capture purpose, actor, result status, and configuration version without logging secrets.
- Timeouts, rate limits, cost boundaries, retries, and graceful fallback to the normal manual workflow.
- Tests that use deterministic fakes; CI and local tests must not require external model credentials.

Exit criteria:

- Administrators can understand exactly what is sent, when it is sent, and how to turn it off.
- A failed or unavailable model cannot block normal short-link or live-QR operations.
- Model output cannot directly publish or approve a resource without an explicit policy and human confirmation.

## Stage 2: Configuration assistant

Status: **Planned**

Turn natural-language intent into a reviewable draft. Example:

> Create a campaign live QR for the next seven days, rotate these three codes by weight, and fall back to the support page after expiration.

Expected interaction:

1. Parse the intent into a typed draft.
2. Show the exact field-level diff, assumptions, and validation errors.
3. Let the user edit or reject the proposal.
4. Save only after explicit confirmation, using the same ownership and review rules as manual edits.

Initial scope:

- Short-link and live-QR draft creation.
- Explanation of schedules, rotation strategies, limits, and review consequences.
- Configuration checks for missing fallback URLs, contradictory dates, and likely scanability risks.

## Stage 3: Review assistant

Status: **Planned**

Help administrators review faster without turning a probabilistic model into the final authority.

- Summarize the submitted destination and configuration changes.
- Present deterministic checks separately from model-generated observations.
- Highlight redirects, domain changes, suspicious mismatches, and incomplete fallback behavior.
- Produce a recommendation with reasons and uncertainty, not a silent pass/fail decision.
- Require administrator confirmation by default and retain the final actor in the audit record.

Non-goal: claiming that model output proves a destination is safe. External threat intelligence and deterministic policy checks must remain distinct from language-model judgment.

## Stage 4: Operational insights

Status: **Exploring**

- Natural-language summaries of traffic, device, and live-QR distribution trends.
- Explainable anomaly flags tied to source metrics and time ranges.
- Suggestions for expiration, capacity, fallback, or pool-balance changes.
- Scheduled reports and webhook/API integration after stable event contracts exist.

Every recommendation should link back to measurable data and remain a draft until confirmed.

## Explicit non-goals

- Requiring a paid AI provider for core product use.
- Sending customer data to a model provider without administrator opt-in.
- Fully autonomous approval as the default behavior.
- Presenting model confidence as a security guarantee.
- Becoming a general-purpose marketing automation suite before link and live-QR operations are excellent.
- Claiming that browsers can reliably report whether a user truly recognized a QR code from an image; the product records display and long-press intent signals instead.

## How roadmap work enters development

1. Start with an Issue describing the user problem, evidence, risks, and expected outcome.
2. Link the proposal to a roadmap stage and identify whether it changes data, permissions, public routes, or deployment.
3. Agree on acceptance criteria before implementation for cross-cutting changes.
4. Keep pull requests narrow, tested, documented, and reversible.
5. Update this roadmap when the product boundary or stage status changes.

Use the [feature request form](https://github.com/flyfish-dev/shortlink/issues/new?template=feature_request.yml) for proposals. See [CONTRIBUTING.md](CONTRIBUTING.md) for the implementation workflow.

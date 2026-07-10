# Security Policy

AI Shortlink manages public redirects, authentication, uploaded QR images, account ownership, and administrator review. Security reports are treated as product issues, not as support questions.

## Supported versions

Until tagged releases are published, security fixes target the latest commit on the `main` branch. Older commits, forks, and modified deployments may not receive fixes.

| Version | Supported |
| --- | --- |
| Latest `main` | Yes |
| Older commits and unmaintained forks | No |

## Report a vulnerability

Do not open a public Issue for a suspected vulnerability.

Use GitHub's private vulnerability reporting flow:

<https://github.com/flyfish-dev/shortlink/security/advisories/new>

Include only the information needed to reproduce and assess the issue:

- A concise description and affected component.
- Tested commit, deployment mode, database mode, and relevant configuration with secrets removed.
- Reproduction steps or a minimal proof of concept.
- Expected impact and the permissions required to trigger it.
- Suggested mitigation, if known.

Never include production credentials, OAuth secrets, recovery keys, Magic Links, session cookies, SMTP passwords, personal data, or an unredacted production database.

## Response targets

The maintainers aim to:

- Acknowledge a complete report within 3 business days.
- Provide an initial severity and remediation assessment within 7 business days.
- Coordinate a fix and disclosure timeline based on exploitability and deployment impact.

These are best-effort targets for an open-source project, not a service-level agreement.

## In scope

- Authentication or session bypass.
- Cross-account access to user-owned short links, live QR codes, items, or analytics.
- Administrator privilege escalation or approval bypass.
- Open redirect behavior outside the configured short-link contract.
- Stored or reflected script injection in the admin or public pages.
- Unsafe upload handling, path traversal, request forgery, or secret disclosure.
- OAuth state, callback, or verified-email handling flaws.
- QR rendering behavior that allows attacker-controlled active content.
- Model-integration data disclosure or authorization bypass once AI features are introduced.

## Usually out of scope

- Spam or reputation caused by an operator's own links, SMTP domain, or content.
- Missing DNS email-authentication records on an operator-controlled domain.
- Denial of service requiring unrealistic local access or unbounded infrastructure assumptions.
- Findings that require a deliberately weakened deployment, such as public HTTP with secure cookies disabled contrary to the deployment guide.
- Social engineering without a product vulnerability.

## Safe research

Use your own local deployment and test data. Do not test against [s.flyfish.dev](https://s.flyfish.dev) or another operator's instance without explicit permission. Stop testing if you access data that is not yours, preserve minimal evidence, and report privately.

## Disclosure

Please allow time for supported deployments to receive a fix before publishing technical details. The project will credit reporters when requested and appropriate.

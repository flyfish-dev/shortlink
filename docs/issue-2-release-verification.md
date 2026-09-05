# Issue #2 SaaS release verification

Date: 2026-09-05  
Branch: `feat/saas-tenants-subscriptions-routing`  
Source: delivery packages `shortlink-saas-delivery-20260905.zip` / `shortlink-saas-complete.zip`

## Gate results

```text
gofmt -l $(find . -name '*.go' -not -path './vendor/*')  # clean
go test ./...                                           # pass
go vet ./...                                            # pass
go build ./...                                          # pass
node --check web/static/*.js                            # pass
git diff --check                                        # pass
```

## Compatibility fixes applied during integration

- Legacy `CreateShortLink` / `CreateLiveQR` now assign the owner's personal tenant and use `tenant_pending`.
- `EnsurePersonalTenant` backfills orphaned `tenant_id` rows for the account.
- Authorization tests updated for tenant-scoped lists and intentional cross-tenant `404` responses.

## Notes

- Public short codes remain globally unique.
- Existing single-target links continue to work via `target_url` fallback.
- Platform admins remain separated from tenant roles; resource lists are workspace-scoped.

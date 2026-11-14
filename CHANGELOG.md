# Changelog

All notable changes to this project are documented here. Format loosely
follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] - 2026-11-14

### Added

- `TreasuryClient` (agent key): `Payments.Submit`/`Get`,
  `Beneficiaries.Register`.
- `MerchantClient` (merchant key): `Customers`, `MerchantAgents`,
  `FundingMethods`, `Mandates`, `Checkouts`, `Grants`, `Executions` -
  the full commerce surface a merchant integration key reaches.
- `enquoir.APIError`/`IsCode` for branching on Enquoir's RFC 9457 Problem
  response codes.
- `PaymentState.Terminal()` to check whether a payment has reached a final
  state (`APPLIED` or `BLOCKED` - notably not `FAILED`, which can still
  move on).
- `contract_test.go` / `internal/contracttest`: every table-driven test's
  request and response is validated against the vendored OpenAPI spec with
  `kin-openapi`, so a hand-written wire type can't silently drift from the
  API it's meant to match.
- `scripts/sync-openapi.sh` (and `--check`) to re-vendor `openapi.yaml` from
  the backend repo.
- Runnable examples: `examples/submit-payment` (treasury, register → submit
  → poll to terminal) and `examples/commerce-checkout` (commerce, submit →
  authorize → issue grant → redeem → report outcome, against an existing
  active mandate).
- GitHub Actions CI: build, vet, test, `gofmt`, and `sync-openapi.sh
  --check` against a sibling checkout of the backend repo.

### Verified live, before this tag

- `examples/commerce-checkout` ran end to end against a real local
  sandbox: registered a customer, agent and funding method; requested a
  mandate and approved its emailed consent link; submitted a checkout
  (authorized); issued and redeemed its grant; reported the outcome as
  succeeded. Every commerce method on that path was exercised for real,
  not just against table-driven test fixtures.
- `examples/submit-payment` was **not** verified live for this tag: the
  backend's Airwallex sandbox credentials were returning `403 Forbidden`
  on login itself at release time, an external/environment issue
  unrelated to the SDK (confirmed via the backend's own attempt records;
  the SDK's request was correctly formed up to that point). Treasury's
  full behavior is still covered by its table-driven unit and
  contract-validation tests. Re-verify live once the sandbox credentials
  are rotated.

[0.1.0]: https://github.com/Tee-Stark/enquoir-go/releases/tag/v0.1.0

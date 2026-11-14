# Changelog

All notable changes to this project are documented here. Format loosely
follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

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

[Unreleased]: https://github.com/Tee-Stark/enquoir-go/commits/main

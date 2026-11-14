// Package openapi holds the vendored copy of Enquoir's API contract,
// re-synced from the backend by scripts/sync-openapi.sh. contract_test.go
// validates every SDK request and response against it.
package openapi

import _ "embed"

// Spec is openapi.yaml, the OpenAPI 3.1 description of every route
// Enquoir serves.
//
//go:embed openapi.yaml
var Spec []byte

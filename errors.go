package enquoir

import (
	"errors"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

// APIError is Enquoir's RFC 9457 Problem response, returned by every
// client method. Code is stable and meant to be branched on with IsCode;
// Detail is for people and may change between releases.
type APIError = transport.Error

// IsCode reports whether err is an *APIError with the given code, such as
// "payee_unknown" or "policy_denied" - the full list is in Enquoir's
// OpenAPI spec under Problem.code.
func IsCode(err error, code string) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == code
	}
	return false
}

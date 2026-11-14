package enquoir

import (
	"io"
	"net/http"
)

// readAll reads and returns a request's body, for tests that need to
// hand it to validateRequest after the handler has already consumed it.
func readAll(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

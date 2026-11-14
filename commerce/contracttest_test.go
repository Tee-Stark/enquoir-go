package commerce

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/Tee-Stark/enquoir-go/internal/contracttest"
)

func validateRequest(t *testing.T, method, path string, body []byte) {
	contracttest.ValidateRequest(t, method, path, body)
}

func validateResponse(t *testing.T, method, path string, status int, header http.Header, body []byte) {
	contracttest.ValidateResponse(t, method, path, status, header, body)
}

// readAll reads and returns a request's body, for tests that need to
// hand it to validateRequest after the handler has already consumed it.
func readAll(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

func problemHandler(status int, code, detail string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "about:blank", "title": http.StatusText(status),
			"status": status, "detail": detail, "code": code,
		})
	}
}

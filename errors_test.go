package enquoir

import (
	"errors"
	"testing"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

func TestIsCode(t *testing.T) {
	err := &transport.Error{Code: "payee_unknown"}
	if !IsCode(err, "payee_unknown") {
		t.Error("IsCode: want true for matching code")
	}
	if IsCode(err, "policy_denied") {
		t.Error("IsCode: want false for mismatched code")
	}
	if IsCode(errors.New("not an APIError"), "payee_unknown") {
		t.Error("IsCode: want false for a non-APIError")
	}
}

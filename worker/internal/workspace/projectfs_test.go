package workspace

import (
	"errors"
	"testing"

	services "worker/services"
)

func TestLoadRequestPayloadMapMissingFileIsNonRetryable(t *testing.T) {
	_, err := LoadRequestPayloadMap("missing-project")
	var nonRetryable services.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable error, got: %v", err)
	}
}

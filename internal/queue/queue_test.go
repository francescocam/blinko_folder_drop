package queue

import (
	"errors"
	"testing"

	"blinko-folder-drop/internal/blinko"
)

func TestRetryableClassification(t *testing.T) {
	if !isRetryable(errors.New("network")) {
		t.Fatalf("network errors should retry")
	}
	if !isRetryable(&blinko.HTTPError{StatusCode: 500}) {
		t.Fatalf("500 should retry")
	}
	if isRetryable(&blinko.HTTPError{StatusCode: 400}) {
		t.Fatalf("400 should not retry")
	}
}

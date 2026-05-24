package limiter

import (
	"testing"
	"time"
)

func TestParseTokenBucketResult(t *testing.T) {
	decision, err := parseTokenBucketResult([]interface{}{int64(1), int64(7), int64(2)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed || decision.Remaining != 7 || decision.RetryAfter != 2*time.Second || decision.Backend != "redis" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestParseTokenBucketResultRejectsInvalidResponse(t *testing.T) {
	_, err := parseTokenBucketResult("bad response")
	if err == nil {
		t.Fatal("expected error for invalid redis response")
	}
}

package digestworkspace_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"oralhistory/internal/evidence"
)

type gatedJSON struct {
	entered chan<- struct{}
	release <-chan struct{}
}

func (value gatedJSON) MarshalJSON() ([]byte, error) {
	close(value.entered)
	<-value.release
	return []byte(`{"case_id":"sealed-case"}`), nil
}

func TestConcurrentDigestWorkspaceIsIsolated(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	type result struct {
		digest string
		err    error
	}
	finished := make(chan result, 1)

	go func() {
		digest, err := evidence.Digest(gatedJSON{entered: entered, release: release})
		finished <- result{digest: digest, err: err}
	}()

	<-entered
	evidence.DigestBytes([]byte(`{"request_id":"parallel-request"}`))
	close(release)
	actual := <-finished
	if actual.err != nil {
		t.Fatalf("Digest 返回错误: %v", actual.err)
	}
	wantBytes := sha256.Sum256([]byte(`{"case_id":"sealed-case"}`))
	want := hex.EncodeToString(wantBytes[:])
	if actual.digest != want {
		t.Fatalf("并发请求污染封存摘要: got %s, want %s", actual.digest, want)
	}
}

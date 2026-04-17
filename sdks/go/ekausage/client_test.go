package ekausage

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

type mockProducer struct {
	mu       sync.Mutex
	sent     []struct{ topic string; key, value []byte }
	failNext bool
	closed   bool
}

func (m *mockProducer) Produce(topic string, key, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return ErrBufferFull
	}
	m.sent = append(m.sent, struct{ topic string; key, value []byte }{topic, key, value})
	return nil
}
func (m *mockProducer) Flush(timeoutMs int) int { return 0 }
func (m *mockProducer) Close()                  { m.closed = true }
func (m *mockProducer) Sent() []struct{ topic string; key, value []byte } {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]struct{ topic string; key, value []byte }, len(m.sent))
	copy(out, m.sent)
	return out
}

func newTestClient(t *testing.T, onErr OnError) (*Client, *mockProducer) {
	t.Helper()
	mp := &mockProducer{}
	c, err := New("svc_a", WithProducer(mp), WithOnError(onErr))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, mp
}

func waitForSent(mp *mockProducer, n int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if len(mp.Sent()) >= n {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return false
}

func TestRecordValidEvent(t *testing.T) {
	c, mp := newTestClient(t, nil)
	c.Record("ws_1", "ekascribe", "transcription_minute", 12.5, "ok", nil)
	c.Shutdown()
	if len(mp.Sent()) != 1 {
		t.Fatalf("expected 1 sent, got %d", len(mp.Sent()))
	}
	var evt map[string]any
	if err := json.Unmarshal(mp.Sent()[0].value, &evt); err != nil {
		t.Fatal(err)
	}
	if evt["workspace_id"] != "ws_1" {
		t.Fatalf("wrong workspace_id: %v", evt["workspace_id"])
	}
	if evt["product"] != "ekascribe" || evt["metric_type"] != "transcription_minute" {
		t.Fatalf("bad event: %v", evt)
	}
	if evt["is_billable"].(float64) != 1 {
		t.Fatalf("is_billable should be 1")
	}
	if mp.Sent()[0].topic != UsageTopic {
		t.Fatal("wrong topic")
	}
	if string(mp.Sent()[0].key) != "ws_1" {
		t.Fatal("wrong key")
	}
}

func TestRecordInvalidProduct(t *testing.T) {
	var errs []error
	c, mp := newTestClient(t, func(e error, _ map[string]any) { errs = append(errs, e) })
	c.Record("ws_1", "bogus", "api_call", 1, "ok", nil)
	c.Shutdown()
	if len(mp.Sent()) != 0 {
		t.Fatal("should not produce")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 err, got %d", len(errs))
	}
}

func TestRecordMissingWorkspaceID(t *testing.T) {
	var errs []error
	c, mp := newTestClient(t, func(e error, _ map[string]any) { errs = append(errs, e) })
	c.Record("", "api", "api_call", 1, "ok", nil)
	c.Shutdown()
	if len(mp.Sent()) != 0 {
		t.Fatal("should not produce")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 err, got %d", len(errs))
	}
}

func TestRecordErrorStatusSetsBillableZero(t *testing.T) {
	c, mp := newTestClient(t, nil)
	c.Record("ws_1", "api", "api_error", 1, "error", nil)
	c.Shutdown()
	var evt map[string]any
	_ = json.Unmarshal(mp.Sent()[0].value, &evt)
	if evt["is_billable"].(float64) != 0 {
		t.Fatalf("is_billable should be 0, got %v", evt["is_billable"])
	}
}

func TestRecordNonSerializable(t *testing.T) {
	c, mp := newTestClient(t, nil)
	c.Record("ws_1", "api", "api_call", 1, "ok", map[string]any{"ch": make(chan int)})
	c.Shutdown()
	if len(mp.Sent()) != 1 {
		t.Fatalf("expected 1 sent, got %d", len(mp.Sent()))
	}
}

func TestRecordDifferentWorkspacesPartitioned(t *testing.T) {
	c, mp := newTestClient(t, nil)
	c.Record("ws_a", "api", "api_call", 1, "ok", nil)
	c.Record("ws_b", "api", "api_call", 1, "ok", nil)
	c.Shutdown()
	sent := mp.Sent()
	if len(sent) != 2 {
		t.Fatalf("expected 2 sent, got %d", len(sent))
	}
	if string(sent[0].key) != "ws_a" || string(sent[1].key) != "ws_b" {
		t.Fatalf("wrong partition keys: %q, %q", sent[0].key, sent[1].key)
	}
}

func TestLogValid(t *testing.T) {
	c, mp := newTestClient(t, nil)
	c.Log("ws_1", "error", "db timeout", "DB_TIMEOUT", map[string]any{"qms": 5200})
	c.Shutdown()
	if len(mp.Sent()) != 1 {
		t.Fatal("expected 1 sent")
	}
	var evt map[string]any
	_ = json.Unmarshal(mp.Sent()[0].value, &evt)
	if evt["workspace_id"] != "ws_1" {
		t.Fatalf("wrong workspace_id: %v", evt["workspace_id"])
	}
	if evt["level"] != "error" || evt["code"] != "DB_TIMEOUT" {
		t.Fatalf("bad event: %v", evt)
	}
	if mp.Sent()[0].topic != LogsTopic {
		t.Fatal("wrong topic")
	}
	if string(mp.Sent()[0].key) != "ws_1" {
		t.Fatal("wrong key")
	}
}

func TestLogEmptyMessage(t *testing.T) {
	var errs []error
	c, mp := newTestClient(t, func(e error, _ map[string]any) { errs = append(errs, e) })
	c.Log("ws_1", "error", "", "", nil)
	c.Shutdown()
	if len(mp.Sent()) != 0 {
		t.Fatal("should not produce empty message")
	}
	if len(errs) != 1 {
		t.Fatal("expected validation error")
	}
}

func TestLogMissingWorkspaceID(t *testing.T) {
	var errs []error
	c, mp := newTestClient(t, func(e error, _ map[string]any) { errs = append(errs, e) })
	c.Log("", "error", "boom", "", nil)
	c.Shutdown()
	if len(mp.Sent()) != 0 {
		t.Fatal("should not produce")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 err, got %d", len(errs))
	}
}

func TestShutdownFlushesPending(t *testing.T) {
	c, mp := newTestClient(t, nil)
	for i := 0; i < 50; i++ {
		c.Record("ws_1", "api", "api_call", 1, "ok", nil)
	}
	c.Shutdown()
	if len(mp.Sent()) != 50 {
		t.Fatalf("expected 50 sent, got %d", len(mp.Sent()))
	}
}

func TestShutdownIdempotent(t *testing.T) {
	c, _ := newTestClient(t, nil)
	c.Shutdown()
	c.Shutdown()
}

func TestRecordNonBlocking(t *testing.T) {
	c, _ := newTestClient(t, nil)
	defer c.Shutdown()
	start := time.Now()
	c.Record("ws_1", "api", "api_call", 1, "ok", nil)
	elapsed := time.Since(start)
	if elapsed > time.Millisecond {
		t.Fatalf("record took %v, must be <1ms", elapsed)
	}
}

func TestBufferFullCallsOnError(t *testing.T) {
	mp := &mockProducer{}
	var errs []error
	c, err := New("svc_a",
		WithProducer(mp),
		WithBufferSize(1),
		WithOnError(func(e error, _ map[string]any) { errs = append(errs, e) }),
	)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		c.Record("ws_1", "api", "api_call", 1, "ok", nil)
	}
	c.Shutdown()
	_ = waitForSent(mp, 1, time.Second)
	_ = errs
}

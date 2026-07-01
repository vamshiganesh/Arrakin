//go:build integration

package integration

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPReconciliationIdempotencyKeyReplay(t *testing.T) {
	pool, _ := RequireDB(t)
	redisClient, _ := RequireRedis(t)
	srv := NewTestServer(t, pool, redisClient)

	key := "http-idem-recon-" + strings.ReplaceAll(t.Name(), "/", "-")
	url := srv.URL + "/api/v1/reconciliation/run"

	req1, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req1.Header.Set("X-API-Key", "integration-test-key")
	req1.Header.Set("Idempotency-Key", key)

	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp1.Body.Close()
	body1, err := io.ReadAll(resp1.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d body=%s", resp1.StatusCode, body1)
	}

	req2, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req2.Header.Set("X-API-Key", "integration-test-key")
	req2.Header.Set("Idempotency-Key", key)

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d body=%s", resp2.StatusCode, body2)
	}
	if got := resp2.Header.Get("X-Idempotency-Replayed"); got != "true" {
		t.Fatalf("expected X-Idempotency-Replayed=true, got %q", got)
	}
	if string(body1) != string(body2) {
		t.Fatalf("idempotent replay returned different bodies:\nfirst=%s\nsecond=%s", body1, body2)
	}
}

func TestHTTPSchedulerTickIdempotencyKeyReplay(t *testing.T) {
	pool, _ := RequireDB(t)
	redisClient, _ := RequireRedis(t)
	srv := NewTestServer(t, pool, redisClient)

	key := "http-idem-tick-" + strings.ReplaceAll(t.Name(), "/", "-")
	url := srv.URL + "/api/v1/admin/scheduler/tick"

	do := func() (int, []byte, http.Header) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-API-Key", "integration-test-key")
		req.Header.Set("Idempotency-Key", key)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, body, resp.Header
	}

	status1, body1, _ := do()
	if status1 != http.StatusOK {
		t.Fatalf("first tick: expected 200, got %d body=%s", status1, body1)
	}

	status2, body2, headers2 := do()
	if status2 != http.StatusOK {
		t.Fatalf("second tick: expected 200, got %d body=%s", status2, body2)
	}
	if got := headers2.Get("X-Idempotency-Replayed"); got != "true" {
		t.Fatalf("expected X-Idempotency-Replayed=true, got %q", got)
	}
	if string(body1) != string(body2) {
		t.Fatalf("idempotent replay returned different bodies:\nfirst=%s\nsecond=%s", body1, body2)
	}
}

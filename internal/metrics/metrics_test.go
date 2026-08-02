package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New() returned nil")
	}

	if m.registry == nil {
		t.Error("registry is nil")
	}
}

func TestUpdateSystemMetrics(t *testing.T) {
	m := New()

	m.UpdateSystemMetrics(1.5, 2.5, 3.5, 8, 16*1024*1024*1024, 4*1024*1024*1024)

	// We can't easily read the values back without a gather,
	// but we can verify no panic occurred.
}

func TestUpdateProcessCount(t *testing.T) {
	m := New()

	m.UpdateProcessCount(100)
	m.UpdateProcessCount(200)
}

func TestUpdateProcessCountByName(t *testing.T) {
	m := New()

	m.UpdateProcessCountByName("rg", 10)
	m.UpdateProcessCountByName("node", 50)
	m.UpdateProcessCountByName("rg", 15) // Update existing.
}

func TestRecordPolicyEvaluation(t *testing.T) {
	m := New()

	m.RecordPolicyEvaluation("runaway-rg")
	m.RecordPolicyEvaluation("runaway-rg")
	m.RecordPolicyEvaluation("elevated-rg")
}

func TestRecordPolicyTrigger(t *testing.T) {
	m := New()

	m.RecordPolicyTrigger("runaway-rg")
	m.RecordPolicyTrigger("elevated-rg")
}

func TestRecordPolicyAction(t *testing.T) {
	m := New()

	m.RecordPolicyAction("runaway-rg", "log")
	m.RecordPolicyAction("runaway-rg", "terminate")
	m.RecordPolicyAction("elevated-rg", "notify")
}

func TestRecordCheckDuration(t *testing.T) {
	m := New()

	m.RecordCheckDuration(100 * time.Millisecond)
	m.RecordCheckDuration(500 * time.Millisecond)
	m.RecordCheckDuration(1 * time.Second)
}

func TestRecordCheck(t *testing.T) {
	m := New()

	m.RecordCheck("startup")
	m.RecordCheck("periodic")
	m.RecordCheck("load-triggered")
}

func TestRecordTermination(t *testing.T) {
	m := New()

	m.RecordTermination("runaway-rg", "rg", false) // SIGTERM
	m.RecordTermination("runaway-rg", "rg", true)  // SIGKILL
}

func TestHandler(t *testing.T) {
	m := New()

	// Update some metrics.
	m.UpdateSystemMetrics(1.5, 2.5, 3.5, 8, 16*1024*1024*1024, 4*1024*1024*1024)
	m.UpdateProcessCount(100)
	m.RecordCheck("startup")

	handler := m.Handler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	bodyStr := string(body)

	// Verify some expected metrics are present.
	expectedMetrics := []string{
		"workloadguard_system_load_average_1m",
		"workloadguard_system_cpu_count",
		"workloadguard_check_total",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(bodyStr, metric) {
			t.Errorf("expected metric %q in output", metric)
		}
	}
}

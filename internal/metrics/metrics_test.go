package metrics

import (
	"testing"

	"go.uber.org/zap"
)

func TestMetrics(t *testing.T) {
	logger := zap.NewNop()
	m := New(logger)

	// Test counter increment
	m.IncrementCounter("user_logins_total", "total")

	// Test gauge set
	m.SetGauge("active_users", 100.0)

	// Test histogram observe
	m.ObserveHistogram("ai_response_time", 1.5, "english_practice")

	// Test high-level methods
	m.RecordUserLogin(123)
	m.RecordUserMessage("text")
	m.RecordAIRequest("english_practice", true, 2.0)
	m.RecordXP(123, 10, "exercise_request")
}

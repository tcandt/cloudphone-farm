package telemetry

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsActualIncrements(t *testing.T) {
	m := GetMetrics()
	m.IncrCommandsDispatched("control", "success")
	m.IncrCommandsDispatched("control", "success")
	m.IncrWebRTCSessions()
	m.IncrWebRTCReconnects("ice_grace_window_expired")
	m.IncrAgentConnections("connected")
	m.IncrRateLimitRejections("login")
	m.IncrClusterRoutedMessages("command.route.request", "success")

	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()

	handler := PrometheusHandler()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()

	if !strings.Contains(body, `commands_dispatched_total{type="control",result="success"} 2`) {
		t.Errorf("Expected commands_dispatched_total to contain count 2, got:\n%s", body)
	}

	if !strings.Contains(body, `webrtc_sessions_active 1`) {
		t.Errorf("Expected webrtc_sessions_active count 1, got:\n%s", body)
	}

	if !strings.Contains(body, `webrtc_reconnects_total{reason="ice_grace_window_expired"} 1`) {
		t.Errorf("Expected webrtc_reconnects_total count 1, got:\n%s", body)
	}

	if !strings.Contains(body, `rate_limit_rejections_total{scope="login"} 1`) {
		t.Errorf("Expected rate_limit_rejections_total count 1, got:\n%s", body)
	}

	if !strings.Contains(body, `cluster_routed_messages_total{type="command.route.request",result="success"} 1`) {
		t.Errorf("Expected cluster_routed_messages_total count 1, got:\n%s", body)
	}
}

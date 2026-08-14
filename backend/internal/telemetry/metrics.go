package telemetry

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
)

type Metrics struct {
	commandsDispatchedTotal atomic.Uint64
	webrtcSessionsActive    atomic.Int64
	webrtcReconnectsTotal   atomic.Uint64
	agentConnectionsActive  atomic.Int64
	rateLimitRejectionsTotal atomic.Uint64
	clusterRoutedMessagesTotal atomic.Uint64
	mu                      sync.Mutex
}

var globalMetrics = &Metrics{}

func GetMetrics() *Metrics {
	return globalMetrics
}

func (m *Metrics) IncrCommandsDispatched() {
	m.commandsDispatchedTotal.Add(1)
}

func (m *Metrics) IncrWebRTCSessions() {
	m.webrtcSessionsActive.Add(1)
}

func (m *Metrics) DecrWebRTCSessions() {
	m.webrtcSessionsActive.Add(-1)
}

func (m *Metrics) IncrWebRTCReconnects() {
	m.webrtcReconnectsTotal.Add(1)
}

func (m *Metrics) IncrAgentConnections() {
	m.agentConnectionsActive.Add(1)
}

func (m *Metrics) DecrAgentConnections() {
	m.agentConnectionsActive.Add(-1)
}

func (m *Metrics) IncrRateLimitRejections() {
	m.rateLimitRejectionsTotal.Add(1)
}

func (m *Metrics) IncrClusterRoutedMessages() {
	m.clusterRoutedMessagesTotal.Add(1)
}

// PrometheusHandler exposes low-cardinality metrics in OpenMetrics/Prometheus format.
func PrometheusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		
		fmt.Fprintf(w, "# HELP commands_dispatched_total Total number of commands dispatched to devices.\n")
		fmt.Fprintf(w, "# TYPE commands_dispatched_total counter\n")
		fmt.Fprintf(w, "commands_dispatched_total %d\n\n", globalMetrics.commandsDispatchedTotal.Load())

		fmt.Fprintf(w, "# HELP webrtc_sessions_active Current number of active WebRTC media sessions.\n")
		fmt.Fprintf(w, "# TYPE webrtc_sessions_active gauge\n")
		fmt.Fprintf(w, "webrtc_sessions_active %d\n\n", globalMetrics.webrtcSessionsActive.Load())

		fmt.Fprintf(w, "# HELP webrtc_reconnects_total Total number of controlled WebRTC reconnect attempts.\n")
		fmt.Fprintf(w, "# TYPE webrtc_reconnects_total counter\n")
		fmt.Fprintf(w, "webrtc_reconnects_total %d\n\n", globalMetrics.webrtcReconnectsTotal.Load())

		fmt.Fprintf(w, "# HELP agent_websocket_connections Current number of active agent WebSocket connections.\n")
		fmt.Fprintf(w, "# TYPE agent_websocket_connections gauge\n")
		fmt.Fprintf(w, "agent_websocket_connections %d\n\n", globalMetrics.agentConnectionsActive.Load())

		fmt.Fprintf(w, "# HELP rate_limit_rejections_total Total number of rate limit rejection events.\n")
		fmt.Fprintf(w, "# TYPE rate_limit_rejections_total counter\n")
		fmt.Fprintf(w, "rate_limit_rejections_total %d\n\n", globalMetrics.rateLimitRejectionsTotal.Load())

		fmt.Fprintf(w, "# HELP cluster_routed_messages_total Total number of cross-node cluster routed envelopes.\n")
		fmt.Fprintf(w, "# TYPE cluster_routed_messages_total counter\n")
		fmt.Fprintf(w, "cluster_routed_messages_total %d\n", globalMetrics.clusterRoutedMessagesTotal.Load())
	})
}

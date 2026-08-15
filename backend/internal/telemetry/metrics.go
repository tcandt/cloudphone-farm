package telemetry

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
)

type LabeledCounter struct {
	counts map[string]*atomic.Uint64
	mu     sync.RWMutex
}

func NewLabeledCounter() *LabeledCounter {
	return &LabeledCounter{
		counts: make(map[string]*atomic.Uint64),
	}
}

func (lc *LabeledCounter) Incr(labelKey string) {
	lc.mu.RLock()
	val, exists := lc.counts[labelKey]
	lc.mu.RUnlock()

	if exists {
		val.Add(1)
		return
	}

	lc.mu.Lock()
	val, exists = lc.counts[labelKey]
	if !exists {
		val = &atomic.Uint64{}
		lc.counts[labelKey] = val
	}
	lc.mu.Unlock()
	val.Add(1)
}

func (lc *LabeledCounter) Dump(w http.ResponseWriter, metricName string) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	for labels, ptr := range lc.counts {
		fmt.Fprintf(w, "%s{%s} %d\n", metricName, labels, ptr.Load())
	}
}

type Metrics struct {
	commandsDispatched    *LabeledCounter
	webrtcSessionsActive  atomic.Int64
	webrtcReconnects      *LabeledCounter
	agentConnectionsState *LabeledCounter
	rateLimitRejections   *LabeledCounter
	clusterRoutedMessages *LabeledCounter
	httpRequestDurations  *LabeledCounter
	commandLatencies      *LabeledCounter
}

var globalMetrics = &Metrics{
	commandsDispatched:    NewLabeledCounter(),
	webrtcReconnects:      NewLabeledCounter(),
	agentConnectionsState: NewLabeledCounter(),
	rateLimitRejections:   NewLabeledCounter(),
	clusterRoutedMessages: NewLabeledCounter(),
	httpRequestDurations:  NewLabeledCounter(),
	commandLatencies:      NewLabeledCounter(),
}

func GetMetrics() *Metrics {
	return globalMetrics
}

func (m *Metrics) IncrCommandsDispatched(cmdType, result string) {
	label := fmt.Sprintf(`type="%s",result="%s"`, cmdType, result)
	m.commandsDispatched.Incr(label)
}

func (m *Metrics) IncrWebRTCSessions() {
	m.webrtcSessionsActive.Add(1)
}

func (m *Metrics) DecrWebRTCSessions() {
	m.webrtcSessionsActive.Add(-1)
}

func (m *Metrics) IncrWebRTCReconnects(reason string) {
	label := fmt.Sprintf(`reason="%s"`, reason)
	m.webrtcReconnects.Incr(label)
}

func (m *Metrics) IncrAgentConnections(state string) {
	label := fmt.Sprintf(`state="%s"`, state)
	m.agentConnectionsState.Incr(label)
}

func (m *Metrics) IncrRateLimitRejections(scope string) {
	label := fmt.Sprintf(`scope="%s"`, scope)
	m.rateLimitRejections.Incr(label)
}

func (m *Metrics) IncrClusterRoutedMessages(msgType, result string) {
	label := fmt.Sprintf(`type="%s",result="%s"`, msgType, result)
	m.clusterRoutedMessages.Incr(label)
}

func (m *Metrics) RecordHTTPRequest(route, method, statusClass string) {
	label := fmt.Sprintf(`route="%s",method="%s",status_class="%s"`, route, method, statusClass)
	m.httpRequestDurations.Incr(label)
}

func (m *Metrics) RecordCommandLatency(phase string) {
	label := fmt.Sprintf(`phase="%s"`, phase)
	m.commandLatencies.Incr(label)
}

// PrometheusHandler exposes dynamic OpenMetrics metrics format.
func PrometheusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		fmt.Fprintf(w, "# HELP commands_dispatched_total Total number of commands dispatched to devices.\n")
		fmt.Fprintf(w, "# TYPE commands_dispatched_total counter\n")
		globalMetrics.commandsDispatched.Dump(w, "commands_dispatched_total")

		fmt.Fprintf(w, "\n# HELP webrtc_sessions_active Current number of active WebRTC media sessions.\n")
		fmt.Fprintf(w, "# TYPE webrtc_sessions_active gauge\n")
		fmt.Fprintf(w, "webrtc_sessions_active %d\n\n", globalMetrics.webrtcSessionsActive.Load())

		fmt.Fprintf(w, "# HELP webrtc_reconnects_total Total number of controlled WebRTC reconnect attempts.\n")
		fmt.Fprintf(w, "# TYPE webrtc_reconnects_total counter\n")
		globalMetrics.webrtcReconnects.Dump(w, "webrtc_reconnects_total")

		fmt.Fprintf(w, "\n# HELP agent_websocket_connections Current number of active agent WebSocket connections.\n")
		fmt.Fprintf(w, "# TYPE agent_websocket_connections gauge\n")
		globalMetrics.agentConnectionsState.Dump(w, "agent_websocket_connections")

		fmt.Fprintf(w, "\n# HELP rate_limit_rejections_total Total number of rate limit rejection events.\n")
		fmt.Fprintf(w, "# TYPE rate_limit_rejections_total counter\n")
		globalMetrics.rateLimitRejections.Dump(w, "rate_limit_rejections_total")

		fmt.Fprintf(w, "\n# HELP cluster_routed_messages_total Total number of cross-node cluster routed envelopes.\n")
		fmt.Fprintf(w, "# TYPE cluster_routed_messages_total counter\n")
		globalMetrics.clusterRoutedMessages.Dump(w, "cluster_routed_messages_total")

		fmt.Fprintf(w, "\n# HELP http_request_duration_seconds HTTP request duration metrics.\n")
		fmt.Fprintf(w, "# TYPE http_request_duration_seconds counter\n")
		globalMetrics.httpRequestDurations.Dump(w, "http_request_duration_seconds")

		fmt.Fprintf(w, "\n# HELP command_latency_seconds Command execution latency phase counters.\n")
		fmt.Fprintf(w, "# TYPE command_latency_seconds counter\n")
		globalMetrics.commandLatencies.Dump(w, "command_latency_seconds")
	})
}

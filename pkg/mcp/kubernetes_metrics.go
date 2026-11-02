package mcp

import (
	"log/slog"
	"time"

	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

const (
	deploymentCheckPathCached    = "cached"
	deploymentCheckPathReadiness = "readiness"
	deploymentCheckPathDeploy    = "deploy"

	// A cached check only reads the informer cache, so anything near a second is abnormal.
	slowCachedDeploymentCheckThreshold = time.Second
	// Readiness and deploy checks wait for pods, so only flag ones that take a large
	// part of a typical client request timeout.
	slowDeploymentCheckThreshold = 10 * time.Second
)

var deploymentCheckDuration = metrics.NewHistogramVec(&metrics.HistogramOpts{
	Namespace: "obot",
	Subsystem: "mcp_kubernetes",
	Name:      "deployment_check_duration_seconds",
	Help:      "Duration of the MCP server deployment check done before proxying a request, by check path and result.",
	Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
}, []string{"path", "result"})

func init() {
	legacyregistry.MustRegister(deploymentCheckDuration)
}

func recordDeploymentCheck(mcpServerName, path string, start time.Time, err error) {
	duration := time.Since(start)
	result := "success"
	if err != nil {
		result = "failure"
	}
	deploymentCheckDuration.WithLabelValues(path, result).Observe(duration.Seconds())

	threshold := slowDeploymentCheckThreshold
	if path == deploymentCheckPathCached {
		threshold = slowCachedDeploymentCheckThreshold
	}
	if duration >= threshold {
		slog.Warn("Slow MCP deployment check", "mcpServerName", mcpServerName, "path", path, "result", result, "duration", duration)
	}
}

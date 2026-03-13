package e2e

import (
	"os"
	"testing"

	"rec53/monitor"

	"go.uber.org/zap"
)

// TestMain is the package-level entry point for all e2e tests.
// It initialises the monitor singletons (Rec53Log and Rec53Metric) once,
// before any test function runs, so that server code calling
// monitor.Rec53Metric.InCounterAdd or monitor.Rec53Log.Debugf never
// encounters a nil pointer.
func TestMain(m *testing.M) {
	monitor.Rec53Log = zap.NewNop().Sugar()
	monitor.InitMetricForTest()
	os.Exit(m.Run())
}

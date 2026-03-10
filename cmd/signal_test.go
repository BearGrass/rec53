package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"rec53/monitor"

	"go.uber.org/zap"
)

func init() {
	// Initialize no-op logger for tests
	monitor.Rec53Log = zap.NewNop().Sugar()
}

// TestGracefulShutdownFunction tests the gracefulShutdown function
func TestGracefulShutdownFunction(t *testing.T) {
	tests := []struct {
		name      string
		shutdowns []shutdownFunc
		expectErr bool
	}{
		{
			name:      "nil shutdown function",
			shutdowns: []shutdownFunc{nil},
			expectErr: false,
		},
		{
			name: "successful shutdown",
			shutdowns: []shutdownFunc{func(ctx context.Context) error {
				return nil
			}},
			expectErr: false,
		},
		{
			name: "shutdown with error",
			shutdowns: []shutdownFunc{func(ctx context.Context) error {
				return fmt.Errorf("shutdown error")
			}},
			expectErr: false, // gracefulShutdown logs errors but doesn't return them
		},
		{
			name: "multiple shutdowns mixed",
			shutdowns: []shutdownFunc{
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return fmt.Errorf("error 1") },
				func(ctx context.Context) error { return nil },
			},
			expectErr: false,
		},
		{
			name:      "empty shutdowns list",
			shutdowns: []shutdownFunc{},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// gracefulShutdown doesn't return errors, just logs them
			gracefulShutdown(ctx, tt.shutdowns...)
		})
	}
}

// TestGracefulShutdownWithCanceledContext tests shutdown with canceled context
func TestGracefulShutdownWithCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	called := false
	shutdowns := []shutdownFunc{
		func(ctx context.Context) error {
			called = true
			return nil
		},
	}

	gracefulShutdown(ctx, shutdowns...)

	if !called {
		t.Error("expected shutdown function to be called even with canceled context")
	}
}

// TestWaitForSignalWithSignal tests waitForSignal receiving a signal
func TestWaitForSignalWithSignal(t *testing.T) {
	sigChan := make(chan os.Signal, 1)
	errChan := make(chan error, 1)

	// Send signal in goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		sigChan <- syscall.SIGTERM
	}()

	// Wait for signal
	result := waitForSignal(sigChan, errChan)

	if result != syscall.SIGTERM {
		t.Errorf("expected SIGTERM, got %v", result)
	}
}

// TestWaitForSignalWithServerError tests waitForSignal receiving server error
func TestWaitForSignalWithServerError(t *testing.T) {
	sigChan := make(chan os.Signal, 1)
	errChan := make(chan error, 1)

	// Send error in goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		errChan <- fmt.Errorf("server error")
	}()

	// Wait for error
	result := waitForSignal(sigChan, errChan)

	if result != nil {
		t.Errorf("expected nil signal on server error, got %v", result)
	}
}

// TestWaitForSignalWithNilError tests waitForSignal receiving nil error
func TestWaitForSignalWithNilError(t *testing.T) {
	sigChan := make(chan os.Signal, 1)
	errChan := make(chan error, 1)

	// Send nil error in goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		errChan <- nil
	}()

	// Wait for nil error
	result := waitForSignal(sigChan, errChan)

	if result != nil {
		t.Errorf("expected nil signal on nil error, got %v", result)
	}
}

// TestWaitForSignalWithClosedErrChan tests waitForSignal with closed error channel
func TestWaitForSignalWithClosedErrChan(t *testing.T) {
	sigChan := make(chan os.Signal, 1)
	errChan := make(chan error, 1)

	// Close error channel in goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(errChan)
	}()

	// Wait for closed channel (will receive zero value)
	result := waitForSignal(sigChan, errChan)

	if result != nil {
		t.Errorf("expected nil signal on closed error channel, got %v", result)
	}
}

// TestShutdownFuncType tests that shutdownFunc type works correctly
func TestShutdownFuncType(t *testing.T) {
	var f shutdownFunc

	// nil should be handled
	if f != nil {
		t.Error("expected nil shutdownFunc")
	}

	// Assign a function
	called := false
	f = func(ctx context.Context) error {
		called = true
		return nil
	}

	if f == nil {
		t.Error("expected non-nil shutdownFunc")
	}

	// Call the function
	err := f(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected function to be called")
	}
}

// TestSignalHandling_SIGINT tests that SIGINT triggers graceful shutdown
func TestSignalHandling_SIGINT(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping signal test in short mode")
	}

	// Build the binary first
	binaryPath, err := buildTestBinary(t)
	if err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove(binaryPath)

	// Find available port
	port := getAvailablePort(t)
	metricPort := getAvailablePort(t)

	// Start the process
	cmd := exec.Command(binaryPath,
		"-listen", fmt.Sprintf("127.0.0.1:%d", port),
		"-metric", fmt.Sprintf(":%d", metricPort),
		"-log-level", "error",
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Verify server is running by checking if port is in use
	if !isPortInUse(port) {
		t.Fatalf("Server did not start on port %d", port)
	}

	// Send SIGINT
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("Failed to send SIGINT: %v", err)
	}

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			// Process should exit cleanly, not with an error
			if !strings.Contains(err.Error(), "signal") {
				t.Errorf("Process exited with unexpected error: %v", err)
			}
		}
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("Process did not exit within timeout after SIGINT")
	}

	// Verify port is released
	time.Sleep(100 * time.Millisecond)
	if isPortInUse(port) {
		t.Error("Port still in use after shutdown")
	}
}

// TestSignalHandling_SIGTERM tests that SIGTERM triggers graceful shutdown
func TestSignalHandling_SIGTERM(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping signal test in short mode")
	}

	// Build the binary first
	binaryPath, err := buildTestBinary(t)
	if err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove(binaryPath)

	// Find available port
	port := getAvailablePort(t)
	metricPort := getAvailablePort(t)

	// Start the process
	cmd := exec.Command(binaryPath,
		"-listen", fmt.Sprintf("127.0.0.1:%d", port),
		"-metric", fmt.Sprintf(":%d", metricPort),
		"-log-level", "error",
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Verify server is running
	if !isPortInUse(port) {
		t.Fatalf("Server did not start on port %d", port)
	}

	// Send SIGTERM
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Failed to send SIGTERM: %v", err)
	}

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			// Process should exit cleanly
			if !strings.Contains(err.Error(), "signal") {
				t.Errorf("Process exited with unexpected error: %v", err)
			}
		}
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("Process did not exit within timeout after SIGTERM")
	}

	// Verify port is released
	time.Sleep(100 * time.Millisecond)
	if isPortInUse(port) {
		t.Error("Port still in use after shutdown")
	}
}

// TestGracefulShutdown tests the graceful shutdown with context timeout
func TestGracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping graceful shutdown test in short mode")
	}

	// Build the binary first
	binaryPath, err := buildTestBinary(t)
	if err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove(binaryPath)

	// Find available port
	port := getAvailablePort(t)
	metricPort := getAvailablePort(t)

	// Start the process
	cmd := exec.Command(binaryPath,
		"-listen", fmt.Sprintf("127.0.0.1:%d", port),
		"-metric", fmt.Sprintf(":%d", metricPort),
		"-log-level", "error",
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Verify both DNS and metrics servers are running
	if !isPortInUse(port) {
		t.Fatalf("DNS server did not start on port %d", port)
	}
	if !isPortInUse(metricPort) {
		t.Fatalf("Metrics server did not start on port %d", metricPort)
	}

	// Send shutdown signal
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Failed to send SIGTERM: %v", err)
	}

	// Measure shutdown time
	start := time.Now()

	// Wait for process to exit
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		shutdownTime := time.Since(start)
		t.Logf("Shutdown completed in %v", shutdownTime)

		// Graceful shutdown should complete within 5 seconds (the timeout in main)
		if shutdownTime > 6*time.Second {
			t.Errorf("Shutdown took too long: %v (expected < 6s)", shutdownTime)
		}
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("Process did not exit within timeout")
	}

	// Verify both ports are released
	time.Sleep(100 * time.Millisecond)
	if isPortInUse(port) {
		t.Error("DNS port still in use after shutdown")
	}
	if isPortInUse(metricPort) {
		t.Error("Metrics port still in use after shutdown")
	}
}

// TestVersionFlag tests the -version flag
func TestVersionFlag(t *testing.T) {
	// Build the binary first
	binaryPath, err := buildTestBinary(t)
	if err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove(binaryPath)

	// Run with -version flag
	cmd := exec.Command(binaryPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run with -version: %v", err)
	}

	// Check output contains version info
	outputStr := string(output)
	if !strings.Contains(outputStr, "rec53 version:") {
		t.Errorf("Expected version output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "built:") {
		t.Errorf("Expected build time in output, got: %s", outputStr)
	}
}

// TestLogLevelFlag tests the -log-level flag
func TestLogLevelFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	// Build the binary first
	binaryPath, err := buildTestBinary(t)
	if err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove(binaryPath)

	// Find available port
	port := getAvailablePort(t)
	metricPort := getAvailablePort(t)

	// Start with invalid log level (should default to info)
	cmd := exec.Command(binaryPath,
		"-listen", fmt.Sprintf("127.0.0.1:%d", port),
		"-metric", fmt.Sprintf(":%d", metricPort),
		"-log-level", "invalid",
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Verify server is running (invalid log level should not prevent startup)
	if !isPortInUse(port) {
		t.Error("Server did not start with invalid log level")
	}

	// Cleanup
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()
}

// buildTestBinary builds the rec53 binary for testing
func buildTestBinary(t *testing.T) (string, error) {
	// Create temp file path for the binary
	binaryPath := fmt.Sprintf("/tmp/rec53_test_%d", time.Now().UnixNano())

	// Build the binary from project root
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd")
	buildCmd.Dir = getProjectRoot()

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build failed: %v, output: %s", err, output)
	}

	return binaryPath, nil
}

// getProjectRoot returns the project root directory
func getProjectRoot() string {
	// Get the directory of this test file
	_, filename, _, _ := runtime.Caller(0)
	dir := strings.TrimSuffix(filename, "/cmd/signal_test.go")
	return dir
}

// getAvailablePort finds an available port on localhost
func getAvailablePort(t *testing.T) int {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to resolve TCP addr: %v", err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port
}

// isPortInUse checks if a port is in use
func isPortInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// TestGracefulShutdownWithContext tests context-based shutdown behavior
func TestGracefulShutdownWithContext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	// Build the binary first
	binaryPath, err := buildTestBinary(t)
	if err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove(binaryPath)

	// Find available port
	port := getAvailablePort(t)
	metricPort := getAvailablePort(t)

	// Start the process
	cmd := exec.Command(binaryPath,
		"-listen", fmt.Sprintf("127.0.0.1:%d", port),
		"-metric", fmt.Sprintf(":%d", metricPort),
		"-log-level", "debug",
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Create a context with timeout for cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send shutdown signal
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Failed to send SIGTERM: %v", err)
	}

	// Wait for process to exit or context timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		cmd.Process.Kill()
		t.Fatal("Context timeout - process did not exit")
	case err := <-done:
		if err != nil {
			t.Logf("Process exited: %v", err)
		}
	}
}
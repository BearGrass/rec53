package main

import (
	"testing"

	"go.uber.org/zap"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zap.AtomicLevel
	}{
		{"debug", zap.NewAtomicLevelAt(zap.DebugLevel)},
		{"info", zap.NewAtomicLevelAt(zap.InfoLevel)},
		{"warn", zap.NewAtomicLevelAt(zap.WarnLevel)},
		{"error", zap.NewAtomicLevelAt(zap.ErrorLevel)},
		{"unknown", zap.NewAtomicLevelAt(zap.InfoLevel)}, // default case
		{"", zap.NewAtomicLevelAt(zap.InfoLevel)},         // empty string
		// Case-insensitive tests
		{"DEBUG", zap.NewAtomicLevelAt(zap.DebugLevel)},
		{"INFO", zap.NewAtomicLevelAt(zap.InfoLevel)},
		{"WARN", zap.NewAtomicLevelAt(zap.WarnLevel)},
		{"ERROR", zap.NewAtomicLevelAt(zap.ErrorLevel)},
		{"DeBuG", zap.NewAtomicLevelAt(zap.DebugLevel)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			if result.Level() != tt.expected.Level() {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, result.Level(), tt.expected.Level())
			}
		})
	}
}

func TestParseLogLevelConcurrency(t *testing.T) {
	// Test that parseLogLevel is safe for concurrent use
	levels := []string{"debug", "info", "warn", "error", "unknown"}
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for _, level := range levels {
				_ = parseLogLevel(level)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
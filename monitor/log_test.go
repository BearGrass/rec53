package monitor

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestInitLogger(t *testing.T) {
	// InitLogger creates a logger - we need to be careful about file creation
	// Skip actual file creation in tests
	t.Run("atomicLevel initial value", func(t *testing.T) {
		// Verify atomicLevel is initialized
		level := atomicLevel.Level()
		if level != zapcore.DebugLevel {
			t.Errorf("Expected initial level to be DebugLevel, got %v", level)
		}
	})
}

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		level     zapcore.Level
		wantLevel zapcore.Level
	}{
		{
			name:      "debug level",
			level:     zapcore.DebugLevel,
			wantLevel: zapcore.DebugLevel,
		},
		{
			name:      "info level",
			level:     zapcore.InfoLevel,
			wantLevel: zapcore.InfoLevel,
		},
		{
			name:      "warn level",
			level:     zapcore.WarnLevel,
			wantLevel: zapcore.WarnLevel,
		},
		{
			name:      "error level",
			level:     zapcore.ErrorLevel,
			wantLevel: zapcore.ErrorLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLogLevel(tt.level)
			gotLevel := atomicLevel.Level()
			if gotLevel != tt.wantLevel {
				t.Errorf("SetLogLevel(%v) = %v, want %v", tt.level, gotLevel, tt.wantLevel)
			}
		})
	}

	// Reset to debug level after tests
	SetLogLevel(zapcore.DebugLevel)
}

func TestAtomicLevel(t *testing.T) {
	// Test that atomicLevel can be modified
	initialLevel := atomicLevel.Level()
	t.Logf("Initial level: %v", initialLevel)

	// Change level
	atomicLevel.SetLevel(zapcore.InfoLevel)
	newLevel := atomicLevel.Level()
	if newLevel != zapcore.InfoLevel {
		t.Errorf("Expected InfoLevel, got %v", newLevel)
	}

	// Restore initial level
	atomicLevel.SetLevel(initialLevel)
}

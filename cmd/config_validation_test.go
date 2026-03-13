package main

import (
	"testing"
	"time"

	"rec53/server"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			cfg:     &Config{DNS: DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999", LogLevel: "info"}, Warmup: server.WarmupConfig{Enabled: true, Timeout: 5 * time.Second}},
			wantErr: false,
		},
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: true,
			errMsg:  "configuration is nil",
		},
		{
			name:    "empty listen address",
			cfg:     &Config{DNS: DNSConfig{Listen: "", Metric: ":9999"}, Warmup: server.WarmupConfig{}},
			wantErr: true,
			errMsg:  "dns.listen address is required",
		},
		{
			name:    "empty metric address",
			cfg:     &Config{DNS: DNSConfig{Listen: "127.0.0.1:5353", Metric: ""}, Warmup: server.WarmupConfig{}},
			wantErr: true,
			errMsg:  "dns.metric address is required",
		},
		{
			name:    "invalid listen address",
			cfg:     &Config{DNS: DNSConfig{Listen: "invalid:address:format", Metric: ":9999"}, Warmup: server.WarmupConfig{}},
			wantErr: true,
			errMsg:  "invalid dns.listen address",
		},
		{
			name:    "invalid metric port",
			cfg:     &Config{DNS: DNSConfig{Listen: "127.0.0.1:5353", Metric: ":99999"}, Warmup: server.WarmupConfig{}},
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},
		{
			name:    "invalid warmup timeout",
			cfg:     &Config{DNS: DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"}, Warmup: server.WarmupConfig{Timeout: 50 * time.Millisecond}},
			wantErr: true,
			errMsg:  "warmup.timeout must be at least 100ms",
		},
		{
			name:    "valid full address metric",
			cfg:     &Config{DNS: DNSConfig{Listen: "127.0.0.1:5353", Metric: "127.0.0.1:9999"}, Warmup: server.WarmupConfig{}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("validateConfig() error = %v, expected to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

package main

import (
	"os"
	"path/filepath"
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
		{
			name:    "invalid log level",
			cfg:     &Config{DNS: DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999", LogLevel: "verbose"}, Warmup: server.WarmupConfig{}},
			wantErr: true,
			errMsg:  "dns.log_level must be one of",
		},
		{
			name: "pprof enabled with invalid listen address",
			cfg: &Config{
				DNS:   DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999", LogLevel: "info"},
				Debug: DebugConfig{PprofEnabled: true, PprofListen: "bad-addr"},
			},
			wantErr: true,
			errMsg:  "invalid debug.pprof_listen address",
		},
		// Hosts validation
		{
			name: "valid hosts entry A",
			cfg: &Config{
				DNS:   DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				Hosts: []server.HostEntry{{Name: "test.local", Type: "A", Value: "10.0.0.1"}},
			},
			wantErr: false,
		},
		{
			name: "valid hosts entry AAAA",
			cfg: &Config{
				DNS:   DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				Hosts: []server.HostEntry{{Name: "test.local", Type: "AAAA", Value: "::1"}},
			},
			wantErr: false,
		},
		{
			name: "valid hosts entry CNAME",
			cfg: &Config{
				DNS:   DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				Hosts: []server.HostEntry{{Name: "alias.local", Type: "CNAME", Value: "real.local"}},
			},
			wantErr: false,
		},
		{
			name: "hosts empty name",
			cfg: &Config{
				DNS:   DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				Hosts: []server.HostEntry{{Name: "", Type: "A", Value: "10.0.0.1"}},
			},
			wantErr: true,
			errMsg:  "name must not be empty",
		},
		{
			name: "hosts empty value",
			cfg: &Config{
				DNS:   DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				Hosts: []server.HostEntry{{Name: "test.local", Type: "A", Value: ""}},
			},
			wantErr: true,
			errMsg:  "value must not be empty",
		},
		{
			name: "hosts invalid IPv4 for A record",
			cfg: &Config{
				DNS:   DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				Hosts: []server.HostEntry{{Name: "test.local", Type: "A", Value: "not-an-ip"}},
			},
			wantErr: true,
			errMsg:  "invalid IPv4 address",
		},
		{
			name: "hosts invalid IPv6 for AAAA record",
			cfg: &Config{
				DNS:   DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				Hosts: []server.HostEntry{{Name: "test.local", Type: "AAAA", Value: "not-an-ipv6"}},
			},
			wantErr: true,
			errMsg:  "invalid IPv6 address",
		},
		{
			name: "hosts unsupported record type",
			cfg: &Config{
				DNS:   DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				Hosts: []server.HostEntry{{Name: "test.local", Type: "MX", Value: "mail.local"}},
			},
			wantErr: true,
			errMsg:  "unsupported record type",
		},
		// Forwarding validation
		{
			name: "valid forwarding entry",
			cfg: &Config{
				DNS:        DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				Forwarding: []server.ForwardZone{{Zone: "corp.internal", Upstreams: []string{"10.0.0.1:53"}}},
			},
			wantErr: false,
		},
		{
			name: "forwarding empty zone",
			cfg: &Config{
				DNS:        DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				Forwarding: []server.ForwardZone{{Zone: "", Upstreams: []string{"10.0.0.1:53"}}},
			},
			wantErr: true,
			errMsg:  "zone must not be empty",
		},
		{
			name: "forwarding empty upstreams",
			cfg: &Config{
				DNS:        DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				Forwarding: []server.ForwardZone{{Zone: "corp.internal", Upstreams: []string{}}},
			},
			wantErr: true,
			errMsg:  "upstreams list must not be empty",
		},
		{
			name: "forwarding invalid upstream address",
			cfg: &Config{
				DNS:        DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				Forwarding: []server.ForwardZone{{Zone: "corp.internal", Upstreams: []string{"bad-address"}}},
			},
			wantErr: true,
			errMsg:  "invalid address",
		},
		// XDP config validation
		{
			name: "xdp disabled is valid",
			cfg: &Config{
				DNS: DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				XDP: XDPConfig{Enabled: false},
			},
			wantErr: false,
		},
		{
			name: "xdp enabled with interface is valid",
			cfg: &Config{
				DNS: DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				XDP: XDPConfig{Enabled: true, Interface: "eth0"},
			},
			wantErr: false,
		},
		{
			name: "xdp enabled without interface is invalid",
			cfg: &Config{
				DNS: DNSConfig{Listen: "127.0.0.1:5353", Metric: ":9999"},
				XDP: XDPConfig{Enabled: true, Interface: ""},
			},
			wantErr: true,
			errMsg:  "xdp.interface is required",
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

func TestLoadConfig(t *testing.T) {
	t.Run("missing file returns actionable error", func(t *testing.T) {
		_, err := loadConfig("/tmp/rec53-does-not-exist.yaml")
		if err == nil {
			t.Fatal("expected error for missing config file")
		}
		if !contains(err.Error(), "Config file not found") {
			t.Fatalf("expected missing-file guidance, got: %v", err)
		}
	})

	t.Run("applies defaults for pprof listen and warmup concurrency", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.yaml")
		content := []byte(`
dns:
  listen: "127.0.0.1:5353"
  metric: ":9999"
  log_level: "info"
warmup:
  enabled: true
`)
		if err := os.WriteFile(configPath, content, 0o644); err != nil {
			t.Fatalf("failed to write temp config: %v", err)
		}

		cfg, err := loadConfig(configPath)
		if err != nil {
			t.Fatalf("loadConfig() error = %v", err)
		}
		if cfg.Debug.PprofListen != "127.0.0.1:6060" {
			t.Fatalf("expected default pprof listen, got %q", cfg.Debug.PprofListen)
		}
		if cfg.Warmup.Concurrency != server.DefaultWarmupConfig.Concurrency {
			t.Fatalf("expected default warmup concurrency %d, got %d", server.DefaultWarmupConfig.Concurrency, cfg.Warmup.Concurrency)
		}
	})
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

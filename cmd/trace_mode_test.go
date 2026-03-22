package main

import (
	"bytes"
	"strings"
	"testing"

	"rec53/server"

	"github.com/miekg/dns"
)

func TestRunTraceModeWritesOrderedTrace(t *testing.T) {
	var out bytes.Buffer

	cfg := &Config{
		DNS: DNSConfig{
			Listen:   "127.0.0.1:5353",
			Metric:   ":9999",
			LogLevel: "error",
		},
		Hosts: []server.HostEntry{
			{Name: "trace.test", Type: "A", Value: "10.0.0.8"},
		},
	}

	if err := runTraceMode(&out, cfg, "trace.test", dns.TypeA, 0); err != nil {
		t.Fatalf("runTraceMode: %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"query: trace.test. A",
		"state_init",
		"hosts_lookup",
		"success_exit",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("trace output missing %q\n%s", want, text)
		}
	}
}

func TestParseTraceQTypeRejectsUnknownType(t *testing.T) {
	if _, err := parseTraceQType("not-a-real-type"); err == nil {
		t.Fatal("expected parseTraceQType to reject unknown type")
	}
}

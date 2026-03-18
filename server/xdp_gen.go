package server

// Generate Go bindings from the XDP eBPF program dns_cache.c.
// Prerequisites: clang >= 14, linux headers, bpf2go (go install github.com/cilium/ebpf/cmd/bpf2go@latest)
//
// Run: go generate ./server/
//
// NOTE: The generate-bpf.sh script handles architecture-specific include paths
// (asm headers, libbpf) so the go:generate directive remains portable.
//go:generate bash ./xdp/generate-bpf.sh

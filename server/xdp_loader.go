package server

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"

	"rec53/monitor"
)

// XDPLoader manages the lifecycle of the XDP/eBPF DNS cache program:
// loading eBPF objects, attaching to a network interface, and cleanup.
type XDPLoader struct {
	iface   string
	objs    *dnsCacheObjects
	xdpLink link.Link
}

// NewXDPLoader creates a new XDP loader for the given network interface.
func NewXDPLoader(iface string) *XDPLoader {
	return &XDPLoader{iface: iface}
}

// LoadAndAttach loads the eBPF program and attaches it to the configured
// network interface. It tries native XDP mode first, then falls back to
// generic mode.
func (l *XDPLoader) LoadAndAttach() error {
	// Load bpf2go-generated eBPF objects.
	objs := &dnsCacheObjects{}
	if err := loadDnsCacheObjects(objs, nil); err != nil {
		return fmt.Errorf("[XDP] failed to load eBPF objects: %w", err)
	}

	// cleanup closes eBPF objects if set (non-nil). Cleared on success to
	// prevent double-close.
	cleanup := objs
	defer func() {
		if cleanup != nil {
			cleanup.Close()
		}
	}()

	// Resolve interface index.
	iface, err := net.InterfaceByName(l.iface)
	if err != nil {
		return fmt.Errorf("[XDP] interface %q not found: %w", l.iface, err)
	}

	// Try native XDP mode first.
	xdpLink, err := link.AttachXDP(link.XDPOptions{
		Program:   objs.XdpDnsCache,
		Interface: iface.Index,
		Flags:     link.XDPDriverMode,
	})
	if err == nil {
		l.objs = objs
		l.xdpLink = xdpLink
		cleanup = nil // prevent deferred Close
		monitor.Rec53Log.Infof("[XDP] attached to %s in native mode", l.iface)
		return nil
	}

	nativeErr := err
	monitor.Rec53Log.Debugf("[XDP] native mode attach failed for %s: %v, trying generic mode", l.iface, nativeErr)

	// Fall back to generic (SKB) mode.
	xdpLink, err = link.AttachXDP(link.XDPOptions{
		Program:   objs.XdpDnsCache,
		Interface: iface.Index,
		Flags:     link.XDPGenericMode,
	})
	if err != nil {
		return fmt.Errorf("[XDP] failed to attach to %s (native: %v, generic: %v)", l.iface, nativeErr, err)
	}

	l.objs = objs
	l.xdpLink = xdpLink
	cleanup = nil // prevent deferred Close
	monitor.Rec53Log.Infof("[XDP] attached to %s in generic mode (native not supported)", l.iface)
	return nil
}

// CacheMap returns the BPF cache_map handle, or nil if not loaded.
func (l *XDPLoader) CacheMap() *ebpf.Map {
	if l.objs == nil {
		return nil
	}
	return l.objs.CacheMap
}

// Close detaches the XDP program and closes all BPF objects.
// Safe to call multiple times or before LoadAndAttach.
func (l *XDPLoader) Close() error {
	var errs []error
	if l.xdpLink != nil {
		if err := l.xdpLink.Close(); err != nil {
			errs = append(errs, fmt.Errorf("detach XDP link: %w", err))
		}
		l.xdpLink = nil
	}
	if l.objs != nil {
		if err := l.objs.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close eBPF objects: %w", err))
		}
		l.objs = nil
	}
	if len(errs) > 0 {
		return fmt.Errorf("[XDP] close: %v", errs)
	}
	return nil
}

// XDPStatsForTest reads the per-CPU xdp_stats map and returns aggregated
// (hit, miss, pass, error) counts. Only for use in tests.
func (l *XDPLoader) XDPStatsForTest() (hit, miss, pass, xerr uint64, err error) {
	if l.objs == nil {
		return 0, 0, 0, 0, fmt.Errorf("XDP not loaded")
	}
	ptrs := [4]*uint64{&hit, &miss, &pass, &xerr}
	for i, ptr := range ptrs {
		key := uint32(i)
		var perCPU []uint64
		if err := l.objs.XdpStats.Lookup(key, &perCPU); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("stats lookup key %d: %w", i, err)
		}
		for _, v := range perCPU {
			*ptr += v
		}
	}
	return
}

// classifyXDPError examines the error returned by LoadAndAttach and returns
// an actionable hint for the operator. Used by the server degradation path
// to provide clear guidance on why XDP is unavailable.
func classifyXDPError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()

	// Permission errors: EPERM from BPF syscall or XDP attach.
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) ||
		strings.Contains(s, "permission denied") || strings.Contains(s, "operation not permitted") {
		return "hint: run as root or grant CAP_BPF+CAP_NET_ADMIN capabilities"
	}

	// Interface not found.
	if strings.Contains(s, "interface") && strings.Contains(s, "not found") {
		return "hint: verify xdp.interface in config matches an existing network interface (ip link show)"
	}

	// eBPF object load failure (kernel too old, BPF verifier error).
	if strings.Contains(s, "failed to load eBPF objects") {
		return "hint: requires Linux kernel >= 5.15 with CONFIG_BPF=y and CONFIG_XDP_SOCKETS=y"
	}

	// Attach failure (driver doesn't support XDP).
	if strings.Contains(s, "failed to attach") {
		return "hint: the network interface driver may not support XDP; check dmesg for details"
	}

	return "hint: see error details above; check kernel version, capabilities, and interface name"
}

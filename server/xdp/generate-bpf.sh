#!/usr/bin/env bash
# generate-bpf.sh — portable bpf2go invocation with auto-detected include paths.
# Called by: go generate ./server/
# Working directory: server/  (set by go generate)
set -euo pipefail

# Detect architecture-specific include path for asm/ headers.
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH_INCLUDE="/usr/include/x86_64-linux-gnu" ;;
    aarch64) ARCH_INCLUDE="/usr/include/aarch64-linux-gnu" ;;
    *)       ARCH_INCLUDE="" ;;
esac

# Detect libbpf include path: prefer system, fall back to kernel headers.
if [ -f /usr/include/bpf/bpf_helpers.h ]; then
    BPF_INCLUDE=""
else
    KVER=$(uname -r)
    KPATH="/usr/src/linux-headers-${KVER}/tools/bpf/resolve_btfids/libbpf/include"
    if [ -f "${KPATH}/bpf/bpf_helpers.h" ]; then
        BPF_INCLUDE="-I${KPATH}"
    else
        echo "ERROR: Cannot find bpf_helpers.h — install libbpf-dev or linux-headers-${KVER}" >&2
        exit 1
    fi
fi

CFLAGS="-O2 -g -Wall -Werror"
[ -n "$ARCH_INCLUDE" ] && CFLAGS="$CFLAGS -I${ARCH_INCLUDE}"
[ -n "$BPF_INCLUDE" ]  && CFLAGS="$CFLAGS ${BPF_INCLUDE}"

exec go run github.com/cilium/ebpf/cmd/bpf2go \
    -cc clang \
    -cflags "$CFLAGS" \
    -type cache_key \
    -type cache_value \
    -output-dir . \
    dnsCache ./xdp/dns_cache.c

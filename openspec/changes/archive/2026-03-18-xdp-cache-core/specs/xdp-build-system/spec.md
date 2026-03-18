## ADDED Requirements

### Requirement: Shared header definitions
`dns_cache.h` SHALL 定义 eBPF 程序和 Go 侧共用的结构体：`cache_key`（qname[MAX_QNAME_LEN] + qtype）、`cache_value`（expire_ts + resp_len + response[MAX_DNS_RESPONSE_LEN]）、stats index 常量（STAT_HIT, STAT_MISS, STAT_PASS, STAT_ERROR）。

#### Scenario: Header included by eBPF program
- **WHEN** `dns_cache.c` include `dns_cache.h`
- **THEN** 编译 SHALL 成功且结构体定义可用

#### Scenario: Constants consistent
- **WHEN** `MAX_QNAME_LEN` 定义为 255，`MAX_DNS_RESPONSE_LEN` 定义为 512
- **THEN** 所有引用这些常量的代码 SHALL 使用相同的值

### Requirement: BPF compilation with clang
`Makefile` SHALL 使用 `clang -target bpf` 将 `dns_cache.c` 编译为 eBPF 对象文件（`.o`）。编译目标 MUST 支持 little-endian（`bpfel`）和 big-endian（`bpfeb`）两种架构。

#### Scenario: Successful compilation
- **WHEN** 执行 `make` 且 clang >= 14 可用
- **THEN** SHALL 生成 `dns_cache_bpfel.o` 和 `dns_cache_bpfeb.o`

#### Scenario: Missing clang
- **WHEN** 系统未安装 clang
- **THEN** `make` SHALL 失败并输出明确的错误信息指导安装

### Requirement: bpf2go code generation
构建系统 SHALL 使用 cilium/ebpf 的 `bpf2go` 工具从编译后的 eBPF 对象生成 Go 绑定代码。生成的文件 SHALL 通过 `//go:generate` 指令触发，并提交到仓库（CI 不需要 clang）。

#### Scenario: Go generate produces bindings
- **WHEN** 执行 `go generate ./server/xdp/...`
- **THEN** SHALL 生成 `dnscache_bpfel.go`、`dnscache_bpfeb.go` 及对应的 `.o` 嵌入文件

#### Scenario: Generated code compiles
- **WHEN** 生成的 Go 绑定代码与项目一起编译
- **THEN** `go build ./...` SHALL 成功
- **AND** eBPF 对象 SHALL 嵌入最终 binary（无需额外文件部署）

### Requirement: go.mod dependency
`go.mod` SHALL 添加 `github.com/cilium/ebpf` 依赖。版本 SHALL 支持 Linux kernel >= 5.15 的 BPF 特性。

#### Scenario: Dependency resolution
- **WHEN** 执行 `go mod tidy`
- **THEN** `github.com/cilium/ebpf` SHALL 在 `go.mod` 中出现
- **AND** `go build ./...` SHALL 成功

### Requirement: Skeleton eBPF program verification
作为构建系统的 checkpoint，一个最小化的 eBPF 骨架程序（全 `XDP_PASS`）MUST 能加载到 loopback interface，且 `bpftool prog show` 可见。

#### Scenario: Skeleton loads on loopback
- **WHEN** 骨架 eBPF 程序（仅返回 `XDP_PASS`）编译并加载到 `lo`
- **THEN** `bpftool prog show` SHALL 显示该 XDP 程序
- **AND** 网络流量 SHALL 不受影响（全透传）

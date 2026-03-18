## 1. 构建系统 + 骨架

- [x] 1.1 创建 `server/xdp/` 目录，编写 `dns_cache.h` 共享头文件（`cache_key`、`cache_value` 结构体、`MAX_QNAME_LEN=255`、`MAX_DNS_RESPONSE_LEN=512`、stats index 常量）
- [x] 1.2 编写 `server/xdp/Makefile`（clang `-target bpf` 编译规则，支持 bpfel/bpfeb 双架构）
- [x] 1.3 编写骨架 eBPF 程序 `server/xdp/dns_cache.c`（仅 `XDP_PASS`，include header，定义两个 BPF maps）
- [x] 1.4 `go.mod` 添加 `github.com/cilium/ebpf` 依赖，配置 `//go:generate` bpf2go 指令
- [x] 1.5 验证骨架：编译 eBPF 对象 → bpf2go 生成 Go 绑定 → `go build ./...` 成功

## 2. 完整 eBPF 程序

- [x] 2.1 实现 ETH/IPv4/UDP/DNS header 解析，端口 53 识别，非匹配流量 `XDP_PASS`
- [x] 2.2 实现 DNS header 验证（QDCOUNT==1、QR==0），非标准查询 `XDP_PASS`
- [x] 2.3 实现 `extract_qname()` — bounded loop qname 提取 + inline lowercase，MAX_QNAME_LEN 限制
- [x] 2.4 实现 BPF map cache 查找 + expire_ts 过期检查（`bpf_ktime_get_ns() / 1e9`）
- [x] 2.5 实现 XDP_TX 响应构建：swap ETH/IP/UDP header → `bpf_xdp_adjust_tail()` → memcpy 响应 → patch Transaction ID → IP TTL=64 → IP checksum → UDP checksum=0
- [x] 2.6 实现 resp_len bounds check（`<= MAX_DNS_RESPONSE_LEN`），超限 `XDP_PASS`
- [x] 2.7 实现 per-CPU stats 计数器更新（hit/miss/pass/error），所有代码路径覆盖
- [x] 2.8 编译验证：clang 编译成功，无 verifier 警告

## 3. Go Loader

- [x] 3.1 创建 `server/xdp_loader.go` — 使用 cilium/ebpf 加载 bpf2go 生成的 eBPF 对象
- [x] 3.2 实现 XDP attach：先尝试 native mode，失败回退 generic mode，记录实际模式日志
- [x] 3.3 实现生命周期管理：context 取消时 detach XDP + 关闭所有 BPF objects
- [x] 3.4 暴露 `CacheMap()` 和 `StatsMap()` 方法，返回 `*ebpf.Map` handle

## 4. Cache 同步（Go → BPF map）

- [x] 4.1 创建 `server/xdp_sync.go` — 实现 `domainToWireFormat()` presentation → wire format 转换 + 小写归一化
- [x] 4.2 实现 `syncToBPFMap()` — Pack() 预序列化 + resp_len 校验（>512 跳过）+ monotonic clock expire_ts 计算 + BPF map Update
- [x] 4.3 修改 `server/cache.go` — `setCacheCopy()` / `setCacheCopyByType()` 写入后调用 `syncToBPFMap()`（XDP 启用时）
- [x] 4.4 实现全局 XDP sync 开关：包级变量持有 `*ebpf.Map` handle（nil 时跳过同步）

## 5. 配置接入

- [x] 5.1 `cmd/rec53.go` — 添加 `XDPConfig` 结构体（`Enabled bool`、`Interface string`）+ YAML 解析 + 校验（启用时 interface 必填）
- [x] 5.2 `generate-config.sh` — 添加 `xdp:` 配置块（默认 `enabled: false`）+ 注释说明
- [x] 5.3 `config.yaml` 示例更新

## 6. Server 集成

- [x] 6.1 修改 `server/server.go` — `Run()` 中 XDP 启用时在 DNS listener 启动前初始化 XDP loader
- [x] 6.2 修改 `server/server.go` — `Shutdown()` 中关闭 DNS listener 后 detach XDP + 关闭 BPF objects
- [x] 6.3 `NewServerWithFullConfig` 签名增加 XDP 配置参数，传递到 loader 和 cache sync

## 7. 验证

- [x] 7.1 `go build ./...` 编译通过
- [x] 7.2 `go test -race ./...` 全部通过且无竞争（XDP 默认关闭，不影响现有测试）
- [x] 7.3 手动测试：启用 XDP → `dig @127.0.0.1` 查询已缓存域名 → 确认 XDP_TX 路径工作
- [x] 7.4 手动测试：BPF stats map 读取确认 hit 计数递增
- [x] 7.5 手动测试：cache miss 透传到 Go 正常解析
- [x] 7.6 所有现有 e2e 测试通过（XDP 透传不影响功能）

## 8. 文档

- [x] 8.1 `docs/architecture.md` — 新增 XDP Cache 层描述
- [x] 8.2 `README.md` / `README.zh.md` — XDP 配置说明、构建依赖（clang >= 14）、运行时要求（kernel >= 5.15、CAP_BPF）
- [x] 8.3 `.rec53/ROADMAP.md` — 标记 v0.6.0 为已完成，更新版本历史

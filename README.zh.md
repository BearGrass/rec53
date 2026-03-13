# rec53

用 Go 实现的迭代 DNS 解析器，采用状态机架构，内置 IP 质量追踪和 Prometheus 指标。

[English](README.md) | 中文

## 功能特性

- **完整迭代解析** — 从根服务器出发逐级解析，不依赖上游转发
- **UDP/TCP 双协议** — 同一端口同时监听 UDP 和 TCP
- **状态机架构** — 清晰可审计的 7 状态解析流水线
- **IPQualityV2** — 基于滑动窗口的延迟直方图，支持自动故障恢复
- **基于 TTL 的缓存** — 深拷贝安全缓存，支持否定缓存（NXDOMAIN/NODATA）
- **NS 预热** — 启动时预填充 IP 池，降低冷启动延迟
- **Prometheus 指标** — 每次查询和每个 NS 服务器均可观测
- **优雅关闭** — 基于 context 的取消机制，5 秒超时

---

## 快速开始

```bash
# 构建
go build -o rec53 ./cmd

# 生成默认配置（首次运行）
./generate-config.sh

# 使用配置文件运行
./rec53 --config ./config.yaml

# 带参数覆盖运行
./rec53 --config ./config.yaml -listen 0.0.0.0:53 -metric :9099 -log-level debug

# 测试解析
dig @127.0.0.1 -p 5353 google.com
dig @127.0.0.1 -p 5353 google.com AAAA
```

---

## CLI 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--config` | *(必填)* | YAML 配置文件路径 |
| `-listen` | `127.0.0.1:5353` | DNS 监听地址（覆盖配置文件） |
| `-metric` | `:9999` | Prometheus 指标地址（覆盖配置文件） |
| `-log-level` | `info` | 日志级别：`debug`、`info`、`warn`、`error` |
| `-no-warmup` | `false` | 禁用启动时 NS 预热 |
| `-version` | `false` | 打印版本号后退出 |

CLI 参数优先级高于配置文件。

---

## 配置文件

```yaml
dns:
  listen: "127.0.0.1:5353"
  metric: ":9999"
  log_level: "info"

warmup:
  enabled: true
  timeout: 5s        # 预热阶段每次查询的超时时间
  duration: 5s       # 预热总时间预算
  concurrency: 0     # 0 = 自动（min(NumCPU*2, 8)）；>0 = 手动指定
  tlds:              # 留空则使用内置 30 个 TLD 默认列表
    - com
    - net
    - org
```

### 预热 TLD 列表

默认预热 30 个高流量 TLD，覆盖全球 85%+ 的域名注册量：

- **第一梯队**（8 个）：`.com`、`.cn`、`.de`、`.net`、`.org`、`.uk`、`.ru`、`.nl`
- **第二梯队**（22 个）：主要国家顶级域（`.br`、`.au`、`.in`、`.us`、`.fr`、`.it` 等）以及战略性通用顶级域（`.io`、`.ai`、`.app`、`.xyz` 等）

如需自定义列表，请在 `warmup.tlds` 中指定；留空则使用内置默认值。

---

## 规格指标

> 以下延迟数据均在 Intel i7-1165G7 @ 2.80GHz / Linux 上实测。
> 网络 benchmark 基于中国大陆家庭/办公室网络环境下的真实迭代解析结果。
> 你的硬件和网络条件不同，结果会有差异 —— 参见[自定义 benchmark](#自定义-benchmark)自行复现。

### 首包解析延迟（真实网络，3 次均值）

以下三个场景展示了从最差到最优的解析路径。
数据反映了 **Happy Eyeballs** 优化（双上游并发竞速）和 **1.5 s 上游超时**（从 5 s 降低）的效果：

| 域名 | 冷启动（无 warmup） | 首包（warmup 后） | 缓存命中 |
|------|-------------------|-----------------|---------|
| `www.qq.com` | ~826 ms | ~717 ms | ~0.12 ms |
| `www.baidu.com` | ~423 ms | ~488 ms | ~0.10 ms |
| `www.taobao.com` | ~563 ms | ~610 ms | ~0.06 ms |

- **冷启动** — IP 池为空，解析器对所有 NS 无任何延迟先验数据，是绝对最差情况。`www.qq.com` 冷启动延迟从上个版本的 ~2,500 ms 降至 ~826 ms，主要得益于 Happy Eyeballs 双路并发机制。
- **warmup 后首包** — 默认 warmup 为 `.com` 顶级域 NS 预填充 RTT 数据，使 NS 选择更精准。
- **缓存命中** — 已解析过的域名从内存直接返回，延迟降至 **< 0.2 ms**，比迭代解析快 1,000–10,000 倍。

### 缓存容量估算（每条约 450 字节，单 A record）

| 可用内存 | 估算最大缓存域名数 |
|---------|-----------------|
| 128 MB | ~280,000 |
| 256 MB | ~570,000 |
| 512 MB | ~1,130,000 |
| 1 GB | ~2,270,000 |

含 CNAME 链或多条 RR 的复杂响应每条占用更多内存。

### 缓存命中 QPS（单核，进程内 benchmark）

| 场景 | 吞吐量 |
|------|-------|
| 端到端缓存命中（STATE_INIT → RETURN_RESP） | ~520,000 QPS |
| 缓存层读取（命中） | ~1,500,000 QPS |
| 8 核并发混合读写 | ~12,000,000 ops/s |

以上为 CPU 密集型进程内测量值；实际网络 QPS 受连接处理和操作系统网络栈开销限制。

### IP 池容量（每个 NS IP 约 400 字节）

| 可用内存 | 可追踪 NS IP 数 |
|---------|----------------|
| 10 MB | ~25,000 |
| 50 MB | ~125,000 |
| 100 MB | ~250,000 |

### 自定义 benchmark

使用内置 benchmark 在你自己的服务器上测量与实际业务相关域名的首包延迟：

```bash
# 使用默认域名列表（www.qq.com、www.baidu.com、www.taobao.com）
go test -v -run='^$' -bench='BenchmarkFirstPacket' \
    -benchtime=5x -timeout=300s ./e2e/...

# 使用自定义域名
REC53_BENCH_DOMAINS="www.example.com,api.myservice.net" \
    go test -v -run='^$' -bench='BenchmarkFirstPacket' \
    -benchtime=5x -timeout=300s ./e2e/...

# 一次性输出三场景对比表（冷启动 / warmup 后 / 缓存命中）
REC53_BENCH_DOMAINS="www.example.com,api.myservice.net" \
    go test -v -run='^$' -bench=BenchmarkFirstPacketComparison \
    -benchtime=1x -timeout=120s ./e2e/...
```

`REC53_BENCH_DOMAINS` 接受逗号分隔的域名列表，结尾的 `.` 会自动添加，域名间用逗号分隔，不加空格。

---

## 系统设计

### 目录结构

```
rec53/
├── cmd/                    # 入口与 CLI
│   ├── rec53.go            # main()、flag 解析、配置加载、信号处理
│   └── loglevel.go         # 日志级别解析
├── server/                 # DNS 解析核心逻辑
│   ├── server.go           # UDP/TCP 服务器、ServeDNS()、截断、预热生命周期
│   ├── state_machine.go    # Change() 循环、CNAME 链、迭代守卫
│   ├── state_define.go     # 状态常量、返回码、状态构造函数
│   ├── state.go            # 各状态的 handle() 方法实现
│   ├── cache.go            # TTL 缓存封装（go-cache）
│   ├── ip_pool.go          # IPQualityV2 环形缓冲区、评分、探针循环
│   └── warmup.go           # WarmupNSRecords()、TLD 列表
├── monitor/                # 可观测性
│   ├── metric.go           # Prometheus 指标方法、HTTP 服务器
│   ├── log.go              # Zap Logger 初始化、级别控制
│   └── var.go              # 全局指标/日志单例、指标定义
├── utils/                  # 工具函数
│   ├── root.go             # 根 DNS 服务器地址（13 个根）
│   ├── zone.go             # Zone 解析辅助函数
│   └── net.go              # 网络工具函数
└── e2e/                    # 集成测试
    ├── helpers.go           # MockAuthorityServer、测试工具
    ├── resolver_test.go     # 端到端解析测试
    ├── cache_test.go        # 缓存行为测试
    └── server_test.go       # 服务器生命周期测试
```

### 请求生命周期

```
客户端 UDP/TCP 查询
        │
        ▼
  server.ServeDNS()           ← server/server.go
  - 检查 QDCOUNT == 0
  - 保存 originalQuestion
  - InCounterAdd(request)
  - newStateInitState()
        │
        ▼
  Change(stm)                 ← server/state_machine.go
  - 状态机循环（最多 50 次迭代）
  - 累积 cnameChain
        │
        ▼
  reply = result
  - 恢复 originalQuestion
  - UDP：如需要则 truncateResponse()
  - OutCounterAdd / LatencyHistogramObserve
  - w.WriteMsg(reply)
```

### 组件映射

| 组件 | 文件 | 职责 |
|------|------|------|
| `server` | `server/server.go` | UDP/TCP 监听器，请求入口 |
| `Change()` | `server/state_machine.go` | 状态机循环调度器 |
| 状态处理器 | `server/state_define.go`、`state.go` | 各状态的 `handle()` 逻辑 |
| `globalDnsCache` | `server/cache.go` | TTL 响应缓存 |
| `globalIPPool` | `server/ip_pool.go` | NS 延迟追踪与选择 |
| `WarmupNSRecords` | `server/warmup.go` | 启动时 IP 池引导预热 |
| `Rec53Metric` | `monitor/metric.go` | Prometheus 计数器/直方图/仪表盘 |
| `Rec53Log` | `monitor/log.go` | Zap 结构化日志 |

---

## 核心子系统：状态机

### 概述

所有 DNS 解析均在 `server/state_machine.go` 的 `Change()` 循环中完成。每次调用 `Change()` 会驱动状态机执行最多 **50 次状态转移**（CNAME 循环守卫）。每个状态均为实现以下接口的 struct：

```go
type stateMachine interface {
    getCurrentState() int
    getRequest()      *dns.Msg
    getResponse()     *dns.Msg
    handle(req, resp *dns.Msg) (int, error)
}
```

`handle()` 返回 `(nextStateCode, error)`。循环持续运行，直到收到 `RETURN_RESP` 或发生错误。

### 状态列表

| 状态 | 常量 | 用途 |
|------|------|------|
| `STATE_INIT` | `0` | 验证请求；初始化响应头 |
| `CACHE_LOOKUP` | `1` | 在 `globalDnsCache` 中查找查询 |
| `CLASSIFY_RESP` | `2` | 分类当前响应：Answer / CNAME / NS 委托 |
| `EXTRACT_GLUE` | `3` | 从当前响应的 glue 记录中提取 NS IP |
| `LOOKUP_NS_CACHE` | `4` | 无 glue IP 时回退到缓存或根服务器 |
| `QUERY_UPSTREAM` | `5` | 向最优 NS IP 发送查询；记录延迟或失败 |
| `RETURN_RESP` | `6` | 追加 CNAME 链；写入最终响应 |

### 状态转移图

三条循环路径贯穿整个状态机：

```
                      ┌─────────────────────────────────────────────────┐
                      │           循环 A：迭代委托下钻                  │
                      │   （每次 NS 委托 → 再深入一层）                 │
                      │                                                 │
                      │  ┌──────────────────────────────────────┐       │
                      │  │        循环 B：CNAME 链追踪          │       │
                      │  │  （每个 CNAME target 重新解析）      │       │
                      │  │                                      │       │
    ┌─────────────┐   │  │                                      │       │
    │  STATE_INIT │   │  │                                      │       │
    └──────┬──────┘   │  │                                      │       │
           │ always   │  │                                      │       │
           ▼          │  │                                      │       │
    ┌─────────────┐   │  │   hit                                │       │
    │ CACHE_LOOKUP│───┼──┼──────────────────┐                   │       │
    └──────┬──────┘   │  │                  ▼                   │       │
           │ miss     │  │         ┌──────────────────┐         │       │
           ▼          │  │         │  CLASSIFY_RESP   │         │       │
    ┌─────────────┐   │  │         └────────┬─────────┘         │       │
    │ EXTRACT_GLUE│◄──┼──┼──────────────────┤ NS 委托           │       │
    └──────┬──────┘   │  │                  │                   │       │
           │          │  │                  │ CNAME ────────────┘       │
           │ 有 glue  │  │                  │                           │
           │ IP       │  │                  │ answer / negative         │
           │          │  │                  ▼                           │
           │          │  │         ┌──────────────────┐                 │
           │          │  │         │   RETURN_RESP    │ ──► (完成)      │
           │          │  │         └──────────────────┘                 │
           │ 无 glue  │  │                                              │
           ▼          │  │                                              │
    ┌──────────────┐  │  │                                              │
    │LOOKUP_NS_CACHE│ │  │                                              │
    └──────┬───────┘  │  │                                              │
           │ 命中或   │  │                                              │
           │ 未命中   │  │                                              │
           │ (根服务器)│  │                                             │
           ▼          │  │                                              │
    ┌──────────────┐  │  │                                              │
    │QUERY_UPSTREAM│──┴──┘  成功 → CLASSIFY_RESP ──────────────────────┘
    └──────┬───────┘         （新 NS 委托关闭循环 A）
           │
           │ 错误 → SERVFAIL（终态）
```

**循环 A — 迭代下钻**（主循环，最多 50 次迭代）

每次 QUERY_UPSTREAM 从上游权威服务器拿到 NS referral（有 Ns + Extra，但没有 Answer），CLASSIFY_RESP 识别为 NS referral 并转到 EXTRACT_GLUE，循环继续，直到某一层服务器返回最终答案。

```
EXTRACT_GLUE → QUERY_UPSTREAM → CLASSIFY_RESP →(NS 委托)→ EXTRACT_GLUE → QUERY_UPSTREAM → CLASSIFY_RESP → …
   (根)           (根)              (TLD NS)        (TLD)       (TLD)           (权威)            (答案！)
```

**循环 B — CNAME 链追踪**（每个 CNAME target 触发一次完整解析）

CLASSIFY_RESP 发现 CNAME 时，将 CNAME record 追加到 `cnameChain`，修改 Question 为 target，转回 CACHE_LOOKUP 重新走完整解析流程，直到拿到非 CNAME 记录。

```
CLASSIFY_RESP →(CNAME a→b)→ CACHE_LOOKUP →(miss)→ EXTRACT_GLUE → QUERY_UPSTREAM → CLASSIFY_RESP
               →(CNAME b→c)→ CACHE_LOOKUP → …
               →(answer c)→  RETURN_RESP  （prepend cnameChain: [a→b, b→c] + answer）
```

**LOOKUP_NS_CACHE 回退路径**（循环 A 的分支，非独立循环）

EXTRACT_GLUE 发现无 glue 记录时，LOOKUP_NS_CACHE 从缓存中查找父级 zone 的 NS+glue，或退回根服务器。cache hit / miss 均进入 QUERY_UPSTREAM 继续循环 A。

```
EXTRACT_GLUE →(无 glue)→ LOOKUP_NS_CACHE →(命中：缓存 zone)→ QUERY_UPSTREAM
                                          →(未命中：根服务器)→ QUERY_UPSTREAM
```

### CNAME 链处理

`CLASSIFY_RESP` 在 Answer 节中检测 CNAME 记录，并将其追加到存储于状态机中的 `cnameChain []dns.RR`。随后以 CNAME target 为目标通过 `CACHE_LOOKUP` 重新发起查询。在 `RETURN_RESP` 时，累积的链会被前置到最终 Answer 中。

**循环检测**：使用 `visitedDomains` map 防止无限 CNAME 循环。

**B-004 修复**：`isNSRelevantForCNAME` 在 NS 委托记录属于原始查询的 zone 而非 CNAME target 的 zone 时，保留该 NS 记录，以防止错误的委托循环。

### 无 Glue 的 NS 解析

当 `LOOKUP_NS_CACHE` 在缓存或根服务器中均无法找到 NS IP 时，`resolveNSIPsConcurrently` 会并发启动多个递归状态机调用（每个 NS 主机名一个）。通过 `contextKeyNSResolutionDepth` 的深度守卫，可防止 NS 主机名本身被委托时引发死锁。

### 返回码

返回码定义于 `server/state_machine.go` 和 `server/state_define.go`：

| 代码 | 含义 |
|------|------|
| `CACHE_LOOKUP_HIT` | 缓存命中 — 转到 `CLASSIFY_RESP` |
| `CACHE_LOOKUP_MISS` | 缓存未命中 — 转到 `EXTRACT_GLUE` |
| `CLASSIFY_RESP_GET_ANS` | 最终答案就绪 — 转到 `RETURN_RESP` |
| `CLASSIFY_RESP_GET_CNAME` | 发现 CNAME — 重新进入 `CACHE_LOOKUP` |
| `CLASSIFY_RESP_GET_NS` | NS 委托 — 转到 `EXTRACT_GLUE` |
| `EXTRACT_GLUE_EXIST` | 找到 glue IP — 转到 `QUERY_UPSTREAM` |
| `EXTRACT_GLUE_NOT_EXIST` | 无 glue — 转到 `LOOKUP_NS_CACHE` |
| `QUERY_UPSTREAM_COMMON_ERROR` | 上游查询失败 |
| `RETURN_RESP_NO_ERROR` | 终态，返回响应 |

---

## 核心子系统：缓存

### 设计

缓存是对 [`patrickmn/go-cache`](https://github.com/patrickmn/go-cache) 的轻量封装，提供以下保证：

- **键格式**：`"name.:qtype_number"` — 例如 A 记录为 `"example.com.:1"`，AAAA 为 `"example.com.:28"`
- **读写均深拷贝**：每个缓存的 `*dns.Msg` 均通过 `msg.Copy()` 存储和读取，防止调用方修改已缓存数据
- **TTL 来源**：从 `Answer[0].Header().Ttl`（正向响应）或 `Ns[0].Header().Ttl`（NS 委托）中提取；默认 5 分钟
- **go-cache 参数**：默认 TTL 5 分钟，清理间隔 10 分钟

### 否定缓存

NXDOMAIN 和 NODATA（空 Answer，无错误）响应使用 Authority 节中 SOA 的 `Minttl` 字段进行缓存。若无 SOA，则使用 60 秒默认 TTL。这可防止对不存在的域名重复进行迭代解析。

### 缓存 API

```go
// 读取 — 始终返回深拷贝；未命中返回 nil
msg := getCacheCopyByType(name, qtype)

// 写入 — 存储深拷贝；TTL 来自 msg 或默认 5 分钟
setCacheCopyByType(name, qtype, msg)
```

### 线程安全

`go-cache` 内部自带锁。`getCacheCopyByType`/`setCacheCopyByType` 封装器不额外加锁。深拷贝机制确保并发读取时无数据竞争。

---

## 核心子系统：IP 池（IPQualityV2）

### 概述

`globalIPPool` 追踪解析过程中遇到的每个 NS IP 的延迟质量。每个 IP 使用 **64 样本滑动窗口环形缓冲区**，并向 Prometheus 导出 P50/P95/P99 百分位数。选择算法使用**综合评分**，平衡实测延迟、置信度和故障状态。

### 单 IP 数据结构

```go
type IPQualityV2 struct {
    samples      [64]float64   // RTT 样本环形缓冲区（毫秒）
    sampleCount  int           // 已记录样本总数（上限 64）
    head         int           // 环形缓冲区下一个写入位置
    p50, p95, p99 float64      // 计算得出的百分位数
    failCount    int           // 连续失败计数
    state        int           // ACTIVE / DEGRADED / SUSPECT / RECOVERED
}
```

### 生命周期

```
发现新 IP
    │  state=ACTIVE，confidence=0%，score=2000（鼓励采样）
    ▼
RecordLatency(ip, rtt)
    │  将 rtt 加入环形缓冲区，重新计算 P50/P95/P99，重置 failCount=0
    ▼
查询成功 ──► state 保持 ACTIVE；confidence 逐渐增大至 100%
查询失败 ──► RecordFailure(ip)
                  failCount 1-3：state=DEGRADED（score ×1.5）
                  failCount 4-6：state=SUSPECT（score ×100，p50=10000）
                  failCount 7+： state=SUSPECT（可触发探针）
                      │
                      ▼ 每 30 秒（后台）
                  periodicProbeLoop()
                      探测 A 记录 → 成功 → ResetForProbe()
                                              state=ACTIVE，failCount=0
```

### 综合评分公式

```
score = p50_ms × confidence_multiplier × state_weight

confidence_multiplier：
  置信度   0% → 2.0   （新 IP 会被积极尝试）
  置信度 100% → 1.0   （已充分测量的 IP 仅按延迟评判）

state_weight：
  ACTIVE    → 1.0
  RECOVERED → 1.1   （轻微惩罚：刚恢复）
  DEGRADED  → 1.5   （中等惩罚：有失败记录）
  SUSPECT   → 100.0 （尽量回避：严重失败）
```

### 评分示例

| 状态 | 置信度 | P50 (ms) | 置信度系数 | 状态权重 | 评分 |
|------|--------|----------|-----------|---------|------|
| ACTIVE | 0% | 100 | 2.0 | 1.0 | **200**（新 IP，鼓励采样） |
| ACTIVE | 100% | 100 | 1.0 | 1.0 | **100**（优选） |
| ACTIVE | 100% | 50 | 1.0 | 1.0 | **50**（最优） |
| RECOVERED | 100% | 100 | 1.0 | 1.1 | **110**（轻微惩罚） |
| DEGRADED | 100% | 100 | 1.0 | 1.5 | **150**（惩罚） |
| SUSPECT | 100% | 10000 | 1.0 | 100.0 | **1,000,000**（回避） |

### 选择 API

```go
// 按综合评分返回（最优，次优）
best, secondary := globalIPPool.GetBestIPsV2(ips)

// 记录成功查询
globalIPPool.RecordLatency(ip, rtt_ms)

// 记录失败查询
globalIPPool.RecordFailure(ip)
```

### 并发访问

- `IPQualityV2` 字段在热路径中通过原子操作进行无锁访问
- `IPPool.pool`（IP → `*IPQualityV2` 的 map）由 `sync.RWMutex` 保护：
  - 查询路径（`RecordLatency`、`RecordFailure`、`GetScore`）使用 `RLock`
  - 仅后台探针循环（`ResetForProbe`）使用 `Lock`
- 后台探针 goroutine 每 30 秒运行一次，不阻塞查询路径

### 预热引导

启动时，`WarmupNSRecords()` 为可配置的 TLD 列表解析 NS 记录。所有解析到的 NS IP 通过 `RecordLatency` 写入 `globalIPPool`，使 IP 池在第一个用户查询到来前就具备实测基线，消除冷启动时所有 IP 置信度为 0% 的性能损失。

---

## 监控

### Prometheus 指标

指标端点：`http://localhost:9999/metric`

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `rec53_in_total` | Counter | `stage`、`name`、`type` | 入站查询计数 |
| `rec53_out_total` | Counter | `stage`、`name`、`type`、`code` | 出站响应计数 |
| `rec53_latency_ms` | Histogram | `stage`、`name`、`type`、`code` | 端到端查询延迟（毫秒） |
| `rec53_ipv2_p50_latency_ms` | Gauge | `ip` | NS 中位 RTT |
| `rec53_ipv2_p95_latency_ms` | Gauge | `ip` | NS P95 RTT |
| `rec53_ipv2_p99_latency_ms` | Gauge | `ip` | NS P99 RTT |

### 常用 PromQL 查询

```promql
# 查询速率
rate(rec53_in_total[1m])

# 错误率（SERVFAIL）
rate(rec53_out_total{code="SERVFAIL"}[1m]) / rate(rec53_out_total[1m])

# P99 端到端延迟
histogram_quantile(0.99, rate(rec53_latency_ms_bucket[5m]))

# 延迟劣化的 NS（P50 > 500ms）
rec53_ipv2_p50_latency_ms > 500
```

---

## Docker 部署

```bash
# 构建镜像
docker build -t rec53 .

# 独立运行
docker run -d \
  -p 5353:5353/udp \
  -p 5353:5353/tcp \
  -p 9999:9999 \
  rec53

# 使用 Docker Compose 运行（含 Prometheus + node-exporter）
cd single_machine && docker-compose up -d
```

### Docker Compose 服务

| 服务 | 端口 | 说明 |
|------|------|------|
| rec53 | 5353 (UDP/TCP)、9999 | DNS 服务器 + Prometheus 指标 |
| prometheus | 9090 | 指标采集 |
| node-exporter | 9100 | 主机指标 |

---

## 开发

```bash
# 完整测试套件（始终使用 -race）
go test -race ./...

# 禁用测试缓存
go test -race -count=1 ./...

# 单个测试
go test -v -run TestResolverIntegration ./e2e/...
go test -v -run TestIPPoolSelection ./server/...

# 覆盖率
go test -cover ./...

# 格式化
gofmt -w .

# 静态检查
go vet ./...
```

---

## 已知限制

- 未实现 DNSSEC 验证
- 不支持 DoT / DoH
- `www.huawei.com` 等复杂 CNAME 链在最终 A/AAAA 解析失败时可能返回 SERVFAIL

## 路线图

详见 [`.rec53/ROADMAP.md`](.rec53/ROADMAP.md)：

- DNSSEC 验证
- DoT/DoH 支持
- 并发上游查询
- 查询速率限制

## 文档

- [`.rec53/ARCHITECTURE.md`](.rec53/ARCHITECTURE.md) — 详细架构参考
- [`.rec53/CONVENTIONS.md`](.rec53/CONVENTIONS.md) — 代码规范与模式
- [`.rec53/ROADMAP.md`](.rec53/ROADMAP.md) — 路线图与需求

## 参考资料

- [miekg/dns](https://github.com/miekg/dns) — Go DNS 协议库
- [Unbound](https://nlnetlabs.nl/projects/unbound/about/) — 参考递归解析器架构
- [RFC 1034](https://datatracker.ietf.org/doc/html/rfc1034) — DNS 概念与设施
- [RFC 1035](https://datatracker.ietf.org/doc/html/rfc1035) — DNS 实现与规范
- [RFC 2308](https://datatracker.ietf.org/doc/html/rfc2308) — DNS 查询否定缓存

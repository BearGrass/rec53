# 架构设计

本文档描述 rec53 递归 DNS 解析器的核心架构设计。

## 系统概述

rec53 是一个递归 DNS 解析器，采用状态机架构处理 DNS 查询请求。核心设计参考了 Unbound 的状态机模型。

```
┌─────────────────────────────────────────────────────────────────┐
│                         DNS Query                                │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                        ServeDNS (Entry)                          │
│  - Save original question                                        │
│  - Create state machine                                          │
│  - Restore question before response                              │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
                    ┌───────────────────┐
                    │    STATE_INIT     │
                    └───────────────────┘
                                │
                                ▼
                    ┌───────────────────┐
                    │    IN_CACHE       │── HIT ──┐
                    └───────────────────┘         │
                                │ MISS            │
                                ▼                 │
                    ┌───────────────────┐         │
                    │    IN_GLUE        │         │
                    └───────────────────┘         │
                      │           │               │
                 EXIST│           │NOT_EXIST      │
                      ▼           ▼               │
              ┌───────────┐ ┌───────────────┐     │
              │   ITER    │◄│ IN_GLUE_CACHE │     │
              └───────────┘ └───────────────┘     │
                      │                           │
                      │                           │
                      ▼                           │
              ┌───────────────────┐ ◄─────────────┘
              │   CHECK_RESP      │
              └───────────────────┘
                      │
        ┌─────────────┼─────────────┐
        │             │             │
    GET_ANS      GET_CNAME     GET_NS
        │             │             │
        ▼             ▼             ▼
┌─────────────┐ ┌───────────┐ ┌───────────┐
│  RET_RESP   │ │ IN_CACHE  │ │  IN_GLUE  │
│  (done)     │ │ (follow)  │ │ (next NS) │
└─────────────┘ └───────────┘ └───────────┘
```

## 核心组件

### 1. 状态机 (State Machine)

位置: `server/state_machine.go`, `server/state_define.go`

状态机是 DNS 解析的核心，每个状态实现 `stateMachine` 接口：

```go
type stateMachine interface {
    getCurrentState() int
    getRequest() *dns.Msg
    getResponse() *dns.Msg
    handle(request *dns.Msg, response *dns.Msg) (int, error)
}
```

**状态说明：**

| 状态 | 职责 | 转换目标 |
|------|------|----------|
| STATE_INIT | 初始化响应消息 | IN_CACHE |
| IN_CACHE | 检查缓存命中/未命中 | CHECK_RESP / IN_GLUE |
| CHECK_RESP | 分析响应内容 | RET_RESP / IN_CACHE / IN_GLUE |
| IN_GLUE | 检查是否有 Glue records | ITER / IN_GLUE_CACHE |
| IN_GLUE_CACHE | 从缓存查找上级 NS | ITER |
| ITER | 向上游服务器发送查询 | CHECK_RESP |
| RET_RESP | 返回最终响应 | (结束) |

**安全机制：**
- `MaxIterations = 50` 防止无限循环
- `visitedDomains` map 检测 CNAME 循环
- 原始 Question 在入口保存、出口恢复

### 2. 缓存 (Cache)

位置: `server/cache.go`

使用 `patrickmn/go-cache` 实现，特性：
- 默认 TTL: 5 分钟
- 清理间隔: 10 分钟
- 缓存键格式: `domain:qtype`（如 `google.com.:1` 表示 A 记录）
- 深拷贝存取，防止并发修改

```go
// 全局缓存实例
var globalDnsCache = newCache()

// 缓存键包含查询类型
func getCacheKey(name string, qtype uint16) string {
    return fmt.Sprintf("%s:%d", name, qtype)
}
```

### 3. IP 质量池 (IP Pool)

位置: `server/ip_pool.go`

跟踪上游 Nameserver 的质量，用于选择最佳服务器：
- 初始延迟: 1000ms
- 最大延迟: 10000ms
- Prefetch 并发限制: 10
- 使用 `atomic` 操作保证并发安全

**质量评分逻辑：**
- 查询成功: 更新为实际 RTT
- 查询失败: 设置为 MAX_IP_LATENCY
- Prefetch: 后台探测未初始化的 IP

### 4. 服务器 (Server)

位置: `server/server.go`

统一处理 UDP 和 TCP 请求：
- 单一 `ServeDNS` 处理器
- UDP 截断处理（TC flag）
- EDNS0 支持（4096 buffer size）
- 优雅关闭（5 超时）

## 数据流

### 查询处理流程

1. **入口 (ServeDNS)**
   - 保存原始 Question
   - 创建初始状态机
   - 执行状态转换

2. **缓存检查 (IN_CACHE)**
   - 命中：直接进入 CHECK_RESP
   - 未命中：进入 IN_GLUE

3. **迭代查询 (ITER)**
   - 选择最佳上游 IP
   - 发送查询，处理响应
   - 更新缓存和 IP 质量

4. **响应检查 (CHECK_RESP)**
   - 有匹配记录：返回
   - 有 CNAME：跟随后继续
   - 无答案：继续迭代

5. **出口**
   - 恢复原始 Question
   - 处理 UDP 截断
   - 记录指标

### CNAME 链处理

```
Query: www.example.com (A)
  │
  ├─ Cache Miss → Iterate
  │
  ├─ Response: www.example.com → CNAME → alias.example.com
  │
  ├─ CHECK_RESP: GET_CNAME
  │
  ├─ Query: alias.example.com (A)
  │
  └─ Response: alias.example.com → A → 192.0.2.1
     │
     └─ RET_RESP
```

## 并发模型

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  UDP Server │     │  TCP Server │     │  Metrics    │
└─────────────┘     └─────────────┘     └─────────────┘
       │                   │                   │
       └───────────────────┴───────────────────┘
                           │
                    ┌──────▼──────┐
                    │  ServeDNS   │  (无锁，每个请求独立)
                    └─────────────┘
                           │
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
    ┌───────────┐    ┌───────────┐    ┌───────────┐
    │  Cache    │    │  IP Pool  │    │  Metrics  │
    │ (RWMutex) │    │ (RWMutex) │    │ (atomic)  │
    └───────────┘    └───────────┘    └───────────┘
```

**并发安全策略：**
- Cache: `sync.RWMutex` 保护
- IP Pool: `sync.RWMutex` + `atomic` 操作
- Metrics: `sync/atomic` 计数器
- Prefetch: semaphore 限制并发数

## 监控

位置: `monitor/metric.go`, `monitor/log.go`

**Prometheus 指标：**

| 指标名 | 类型 | 描述 |
|--------|------|------|
| `rec53_in_total` | Counter | 入站查询数 |
| `rec53_out_total` | Counter | 出站响应数 |
| `rec53_latency_ms` | Histogram | 查询延迟 |
| `rec53_ip_quality` | Gauge | 上游 IP 延迟 |

## 目录结构

```
rec53/
├── cmd/
│   └── rec53.go          # 入口：flag 解析、信号处理、启动
├── server/
│   ├── server.go         # UDP/TCP 服务器
│   ├── state_machine.go  # 状态机引擎
│   ├── state_define.go   # 状态定义与实现
│   ├── cache.go          # DNS 缓存
│   └── ip_pool.go        # IP 质量管理
├── monitor/
│   ├── metric.go         # Prometheus 指标
│   └── log.go            # zap 日志
├── utils/
│   └── zone.go           # Zone 解析、Root servers
└── e2e/
    └── e2e_test.go       # 端到端测试
```

## 设计决策

### 为什么用状态机？

参考 Unbound 设计：
- 每个 state 职责单一，易于测试
- 状态转换显式，流程可控
- 方便添加新功能（如 DNSSEC 验证）

### 为什么用全局变量？

当前阶段简化设计：
- 快速迭代原型
- 单实例部署场景
- 后续可重构为依赖注入

### 为什么缓存键包含类型？

避免 A/AAAA 混淆：
- `example.com.:1` → A 记录
- `example.com.:28` → AAAA 记录
- 同一域名不同类型独立缓存

## 参考资源

- [RFC 1034] Domain Names - Concepts and Facilities
- [RFC 1035] Domain Names - Implementation and Specification
- [Unbound Architecture](https://nlnetlabs.nl/documentation/unbound/)
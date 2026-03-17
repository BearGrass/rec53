## Context

rec53 是一个纯递归 DNS 解析器，核心是一个状态机（`server/state_machine.go`）。当前查询处理链为：

```
STATE_INIT → CACHE_LOOKUP → CLASSIFY_RESP → EXTRACT_GLUE → LOOKUP_NS_CACHE → QUERY_UPSTREAM → RETURN_RESP
```

目前缺少两类常见需求：
1. **Hosts 本地权威**：为内部主机名（如 `db.internal`）直接返回静态记录，无需递归。
2. **Forwarding 转发规则**：将特定域名后缀（如 `.corp.example.com`）的查询转发给内部 DNS，不走公网迭代。

配置由 `cmd/rec53.go` 解析后传入 `server.NewServerWithConfig()`，目前 `server` 包通过 `WarmupConfig` 接受配置；新功能同样遵循此模式。

## Goals / Non-Goals

**Goals:**
- 支持在 `config.yaml` 中声明 `hosts` 静态映射（A、AAAA、CNAME 类型）
- 支持在 `config.yaml` 中声明 `forwarding` 规则，每条规则包含域名后缀 + 上游地址列表
- hosts 匹配优先于 forwarding，forwarding 优先于 cache 和迭代
- 转发使用已有 `github.com/miekg/dns` 客户端，超时复用 `UpstreamTimeout`
- 单元测试覆盖 hosts/forwarding 两个新状态；E2E 场景验证端到端行为

**Non-Goals:**
- 不支持动态 hosts 热重载（重启生效即可）
- 不支持 zone file 格式，仅支持 YAML 配置
- 转发结果不写入 `globalDnsCache`（结果时效性由上游控制）
- 不实现 DNSSEC 验证
- 不支持 DNS-over-TLS/HTTPS 转发

## Decisions

### D-1：在状态机中插入两个新状态

**决策**：新增 `HOSTS_LOOKUP`（紧接 `STATE_INIT` 之后）和 `FORWARD_LOOKUP`（紧接 `HOSTS_LOOKUP` 之后），处理链变为：

```
STATE_INIT → HOSTS_LOOKUP → FORWARD_LOOKUP → CACHE_LOOKUP → ... → RETURN_RESP
```

**理由**：状态机是 rec53 的核心扩展点，每个状态职责单一、可独立测试。在 `STATE_INIT` 后插入不改变现有状态的返回码，只需在 `state_machine.go` 的 `Change()` 中调整跳转逻辑。

**备选方案**：在 `ServeDNS` 入口做 if/else 检查——耦合度高，难以测试，不符合项目状态机风格，已排除。

---

### D-2：配置通过 `HostsConfig` / `ForwardingConfig` 结构传入 Server

**决策**：

```go
// cmd/rec53.go
type Config struct {
    DNS        DNSConfig              `yaml:"dns"`
    Warmup     server.WarmupConfig    `yaml:"warmup"`
    Hosts      []server.HostEntry     `yaml:"hosts"`
    Forwarding []server.ForwardZone   `yaml:"forwarding"`
}

// server 包
type HostEntry struct {
    Name  string `yaml:"name"`   // FQDN，不含尾点也可，内部规范化
    Type  string `yaml:"type"`   // "A" | "AAAA" | "CNAME"
    Value string `yaml:"value"`  // IP 地址或 CNAME 目标
    TTL   uint32 `yaml:"ttl"`    // 默认 60
}

type ForwardZone struct {
    Zone      string   `yaml:"zone"`       // 后缀匹配，如 "corp.example.com"
    Upstreams []string `yaml:"upstreams"`  // "1.2.3.4:53"
}
```

Server 构造时将 hosts 预编译为 `map[qkey]*dns.Msg`（qkey = `"name. qtype"`），forwarding 存为有序切片（最长后缀优先匹配）。

**理由**：与 `WarmupConfig` 一致的传参风格；预编译 map 使 `HOSTS_LOOKUP` 为 O(1) 查找；`server` 包类型定义放在 `server/` 内以避免 `cmd` 包与 `server` 包循环依赖。

---

### D-3：Forwarding 使用最长后缀匹配

**决策**：遍历 `ForwardZone` 切片时，选取与查询名匹配的**最长** `zone` 后缀对应的上游列表。切片在 Server 初始化时按 zone 长度降序排列。

**理由**：符合 DNS split-horizon 惯例；用户可以为 `example.com` 和 `db.example.com` 分别配置不同上游，更精确的规则优先生效。

---

### D-4：Forwarding 不走 `globalDnsCache`

**决策**：`FORWARD_LOOKUP` 直接向上游发送标准递归查询（`rd=1`），不查也不写 `globalDnsCache`。

**理由**：内部 DNS 的答案往往有定制 TTL 且频繁变化，写入全局缓存可能导致缓存污染；同时保持 hosts/forwarding 与全局缓存的隔离，方便故障排查。

---

### D-5：转发失败回退策略

**决策**：若所有上游均失败（超时/SERVFAIL），返回 `SERVFAIL` 给客户端，**不**回退到迭代解析。

**理由**：forwarding 配置代表管理员明确意图（该域名必须走内部 DNS），静默回退到公网解析会导致意外的数据泄漏或不一致。

## Risks / Trade-offs

- **[性能]** hosts map 在每次查询时只做一次 `map` 查找，overhead 可忽略不计 → 无需额外缓存层。
- **[CNAME hosts]** 如果 hosts 返回 CNAME，当前设计不自动跟随（下一次迭代会走 cache/iter 解析 target）→ 已知限制，可在后续版本支持。
- **[Forwarding + CNAME]** 转发结果中若包含 CNAME 指向公网域名，当前不自动递归解析 target → 与 D-4 一致，客户端（或上游 DNS）负责完整解析。
- **[配置错误]** 用户配置错误的 IP/zone 只在启动时做格式校验，运行时错误以 SERVFAIL 形式返回 → `validateConfig` 中增加 hosts/forwarding 字段的基本校验。
- **[热重载]** 当前不支持，重启才能生效 → 对于生产环境可通过 rolling restart 解决；热重载列入 Non-Goal。

## Migration Plan

1. 本变更为纯增量：未配置 `hosts` / `forwarding` 时行为与当前完全一致。
2. 升级步骤：
   - 更新二进制
   - （可选）在 `config.yaml` 中添加 `hosts:` / `forwarding:` 节
   - 重启 rec53
3. 回滚：移除 `config.yaml` 中新增节并重启，或直接回滚二进制。

## Open Questions

- Q1：hosts 是否需要支持泛域名（`*.internal`）？当前设计不支持，如需要可在后续迭代中以前缀树实现。
- Q2：forwarding 上游是否需要轮询/健康探测？当前仅顺序尝试 + 超时，与 IPPool 的探测机制相互独立。

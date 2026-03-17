## ADDED Requirements

### Requirement: 衰减 LFU 查询统计
rec53 SHALL 在每次 DNS 查询成功返回时，提取请求域名的 eTLD+1（使用 `golang.org/x/net/publicsuffix`），并对其热度分执行 `score += 1.0`。无法提取 eTLD+1 的域名（单标签、纯 IP 等）SHALL 被静默跳过，不影响查询处理。统计模块 SHALL 通过配置项 `learned_warmup.enabled` 控制；`enabled: false` 时所有统计操作 SHALL 为 no-op。

#### Scenario: 普通域名查询时记录 eTLD+1
- **WHEN** 查询 `api.github.com` 成功返回
- **THEN** SHALL 将 `github.com` 的热度分加 1.0

#### Scenario: 单标签域名跳过记录
- **WHEN** 查询 `localhost` 成功返回
- **THEN** SHALL 静默跳过，不记录任何条目，不报错

#### Scenario: 纯 IP 反查跳过记录
- **WHEN** 查询 `1.0.168.192.in-addr.arpa` 成功返回
- **THEN** SHALL 静默跳过，不记录任何条目

#### Scenario: 功能禁用时不统计
- **WHEN** `learned_warmup.enabled: false`，查询任意域名成功返回
- **THEN** SHALL 不记录任何数据，不读写学习文件

---

### Requirement: 衰减 LFU 定期衰减
rec53 SHALL 启动一个后台 goroutine，按 `learned_warmup.decay_interval`（默认 1h）对所有条目执行 `score × decay_factor`（默认 0.9）。热度分低于 `0.01` 的条目 SHALL 被自动删除。衰减 goroutine SHALL 在 context 取消时退出。

#### Scenario: 定期衰减降低过时条目热度
- **WHEN** 距上次衰减已过 `decay_interval`
- **THEN** SHALL 对所有条目 `score × decay_factor`，低于 0.01 的条目被删除

#### Scenario: 长期不访问的条目最终被清除
- **WHEN** 某条目在若干个 `decay_interval` 内未被访问，分数低于 0.01
- **THEN** SHALL 从内存中删除该条目

#### Scenario: context 取消时衰减 goroutine 退出
- **WHEN** 传入的 context 被取消
- **THEN** 衰减 goroutine SHALL 在下一个 tick 处退出

---

### Requirement: 学习文件持久化
rec53 SHALL 每隔 `learned_warmup.flush_interval`（默认 300s）将内存中热度分最高的 `top_n` 条目（默认 200）以 JSON 格式覆写到 `learned_warmup.file`（默认 `~/.rec53/learned.json`）。文件写失败 SHALL 记录 error 日志，不影响 DNS 服务，内存数据保留至下次重试。

#### Scenario: 定期 flush 写入文件
- **WHEN** 距上次 flush 已过 `flush_interval`
- **THEN** SHALL 将 top-N 条目（`{domain, score}` 列表）覆写到学习文件

#### Scenario: 文件不存在时自动创建
- **WHEN** 学习文件路径不存在
- **THEN** SHALL 自动创建父目录（`mkdir -p`）并写入文件

#### Scenario: 写入失败时降级处理
- **WHEN** 磁盘满或权限不足导致文件写入失败
- **THEN** SHALL 记录 error 日志，保留内存数据，下次 flush 继续重试，DNS 服务不受影响

#### Scenario: flush 只保留 top-N 条目
- **WHEN** 内存中有超过 `top_n` 个条目
- **THEN** SHALL 仅将热度分最高的 `top_n` 条目写入文件

---

### Requirement: 启动时从学习文件加载（Round 2 预热）
rec53 SHALL 在 Round 1 预热完成后，若 `learned_warmup.enabled: true`，读取学习文件并对其中的域名并发查询 NS 记录（Round 2）。加载文件失败（文件不存在或格式错误）SHALL 降级为空列表，打印 warning，不 fatal。Round 2 SHALL 共享 warmup deadline context，超时自动截止。

#### Scenario: 正常启动时 Round 2 并发预热
- **WHEN** 启动时学习文件存在且 `enabled: true`
- **THEN** SHALL 读取文件中的域名列表，并发查询其 NS 记录，打印 Round 2 完成摘要

#### Scenario: 学习文件不存在时降级
- **WHEN** 启动时学习文件不存在
- **THEN** SHALL 打印 warning，跳过 Round 2，不 fatal

#### Scenario: 学习文件格式错误时降级
- **WHEN** 学习文件 JSON 格式错误
- **THEN** SHALL 打印 warning，以空列表执行 Round 2（即跳过），不 fatal

#### Scenario: Round 2 在 warmup deadline 内截止
- **WHEN** Round 2 预热进行中，warmup context deadline 触发
- **THEN** 未完成的查询 SHALL 被取消，已完成的结果保留，打印部分完成摘要

#### Scenario: 功能禁用时跳过 Round 2
- **WHEN** `learned_warmup.enabled: false`
- **THEN** SHALL 跳过 Round 2，不读取学习文件

---

### Requirement: learned_warmup 配置块
`config.yaml` SHALL 支持可选的 `learned_warmup:` 顶层配置块。所有字段均有默认值，省略该块等同于 `enabled: false`（向后兼容，不改变现有行为）。

#### Scenario: 省略配置块时默认禁用
- **WHEN** `config.yaml` 中不包含 `learned_warmup:` 块
- **THEN** SHALL 以 `enabled: false` 运行，不统计、不预热、不读写文件

#### Scenario: 显式启用时按配置运行
- **WHEN** `learned_warmup.enabled: true`，其余字段使用默认值
- **THEN** SHALL 启用统计、定期 flush、Round 2 预热，使用默认路径 `~/.rec53/learned.json`

#### Scenario: 自定义文件路径
- **WHEN** 配置 `learned_warmup.file: /var/lib/rec53/learned.json`
- **THEN** SHALL 读写指定路径，不使用默认路径

## 1. 依赖与模块准备

- [x] 1.1 在 `go.mod` 中添加 `golang.org/x/net` 依赖，执行 `go mod tidy` 生成 `go.sum` 条目

## 2. LearnedWarmup 核心模块

- [ ] 2.1 新建 `server/learned_warmup.go`，定义 `LearnedWarmupConfig` 结构体（字段：`Enabled bool`、`File string`、`TopN int`、`DecayFactor float64`、`DecayInterval time.Duration`、`FlushInterval time.Duration`）及默认值常量
- [ ] 2.2 定义 `learnedEntry` 结构体（`Domain string`、`Score float64`）和 `LearnedWarmup` 结构体（包含 `sync.Mutex` 保护的 `map[string]float64` scores、config、context）
- [ ] 2.3 实现 `NewLearnedWarmup(ctx context.Context, cfg LearnedWarmupConfig) *LearnedWarmup` 构造函数；`enabled: false` 时返回 no-op 实例
- [ ] 2.4 实现 `(*LearnedWarmup).Record(qname string)` 方法：提取 eTLD+1（`publicsuffix.EffectiveTLDPlusOne`），分数加 1.0；无法提取时静默跳过；`enabled: false` 时为 no-op
- [ ] 2.5 实现 `(*LearnedWarmup).decay()` 内部方法：遍历 scores map，每个条目乘以 `DecayFactor`，低于 0.01 的条目删除
- [ ] 2.6 实现 `(*LearnedWarmup).topN() []learnedEntry` 内部方法：返回热度分最高的 TopN 条目（按分数降序）
- [ ] 2.7 实现 `(*LearnedWarmup).flush() error` 内部方法：调用 `topN()`，JSON 序列化，`mkdir -p` 确保目录存在，覆写文件；写失败只记录 error 日志，返回 error
- [ ] 2.8 实现 `(*LearnedWarmup).LoadFile() ([]string, error)` 方法：读取并反序列化学习文件，返回域名列表；文件不存在或格式错误时返回空列表 + 打印 warning，不 fatal
- [ ] 2.9 实现 `(*LearnedWarmup).Start()` 方法：启动两个后台 goroutine（衰减定时器、flush 定时器），监听 context 取消退出

## 3. 包级全局变量与初始化

- [ ] 3.1 在 `server/learned_warmup.go` 中声明 `globalLearnedWarmup *LearnedWarmup`（初始值为 disabled no-op 实例）
- [ ] 3.2 新增 `InitLearnedWarmup(ctx context.Context, cfg LearnedWarmupConfig)` 函数，构造实例并赋值给 `globalLearnedWarmup`，调用 `Start()` 启动后台 goroutine

## 4. Round 2 预热

- [ ] 4.1 在 `server/warmup.go` 中新增 `WarmupLearnedDomains(ctx context.Context, warmupCfg WarmupConfig, lw *LearnedWarmup) WarmupStats` 函数：调用 `lw.LoadFile()` 获取域名列表，复用 `queryNSRecords()` 并发预热，打印完成摘要
- [ ] 4.2 `WarmupLearnedDomains` 中并发度复用 `warmupCfg.Concurrency`，共享传入的 context（deadline 超时自动截止）

## 5. 集成到主程序

- [ ] 5.1 在 `cmd/rec53.go` 的 `Config` 结构体中新增 `LearnedWarmup server.LearnedWarmupConfig` 字段（yaml tag: `learned_warmup`）
- [ ] 5.2 在 `main()` 中 logger 初始化后调用 `server.InitLearnedWarmup(ctx, cfg.LearnedWarmup)`，传入 server 主 context
- [ ] 5.3 在 `server/server.go` 的 `ServeDNS` 成功返回路径调用 `globalLearnedWarmup.Record(req.Question[0].Name)`
- [ ] 5.4 在 `cmd/rec53.go` 的 `main()` 中，Round 1（`WarmupNSRecords`）完成后调用 `server.WarmupLearnedDomains(ctx, cfg.Warmup, server.GlobalLearnedWarmup())`

## 6. 配置示例

- [ ] 6.1 在 `generate-config.sh` 中新增注释掉的 `learned_warmup:` 示例块（含所有字段及说明）

## 7. 单元测试

- [ ] 7.1 新建 `server/learned_warmup_test.go`，测试 `Record()` 正确提取 eTLD+1 并累加分数
- [ ] 7.2 测试单标签域名、纯 IP 反查（`in-addr.arpa`）调用 `Record()` 时不记录任何条目
- [ ] 7.3 测试 `decay()` 正确衰减分数，低于 0.01 的条目被删除
- [ ] 7.4 测试 `topN()` 返回正确的 top-N 条目并按分数降序排列
- [ ] 7.5 测试 `flush()` 写入文件内容正确，文件不存在时自动创建目录
- [ ] 7.6 测试 `LoadFile()` 在文件不存在、JSON 格式错误时返回空列表，不 panic
- [ ] 7.7 测试 `enabled: false` 时 `Record()`、`LoadFile()`、`Start()` 均为 no-op，不读写任何文件
- [ ] 7.8 运行 `go test -race ./server/...` 验证无数据竞争

## 8. 文档更新

- [ ] 8.1 在 `README.md` 的 Configuration 部分新增 `learned_warmup:` 配置块说明（字段表格 + 示例）
- [ ] 8.2 更新 `docs/architecture.md`，在预热流程中描述 Round 2 和学习文件

## 1. 拆分 ip_pool.go 文件

- [x] 1.1 创建 `server/ip_pool_quality_v2.go`，将所有相关常量（`INIT_IP_LATENCY`、`MAX_IP_LATENCY`、`IP_STATE_*`）及 `IPQualityV2` 类型和全部方法从 `ip_pool.go` 移入
- [x] 1.2 在 `server/ip_pool.go` 中删除已移走的常量和 `IPQualityV2` 代码，保留 `IPPool` 结构体、`globalIPPool`、`NewIPPool`、生命周期方法、`GetBestIPsV2`、`GetIPQualityV2`、`SetIPQualityV2`、`ResetIPPoolForTest`
- [x] 1.3 运行 `go build ./server/...` 确认拆分后编译通过

## 2. 删除 V1 IPQuality 类型及方法

- [x] 2.1 删除 `server/ip_pool.go` 中 `IPQuality` 结构体及其所有方法（`NewIPQuality`、`Init`、`IsInit`、`GetLatency`、`SetLatency`、`SetLatencyAndState`）
- [x] 2.2 删除 `IPPool` 结构体中的 `pool map[string]*IPQuality` 字段
- [x] 2.3 删除 `NewIPPool` 中 V1 `pool` 字段的初始化代码
- [x] 2.4 删除 V1 pool 操作方法：`isTheIPInit`、`GetIPQuality`、`SetIPQuality`、`updateIPQuality`、`UpIPsQuality`、`getBestIPs`

## 3. 删除 V1 prefetch 链

- [x] 3.1 删除 `IPPool.GetPrefetchIPs` 方法
- [x] 3.2 删除 `IPPool.PrefetchIPs` 方法
- [x] 3.3 删除 `IPPool.prefetchIPQuality` 方法
- [x] 3.4 删除 `IPPool` 结构体中的 `dnsClient *dns.Client` 字段（仅 prefetch 使用）及 `NewIPPool` 中的初始化
- [x] 3.5 删除 `IPPool` 结构体中的 `sem chan struct{}` 字段（仅 prefetch 使用）及相关常量 `MAX_PREFETCH_CONCUR`、`PREFETCH_TIMEOUT`

## 4. 清理调用方代码

- [x] 4.1 删除 `server/state_query_upstream.go` 中 `getBestAddressAndPrefetchIPs` 函数内的 2 行 V1 prefetch 调用（`GetPrefetchIPs` + `PrefetchIPs`）
- [x] 4.2 运行 `go build ./...` 确认全部编译通过

## 5. 清理 monitor 包 V1 指标

- [x] 5.1 删除 `monitor/` 中 `IPQuality` GaugeVec 变量声明
- [x] 5.2 删除 `monitor/` 中 `IPQualityGaugeSet` 方法
- [x] 5.3 删除 `monitor/` 中 `IPQuality` GaugeVec 的 Prometheus 注册调用
- [x] 5.4 运行 `go build ./...` 确认编译通过

## 6. 清理测试代码

- [x] 6.1 删除 `server/ip_pool_test.go` 中的 V1 测试函数：`TestIPPoolGetBestIPs`、`TestIPPoolUpdateIPQuality`、`TestIPPoolUpIPsQuality`、`TestIPPoolGetPrefetchIPs`、`TestIPPoolIsTheIPInit` 及相关辅助代码
- [x] 6.2 删除 `server/state_query_upstream_test.go` 中直接操作 V1 pool 的测试代码（`SetIPQuality`、`updateIPQuality` 调用及 `TestIPPool_UpdateIPQuality`、`TestIPPool_GetBestIPs` 中的 V1 相关断言）
- [x] 6.3 检查 `server/ip_pool_integration_test.go` 是否有 V1 引用，有则删除
- [x] 6.4 运行 `go test -race ./server/...` 确认测试通过

## 7. 验证

- [x] 7.1 运行 `go test -race ./...` 全套测试通过
- [x] 7.2 运行 `go build -o /dev/null ./cmd` 确认二进制编译成功
- [x] 7.3 运行 `gofmt -l ./server/` 确认无格式问题

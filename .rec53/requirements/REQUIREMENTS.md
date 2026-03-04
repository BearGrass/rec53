# rec53 需求列表

## 功能需求

### 核心功能
- [x] 递归DNS解析
- [x] UDP/TCP 支持
- [x] 缓存机制 (LRU)
- [ ] DNSSEC 验证
- [ ] EDNS(0) 支持

### 解析特性
- [x] Glue records 处理
- [x] 迭代解析
- [ ] CNAME 链追踪优化
- [ ] 负缓存 (Negative Caching)

### 性能优化
- [x] IP 质量评估
- [x] IP 预取 (Prefetch)
- [ ] 并发查询
- [ ] 查询流水线

### 监控与运维
- [x] Prometheus 指标
- [x] 日志级别控制
- [ ] 配置热加载
- [ ] 健康检查接口

### 安全特性
- [ ] DNS-over-TLS (DoT)
- [ ] DNS-over-HTTPS (DoH)
- [ ] 查询速率限制
- [ ] 源地址白名单

## 非功能需求

### 性能目标
- QPS: 目标 50,000+ queries/sec
- 延迟: P99 < 50ms (缓存命中)
- 内存: < 500MB (10万缓存条目)

### 可靠性
- 优雅关闭
- 无缝重启
- 故障恢复

## 已知问题
- [ ] `www.huawei.com` 解析存在BUG (见 README.md)

## 需求优先级

| 优先级 | 需求 | 状态 |
|--------|------|------|
| P0 | 优雅关闭 | 已完成 |
| P0 | 错误处理改进 | 已完成 |
| P1 | CNAME 链优化 | 待开始 |
| P1 | 并发查询 | 待开始 |
| P2 | DNSSEC 验证 | 待开始 |
| P2 | DoT/DoH | 待开始 |
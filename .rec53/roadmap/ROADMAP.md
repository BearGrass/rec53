# rec53 开发路线图

## 版本规划

### v0.1.0 - 基础功能 (当前)
- [x] 基本递归解析
- [x] UDP/TCP 支持
- [x] 缓存机制
- [x] Prometheus 监控
- [x] 优雅关闭

### v0.2.0 - 性能优化
- [ ] 并发查询支持
- [ ] 查询流水线
- [ ] 缓存预取优化
- [ ] 负缓存支持

### v0.3.0 - 安全增强
- [ ] DNSSEC 验证
- [ ] DNS-over-TLS
- [ ] 查询速率限制

### v0.4.0 - 高级特性
- [ ] EDNS(0) 完整支持
- [ ] 配置热加载
- [ ] DNS-over-HTTPS

## 技术债务
- [ ] 增加 unit test 覆盖率
- [ ] 添加集成测试
- [ ] 性能基准测试

## 参考资源
- [RFC 1034] Domain Names - Concepts and Facilities
- [RFC 1035] Domain Names - Implementation and Specification
- [RFC 6891] EDNS(0)
- [RFC 4033-4035] DNSSEC
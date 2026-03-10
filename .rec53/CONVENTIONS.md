# rec53 编码规范

本项目为递归 DNS 解析器，参考 BIND、Unbound、CoreDNS 等开源实现。

## 编码规范

### Go 代码风格
- 遵循 [Effective Go](https://golang.org/doc/effective_go) 和 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- 使用 `gofmt` 格式化代码
- 错误处理优先，避免嵌套过深

### 命名约定
- 包名：小写单词，不使用下划线或驼峰
- 导出函数/类型：驼峰命名，首字母大写
- 私有函数/类型：驼峰命名，首字母小写
- 常量：驼峰命名或全大写下划线分隔

### 注释规范
- 导出函数必须有文档注释
- 注释以函数名开头，描述功能而非实现
- 复杂逻辑添加行内注释说明

### 错误处理
- 不要忽略错误，必须处理或向上传递
- 使用 `fmt.Errorf` 包装错误时添加上下文
- 错误信息小写开头，不以句号结尾

## 架构原则

### 状态机设计
- 参考 Unbound 的状态机模型
- 状态转换清晰，每个状态职责单一
- 参考 `server/state_define.go`

### 缓存策略
- LRU 缓存，默认 TTL 5分钟
- 参考 BIND 的缓存实现

### 并发安全
- 共享资源使用 `sync.RWMutex` 保护
- 优先使用 channel 进行 goroutine 通信

## 测试要求
- 核心功能必须有单元测试
- 测试覆盖率目标 > 60%
- 使用 table-driven 测试

## 参考实现

| 项目 | 特点 |
|------|------|
| BIND 9 | 权威+递归，配置复杂，功能完整 |
| Unbound | 纯递归，状态机架构，验证 DNSSEC |
| CoreDNS | 插件化，Kubernetes 集成，Go 实现 |

## 相关文档
- [架构设计](ARCHITECTURE.md)
- [路线图](ROADMAP.md)
- [测试计划](TEST_PLAN.md)
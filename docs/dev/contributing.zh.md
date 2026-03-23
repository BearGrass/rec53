# 贡献指南

[English](contributing.md) | 中文

本文说明 rec53 预期的开发流程。

## 本地准备

常用命令：

```bash
mkdir -p dist && go build -o dist/rec53 ./cmd
./generate-config.sh
./dist/rec53 --config ./config.yaml
```

开发时推荐使用运维式流程：

```bash
./rec53ctl build
./rec53ctl run
```

## 修改代码前

先阅读：

- [架构说明](../architecture.zh.md)
- [测试说明](testing.zh.md)
- [编码约定](../../.rec53/CONVENTIONS.zh.md)
- [AGENTS.md](../../AGENTS.md)

重点关注：

- `server/` 中的状态机流程
- 缓存 copy 不变量
- IP 池并发
- 启动与关闭顺序

## 变更范围

推荐做法：

- 小而明确的改动
- 对生命周期或并发修复写明确测试
- 不要做推测性的抽象
- 功能变更里不要混入无关清理

避免：

- 在发布前大改状态机
- 把默认行为和文档拆开提交
- 没经过运维验证就把可选特性改成默认开启

## 文档同步

行为变更时保持以下内容同步：

- `README.md` 与 `README.zh.md`
- 相关 `docs/user/*`
- 相关 `docs/dev/*`
- 当开发者可见的行为或结构变化时更新 `docs/architecture.md`

如果新增依赖，请在同一个变更里更新相关文档。

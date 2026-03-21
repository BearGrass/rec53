# 发布清单

[English](release.md) | 中文

本文用于准备类似 `v1.0.0` 这样的可部署发布。

## 1. 范围控制

- 除非是 release readiness 必需项，否则冻结新功能工作
- 优先保证文档清晰、启动/关闭稳定、回归可预防
- 确认默认、可选和平台相关功能都写清楚了

## 2. 文档

- 同步 `README.md` 和 `README.zh.md`
- 确认 `docs/user/*` 与当前推荐运维路径一致
- 确认 `docs/dev/*` 与当前开发流程一致
- 如果代码结构或生命周期行为变化，更新 `docs/architecture.md`

## 3. 验证

- 运行 `go test -short ./...`
- 对变更区域运行针对性 package test
- release candidate 尽量运行 `go test -race ./...`
- 验证默认部署路径：
  - `./generate-config.sh`
  - build
  - 前台运行
  - 基础 `dig` 验证

## 4. 运维验证

- 确认 `rec53ctl run` 时日志可读
- 确认安装后的服务使用显式 `LOG_FILE` 路径，并且 `tail -f` 能跟到文档里的文件
- 确认默认日志轮转仍能限制应用管理的磁盘占用
- 确认指标端点可访问
- 确认 systemd 的 install、upgrade、uninstall、`uninstall --purge` 流程和文档一致
- 确认 install 会拒绝覆盖未受管的 unit/binary
- 把 XDP 当作可选项，先验证 Go 路径

## 5. Release notes

- 更新 `CHANGELOG.md`
- 说明所有面向运维者的变更
- 说明任何 config、指标或行为相关的迁移说明

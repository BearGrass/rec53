## Context

rec53 是一个 Go 编写的递归 DNS 解析器，以单个二进制 + YAML 配置文件的方式部署。当前仓库只有 `generate-config.sh`，缺少覆盖"构建 → 部署 → systemd 服务管理 → 升级 → 临时测试"全生命周期的运维脚本。目标机器通常是 Linux 服务器，使用 systemd 管理守护进程。

## Goals / Non-Goals

**Goals:**
- 提供单入口 `rec53ctl` 脚本，子命令覆盖 rec53 的完整运维生命周期
- 各子命令幂等，共享路径变量定义，行为一致
- 脚本对环境依赖友好报错（缺 go、缺 systemd 等）
- 默认值合理，支持环境变量覆盖关键路径

**Non-Goals:**
- 不支持 Docker/容器化部署（另立变更）
- 不处理多节点/集群分发
- 不实现配置文件模板渲染（`generate-config.sh` 已有）
- 不在脚本内嵌 CI/CD 逻辑

## Decisions

### 决策 1：单入口脚本 `rec53ctl` 替代多个独立脚本

**选择**: 单文件 `rec53ctl`，通过 `case "$1"` 分发子命令（`build` / `install` / `upgrade` / `uninstall` / `run`）

**原因**:
- 路径记忆成本低，一个入口记所有操作
- 共享变量（`INSTALL_DIR`、`CONFIG_DIR` 等）定义一次，各子命令直接复用，不会因多文件造成不一致
- `--help` 一屏看清所有能力
- CI/CD 调用更整洁：`./rec53ctl build && ./rec53ctl install`

**替代方案（放弃）**: 多独立脚本 —— 初期直觉合理，但共享变量需重复定义，文件多易混淆；Makefile —— 依赖 make，最小化服务器可能不可用。

### 决策 2：通过环境变量覆盖默认路径

关键路径（`INSTALL_DIR`、`CONFIG_DIR`、`BINARY_NAME`）使用环境变量，默认值覆盖 90% 场景：

| 变量 | 默认值 |
|------|--------|
| `INSTALL_DIR` | `/usr/local/bin` |
| `CONFIG_DIR` | `/etc/rec53` |
| `BINARY_NAME` | `rec53` |
| `SERVICE_NAME` | `rec53` |
| `BUILD_OUTPUT` | `dist/rec53` |

**原因**: 比命令行参数更易在 CI 环境中传递，且与 shell 约定一致。

### 决策 3：`install` 子命令内联完成 build + deploy + systemd 安装

`install` 直接完成全流程：编译 → 复制文件 → 写 unit → enable/start。不拆分为独立的 deploy 阶段，减少用户需要记忆的步骤数。

### 决策 4：systemd unit 文件内联 heredoc 生成

**原因**: 避免独立模板文件引入路径依赖；unit 内容简单，内联即可。

### 决策 5：`upgrade` 先备份再替换，失败自动回滚

升级流程：备份旧二进制 → [重编译] → 停服 → 替换 → 启服 → 验证。启动失败时恢复备份并重启，保证服务可用性。支持 `SKIP_BUILD=1` 跳过编译步骤，适合在开发机编好再传到服务器的场景。

### 决策 6：`run` 子命令使用 `exec` 前台运行

`run` 使用 `exec` 直接替换 shell 进程，确保 Ctrl-C 信号正确传递给 rec53，不产生孤儿进程。

## Risks / Trade-offs

- **[风险] 需要 root 权限** → `install` / `uninstall` / `upgrade` 写入系统目录，需要 `sudo`。脚本检测到非 root 时打印提示，不静默失败。
- **[风险] systemd 不可用** → 在 WSL、Docker、旧系统中 systemd 可能不存在。`install` / `uninstall` / `upgrade` 在执行前检查 `systemctl`，否则友好报错。
- **[Trade-off] 单文件稍长** → 约 200 行，但用 `## --- section ---` 注释分隔各子命令函数，可读性可接受。

## Migration Plan

1. 在项目根目录新增 `rec53ctl`（`chmod +x`）
2. 在 `README.md` 的 "Build & Run" 部分补充 `rec53ctl` 用法
3. 无需数据迁移；不影响现有 `generate-config.sh`

## Open Questions

- 是否需要支持 `GOOS/GOARCH` 交叉编译参数？暂按支持处理，通过环境变量透传给 `go build`。

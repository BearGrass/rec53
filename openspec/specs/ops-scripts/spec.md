## Requirements

### Requirement: rec53ctl 单入口运维脚本
`rec53ctl` SHALL 是项目根目录下的单一可执行 bash 脚本，通过子命令（`build` / `install` / `upgrade` / `uninstall` / `run`）覆盖 rec53 的完整运维生命周期。执行 `./rec53ctl` 或 `./rec53ctl help` 时 SHALL 打印所有子命令的简要说明。

所有子命令共享以下环境变量，均可在调用时覆盖：

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `INSTALL_DIR` | `/usr/local/bin` | 二进制安装目录 |
| `CONFIG_DIR` | `/etc/rec53` | 配置文件目录 |
| `BINARY_NAME` | `rec53` | 二进制文件名 |
| `SERVICE_NAME` | `rec53` | systemd 服务名 |
| `BUILD_OUTPUT` | `dist/rec53` | 编译产物路径 |

unit 文件路径 SHALL 由变量推导：`/etc/systemd/system/${SERVICE_NAME}.service`。

#### Scenario: 无参数显示帮助
- **WHEN** 执行 `./rec53ctl` 或 `./rec53ctl help`
- **THEN** SHALL 打印所有可用子命令及其简要描述，以零状态退出

#### Scenario: 未知子命令报错
- **WHEN** 执行 `./rec53ctl foo`
- **THEN** SHALL 打印 "Unknown command: foo" 及用法提示，以非零状态退出

---

### Requirement: rec53ctl build 编译 rec53 二进制
`rec53ctl build` SHALL 执行 `go build` 将 `./cmd` 编译为二进制，输出到 `BUILD_OUTPUT`，编译前自动创建输出目录。支持通过环境变量 `GOOS`、`GOARCH` 控制目标平台，默认编译当前平台。

#### Scenario: 默认本地编译成功
- **WHEN** 执行 `./rec53ctl build`，且 Go 工具链可用
- **THEN** SHALL 在 `BUILD_OUTPUT` 生成可执行二进制，并打印输出路径

#### Scenario: 交叉编译
- **WHEN** 执行 `GOOS=linux GOARCH=amd64 ./rec53ctl build`
- **THEN** SHALL 生成目标平台二进制到 `BUILD_OUTPUT`

#### Scenario: Go 工具链缺失时报错
- **WHEN** 系统中未安装 `go`
- **THEN** SHALL 打印缺少 Go 工具链的错误信息并以非零状态退出

---

### Requirement: rec53ctl install 安装并启动 systemd 服务
`rec53ctl install` SHALL 完整执行：编译二进制 → 将二进制复制到 `INSTALL_DIR` → 处理配置文件（见配置策略）→ 写入 `/etc/systemd/system/${SERVICE_NAME}.service` unit 文件 → `daemon-reload` → `enable --now ${SERVICE_NAME}`，并打印服务状态。

配置文件处理策略：
- `CONFIG_DIR/config.yaml` 已存在且未指定 `--force-config`：跳过配置复制，不检查源 `config.yaml`
- `CONFIG_DIR/config.yaml` 不存在（首次安装）：必须有源 `config.yaml`，否则报错退出
- 指定 `--force-config`：必须有源 `config.yaml`，覆盖写入 `CONFIG_DIR/config.yaml`

#### Scenario: 首次安装
- **WHEN** 执行 `sudo ./rec53ctl install`，systemd 可用，源 `config.yaml` 存在
- **THEN** SHALL 完成编译、部署二进制、复制配置到 `CONFIG_DIR`、安装服务并启动，打印 `systemctl status ${SERVICE_NAME}` 输出

#### Scenario: 重复安装只更新二进制，不覆盖配置
- **WHEN** 服务已安装（`CONFIG_DIR/config.yaml` 已存在），再次执行 `sudo ./rec53ctl install`
- **THEN** SHALL 重新编译、更新 `INSTALL_DIR/BINARY_NAME`、重载 daemon、重启服务，但 SHALL NOT 覆盖 `CONFIG_DIR/config.yaml`

#### Scenario: --force-config 强制覆盖配置
- **WHEN** 执行 `sudo ./rec53ctl install --force-config`
- **THEN** SHALL 用源 `config.yaml` 覆盖 `CONFIG_DIR/config.yaml`

#### Scenario: 源配置文件不存在且目标已存在时跳过检查
- **WHEN** 源 `config.yaml` 不存在，但 `CONFIG_DIR/config.yaml` 已存在，且未指定 `--force-config`
- **THEN** SHALL 跳过配置复制，继续完成二进制更新和服务重启，不报错

#### Scenario: 源配置文件不存在且目标也不存在时报错
- **WHEN** 源 `config.yaml` 不存在，且 `CONFIG_DIR/config.yaml` 也不存在
- **THEN** SHALL 打印提示（建议先运行 `./generate-config.sh`）并以非零状态退出

#### Scenario: --force-config 时源配置文件不存在报错
- **WHEN** 指定 `--force-config`，但源 `config.yaml` 不存在
- **THEN** SHALL 打印提示（建议先运行 `./generate-config.sh`）并以非零状态退出

#### Scenario: systemd 不可用时报错
- **WHEN** 系统中 `systemctl` 命令不存在
- **THEN** SHALL 打印 "systemd not available" 错误并以非零状态退出

---

### Requirement: rec53ctl upgrade 原地升级已安装服务
`rec53ctl upgrade` SHALL 在不修改现有配置的前提下，重新编译并热替换已安装的 `BINARY_NAME` 二进制，重启服务使新版本生效。升级前 SHALL 备份旧二进制到 `INSTALL_DIR/${BINARY_NAME}.bak`；若启动失败 SHALL 自动回滚并以非零状态退出。

#### Scenario: 代码更新后一键升级
- **WHEN** 执行 `sudo ./rec53ctl upgrade`，服务已安装且运行中
- **THEN** SHALL 重新编译、停止 `SERVICE_NAME`、替换 `INSTALL_DIR/BINARY_NAME`、重启服务，打印升级完成摘要

#### Scenario: 跳过编译直接升级
- **WHEN** 执行 `SKIP_BUILD=1 sudo ./rec53ctl upgrade`，且 `BUILD_OUTPUT` 已存在
- **THEN** SHALL 跳过编译，直接替换 `INSTALL_DIR/BINARY_NAME` 并重启服务

#### Scenario: 升级失败时自动回滚
- **WHEN** 替换二进制后 `systemctl start ${SERVICE_NAME}` 返回非零
- **THEN** SHALL 将 `INSTALL_DIR/${BINARY_NAME}.bak` 恢复为 `INSTALL_DIR/BINARY_NAME`，重启服务，打印回滚提示，以非零状态退出

#### Scenario: 服务未安装时拒绝升级
- **WHEN** `INSTALL_DIR/BINARY_NAME` 不存在或 `${SERVICE_NAME}.service` unit 未注册
- **THEN** SHALL 打印提示（建议先运行 `./rec53ctl install`）并以非零状态退出

#### Scenario: 升级全程不修改配置文件
- **WHEN** 升级过程中 `CONFIG_DIR/config.yaml` 已存在
- **THEN** SHALL 保留现有配置文件，不做任何修改

---

### Requirement: rec53ctl uninstall 停止服务并清理受管文件
`rec53ctl uninstall` SHALL 停止并禁用 `SERVICE_NAME` systemd 服务，删除 `/etc/systemd/system/${SERVICE_NAME}.service` unit 文件和 `INSTALL_DIR/BINARY_NAME` 二进制，执行 `daemon-reload`，并打印卸载完成摘要。对于配置目录 `CONFIG_DIR`，SHALL 仅删除由 `install` 写入的受管文件（`config.yaml`）；若目录此后为空则删除目录，否则保留目录并打印提示。

#### Scenario: 完整卸载
- **WHEN** 执行 `sudo ./rec53ctl uninstall`
- **THEN** SHALL 停止 `SERVICE_NAME`、删除 `INSTALL_DIR/BINARY_NAME`、删除 unit 文件、daemon-reload；`CONFIG_DIR` 下仅删除 `config.yaml`，目录为空时删除目录

#### Scenario: CONFIG_DIR 有额外文件时保留目录
- **WHEN** `CONFIG_DIR` 中除 `config.yaml` 外还有其他文件
- **THEN** SHALL 删除 `config.yaml`，保留 `CONFIG_DIR`，并打印 "Config directory retained (not empty): $CONFIG_DIR"

#### Scenario: 服务未安装时优雅处理
- **WHEN** `${SERVICE_NAME}.service` unit 文件不存在
- **THEN** SHALL 跳过 systemctl 步骤，直接清理文件，不以错误退出

---

### Requirement: rec53ctl run 前台临时运行 rec53
`rec53ctl run` SHALL 在前台直接运行 rec53（优先使用 `INSTALL_DIR/BINARY_NAME`，回退到 `BUILD_OUTPUT`），不安装 systemd 服务。SHALL 附加 `-rec53.log /dev/stderr` 参数，强制日志输出到终端而非文件。配置文件查找顺序：`CONFIG_FILE` 环境变量（若设置）> `CONFIG_DIR/config.yaml`（若存在）> `./config.yaml`。Ctrl-C 终止进程。

#### Scenario: 使用已部署二进制临时运行，日志输出终端
- **WHEN** 执行 `./rec53ctl run`，且 `INSTALL_DIR/BINARY_NAME` 存在
- **THEN** SHALL 在前台启动 rec53 并附加 `-rec53.log /dev/stderr`，日志直接输出到终端

#### Scenario: 回退到 BUILD_OUTPUT
- **WHEN** `INSTALL_DIR/BINARY_NAME` 不存在，但 `BUILD_OUTPUT` 存在
- **THEN** SHALL 使用 `BUILD_OUTPUT` 启动，同样附加 `-rec53.log /dev/stderr`

#### Scenario: 配置文件优先使用 CONFIG_DIR
- **WHEN** `CONFIG_DIR/config.yaml` 存在，且未设置 `CONFIG_FILE`
- **THEN** SHALL 以 `CONFIG_DIR/config.yaml` 启动 rec53

#### Scenario: 配置文件回退到当前目录
- **WHEN** `CONFIG_DIR/config.yaml` 不存在，`./config.yaml` 存在，且未设置 `CONFIG_FILE`
- **THEN** SHALL 以 `./config.yaml` 启动 rec53

#### Scenario: 显式指定配置文件路径
- **WHEN** 执行 `CONFIG_FILE=/tmp/test.yaml ./rec53ctl run`
- **THEN** SHALL 以 `/tmp/test.yaml` 启动 rec53，不进行路径回退

#### Scenario: 配置文件均不存在时报错
- **WHEN** `CONFIG_FILE`、`CONFIG_DIR/config.yaml`、`./config.yaml` 均不存在
- **THEN** SHALL 打印提示（建议先运行 `./generate-config.sh` 或 `./rec53ctl install`）并以非零状态退出

#### Scenario: 二进制不存在时报错
- **WHEN** `INSTALL_DIR/BINARY_NAME` 和 `BUILD_OUTPUT` 均不存在
- **THEN** SHALL 打印提示（建议先运行 `./rec53ctl build` 或 `./rec53ctl install`）并以非零状态退出

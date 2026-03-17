## Why

rec53 目前只有 `generate-config.sh` 一个辅助脚本，缺乏覆盖完整生命周期（构建 → 部署 → 安装 systemd 服务 → 升级 → 卸载 → 临时测试）的运维工具。运维人员需要手动执行多条命令，容易遗漏步骤、配置不一致，且在 CI/CD 或新机器初始化时无法一键完成。

## What Changes

- 新增 `rec53ctl` 单入口运维脚本，通过子命令覆盖完整生命周期：
  - `build`：一键交叉编译/本地编译，输出二进制到 `dist/`
  - `install`：编译 + 部署二进制与配置文件 + 安装 systemd 服务并启动
  - `upgrade`：重编译并热替换已安装二进制，失败时自动回滚，不修改配置
  - `uninstall`：停止并禁用服务、删除二进制与配置文件、清理 systemd unit
  - `run`：前台临时运行 rec53（不安装服务），用于快速冒烟测试

## Capabilities

### New Capabilities

- `ops-scripts`: 覆盖 rec53 完整运维生命周期的单入口 shell 脚本（构建、systemd 安装/升级/卸载、临时测试）

### Modified Capabilities

（无现有 spec 需修改）

## Impact

- 新增 `rec53ctl` 脚本（项目根目录）
- 新增 `dist/` 目录（由 `build` 子命令生成，已在 `.gitignore`）
- 不修改任何 Go 源码；不影响现有测试
- 需要目标机器有 systemd（Linux），脚本在非 systemd 环境下友好报错

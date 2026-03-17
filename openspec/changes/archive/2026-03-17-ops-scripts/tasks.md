## 1. 项目结构准备

- [x] 1.1 在 `.gitignore` 中添加 `dist/` 条目（如不存在）

## 2. rec53ctl 骨架与共享变量

- [x] 2.1 创建 `rec53ctl`，在文件顶部统一定义共享变量及默认值：`INSTALL_DIR`（`/usr/local/bin`）、`CONFIG_DIR`（`/etc/rec53`）、`BINARY_NAME`（`rec53`）、`SERVICE_NAME`（`rec53`）、`BUILD_OUTPUT`（`dist/rec53`）；unit 文件路径由变量推导：`UNIT_FILE=/etc/systemd/system/${SERVICE_NAME}.service`
- [x] 2.2 实现 `help` 子命令：列出所有可用子命令及简要描述
- [x] 2.3 实现未知子命令的报错与用法提示
- [x] 2.4 `chmod +x rec53ctl`

## 3. build 子命令

- [x] 3.1 实现 `build`：检查 go 工具链是否存在，缺失则报错退出
- [x] 3.2 `build` 自动创建 `BUILD_OUTPUT` 所在目录（如不存在）
- [x] 3.3 `build` 支持 `GOOS`/`GOARCH` 环境变量，执行 `go build -o "${BUILD_OUTPUT}" ./cmd`
- [x] 3.4 `build` 编译成功后打印输出路径；编译失败则以非零状态退出

## 4. install 子命令

- [x] 4.1 实现 `install`：检查 `systemctl` 是否可用，不可用则报错退出
- [x] 4.2 `install` 检查配置条件：若 `CONFIG_DIR/config.yaml` 已存在且未指定 `--force-config`，则跳过配置检查与复制；若目标不存在或指定了 `--force-config`，则检查源 `config.yaml`，不存在时提示运行 `./generate-config.sh` 并退出
- [x] 4.3 `install` 调用 `build` 逻辑编译二进制
- [x] 4.4 `install` 创建 `CONFIG_DIR`（`mkdir -p`），复制 `INSTALL_DIR/BINARY_NAME`（`chmod 755`）；配置文件复制策略：`CONFIG_DIR/config.yaml` 不存在时复制，已存在则跳过；传入 `--force-config` 时强制覆盖
- [x] 4.5 `install` 生成 `${UNIT_FILE}` unit 文件（heredoc 内联，`ExecStart` 使用 `${INSTALL_DIR}/${BINARY_NAME} --config ${CONFIG_DIR}/config.yaml`，含 `Restart=on-failure`）
- [x] 4.6 `install` 执行 `systemctl daemon-reload && systemctl enable --now "${SERVICE_NAME}"` 并打印 `systemctl status "${SERVICE_NAME}"` 输出

## 5. upgrade 子命令

- [x] 5.1 实现 `upgrade`：检查 `${INSTALL_DIR}/${BINARY_NAME}` 存在且 `${UNIT_FILE}` 已注册，否则报错退出
- [x] 5.2 `upgrade` 备份当前二进制到 `${INSTALL_DIR}/${BINARY_NAME}.bak`
- [x] 5.3 `upgrade` 支持 `SKIP_BUILD=1` 跳过编译；默认调用 `build` 逻辑重新编译
- [x] 5.4 `upgrade` 停止服务（`systemctl stop "${SERVICE_NAME}"`）
- [x] 5.5 `upgrade` 替换二进制（复制 `${BUILD_OUTPUT}` → `${INSTALL_DIR}/${BINARY_NAME}`，`chmod 755`）；不修改 `CONFIG_DIR` 下任何文件
- [x] 5.6 `upgrade` 启动服务（`systemctl start "${SERVICE_NAME}"`），若失败则触发回滚逻辑
- [x] 5.7 实现回滚逻辑：将 `${INSTALL_DIR}/${BINARY_NAME}.bak` 恢复为 `${INSTALL_DIR}/${BINARY_NAME}`，`systemctl start "${SERVICE_NAME}"`，打印回滚提示，以非零状态退出
- [x] 5.8 升级成功后删除 `.bak` 文件，打印升级完成摘要

## 6. uninstall 子命令

- [x] 6.1 实现 `uninstall`：若 `${UNIT_FILE}` 存在则 `systemctl stop "${SERVICE_NAME}" && systemctl disable "${SERVICE_NAME}"`
- [x] 6.2 `uninstall` 删除 `${UNIT_FILE}` 并执行 `systemctl daemon-reload`（若 systemd 可用）
- [x] 6.3 `uninstall` 删除 `${INSTALL_DIR}/${BINARY_NAME}`
- [x] 6.4 `uninstall` 仅删除 `CONFIG_DIR/config.yaml`；若 `CONFIG_DIR` 此后为空则删除目录，否则保留并打印提示
- [x] 6.5 服务未安装时优雅跳过 systemctl 步骤，不以错误退出

## 7. run 子命令

- [x] 7.1 实现 `run`：按优先级查找二进制（`${INSTALL_DIR}/${BINARY_NAME}` → `${BUILD_OUTPUT}`），均不存在则报错退出
- [x] 7.2 `run` 按顺序确定配置文件：`CONFIG_FILE` 环境变量（若设置）> `CONFIG_DIR/config.yaml`（若存在）> `./config.yaml`；均不存在则报错退出
- [x] 7.3 `run` 使用 `exec` 前台启动 rec53，附加 `-rec53.log /dev/stderr` 使日志输出到终端，确保 Ctrl-C 信号正确传递

## 8. 验证与文档

- [x] 8.1 手动测试 `./rec53ctl build`：本地编译成功，`dist/rec53` 可执行
- [x] 8.2 手动测试 `./rec53ctl run`：前台启动 rec53，日志输出到终端，Ctrl-C 正常退出
- [x] 8.3 在 `README.md` 的 "Build & Run" 部分补充 `rec53ctl` 用法说明

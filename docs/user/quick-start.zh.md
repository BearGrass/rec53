# 快速开始

[English](quick-start.md) | 中文

本指南覆盖 rec53 的默认运维路径：生成配置、前台运行、验证 DNS 应答，然后再部署为服务。

## 1. 前置条件

- Go 1.21 或更高版本
- 生产部署建议使用 Linux
- 用于验证的 `dig`
- 若计划使用 `rec53ctl install`，需要 `systemd`

默认路径不需要 XDP。请先保持关闭，直到 Go 路径在你的环境中运行正常。

## 2. 生成配置

```bash
./generate-config.sh
```

这会生成 `config.yaml`，首次运行前请先检查内容。

## 3. 构建并运行

推荐方式：

```bash
./rec53ctl build
./rec53ctl run
```

手动方式：

```bash
go build -o rec53 ./cmd
./rec53 --config ./config.yaml
```

## 4. 验证基础解析

```bash
dig @127.0.0.1 -p 5353 example.com
dig @127.0.0.1 -p 5353 example.com AAAA
dig @127.0.0.1 -p 5353 example.com NS
```

检查点：

- 服务无超时响应
- 返回结果与查询类型一致
- 日志中没有反复出现启动或绑定错误

## 5. 作为服务部署

```bash
sudo ./rec53ctl install
systemctl status rec53
tail -f /var/log/rec53/rec53.log
```

安装后的常见操作：

```bash
sudo ./rec53ctl upgrade
sudo ./rec53ctl uninstall
sudo ./rec53ctl uninstall --purge
```

## 6. 首次生产上线建议

- 先使用默认 Go 路径
- 保持 `xdp.enabled: false`
- 先用本地监听地址，再按需扩大暴露范围
- 在切换节点解析流量前先验证 metrics 和日志

## 相关文档

- [配置说明](configuration.zh.md)
- [运维说明](operations.zh.md)
- [故障排查](troubleshooting.zh.md)

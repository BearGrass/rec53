# 故障排查

[English](troubleshooting.md) | 中文

先走默认路径：生成配置、前台运行、执行简单 `dig` 查询，然后再做服务部署。

## 服务无法启动

检查：

- `config.yaml` 是否存在
- `dns.listen` 是否有效且未被占用
- `dns.metric` 是否有效
- 进程是否有权限绑定配置端口

常用命令：

```bash
./rec53ctl run
ss -lntup | grep 5353
ss -lntp | grep 9999
curl -s -i http://127.0.0.1:9999/healthz/ready
```

## `rec53ctl install` 失败

检查：

- `systemd` 是否可用
- 是否以足够权限运行
- 如果安装流程需要复制配置，`config.yaml` 是否存在

常用命令：

```bash
systemctl status rec53
journalctl -u rec53 -n 100 --no-pager
tail -n 100 /var/log/rec53/rec53.log
```

`rec53ctl install` 不会覆盖未受管的 unit 或二进制。如果碰到这个保护，请先检查目标路径。

## 卸载没有删干净

这是预期行为。默认情况下：

- `sudo ./rec53ctl uninstall` 只删除受管的服务 unit 和二进制
- 配置和日志会保留，避免卸载时破坏运维数据

如果你确实要一起删除受管配置和日志，请使用：

```bash
sudo ./rec53ctl uninstall --purge
```

## 日志位置不对

先确认你用了哪种运行方式：

- `./rec53ctl run` 会写到 stderr
- 安装后的服务默认写到 `/var/log/rec53/rec53.log`
- 直接执行 `./rec53 --config ...` 时仍然使用 `-rec53.log` 参数或二进制默认值 `./log/rec53.log`

常用命令：

```bash
journalctl -u rec53 -n 100 --no-pager
tail -n 100 /var/log/rec53/rec53.log
journalctl -u rec53 -f
tail -f /var/log/rec53/rec53.log
```

## DNS 查询超时

检查：

- 服务是否真的在配置地址上监听
- 本地防火墙或网络策略是否拦截了端口
- 节点是否能访问根服务器或转发上游

尝试：

```bash
dig @127.0.0.1 -p 5353 example.com
dig @127.0.0.1 -p 5353 example.com NS
```

## 启动太慢

可能原因：

- warmup 仍在运行
- 到上游权威服务器的网络路径慢
- 重启后缓存是冷的

先看 readiness probe，再判断是不是启动故障：

```bash
curl -s http://127.0.0.1:9999/healthz/ready
```

建议这样理解：

- `ready=false` 且 `phase=cold-start`：listener 还没准备好
- `ready=true` 且 `phase=warming`：已经可以服务，只是后台 warmup 还没结束
- `ready=false` 且 `phase=shutting-down`：这是主动退出，不是新启动失败

缓解方式：

- 保持 warmup 开启
- 先在基线验证后再启用 snapshot
- 基础启动问题没排清前，不要急着启用 XDP

## forwarding 规则没有匹配

检查：

- zone 后缀是否正确
- 上游是否使用 `host:port`
- 不要假设所有 forwarding 上游失败后还会自动回退到迭代解析

记住：

- 最长后缀优先
- forwarding 响应不缓存

## XDP 不工作

先把它当作可选项，确认 Go 路径可用。

然后检查：

- Linux 内核支持
- interface 名称
- 所需权限或 capability
- 日志里是否有降级到 Go 路径的信息

如果 XDP attach 失败，rec53 仍应通过正常的 Go cache 路径继续运行。

# rec53top 页面与字段

这页是 `rec53top` 的产品级参考，说明每个屏幕和字段具体表示什么。

## 概览页

概览页是第一屏，只回答一个问题：现在最可疑的是哪个区域。

### Traffic

- `QPS`：当前窗口内的请求速率。
- `p99`：当前窗口内的尾延迟。
- `response mix`：响应码比例。

### Cache

- `hit ratio`：缓存命中占比。
- `positive hit rate`：正缓存复用率。
- `negative hit rate`：负缓存复用率。
- `miss rate`：需要继续解析的请求比例。
- `entry count`：当前缓存条目数。
- `lifecycle`：写入、刷新、过期删除等活动。

### Snapshot

- `load success`：启动恢复是否成功。
- `saved/imported/skipped`：snapshot 条目发生了什么。
- `duration`：加载或保存耗时。

### Upstream

- `timeout rate`：上游请求超时比例。
- `bad rcode rate`：SERVFAIL 或 REFUSED 一类失败。
- `fallback`：备用上游是否成功。
- `winner path`：哪条上游路径赢了竞争。

### XDP

- `status`：激活、禁用或不可用。
- `hit ratio`：XDP 命中占比。
- `sync errors`：Go 到 BPF 的同步失败。
- `cleanup`：周期清理删掉的过期条目。
- `entries`：当前活跃 XDP 条目数。

### State Machine

- `top stage`：最常进入的阶段。
- `failure reasons`：主要终态失败分类。

## 详情页

### Summary

Summary 先给当前结论，再给支撑结论的数字。

### 拆分视图

`Cache`、`Upstream` 和 `XDP` 会有子视图，比如 mix、failures、winners、paths、cleanup。它们的作用是把问题从“哪里坏了”缩小到“哪个有界分类在驱动它”。

### 状态标签

- `OK`：正常且可读
- `DEGRADED`：有数据，但信号可疑
- `DISABLED`：功能被有意关闭
- `UNAVAILABLE`：指标族缺失
- `STALE`：最近一次抓取失败
- `DISCONNECTED`：还没有成功抓取过
- `WARMING`：目前只有一个成功样本，短窗口还不稳定

## 阅读规则

先看当前结论，再看支撑计数，再看下一步建议。不要把概览卡片当成完整诊断。

# rec53top 页面与字段

这页不仅说明字段含义，也告诉你新版页面应该怎么读，避免一上来就掉进细节里。

## 先记住这条阅读顺序

`rec53top` 不适合“从左到右逐项读完”。更实用的顺序是：

1. 先看概览页里哪个面板状态最可疑。
2. 进入该面板详情，先看 `Summary` 里的结论句。
3. 只有当你已经知道“问题大概在哪一块”时，才继续看子视图。
4. `State Machine` 不要当作完整调用栈来读，它现在只负责展示聚合计数和终态信号。

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
- `top terminal`：当前增长最快的终态出口。
- `failure reasons`：主要有界失败分类。
- `top stage` 适合回答“哪里最热”，不适合单独回答“请求到底怎么走完的”。
- 如果你要看某个请求的真实路径，直接用 `./rec53 --config ./config.yaml --trace-domain example.com --trace-type A`。

## 详情页

### Summary

`Summary` 是每个详情页的入口页。正确用法是：

- 先读 `Now`
- 再看 `Window`
- 最后看 `Next`

如果只读数字不读结论，很容易把“旧累计”误当成“当前问题”。

### 拆分视图

`Cache`、`Upstream` 和 `XDP` 会有子视图，比如 mix、failures、winners、cleanup。它们的作用是把问题从“哪里坏了”缩小到“哪个有界分类在驱动它”。

经验上，只有在下面两种情况下才需要切子视图：

- `Summary` 已经指出了问题方向，但你还想知道“是哪个细分桶在主导”
- 概览页和 `Summary` 看起来不矛盾，但你想确认更细的分类分布

### State Machine 详情

`State Machine` 现在故意只保留 summary：

- `Stage mix`：聚合工作主要集中在哪些状态
- `Terminal exits`：最近这些请求主要结束在哪些终态
- `Failure reasons`：是否已有一个有界失败桶在集中

建议按这个顺序读：

1. 先看 `top stage`
2. 再看 `top terminal` 是否还是 `success_exit`
3. 如果你需要单个请求的真实路径，不要在 TUI 里猜，直接跑：

```bash
./rec53 --config ./config.yaml --trace-domain example.com --trace-type A
```

### 状态标签

- `OK`：正常且可读
- `DEGRADED`：有数据，但信号可疑
- `DISABLED`：功能被有意关闭
- `UNAVAILABLE`：指标族缺失
- `STALE`：最近一次抓取失败
- `DISCONNECTED`：还没有成功抓取过
- `WARMING`：目前只有一个成功样本，短窗口还不稳定

## 阅读规则

先看当前结论，再看支撑计数，再看下一步建议。不要把概览卡片当成完整诊断，也不要把 `State Machine` 的单个 stage 名称当成完整路径解释。

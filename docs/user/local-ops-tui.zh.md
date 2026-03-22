# 本地运维 TUI

`rec53top` 是 rec53 的本地终端面板。它直接读取 rec53 的 Prometheus 指标端点，并渲染固定六宫格视图：流量、缓存、snapshot、上游、XDP、状态机。

这篇文档是操作指南：怎么启动、哪些按键和状态重要、如何本地自测，以及已经进入 TUI 后该怎么读详情面板。

关于定位、使用场景、边界和稳定的概览入口，请先看 [rec53top 概览](rec53top.zh.md)。

## 第一次上手先看什么

如果你是第一次用新版页面，建议不要一上来就在子视图里来回切。更省力的顺序是：

1. 先留在概览页，找状态不是 `OK` 的面板。
2. 如果每个面板都是 `OK`，优先看数值明显在动的面板，比如 `Traffic`、`Cache` 或 `State Machine`。
3. 进入详情后先停在 `Summary`，只读三块：
   `Now`、`Window`、`Next`
4. 只有当 `Summary` 还不够回答你的问题时，再切去子视图。

这样可以避免把累计 totals 或趋势提示误当成第一页就必须读完的内容。

## 运行

推荐方式：

```bash
./rec53ctl top
```

手动构建：

```bash
go build -o rec53top ./cmd/rec53top
```

默认本地端点运行：

```bash
./rec53top
```

覆盖指标端点：

```bash
./rec53top -target http://127.0.0.1:9999/metric
```

常用参数：

- `-target`：指标端点，默认 `http://127.0.0.1:9999/metric`
- `-refresh`：刷新间隔，默认 `2s`
- `-timeout`：抓取超时，默认 `1500ms`
- `-plain`：输出定期纯文本摘要，不进入全屏 TUI

如果终端打开了但渲染不正常，先试显式指定终端类型：

```bash
TERM=xterm-256color ./rec53top
```

如果终端仍然不支持全屏 UI，就用纯文本兼容模式：

```bash
./rec53top -plain
```

`-plain` 会用同一套面板模型输出周期性纯文本摘要，但不会依赖全屏终端 UI。

## 按键

- `q`：退出
- `r`：立刻刷新
- `h` 或 `?`：切换帮助和状态说明
- `Left` / `Right` / `Up` / `Down`：在固定 2x3 面板网格里移动概览焦点
- `j` / `k` / `l`：向下、向上、向右移动焦点
- `Tab` / `Shift-Tab`：在概览里轮换焦点；在支持详情页里轮换钻取子视图
- `Enter`：打开当前焦点面板的详情页
- `[` / `]`：在当前详情页支持钻取时切换上一个/下一个子视图
- `1` 到 `6`：直接打开 Traffic、Cache、Snapshot、Upstream、XDP、State Machine
- `0` 或 `Esc`：回到概览页

默认导航路径：

- 先停留在概览页，把焦点移到你想看的面板上
- 按 `Enter` 打开该面板的详情页
- 如果当前面板是 `Cache`、`Upstream` 或 `XDP`，用 `Tab`、`Shift-Tab`、`Left`、`Right`、`[` 或 `]` 切换钻取子视图
- 用 `0` 或 `Esc` 回到概览页，焦点会保留在同一面板上
- `1` 到 `6` 作为快速直达键，适合你已经知道目标面板时使用

## 状态模型

TUI 使用一组固定状态：

- `OK`：面板有数据，没有明显退化信号
- `DEGRADED`：面板有数据，当前信号提示问题
- `DISABLED`：功能被有意关闭，通常是 XDP
- `UNAVAILABLE`：目标可达，但对应指标族缺失
- `STALE`：之前成功过，但最近一次抓取失败
- `DISCONNECTED`：目标还没成功抓到过数据
- `WARMING`：只有第一次成功样本，短窗口速率还没准备好

## 每个面板看什么

- `Traffic`：QPS、p99 延迟、响应码比例
- `Cache`：命中率、正/负命中率、miss 率、条目数、生命周期活动
- `Snapshot`：load/save 成功总量、导入条目、跳过条目、耗时
- `Upstream`：timeout 率、bad-rcode 率、fallback 活动、胜出路径
- `XDP`：启用/禁用状态、命中率、同步错误、清理活动、条目数
- `State Machine`：最近最热的阶段、主要终态出口，以及有界失败原因

## 详情页

全屏 TUI 可以把一个面板展开成详情页。它仍然故意保持轻量：不加历史图、不加多级页面树，但 `Cache`、`Upstream` 和 `XDP` 支持在同一详情页里钻取子视图。

最近趋势提示也同样轻量：

- 只使用当前 `rec53top` 会话里的近期样本
- 用来判断一个可疑信号是在继续升高还是已经开始降温
- 不能替代 Prometheus 或 Grafana 的长时间历史

每个详情页都按同样顺序阅读：

- `status`：当前面板状态
- `Now`：当前最主要的信号、异常，或为什么这个面板暂时还不能解释
- `Window`：支撑这个结论的主要短窗口数值
- 该面板特有的 breakdown 区块，比如响应分布、lookup 分布、winner 分布或失败原因
- 可选的 `Trend`：给选定指标的一条很短的会话内趋势提示
- `Next`：下一步该去 rec53top 的哪里看，或者去日志哪里看

支持钻取的面板：

- `Cache`：`Summary`、`Lookup Mix`、`Lifecycle`
- `Upstream`：`Summary`、`Failures`、`Winners`
- `XDP`：`Summary`、`Packet Paths`、`Sync/Cleanup`
- `State Machine`：只有 summary

## 现在的 State Machine 怎么读

这个面板现在只回答聚合问题，不再试图在 TUI 里重建一条“主路径图”。

- `top stage`：最近哪个 resolver stage 最热
- `top terminal`：最近这些请求主要结束在什么终态
- `top failure`：是否已经有一个失败桶在聚集
- `Stage mix` / `Terminal exits` / `Failure reasons`：用来支撑上面的结论

如果你要看“某个域名这一次到底怎么走完的”，不要在 TUI 里猜，直接用：

```bash
./rec53 --config ./config.yaml --trace-domain example.com --trace-type A
```

这个命令会跑一次真实解析，并打印有序状态、最终终态和 rcode。

推荐用法：

- 第一轮排查先停留在概览页
- 用方向键、`j/k/l` 或 `Tab` 把焦点移到可疑面板
- 当某个面板最可疑、你想看当前结论和最相关的 breakdown 时，按 `Enter`
- 如果面板支持钻取，用 `Tab` / `Shift-Tab`、`Left` / `Right` 或 `[` / `]` 切换子视图
- `Summary` 先看结论页，再用专题子视图缩小范围
- `State Machine` 里先看 summary 计数器；如果要追单个请求路径，直接切到 trace mode
- `1` 到 `6` 适合已经知道目标面板时快速直达
- 用 `0` 或 `Esc` 回到概览页

非正常状态也会在详情页里直接解释：

- `WARMING`：只有一个成功抓取样本，短窗口速率还不稳定
- `UNAVAILABLE`：该面板需要的指标族没有被抓到
- `DISABLED`：功能被有意关闭，通常是 XDP
- `DISCONNECTED`：rec53top 还没拿到过一次成功抓取
- `STALE`：最近一次抓取失败，所以在显示旧数据，并附带抓取排障提示

`-plain` 设计上只输出概览。

## 本地自测

1. 启动本地 rec53。

```bash
./rec53ctl run
```

2. 另开一个终端，打开 TUI。

```bash
./rec53top
```

3. 产生流量。

```bash
for i in {1..20}; do dig @127.0.0.1 -p 5353 example.com >/dev/null; done
for i in {1..10}; do dig @127.0.0.1 -p 5353 github.com >/dev/null; done
for i in {1..10}; do dig @127.0.0.1 -p 5353 nosuchname1234.example. >/dev/null; done
```

4. 检查首轮变化：

- 第一次成功抓取可能会显示 `WARMING`；下一次刷新后，基于速率的字段应该有意义
- `Traffic` 会出现非零 QPS
- `Cache` 会从 warming 进入可见的命中或 miss 速率
- 详情页（`1` 到 `6`）会显示简短的 `Now` 结论，而不是只重复概览数字
- 退化或不可用面板会显示 `Next`，指向下一块最可能的面板或排障方向
- `State Machine` 会显示类似 `cache_lookup`、`forward_lookup`、`return_resp` 的活跃阶段
- `State Machine` 详情会直接给出 `Stage mix`、`Terminal exits`、`Failure reasons`
- 类似 `./rec53 --config ./config.yaml --trace-domain example.com --trace-type A` 的 trace 命令会在 TUI 之外打印单次真实路径
- `Upstream` 在迭代查询真的触达上游时会显示胜出路径活动
- `Upstream` 有问题时会显示 fallback 或 timeout 活动
- 正常非 XDP 部署里，`XDP` 应显示 `DISABLED`，而不是假装健康

如果你需要比 TUI 摘要更深入的分析，请继续看 [Metrics](../metrics.zh.md)、[Observability Dashboard](observability-dashboard.zh.md) 和 [Operator Checklist](operator-checklist.zh.md)。

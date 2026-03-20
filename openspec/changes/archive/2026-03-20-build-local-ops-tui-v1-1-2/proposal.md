## Why

`v1.1.1` 已经把 rec53 的指标面补齐，但当前要看这些信号仍然需要 Prometheus、Grafana 或手工写 PromQL。对开发自测和运维首轮排查来说，这个门槛还是偏高；`v1.1.2` 需要把现有 `/metric` 能力直接变成一个本地可打开、可刷新、可快速读懂的终端看板。

## What Changes

- 提供一个本地终端 TUI MVP，直接读取 rec53 的 Prometheus text metrics 暴露，不依赖 Prometheus server、Grafana 或外部数据库。
- 明确 `v1.1.2` 的技术目标：把 `traffic / cache / snapshot / upstream / XDP / state machine` 六类核心信号收敛成固定布局、低认知负担的只读看板。
- 明确给开发者的业务目标：本地跑 `rec53` 后，无需先搭 Prometheus，即可判断“请求有没有进来、缓存有没有工作、上游是否退化、状态机卡在哪一层”。
- 明确给运维/用户的业务目标：在单机或节点本地部署场景下，可以先打开一个本地终端看板判断“服务正常 / 退化 / 不可达”，再决定是否进入 PromQL、日志或 pprof 深挖。
- 将 `v1.1.2` 范围收敛为单实例、单 target、只读、当前状态与短窗口速率，不承诺多实例聚合、历史趋势持久化、告警管理或交互式 drill-down。

## Capabilities

### New Capabilities
- `local-ops-tui`: 提供一个单实例、本地终端使用的 rec53 运行看板，直接消费 `/metric` 数据并展示固定健康面板。
- `local-ops-tui-docs`: 提供 TUI 的启动方式、自测路径、面板含义和已知边界说明，方便开发与运维直接使用。

### Modified Capabilities

## Impact

- 主要影响代码：新增 TUI 目录与独立命令入口，新增 metrics scrape / parse / view-model / render 逻辑。
- 主要影响文档：README、用户文档、roadmap，以及 TUI 的使用与自测说明。
- 预计会引入终端 UI 依赖，但不改变主服务 `rec53` 的启动路径、递归解析逻辑或现有 metrics 契约。
- 该变更是本地运维体验增强，不引入新的网络服务端口，也不改变线上运行语义。

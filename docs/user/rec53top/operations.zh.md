# rec53top 操作说明

这页说明怎么运行 `rec53top`、怎么操作界面、以及怎么本地自测。

## 运行

推荐：

```bash
./rec53ctl top
```

手动构建：

```bash
mkdir -p dist && go build -o dist/rec53top ./cmd/rec53top
```

默认运行：

```bash
./dist/rec53top
```

自定义目标：

```bash
./dist/rec53top -target http://127.0.0.1:9999/metric
```

## 按键

- `q`：退出
- `r`：立即刷新
- `h` 或 `?`：切换帮助
- 方向键 / `j k l`：移动焦点
- `Tab` / `Shift-Tab`：轮换焦点或钻取子视图
- `Enter`：打开当前焦点面板的详情页
- `[` / `]`：切换支持的详情子视图
- `1` 到 `6`：直接跳到某个面板
- `0` 或 `Esc`：回到概览页

## 本地自测

1. 启动 rec53。
2. 启动 `rec53top`。
3. 制造流量。

```bash
for i in {1..20}; do dig @127.0.0.1 -p 5353 example.com >/dev/null; done
for i in {1..10}; do dig @127.0.0.1 -p 5353 github.com >/dev/null; done
for i in {1..10}; do dig @127.0.0.1 -p 5353 nosuchname1234.example. >/dev/null; done
```

4. 检查流量、缓存、上游和状态机面板是否开始变化。

## 阅读习惯

1. 先看概览。
2. 打开可疑面板。
3. 读 `Now`。
4. 只有当结论指向有界原因时，再看 breakdown。
5. 需要更广的排查时，再去看日志或更大范围的可观测性文档。

更广的事故处理请看 [观测看板布局](../observability-dashboard.zh.md) 和 [运维排查清单](../operator-checklist.zh.md)。

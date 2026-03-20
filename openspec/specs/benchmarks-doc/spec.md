## ADDED Requirements

### Requirement: benchmarks.md 包含所有性能基准数据
`docs/testing/benchmarks.md` SHALL 包含从 README.md 迁移的全部性能数据：首包解析延迟表格（Cold start / IPPool only / Full warmup / Cache hit）、缓存容量估算表格、缓存命中 QPS 表格、IP Pool 容量表格，以及自定义 benchmark 的运行命令（`BenchmarkFirstPacket`、`BenchmarkFirstPacketComparison`）。

#### Scenario: 性能数据完整迁移
- **WHEN** `docs/testing/benchmarks.md` 被创建
- **THEN** README 中所有延迟/QPS/容量表格 SHALL 出现在该文件中，且数据不被修改

#### Scenario: Benchmark 命令可执行
- **WHEN** 开发者按照 `docs/testing/benchmarks.md` 中的命令运行 benchmark
- **THEN** 命令 SHALL 能在仓库根目录下正确执行（路径 `./e2e/...` 有效）

### Requirement: benchmarks.md 包含测试环境说明
`docs/testing/benchmarks.md` SHALL 包含测试环境说明（硬件、网络条件），并注明结果因环境而异。

#### Scenario: 环境声明
- **WHEN** 读者查看性能数据
- **THEN** 文件顶部或数据表格前 SHALL 有测试硬件和网络环境的注释说明

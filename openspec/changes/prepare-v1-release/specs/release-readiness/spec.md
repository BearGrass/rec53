## ADDED Requirements

### Requirement: v1 发布基线强调默认可部署路径
The project SHALL define a v1 release baseline that prioritizes the default deployable path over feature expansion, with clear distinction between default, optional, and platform-specific capabilities.

#### Scenario: 用户评估是否可用于初步部署
- **WHEN** 用户阅读 README 和用户文档
- **THEN** 其 SHALL 能分辨哪些能力属于默认推荐路径，哪些仅是可选增强或平台相关能力

### Requirement: 发布前收敛范围受控
The v1 release preparation SHALL focus on documentation cleanup, startup/shutdown stability, configuration clarity, and targeted tests, and SHALL NOT expand scope with new core features.

#### Scenario: 执行发布准备任务
- **WHEN** 维护者按发布任务推进
- **THEN** 任务列表 SHALL 以收敛、校验和清理为主，而不是新增功能开发

### Requirement: 发布前验证覆盖默认路径
The v1 release preparation SHALL verify the minimum supported operator workflow, including config generation, binary run, service-oriented deployment path, and basic DNS validation.

#### Scenario: 验证默认路径
- **WHEN** 维护者执行发布前验证
- **THEN** 其 SHALL 至少检查生成配置、启动 rec53、执行基本 `dig` 查询和查看关键日志/指标是否正常

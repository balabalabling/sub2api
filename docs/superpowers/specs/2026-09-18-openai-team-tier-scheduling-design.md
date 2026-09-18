# OpenAI Team 与 Team Pro 调度映射设计

## 背景

当前 OpenAI 订阅优先调度仅识别 `plus` 与 `pro` 两种 OAuth 套餐：

```text
PLUS -> PRO -> API Key -> COMPAT
```

生产账号中已经出现两种额外套餐：

- `team`
- `self_serve_business_prolite`

两者目前都落入 `COMPAT` 池，排在 API Key 之后。生产回读显示，账号 `#15 zhuji plus 3` 与 `#16 zhuji team 高级版` 均处于启用、可调度状态，但创建后尚未获得请求；相同分组内的 PLUS 账号持续获得请求。

## 目标

1. 普通 Team 按 PLUS 的调度和额度语义处理。
2. Team 高级版按 PRO 的调度和额度语义处理。
3. 保留上游返回的原始 `credentials.plan_type`，避免 OAuth 刷新或 429 套餐同步造成配置漂移。
4. 管理页面将 `self_serve_business_prolite` 显示为 `Team Pro`。
5. 保持现有 PLUS、PRO、API Key 与未知套餐行为稳定。

## 非目标

- 不修改账号 Token、组织 ID、工作区 ID或上游请求路由。
- 不新增数据库字段或数据迁移。
- 不改变现有调度池的总体顺序。
- 不为未知的 `self_serve_business_*` 套餐自动推断等级。

## 方案选择

### 采用方案：保留原始套餐，增加本地语义映射

保留：

```text
credentials.plan_type = team
credentials.plan_type = self_serve_business_prolite
```

调度时映射：

```text
team                         -> PLUS
self_serve_business_prolite  -> PRO
```

显示时映射：

```text
team                         -> Team
self_serve_business_prolite  -> Team Pro
```

选择该方案的原因：

- 上游原始值继续作为真实数据源。
- OAuth 刷新和 429 套餐同步写回原始值后，调度行为保持稳定。
- 无需引入可漂移的手工套餐覆盖值。
- 运维排查仍能看到上游实际返回的套餐名称。

### 未采用方案：直接写入 `team pro`

`team pro` 不是当前上游返回的 canonical 值。直接写入会使其在现有代码中继续落入 `COMPAT`，并可能在 OAuth 刷新或 429 套餐同步时被 `self_serve_business_prolite` 覆盖。同时会隐藏上游原始套餐信息。

### 未采用方案：新增管理员调度等级覆盖字段

单独的 `scheduler_tier_override` 更灵活，但需要新增表单、API、持久化、审计与冲突优先级。本次两个 canonical 套餐已有明确映射，额外覆盖机制超出当前需求。

## 调度行为

调度池顺序保持：

```text
PLUS/Team -> PRO/Team Pro -> API Key -> COMPAT
```

### 普通 Team

`plan_type=team` 进入 PLUS 池：

- 参与 PLUS 的 5 小时额度余量排序。
- 新鲜额度快照中，剩余额度越多越优先。
- 新鲜快照达到 100% 使用率时退出 PLUS 候选池。
- 缺失或过期的额度快照继续保持候选资格，沿用现有 PLUS 容错策略。
- 继续受账号优先级、并发、排队、错误率、首包延迟、接口能力和模型能力约束。

### Team Pro

`plan_type=self_serve_business_prolite` 进入 PRO 池：

- 排在 PLUS/Team 之后、API Key 之前。
- 5 小时数据保留用于展示和诊断。
- 5 小时数据不参与候选排除、调度阈值暂停、自动额度重置或自动暂停。
- 7 天数据继续参与现有阈值与诊断逻辑。
- 继续受账号优先级、并发、排队、错误率、首包延迟、接口能力和模型能力约束。

### 粘性与故障回退

- Team 沿用 PLUS 的额度耗尽粘性释放规则。
- Team Pro 沿用 PRO 的粘性规则，5 小时数据不触发释放。
- 上游返回可重试错误时，继续按现有失败账号排除和后续池回退策略执行。
- `previous_response_id`、`session_hash` 与 guardian parent 行为维持现状。

## 实现边界

### 后端

在 OpenAI 套餐到调度池的单一映射入口中增加 canonical 别名：

- `team` 与 `plus` 返回 PLUS 池。
- `self_serve_business_prolite` 与 `pro` 返回 PRO 池。

所有依赖 `openAIAccountPoolFor` 的逻辑自动获得一致语义，包括：

- 分层账号选择
- PLUS 额度排序与耗尽过滤
- PRO 5 小时门槛豁免
- 调度阈值判断
- 自动额度重置与自动暂停
- 调度指标中的池归属
- 粘性账号资格判断

不修改 `credentials.plan_type` 的写入、刷新或 429 同步流程。

### 前端

统一扩展套餐显示标签映射：

- `self_serve_business_prolite` 显示为 `Team Pro`。
- `team` 继续显示为 `Team`。
- 编辑账号时保留 canonical 原始值，不在保存过程中转换成显示标签。

## 兼容性

- `plus`、`pro`、`free`、API Key 行为保持原样。
- 其他未知套餐继续进入 `COMPAT`。
- 现有数据库无需迁移。
- 已保存的 Team 与 Team Pro 账号在新版本启动后立即按新映射参与调度。
- OAuth 刷新和 429 同步继续保存上游 canonical 值。

## 测试设计

### 后端单元测试

1. `team` 映射为 PLUS。
2. 大小写和首尾空格形式的 `team` 仍映射为 PLUS。
3. `self_serve_business_prolite` 映射为 PRO。
4. 大小写和首尾空格形式的 `self_serve_business_prolite` 仍映射为 PRO。
5. 未知 business 套餐仍进入 COMPAT。
6. Team 与 PLUS 同池时按新鲜 5 小时剩余额度降序选择。
7. Team 新鲜 5 小时使用率 100% 时进入下一层调度。
8. Team Pro 即使存在 100% 的 5 小时快照仍保持候选资格。
9. Team Pro 的调度阈值忽略 5 小时信号并保留 7 天信号。
10. Team Pro 的自动重置与自动暂停忽略 5 小时触发条件。
11. 调度决策指标分别把 Team 记录为 PLUS、Team Pro 记录为 PRO。

### 前端测试

1. `self_serve_business_prolite` 显示为 `Team Pro`。
2. 编辑账号后仍保存 `self_serve_business_prolite` 原始值。
3. `team` 继续显示为 `Team`。
4. 未知套餐继续原样显示。

### 回归验证

- 后端相关调度和额度测试。
- 前端套餐标签与账号编辑测试。
- 后端编译与前端类型检查。
- 生产发布后回读账号池选择日志及账号用量，确认 #15、#16 获得符合层级的请求。

## 上线验收标准

1. Team 账号在存在多个 PLUS/Team 账号时按 5 小时剩余额度参与选择。
2. Team Pro 在所有 PLUS/Team 候选耗尽或失效后、API Key 之前参与选择。
3. Team Pro 的 5 小时值不会触发调度暂停或自动重置。
4. 数据库中的原始 `plan_type` 保持 `team` 与 `self_serve_business_prolite`。
5. 管理页面显示 `Team` 与 `Team Pro`，不再直接展示 `self_serve_business_prolite`。
6. 现有 PLUS、PRO 和 API Key 调度测试继续通过。

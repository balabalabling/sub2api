# Sub2API 发布检查清单与问题记录

本文记录每次升级、合并、镜像构建和 VPS 发布的固定顺序，以及已经遇到过的失败原因，避免把问题留到远端长时间构建后才发现。

## 发布门禁顺序

1. **同步并确认基线**
   - `git fetch origin --prune`
   - 确认当前分支包含 `origin/main`，再合并官方版本或上游补丁。
   - 先查看官方 release、依赖变更和迁移说明。
2. **合并后立即做依赖安全预检**
   - 后端：在有 Go 工具链的环境执行 `govulncheck ./...`。
   - 前端：执行 `pnpm audit --prod --audit-level=high`，异常只通过 `.github/audit-exceptions.yml` 中有负责人和到期时间的条目放行。
   - 重点检查 `go.mod`/`go.sum` 是否引入新的高危依赖；依赖升级后重新跑安全预检。
3. **先跑相关功能测试，再提交**
   - OpenAI 调度改动至少覆盖：PLUS/Team、PRO/Business Premium、配额耗尽回退、并发已满回退、API Key 最终兜底。
   - 生成文件冲突（尤其 `wire_gen.go`）要检查重复 provider 声明。
   - 本机缺少 Go 时，使用前端本地检查并把后端测试交给 Actions；不要把本地未验证状态当成发布通过。
4. **推送功能分支并等待两条工作流**
   - CI：前端、后端单元/集成测试、Lint、部署脚本。
   - Security Scan：`govulncheck` 和前端生产依赖审计。
   - 两条工作流都成功后再进入 `main`，记录 run URL、commit 和失败修复。
5. **更新 `main` 并等待镜像构建**
   - `.github/workflows/docker.yml` 只对 `main`/`master` push 构建并推送 `ghcr.io/balabalabling/sub2api:latest`。
   - 功能分支 CI 通过并不代表 `latest` 镜像已经更新；必须确认 Docker workflow 成功。
6. **VPS 发布前后核对**
   - 发布前记录当前容器状态和镜像摘要。
   - `/opt/sub2api` 执行 `docker compose pull sub2api`、`docker compose up -d sub2api`。
   - 发布后检查 `docker compose ps`、健康状态、最近日志和镜像摘要；异常时保留旧摘要用于回滚。

## 已记录的问题与修复

### 2026-09-18：官方 v0.2.5 合并

- **生成文件重复 provider**：合并 `backend/cmd/server/wire_gen.go` 时出现重复的 `ollamaCloudUsageService` 声明，导致远端 golangci-lint 失败。修复为保留单一声明，并重新运行 CI。
- **依赖安全扫描晚于编译反馈**：官方 v0.2.5 的 `google.golang.org/grpc v1.82.1` 被 `govulncheck` 报告两条公告。已跟随上游安全提交升级到 `v1.83.2`，之后 Security Scan 通过。
- **Team 套餐别名改变调度路径**：按官方套餐识别将 `team` 纳入 PLUS 后，原有“订阅账号并发已满时回退 API Key”测试暴露出高优先级池过早返回等待计划的问题。现改为先尝试后续可用池；只有后续池为空或不兼容请求时，才保留高优先级池等待计划。
- **本机工具链不完整**：本机没有 Go/Docker，后端编译、单元测试、集成测试和镜像构建统一交给 GitHub Actions；前端使用仓库现有二进制完成类型检查和关键 Vitest。
- **分支不会更新生产镜像**：功能分支 CI 成功后，`latest` 仍不会自动变化；必须把已验证提交推进到 `main`，再等待 Docker workflow 成功后发布 VPS。
- **前端发布后仍显示旧套餐名**：生产镜像已包含新映射，但升级前打开的 SPA 页面仍可能持有旧资源；发布验收要重新请求入口 HTML 并强制刷新管理页，确认 `team` 显示为 `Business Standard`、`self_serve_business_prolite` 显示为 `Business Premium`。
- **Go 格式检查发现过晚**：手工新增 Go 测试后第一次远端 Lint 因 `gofmt` 失败。以后提交前先执行格式化，并在本机缺少 Go 工具链时按 gofmt 对齐规则复核结构体字段和复合字面量。
- **1% 配额边界的浮点误差**：按 `100-used` 计算剩余比例时，`99%` 使用量可能产生略大于 `1%` 的浮点结果。PLUS 长期窗口门控改为直接判断已用比例 `>=99%`，并补充周/月、已重置和 2% 剩余的边界测试。

### 2026-09-18：PLUS 长期额度门控发布

- 发布提交：`9b781e438`。
- 行为：PLUS/Team 的 7 天或 30 天/月窗口任一已知剩余量不超过 1% 时退出 PLUS 池；窗口缺失或已重置时继续参与；PRO 不应用此门控，后续仍按 PRO、API Key 回退。
- 功能分支 CI：`35344700698`；Security Scan：`35344700510`；Docker 镜像构建：`35345716927`，均成功。
- VPS 发布前镜像：`sha256:1acff674a351315ae9da0902d440cd2222d3f3d92a5ea7c0479c6f8e9e8c89b1`。
- VPS 发布后镜像：`sha256:6ed4274a1c95e2c8966c378062f50e722ebc51cc40687450dd860bcfc7104801`。
- `sub2api`、PostgreSQL、Redis 均为 healthy，`/health` 返回 `{"status":"ok"}`，发布后的 OpenAI 请求返回 200。
- 已建立每周六 03:00 自动升级任务：发现官方稳定更新后先检查冲突、安全扫描和完整 CI；全部通过才更新 `main`、构建镜像并发布 VPS。

### 2026-09-20：PLUS 最后 1% 预留缓冲发布

- 根因确认：生产库中 PLUS 账号到 `codex_7d_used_percent=99` 后没有新的用量记录，粘性资格检查也已覆盖普通 session、guardian 和 previous-response 续话。最后 1% 被耗尽来自响应后才更新的上游额度快照：请求在 98% 快照时获准，单次大上下文或同一快照下的并发请求可直接把窗口推进到 100%。
- 行为调整：PLUS/Team 的周/月窗口在已用量达到 98% 时停止新准入。2% 头寸由“目标保留 1% + 快照滞后缓冲 1%”组成；旧粘性会话下一次选号时解除 PLUS 绑定并回退到 PRO。PRO 仍不应用 5 小时门槛。
- 回归测试：覆盖周窗口正好 98%、略低于 98%，以及已绑定 PLUS 会话到达准入线后重新选择 PRO。
- 发布提交：功能提交 `cb1c51e56`，格式修复与最终发布提交 `d551a93cb`。
- 首轮 CI 的 golangci-lint 因新增常量未经 `gofmt` 对齐而失败；下载 Go 1.27 工具链到项目 `tmp` 后完成格式化。Windows 本机定向编译受分页文件过小影响退出，因此不以本机结果放行，最终以 GitHub Linux 的单元测试、集成测试和 Lint 全部成功为门槛。
- 最终 CI：`35499101813`、`35499098959`；Security Scan：`35499101808`、`35499098982`；Docker 镜像构建：`35499101812`，均成功。
- VPS 发布前镜像：`sha256:6ed4274a1c95e2c8966c378062f50e722ebc51cc40687450dd860bcfc7104801`。
- VPS 发布后镜像：`sha256:b29bb14ae40dcdfa1417b871d72e208f902e7fc2dc788f7026c2820c924a233e`。
- `sub2api`、PostgreSQL、Redis 均为 healthy，`/health` 返回 `{"status":"ok"}`；发布后的 OpenAI 请求使用账号 13 并返回 200。

## 每次发布必须留下的记录

- 功能分支、最终 commit、官方版本/上游提交。
- CI 和 Security Scan 的 run URL 与结论。
- Docker workflow 的 run URL、镜像 digest/commit tag。
- VPS 发布前后的容器状态、健康检查和异常日志摘要。
- 本次新增问题、根因、修复提交，以及是否补充了回归测试或门禁。

### 2026-09-25：官方 v0.2.8 合并阻塞（已解决）

- 官方稳定版：`v0.2.8`，发布日期 `2026-09-23`；annotated tag `d7a82d78ca51d42be41cb4daa3510ea401defe9f`，剥离提交 `fd80b08c90b55edcad5b00171b53f08721d30da1`。
- 基线：`origin/main`=`0e6d20d6e`；`v0.2.8` 相对基线新增 308 个提交；`upstream/main`=`a3eb7ef30` 另有 1 个发布后版本同步提交。
- 独立分支：`codex/upgrade-v0.2.8`。合并因以下 4 个冲突中止：`backend/cmd/server/wire_gen.go`、`backend/internal/handler/openai_images.go`、`backend/internal/service/gateway_service.go`、`backend/internal/service/setting_parse.go`。
- 关键风险：需要同时保留 PLUS→PRO→API Key 调度、上游 sticky-session/Claude Code 设置、StoreHandler 与 OpenCode Go provider；官方 tag 同时删除自定义迁移 `145_storefront.sql`、`151_subscription_plan_key_quota.sql`、`152_payment_order_api_key_target.sql`，并移除对应 Ent 字段，需先完成数据与生成代码方案。
- 官方新增迁移 `238b_content_moderation_engine_meta.sql`、`239_channel_reasoning_effort_multipliers.sql`、`240_affiliate_ledger_operation_id.sql` 为待合并审查项；未发现 `go.mod`、`go.sum`、`package.json` 或前端锁文件变化，仅新增 release-tool 依赖清单。
- `v0.2.8` tag 内 `backend/cmd/server/VERSION` 仍为 `0.2.7`，`upstream/main` 的后续提交才同步为 `0.2.8`。
- CI/Security/Docker run URL：未生成；测试、安全预检、提交、推送、镜像构建与发布均未执行。VPS 仅做只读回读：三容器 healthy，`/health`=`{"status":"ok"}`，生产镜像 `ghcr.io/balabalabling/sub2api@sha256:4258fc0072ef191376f7455dc6b0a18cc4a2bf5e79e881b4427c8276521b1810`。
- 处理结果：4 个冲突已解决；现有调度、自定义迁移/Ent 字段和 `wire_gen.go` provider 唯一性均已验证，详见 2026-09-26 发布完成记录。

### 2026-09-26：官方 v0.2.8 冲突解决（本地门禁完成，远程发布待执行）

- 合并提交：`68147dc46`（`merge: integrate official Sub2API v0.2.8`）；官方稳定 tag：`v0.2.8`。
- 冲突处理：`backend/cmd/server/wire_gen.go` 同时保留 Ollama Cloud 与 OpenCode Go provider；`backend/internal/handler/openai_images.go` 同时保留图片模型 `routingAccountIDs` 与官方 `RequiredCapabilityForModel(channelMapping.MappedModel)`；`backend/internal/service/gateway_service.go` 同时保留本地调度池/回退字段与官方 sticky-session 字段；`backend/internal/service/setting_parse.go` 加入 Claude Code 默认设置并保留 PLUS→PRO→API Key 默认优先级。
- 图片路由决策：保留兼容代码；生产当前没有启用中的 `model_routing` 分组，但 `OpenAI-plus` 仍保存关闭状态的 `gpt-image-* -> [9]` 历史规则。
- 数据迁移：保留本地 `145_storefront.sql`、`151_subscription_plan_key_quota.sql`、`152_payment_order_api_key_target.sql` 及相关 Ent 字段；纳入官方 `238b_content_moderation_engine_meta.sql`、`239_channel_reasoning_effort_multipliers.sql`、`240_affiliate_ledger_operation_id.sql`，迁移测试通过。
- 本地验证：Go 1.27.0 下 `go test ./... -count=1` 通过；`govulncheck ./...` 报告代码调用链 0 个漏洞；前端全量 334 个测试文件、2487 个测试通过；`vue-tsc -b` 与 Vite 生产构建通过。
- 前端审计：`xlsx` 的 2 个 high 为既有审计例外，`python tools/check_pnpm_audit_exceptions.py --audit frontend/audit.json --exceptions .github/audit-exceptions.yml` 通过；例外到期日为 `2026-10-06`。
- 合并期间发现并处理：官方新增的 URL 归一化测试按官方默认 `OpenAI` provider 查找，与本地保留的历史 `go2me` 默认配置冲突；测试改为验证本地 `go2me` provider 的 `/v1` URL 归一化，图片/调度行为未改变。
- CI/Security/Docker run URL：已在下方“发布完成”记录回填。
- 镜像摘要与 VPS 发布前后状态：已在下方“发布完成”记录回填。
- 当前状态：CI、Security Scan、Build Docker Image 均成功，已记录旧镜像摘要并完成生产发布。

### 2026-09-26：官方 v0.2.8 发布完成

- **发布提交**：`bb99583c02220a54f55805fdd80b2cd3e5e4b80b`；功能合并提交 `68147dc46`；官方稳定版 `v0.2.8`，tag SHA `d7a82d78ca51d42be41cb4daa3510ea401defe9f`；对应 `upstream/main` 为 `a3eb7ef302961cba716dc78b39b93b60c467db0e`。
- **冲突与兼容**：4 个冲突文件已解决；保留图片路由兼容代码、PLUS→PRO→API Key 调度池和本地迁移/Ent 字段，同时接入官方图片能力判断、sticky session、Claude Code 默认设置及新增迁移；`wire_gen.go` provider 无重复声明。
- **验证结果**：Go 全量测试、`govulncheck`、前端 2487 项测试、`vue-tsc`、Vite 构建和前端审计例外检查均通过。`xlsx` 的 2 个 high 仍属于既有例外，当前有效期至 `2026-10-06`。
- **GitHub Actions**：
  - CI：<https://github.com/balabalabling/sub2api/actions/runs/36236893474>，成功。
  - Security Scan：<https://github.com/balabalabling/sub2api/actions/runs/36236893471>，成功。
  - Build Docker Image：<https://github.com/balabalabling/sub2api/actions/runs/36236893482>，成功。
- **镜像**：`ghcr.io/balabalabling/sub2api:latest` 与提交标签 `bb99583c02220a54f55805fdd80b2cd3e5e4b80b` 指向同一镜像，digest 为 `sha256:d68e005d11345bfa30488b3ec8b21bb846b4fd51c536067f8f142f3034b48a03`。
- **VPS 发布前**：`sub2api`、PostgreSQL、Redis 均为 healthy；旧镜像 digest 为 `sha256:4258fc0072ef191376f7455dc6b0a18cc4a2bf5e79e881b4427c8276521b1810`。
- **VPS 发布动作**：在 `/opt/sub2api` 仅执行 `docker compose pull sub2api` 与 `docker compose up -d sub2api`，PostgreSQL/Redis 未重建。
- **VPS 发布后**：三个容器均为 healthy；`http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`；运行镜像 digest 为 `sha256:d68e005d11345bfa30488b3ec8b21bb846b4fd51c536067f8f142f3034b48a03`。启动后的 OpenAI Responses 请求返回 200，图片生成桥接日志正常出现；无需回滚。
- **新发现问题**：本次未发现阻断发布的问题。启动日志仍提示 `server.trusted_proxies` 与 `CORS allowed_origins` 未配置，当前按既有生产配置继续运行，后续需结合反向代理和访问来源人工确认是否补充。

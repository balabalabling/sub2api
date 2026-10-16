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

### 2026-09-26：xlsx 审计例外修复与生产发布

- **官方版本基线**：当前 `main` 已包含官方稳定版 `v0.2.8`；本次为其后的前端依赖安全修复，不是新增官方 release。未改动图片路由兼容代码、PLUS→PRO→API Key 调度和数据库迁移。
- **修复提交**：`ebe461705756ae590ecc25034a98d996159b5b02`（`fix: remove xlsx audit exceptions`）。
- **变更内容**：将 `frontend/package.json` 的 `xlsx` 固定到 SheetJS `0.20.3` 官方 CDN tarball，刷新 `frontend/pnpm-lock.yaml` 并移除旧版 `0.18.5` 及其专属依赖；删除 `.github/audit-exceptions.yml` 中 `GHSA-4r6h-8v6p-xvw6` / `GHSA-5pgg-2g8v-p4x9` 两条例外。管理员 `UsageView` 仍保持动态导入。
- **本地验证**：`pnpm install --frozen-lockfile` 通过；管理员 UsageView 测试 `14/14` 通过。此前本轮前端全量 `334` 个测试文件、`2487` 个测试、`vue-tsc -b`、Vite 生产构建和审计例外校验均通过。
- **审计结果**：`xlsx` 及对应两个 CVE/GHSA 已从审计结果消失；`high=0`、`critical=0`、`moderate=13`、`low=1`。`pnpm audit` 的退出码仍为 1，原因是剩余 moderate/low 依赖项；Security Scan 的 high/critical 门禁已通过。
- **GitHub Actions**：
  - CI：<https://github.com/balabalabling/sub2api/actions/runs/36245614751>，成功。
  - Security Scan：<https://github.com/balabalabling/sub2api/actions/runs/36245614806>，成功。
  - Build Docker Image：<https://github.com/balabalabling/sub2api/actions/runs/36245614775>，成功。
- **镜像**：`ghcr.io/balabalabling/sub2api:latest` 与提交标签 `ebe461705756ae590ecc25034a98d996159b5b02` 指向同一 digest：`sha256:278a1f38e9597176fb7b20fe2d4e593ab66012025edf5cc672133f7eb585e746`。
- **VPS 发布前**：`sub2api`、PostgreSQL、Redis 均为 healthy；旧镜像 digest 为 `sha256:d68e005d11345bfa30488b3ec8b21bb846b4fd51c536067f8f142f3034b48a03`；`http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`。
- **VPS 发布动作**：在 `/opt/sub2api` 仅执行 `docker compose pull sub2api` 与 `docker compose up -d sub2api`，PostgreSQL/Redis 未重建。
- **VPS 发布后**：三个容器均为 healthy；运行镜像 digest 为上述 `sha256:278a1f38…`；健康接口返回 `{"status":"ok"}`。应用启动完成，最近 `/v1/responses`、`/v1/alpha/search` 请求均返回 200，`image_generation` bridge 日志正常；无需回滚。
- **新发现问题**：生产启动日志仍提示 `security.url_allowlist.enabled=false`、`server.trusted_proxies` 未配置、`CORS allowed_origins` 未配置，当前按既有生产配置运行，后续需人工确认是否补齐；VPS 上按服务名调用 `docker compose logs sub2api` 返回 `no such service`，本次使用 `docker logs sub2api` 完成日志验收，容器和健康状态不受影响。

### 2026-10-02：官方 v0.2.13 合并阻塞

- 官方稳定版：`v0.2.13`，发布日期 `2026-10-02`；annotated tag `7d0c0067f406c380f0a94cfc3879cdae7049b467`，剥离提交 `3040209f205472038c1ba745a1bedd2edd9053b1`；`upstream/main`=`b8dece9000c68815a5b867ca5a1e6f236e173905`（发布后的 VERSION 同步提交）。
- 基线：`origin/main`=`3480a105c327345ba33ea48ca2d932064b4be0ff`；独立分支 `codex/upgrade-v0.2.13`。已执行 `git fetch origin --prune --tags`、`git fetch upstream --prune --tags`，并尝试 `git merge --no-ff --no-edit v0.2.13`。
- 合并冲突共 7 个：`backend/cmd/server/wire_gen.go`、`backend/ent/migrate/schema.go`、`backend/internal/server/router.go`、`backend/internal/service/openai_account_scheduler.go`、`frontend/src/components/keys/UseKeyModal.vue`、`frontend/src/views/user/KeysView.vue`、`frontend/src/views/user/PaymentView.vue`。合并已中止，未提交、未推送、未运行发布流水线。
- 关键阻塞：
  - `wire_gen.go` 需要同时核对本地 `StoreHandler`/Ollama Cloud provider 与官方 `IdempotencyCoordinator`/Claude reset provider，确保每个 provider 只声明一次。
  - Ent schema 的 `paymentorder_payment_type_paid_at` 索引列发生 9/10 冲突；官方删除本地 `145_storefront.sql`、`151_subscription_plan_key_quota.sql`、`152_payment_order_api_key_target.sql` 及关联 Ent 字段，同时新增两个 `241_*` 迁移，需先完成数据兼容方案。
  - 路由冲突涉及本地 Storefront 与官方 Payment/Redis 参数；调度器冲突涉及本地 `openAIAccountAllowedByRoute` 与官方 composite-route ownership，必须保留 PLUS→PRO→API Key 回退和现有路由约束。
  - 前端冲突涉及自定义 `providerName` 与官方 `claudeCodeOnly`，以及 PaymentView 的充值赠送导入，需要合并后补测试。
  - 官方 tag 将前端 `xlsx` 从已修复的 SheetJS 0.20.3 CDN 包回退到 `^0.18.5`，并重新加入两个 high 审计例外（到期 `2026-10-06`）；同时删除 `.github/workflows/docker.yml`，会破坏 `latest` 镜像构建门禁。两项均不得直接接受。
  - 依赖/配置还包含 Axios `1.18.x -> 1.20.0`、API Key 创建限制和 billing in-flight reservation；未发现 `go.mod`/`go.sum` 变化。
- 验证结果：因合并冲突门禁，未执行 `govulncheck`、前端生产审计、测试、CI、Security Scan、Docker 构建、commit/push 或 VPS 发布。
- VPS 只读回读：`sub2api`、PostgreSQL、Redis 均 healthy；`http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`；当前生产镜像 `ghcr.io/balabalbling/sub2api:latest`，digest `sha256:278a1f38e9597176fb7b20fe2d4e593ab66012025edf5cc672133f7eb585e746`；近期 `/v1/responses` 请求返回 200。生产未发生变更。
- 处理结果：本次停在发布前；后续需在独立分支逐项解决上述冲突并补迁移、调度、依赖安全和 Docker 门禁验证，再考虑推进 `main`。

### 2026-10-03：v0.2.13 官方支付本地集成（未发布）

- 根据用户确认，支付模块完全跟随官方，退役本地 Storefront/API Key 专属充值与发货源码；保留历史 145/151/152 SQL 与数据、自定义 PLUS→PRO→API Key 调度、图片兼容、provider 配置、xlsx 0.20.3 和 Docker workflow。
- 独立分支 `codex/upgrade-v0.2.13` 基线 `3480a105c327345ba33ea48ca2d932064b4be0ff`，目标 tag commit `3040209f205472038c1ba745a1bedd2edd9053b1`；七处文本冲突已解，Ent/Wire 已再生，无重复关键 provider；新增 ProMax→PRO 回归及双路由过滤验证通过。合并保持未提交。
- 更正上次评估：两端 diff 不等于三方合并结果；真实合并保留 xlsx 0.20.3、Docker workflow 和历史 migrations，不属于自动删除/降级阻断。
- 本地安全：最终 govulncheck exit 0（可达漏洞 0；模块另有 11 条未触达公告）；前端生产审计 high=0/critical=0/moderate=13/low=2，例外校验通过。
- 本地验证：前端 338 文件/2590 测试全部通过，前后端构建通过；支付/调度/图片/TypeSafe 定向 unit 通过。后端全量 unit 尚失败：Ollama 陈旧 probe 覆盖新冷却时间测试单独复现 20/20 失败。repository 缺 sh 的环境错误补 PATH 后整包重试通过。Windows 可执行权限/Apple stat、Docker 环境限制仍需远端 CI 确认。
- 发布门禁保持关闭：需先排查 `backend/internal/service/ratelimit_service_ollama_429_test.go:388-405`，并在数据库恢复副本核对历史自定义订单、退款/发货与新增约束/在途余额预留。
- CI/Security/Docker run URL：无，本次未提交或推送；新镜像 digest：无。VPS 健康与当前生产镜像本次未回读；未 SSH 或部署。
- 完整本地报告：`H:\sub2api\docs\UPGRADE_INTEGRATION_v0.2.13.md`；日志：`H:\sub2api\tmp\upgrade-v0.2.13`。原有未提交清单内容已备份并保留，本次仅追加记录。

### 2026-10-03：官方 v0.2.13 支付集成生产发布

- 官方版本：v0.2.13；发布提交：`f5e468c844c5f7dbcc6d6830a3993b2772fddb2f`；本地分支 `codex/upgrade-v0.2.13` 通过门禁后快进 `main` 并推送。
- 变更方向：支付模块完全采用官方实现；保留 PLUS→PRO→API Key 调度、ProMax→PRO 识别、图片桥接/账号过滤、provider 配置、xlsx 0.20.3 和 Docker workflow；历史 145/151/152 migrations 未改写。
- 分支 CI：<https://github.com/balabalbling/sub2api/actions/runs/37118705750>，成功；Security Scan：<https://github.com/balabalbling/sub2api/actions/runs/37118705749>，成功。
- main CI：<https://github.com/balabalbling/sub2api/actions/runs/37119418974>，成功；main Security Scan：<https://github.com/balabalabling/sub2api/actions/runs/37119418932>，成功；Build Docker Image：<https://github.com/balabalbling/sub2api/actions/runs/37119418918>，成功。
- 镜像核对：`ghcr.io/balabalabling/sub2api:latest` 与提交标签共同指向 `sha256:ce4accd90dc4ad6a122caffe9f4a72fb044ca60a4fbe7aa0ed87b4e6df80016f`。
- VPS 发布前：`sub2api`、PostgreSQL、Redis healthy；旧镜像 digest 为 `sha256:278a1f38e9597176fb7b20fe2d4e593ab66012025edf5cc672133f7eb585e746`。
- VPS 发布动作：在 `/opt/sub2api` 仅执行 `docker compose pull sub2api` 与 `docker compose up -d sub2api`，PostgreSQL/Redis 未重建。迁移 `241_add_payment_order_bonus_amount.sql` 与 `241_add_typesafe_platform.sql` 已成功应用。
- VPS 发布后：三容器 healthy；`http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`；运行镜像为上述新 digest；近期 `/v1/responses` 返回 200，未见 panic、fatal 或迁移错误；无需回滚。
- 数据检查：生产 payment_orders 当前仅有历史 subscription 的 EXPIRED/FAILED 记录；无已支付但未交付的 store order；平台约束存量查询为空。生产仍存在 1 个启用的 `key_quota_usd` 套餐配置，按用户确认的官方支付方向保留数据，后续人工决定是否下架该历史配置。
- 新发现问题：启动日志仍提示 `server.trusted_proxies` 与 CORS `allowed_origins` 未配置；属于既有生产配置提醒，本次未改动。发布前后完整日志：`H:\sub2api\tmp\upgrade-v0.2.13\vps-publish.log`、`vps-final-validation.log`。

### 2026-10-10：官方 v0.2.15 合并阻塞（未发布）

- 官方最新稳定版：`v0.2.15`，annotated tag `86a80c13dcba86f52f9ca815b5a471cecc236227`，剥离提交 `f2669c8cf62555cd92389b3f55920e9e6e7c6ff2`，发布时间为 **2026-10-09 11:27 +08:00**；`upstream/main`=`3a6fd1c9db07203ca308aaba69e502bc1f35b307`，仅比 tag 多一个 `VERSION` 同步提交。
- 基线 `origin/main`=`aee70827e38c782309d26ddad7577e17e1a6b9cf`；相对 v0.2.15 有 81 个本地自定义提交，官方 tag 相对基线有 154 个提交。已在独立分支 `codex/upgrade-v0.2.15` 试合并，随后干净中止。
- 三方合并冲突共 3 个：`.github/audit-exceptions.yml`、`backend/cmd/server/VERSION`、`frontend/src/api/__tests__/settings.authSourceDefaults.spec.ts`。
- 关键阻塞与后续建议：
  - `.github/audit-exceptions.yml`：本地已保留 SheetJS `xlsx` 0.20.3 且移除两条 high 例外；官方 tag 重新加入两条 xlsx 例外。保留本地安全修复，不直接接受占位 owner 的例外，合并后重新执行生产依赖审计。
  - `backend/cmd/server/VERSION`：本地为 `0.2.13`、tag 为 `0.2.14`，`upstream/main` 的同步提交为 `0.2.15`；解决时应以官方最新稳定版本 `0.2.15` 为准。
  - `settings.authSourceDefaults.spec.ts`：本地测试固定 6 个额度平台，官方平台清单已扩展到 13 个并加入 `command_code`、`cline`；应以统一平台清单生成期望值，同时保留 `typesafe` 与本地额度默认行为回归。
  - 自动合并还引入 `242_drop_platform_check_constraints.sql`，把两个平台 CHECK 约束转由应用平台清单校验；必须在数据库恢复副本上验证历史数据、迁移顺序、回滚与异常平台行。
  - 调度行为虽无文本冲突，但 `NormalizeOpenAICompatiblePlatform` 和快照平台遍历改为依赖平台清单，需重点回归 PLUS→PRO→API Key 顺序、PRO 五小时豁免、ProMax 识别、sticky/route 过滤与 API Key 最终兜底。`backend/cmd/server/wire_gen.go` 在预览中无差异，未观察到重复 provider，但最终合并后仍需生成文件检查。
- 合并预览的依赖/工具链变化：Go `1.27.0 -> 1.27.2`、golangci-lint `2.13 -> 2.14.0`、Vue `3.5.26 -> 3.5.43`，并升级多项 `golang.org/x/*` 依赖；未执行安全预检或测试。
- 因冲突门禁，本次未运行 `govulncheck`、前端生产审计、相关测试、完整 CI、Security Scan、Docker workflow，也未提交、推送或发布 VPS。当前生产只读回读（2026-10-10）：三容器 healthy，`http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`，运行镜像摘要为 `sha256:e1095d3a2479eabc8eea7029758e621e7208bf1c0c5777e97ee3ee0a299f9f2a`；最近日志未见 `panic`、`fatal`、迁移失败或回滚信号，近期 `/v1/responses` 返回 200。
- 处理结果：停在提交与生产发布之前；主工作区原有未提交的 `docs/RELEASE_CHECKLIST.md` 内容保持不变并仅追加本次记录，等待人工解决上述冲突后再进入安全、测试、CI 和发布门禁。

### 2026-10-17：官方 v0.2.17 合并与本地门禁（远程验证待执行）

- 官方最新稳定版：`v0.2.17`，GitHub release 于 **2026-10-16 12:41 +08:00** 发布；annotated tag `6d6c4e701a463dcf83c261e8428e93423d6c0b3e`，剥离提交 `060d0160417782cd962a0c7f460f6cd4565d4916`；`upstream/main`=`b0a02a3304a0a78cdd003ddda1dd23ba02c03084`，仅比 tag 多一个 `VERSION` 同步提交。
- 基线：`origin/main`=`aee70827e38c782309d26ddad7577e17e1a6b9cf`；官方 tag 相对基线新增 281 个提交。独立分支 `codex/upgrade-v0.2.17` 已合并官方 tag，保留 `docs/RELEASE_CHECKLIST.md` 原有未提交记录。
- 合并冲突共 4 个：`.github/audit-exceptions.yml`、`backend/cmd/server/VERSION`、`backend/internal/service/openai_ws_v2_passthrough_adapter.go`、`frontend/src/api/__tests__/settings.authSourceDefaults.spec.ts`。处理为：保留本地 SheetJS `xlsx` 0.20.3 安全修复与无 xlsx 占位例外；VERSION 对齐 0.2.17；保留本地 WebSocket 图片回放并接入官方 config-aware 兼容归一化；前端额度平台断言改为使用官方 13 平台清单。
- 关键兼容检查：`backend/cmd/server/wire_gen.go` 保留 Ollama Cloud、OpenCode Go、Idempotency、Claude reset、Gemini cached content provider，各 provider 仅声明一次；PLUS→PRO→API Key→compat 调度顺序、ProMax 识别、PLUS headroom=true 与现有路由过滤代码未被覆盖。新增 242-245 迁移（平台 CHECK 约束移交应用清单、分组 i18n、Gemini 显式缓存及租约），迁移按完整文件名排序，两个 `242_*.sql` 不发生键冲突。
- 本地前端门禁：pnpm 9.15.9 安装通过；lint、typecheck、全量 Vitest **373 文件 / 2871 测试通过**；生产构建通过；生产审计 **high=0、critical=0、moderate=13、low=4**，审计例外校验通过。
- 本机未安装 Go、govulncheck、Docker；后端编译、govulncheck、完整 CI、Security Scan、Docker workflow 交由 GitHub Actions 验证。当前尚未提交、推送、更新 main 或发布 VPS。
- CI/Security/Docker run URL、最终 commit、镜像摘要、VPS 健康结果：待远程门禁完成后回填。新发现问题：当前未发现阻断性行为冲突；生产现有 `trusted_proxies` / CORS 等启动提醒仍需按既有配置人工评估。
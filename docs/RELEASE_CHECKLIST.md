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

## 每次发布必须留下的记录

- 功能分支、最终 commit、官方版本/上游提交。
- CI 和 Security Scan 的 run URL 与结论。
- Docker workflow 的 run URL、镜像 digest/commit tag。
- VPS 发布前后的容器状态、健康检查和异常日志摘要。
- 本次新增问题、根因、修复提交，以及是否补充了回归测试或门禁。

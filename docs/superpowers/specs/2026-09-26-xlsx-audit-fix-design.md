# xlsx 审计例外修复设计

## 目标

移除前端 `xlsx@0.18.5` 的两个 high 审计例外，在不改变管理员用量导出行为的前提下，使用已修复的 SheetJS 构建并让生产依赖审计直接通过。

## 当前上下文

- `xlsx` 当前版本为 `0.18.5`，位于 `frontend/package.json` 和 `frontend/pnpm-lock.yaml`。
- 当前唯一运行时使用点是 `frontend/src/views/admin/UsageView.vue` 的管理员用量导出逻辑，使用动态导入 `import('xlsx')`。
- 当前审计例外对应 Prototype Pollution（CVE-2023-30533）和 ReDoS（CVE-2024-22363），有效期至 `2026-10-06`。
- 本次只处理依赖和审计配置，不改变图片路由、调度器、后端或生产配置。

## 方案选择

### 采用：固定官方 CDN 的 SheetJS 0.20.3 tarball

将 `xlsx` 依赖改为精确的官方 tarball URL：

```text
https://cdn.sheetjs.com/xlsx-0.20.3/xlsx-0.20.3.tgz
```

理由：

1. 0.20.3 高于当前审计报告列出的两个修复门槛（Prototype Pollution 修复版本不低于 0.19.3，ReDoS 修复版本不低于 0.20.2）。
2. 保持 `xlsx` 包名和现有 API，管理员导出代码不需要改写。
3. 精确锁定 tarball，避免宽版本范围重新解析到旧 npm 包。
4. 该 tarball URL 已在本机验证可访问。

### 不采用的方案

- 仅延长审计例外：只记录风险，不解决依赖问题。
- 改用其他 Excel 库：需要重新验证工作簿生成、样式、下载和浏览器兼容，超出本次修复范围。
- 将 tarball 下载后直接提交到仓库：会增加二进制供应链维护和仓库体积；本次先使用锁定的官方 URL。

## 修改范围

1. 修改 `frontend/package.json` 的 `xlsx` 依赖为精确 tarball URL。
2. 更新 `frontend/pnpm-lock.yaml`，确保解析结果固定到 0.20.3。
3. 从 `.github/audit-exceptions.yml` 删除两个 `xlsx` 例外，保留其他未相关例外不动。
4. 不修改 `frontend/src/views/admin/UsageView.vue`，除非兼容性测试证明新构建需要极小的适配。

## 验证与验收

- `pnpm install --frozen-lockfile` 成功。
- `pnpm audit --prod --audit-level=high --json` 不再报告 `xlsx`。
- `python tools/check_pnpm_audit_exceptions.py --audit frontend/audit.json --exceptions .github/audit-exceptions.yml` 成功。
- 前端类型检查、管理员用量导出相关测试和生产构建成功。
- `git diff --check` 成功，工作区只包含本次依赖修复文件。

## 回滚

如果新构建导致导出回归，回退 `frontend/package.json`、`frontend/pnpm-lock.yaml` 和 `.github/audit-exceptions.yml` 到本次修复前版本，并重新运行前端安装和验证；本次修复不触碰生产数据库和 VPS。

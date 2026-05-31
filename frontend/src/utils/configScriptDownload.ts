import type { GroupPlatform } from '@/types'

interface ConfigScriptInput {
  apiKey: string
  baseUrl: string
  platform?: GroupPlatform | null
  providerName?: string
}

interface DownloadScript {
  filename: string
  content: string
  mimeType: string
}

const CODEX_MODEL = 'gpt-5.5'

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, '')
}

function escapeToml(value: string): string {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
}

function escapeJson(value: string): string {
  return JSON.stringify(value)
}

function sanitizeFilenamePart(value: string): string {
  return value
    .trim()
    .replace(/[\\/:*?"<>|]+/g, '-')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
    .toLowerCase()
}

function buildCodexConfig(baseUrl: string, providerName: string): string {
  const safeProviderName = escapeToml(providerName)
  const safeBaseUrl = escapeToml(baseUrl)

  return `model_provider = "${safeProviderName}"
model = "${CODEX_MODEL}"
review_model = "${CODEX_MODEL}"
model_reasoning_effort = "xhigh"
disable_response_storage = true

[model_providers."${safeProviderName}"]
name = "${safeProviderName}"
base_url = "${safeBaseUrl}"
wire_api = "responses"
requires_openai_auth = true

[features]
web_search_request = true
`
}

function buildAuthJson(apiKey: string): string {
  return `{
  "OPENAI_API_KEY": ${escapeJson(apiKey)}
}
`
}

function buildUnixScript(input: ConfigScriptInput): DownloadScript {
  const baseUrl = trimTrailingSlash(input.baseUrl || window.location.origin)
  const providerName = sanitizeFilenamePart(input.providerName || 'sub2api') || 'sub2api'
  const configToml = buildCodexConfig(baseUrl, providerName)
  const authJson = buildAuthJson(input.apiKey)

  return {
    filename: `configure-${providerName}-codex.sh`,
    mimeType: 'text/x-shellscript;charset=utf-8',
    content: `#!/usr/bin/env bash
set -euo pipefail

CONFIG_DIR="\${HOME}/.codex"
CONFIG_FILE="\${CONFIG_DIR}/config.toml"
AUTH_FILE="\${CONFIG_DIR}/auth.json"
BACKUP_SUFFIX="$(date +%Y%m%d%H%M%S)"

mkdir -p "\${CONFIG_DIR}"

if [ -f "\${CONFIG_FILE}" ]; then
  cp "\${CONFIG_FILE}" "\${CONFIG_FILE}.bak.\${BACKUP_SUFFIX}"
fi

if [ -f "\${AUTH_FILE}" ]; then
  cp "\${AUTH_FILE}" "\${AUTH_FILE}.bak.\${BACKUP_SUFFIX}"
fi

cat > "\${CONFIG_FILE}" <<'SUB2API_CODEX_CONFIG'
${configToml}SUB2API_CODEX_CONFIG

cat > "\${AUTH_FILE}" <<'SUB2API_CODEX_AUTH'
${authJson}SUB2API_CODEX_AUTH

chmod 600 "\${CONFIG_FILE}" "\${AUTH_FILE}"

echo "Codex config updated: \${CONFIG_FILE}"
echo "Codex auth updated: \${AUTH_FILE}"
echo "Restart Codex to use ${providerName}."
`
  }
}

function buildWindowsScript(input: ConfigScriptInput): DownloadScript {
  const baseUrl = trimTrailingSlash(input.baseUrl || window.location.origin)
  const providerName = sanitizeFilenamePart(input.providerName || 'sub2api') || 'sub2api'
  const configToml = buildCodexConfig(baseUrl, providerName)
  const authJson = buildAuthJson(input.apiKey)

  return {
    filename: `configure-${providerName}-codex.ps1`,
    mimeType: 'text/plain;charset=utf-8',
    content: `$ErrorActionPreference = "Stop"

$ConfigDir = Join-Path $HOME ".codex"
$ConfigFile = Join-Path $ConfigDir "config.toml"
$AuthFile = Join-Path $ConfigDir "auth.json"
$BackupSuffix = Get-Date -Format "yyyyMMddHHmmss"

New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null

if (Test-Path $ConfigFile) {
  Copy-Item $ConfigFile "$ConfigFile.bak.$BackupSuffix" -Force
}

if (Test-Path $AuthFile) {
  Copy-Item $AuthFile "$AuthFile.bak.$BackupSuffix" -Force
}

@'
${configToml}'@ | Set-Content -Path $ConfigFile -Encoding UTF8

@'
${authJson}'@ | Set-Content -Path $AuthFile -Encoding UTF8

Write-Host "Codex config updated: $ConfigFile"
Write-Host "Codex auth updated: $AuthFile"
Write-Host "Restart Codex to use ${providerName}."
`
  }
}

export function buildConfigScript(input: ConfigScriptInput): DownloadScript {
  const isWindows = navigator.platform.toLowerCase().includes('win')
  return isWindows ? buildWindowsScript(input) : buildUnixScript(input)
}

export function downloadConfigScript(input: ConfigScriptInput): void {
  const script = buildConfigScript(input)
  const blob = new Blob([script.content], { type: script.mimeType })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')

  link.href = url
  link.download = script.filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

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
const CODEX_PROVIDER_NAME = 'go2me'
const CODEX_PROVIDER_PLACEHOLDER = '__CODEX_PROVIDER_NAME__'

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, '')
}

function escapeToml(value: string): string {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
}

function buildProviderConfigBlock(baseUrl: string, providerName: string, apiKey: string): string {
  const safeProviderName = escapeToml(providerName)
  const safeBaseUrl = escapeToml(baseUrl)
  const safeApiKey = escapeToml(apiKey)

  return `model_provider = "${safeProviderName}"
model = "${CODEX_MODEL}"
review_model = "${CODEX_MODEL}"
model_reasoning_effort = "medium"
disable_response_storage = true
network_access = "enabled"
windows_wsl_setup_acknowledged = true

[model_providers."${safeProviderName}"]
name = "${safeProviderName}"
base_url = "${safeBaseUrl}"
env_key = "OPENAI_API_KEY"
wire_api = "responses"
requires_openai_auth = true
experimental_bearer_token = "${safeApiKey}"`
}

function buildFeaturesConfigBlock(): string {
  return `[features]
web_search_request = true`
}

function encodePowerShellCommand(script: string): string {
  let binary = ''

  for (let index = 0; index < script.length; index += 1) {
    const code = script.charCodeAt(index)
    binary += String.fromCharCode(code & 0xff, code >> 8)
  }

  return btoa(binary)
}

function buildUnixScript(input: ConfigScriptInput): DownloadScript {
  const baseUrl = trimTrailingSlash(input.baseUrl || window.location.origin)
  const providerName = CODEX_PROVIDER_NAME
  const providerConfig = buildProviderConfigBlock(baseUrl, CODEX_PROVIDER_PLACEHOLDER, input.apiKey)
  const featuresConfig = buildFeaturesConfigBlock()

  return {
    filename: `configure-${providerName}-codex.sh`,
    mimeType: 'text/x-shellscript;charset=utf-8',
    content: `#!/usr/bin/env bash
set -euo pipefail

CONFIG_DIR="\${HOME}/.codex"
CONFIG_FILE="\${CONFIG_DIR}/config.toml"
AUTH_FILE="\${CONFIG_DIR}/auth.json"
BACKUP_SUFFIX="$(date +%Y%m%d%H%M%S)"
PROVIDER_NAME="${providerName}"
API_KEY='${input.apiKey.replace(/'/g, "'\\''")}'

mkdir -p "\${CONFIG_DIR}"

if [ -f "\${CONFIG_FILE}" ]; then
  cp "\${CONFIG_FILE}" "\${CONFIG_FILE}.bak.\${BACKUP_SUFFIX}"
fi

if [ -f "\${AUTH_FILE}" ]; then
  cp "\${AUTH_FILE}" "\${AUTH_FILE}.bak.\${BACKUP_SUFFIX}"
fi

EXISTING_PROVIDER="$(awk '
  /^[[:space:]]*\[/ { exit }
  /^[[:space:]]*model_provider[[:space:]]*=/ {
    line = $0
    sub(/^[^=]*=/, "", line)
    gsub(/^[[:space:]"'\\''"]+|[[:space:]"'\\''"]+$/, "", line)
    print line
    exit
  }
' "\${CONFIG_FILE}" 2>/dev/null || true)"
if [ -z "\${EXISTING_PROVIDER}" ]; then
  EXISTING_PROVIDER="$(awk '
    /^\[model_providers\./ {
      line = $0
      sub(/^\[model_providers\./, "", line)
      sub(/\]$/, "", line)
      gsub(/^"|"$/, "", line)
      if (line != "go2me") {
        print line
        exit
      }
    }
  ' "\${CONFIG_FILE}" 2>/dev/null || true)"
fi
if [ -n "\${EXISTING_PROVIDER}" ]; then
  PROVIDER_NAME="\${EXISTING_PROVIDER}"
fi

TMP_CONFIG="\${CONFIG_FILE}.tmp.\${BACKUP_SUFFIX}"
awk -v provider="\${PROVIDER_NAME}" '
  BEGIN { skip = 0 }
  /^model_provider[[:space:]]*=/ || /^model[[:space:]]*=/ || /^review_model[[:space:]]*=/ || /^model_reasoning_effort[[:space:]]*=/ || /^disable_response_storage[[:space:]]*=/ || /^network_access[[:space:]]*=/ || /^windows_wsl_setup_acknowledged[[:space:]]*=/ {
    if (!skip) next
  }
  $0 == "[features]" { skip = 1; next }
  $0 == ("[model_providers.\"" provider "\"]") || $0 == ("[model_providers." provider "]") || $0 == "[model_providers.\"go2me\"]" || $0 == "[model_providers.go2me]" { skip = 1; next }
  /^\[/ { skip = 0 }
  !skip { print }
' "\${CONFIG_FILE}" 2>/dev/null > "\${TMP_CONFIG}" || true

OLD_CONFIG="$(cat "\${TMP_CONFIG}")"
PROVIDER_CONFIG="$(cat <<'SUB2API_CODEX_CONFIG'
${providerConfig}
SUB2API_CODEX_CONFIG
)"
FEATURES_CONFIG="$(cat <<'SUB2API_CODEX_FEATURES'
${featuresConfig}
SUB2API_CODEX_FEATURES
)"
PROVIDER_CONFIG="\${PROVIDER_CONFIG//${CODEX_PROVIDER_PLACEHOLDER}/\${PROVIDER_NAME}}"
printf '%s\n' "\${PROVIDER_CONFIG}" > "\${TMP_CONFIG}"
if [ -n "\${OLD_CONFIG//[[:space:]]/}" ]; then
  printf '\n\n%s\n' "\${OLD_CONFIG}" >> "\${TMP_CONFIG}"
fi
printf '\n\n%s\n' "\${FEATURES_CONFIG}" >> "\${TMP_CONFIG}"

mv "\${TMP_CONFIG}" "\${CONFIG_FILE}"

python3 - "\${AUTH_FILE}" "\${API_KEY}" <<'SUB2API_CODEX_AUTH_PY' || python - "\${AUTH_FILE}" "\${API_KEY}" <<'SUB2API_CODEX_AUTH_PY'
import json
import os
import sys

path, api_key = sys.argv[1], sys.argv[2]
data = {}
if os.path.exists(path):
    try:
        with open(path, "r", encoding="utf-8-sig") as fh:
            loaded = json.load(fh)
            if isinstance(loaded, dict):
                data = loaded
    except Exception:
        data = {}
data["OPENAI_API_KEY"] = api_key
data["auth_mode"] = "apikey"
with open(path, "w", encoding="utf-8") as fh:
    json.dump(data, fh, ensure_ascii=False, indent=2)
    fh.write("\\n")
SUB2API_CODEX_AUTH_PY

chmod 600 "\${CONFIG_FILE}" "\${AUTH_FILE}"

echo "Codex config updated: \${CONFIG_FILE}"
echo "Codex auth updated: \${AUTH_FILE}"
echo "Restart Codex to use \${PROVIDER_NAME}."
`
  }
}

function buildWindowsScript(input: ConfigScriptInput): DownloadScript {
  const baseUrl = trimTrailingSlash(input.baseUrl || window.location.origin)
  const providerName = CODEX_PROVIDER_NAME
  const providerConfig = buildProviderConfigBlock(baseUrl, CODEX_PROVIDER_PLACEHOLDER, input.apiKey)
  const featuresConfig = buildFeaturesConfigBlock()
  const powerShellScript = `$ErrorActionPreference = "Stop"

$UserProfile = [Environment]::GetFolderPath("UserProfile")
if ([string]::IsNullOrWhiteSpace($UserProfile)) {
  $UserProfile = $HOME
}

$ConfigDir = Join-Path -Path $UserProfile -ChildPath ".codex"
$ConfigFile = Join-Path -Path $ConfigDir -ChildPath "config.toml"
$AuthFile = Join-Path -Path $ConfigDir -ChildPath "auth.json"
$BackupSuffix = Get-Date -Format "yyyyMMddHHmmss"
$ProviderName = "${providerName}"
$ApiKey = @'
${input.apiKey}
'@

New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null

if (Test-Path -LiteralPath $ConfigFile) {
  Copy-Item -LiteralPath $ConfigFile -Destination "$ConfigFile.bak.$BackupSuffix" -Force
}

if (Test-Path -LiteralPath $AuthFile) {
  Copy-Item -LiteralPath $AuthFile -Destination "$AuthFile.bak.$BackupSuffix" -Force
}

$ExistingConfig = ""
if (Test-Path -LiteralPath $ConfigFile) {
  $ExistingConfig = Get-Content -LiteralPath $ConfigFile -Raw -Encoding UTF8
}

$ExistingProviderName = $null
foreach ($Line in ($ExistingConfig -split "\\r?\\n")) {
  if ($Line -match '^\\s*\\[') { break }
  if ($Line -match '^\\s*model_provider\\s*=\\s*["'']?([^"'']+)["'']?\\s*$') {
    $ExistingProviderName = $Matches[1].Trim()
    break
  }
}
if ([string]::IsNullOrWhiteSpace($ExistingProviderName)) {
  foreach ($Line in ($ExistingConfig -split "\\r?\\n")) {
    if ($Line -match '^\\[model_providers\\.(?:"([^"]+)"|([^\\]]+))\\]$') {
      $CandidateProviderName = if ($Matches[1]) { $Matches[1] } else { $Matches[2] }
      if ($CandidateProviderName -ne "go2me") {
        $ExistingProviderName = $CandidateProviderName.Trim()
        break
      }
    }
  }
}
if (-not [string]::IsNullOrWhiteSpace($ExistingProviderName)) {
  $ProviderName = $ExistingProviderName
}

$ProviderHeaderQuoted = '[model_providers."' + $ProviderName + '"]'
$ProviderHeaderBare = '[model_providers.' + $ProviderName + ']'
$Go2meProviderHeaderQuoted = '[model_providers."go2me"]'
$Go2meProviderHeaderBare = '[model_providers.go2me]'
$CleanLines = New-Object System.Collections.Generic.List[string]
$SkipBlock = $false

foreach ($Line in ($ExistingConfig -split "\\r?\\n")) {
  if ($Line -match '^(model_provider|model|review_model|model_reasoning_effort|disable_response_storage|network_access|windows_wsl_setup_acknowledged)\\s*=') {
    if (-not $SkipBlock) { continue }
  }

  if (
    $Line -eq '[features]' -or
    $Line -eq $ProviderHeaderQuoted -or
    $Line -eq $ProviderHeaderBare -or
    $Line -eq $Go2meProviderHeaderQuoted -or
    $Line -eq $Go2meProviderHeaderBare
  ) {
    $SkipBlock = $true
    continue
  }

  if ($Line -match '^\\[') {
    $SkipBlock = $false
  }

  if (-not $SkipBlock) {
    $CleanLines.Add($Line)
  }
}

$CleanConfig = ($CleanLines -join [Environment]::NewLine).TrimEnd()
$ProviderConfig = @'
${providerConfig}
'@
$ProviderConfig = $ProviderConfig.Replace('${CODEX_PROVIDER_PLACEHOLDER}', $ProviderName)
$FeaturesConfig = @'
${featuresConfig}
'@

$FinalConfig = if ([string]::IsNullOrWhiteSpace($CleanConfig)) {
  $ProviderConfig + [Environment]::NewLine + [Environment]::NewLine + $FeaturesConfig
} else {
  $ProviderConfig + [Environment]::NewLine + [Environment]::NewLine + $CleanConfig + [Environment]::NewLine + [Environment]::NewLine + $FeaturesConfig
}
$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($ConfigFile, $FinalConfig + [Environment]::NewLine, $Utf8NoBom)

$Auth = [ordered]@{}
if (Test-Path -LiteralPath $AuthFile) {
  try {
    $ExistingAuth = Get-Content -LiteralPath $AuthFile -Raw -Encoding UTF8 | ConvertFrom-Json
    foreach ($Property in $ExistingAuth.PSObject.Properties) {
      $Auth[$Property.Name] = $Property.Value
    }
  } catch {
    $Auth = [ordered]@{}
  }
}
$Auth["OPENAI_API_KEY"] = $ApiKey
$Auth["auth_mode"] = "apikey"
$AuthJson = $Auth | ConvertTo-Json -Depth 20
[System.IO.File]::WriteAllText($AuthFile, $AuthJson + [Environment]::NewLine, $Utf8NoBom)

Write-Host "Codex config updated: $ConfigFile"
Write-Host "Codex auth updated: $AuthFile"
Write-Host "Restart Codex to use $ProviderName."
`

  return {
    filename: `configure-${providerName}-codex.cmd`,
    mimeType: 'application/x-msdownload;charset=utf-8',
    content: `@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -EncodedCommand ${encodePowerShellCommand(powerShellScript)}
if errorlevel 1 (
  echo.
  echo Failed to configure Codex.
  pause
  exit /b 1
)
echo.
echo Done. You can close this window and restart Codex.
pause
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


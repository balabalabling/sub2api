import { describe, expect, it, vi } from 'vitest'

import { buildConfigScript } from '../configScriptDownload'

describe('configScriptDownload', () => {
  it('uses a double-clickable go2me Windows command script by default', () => {
    vi.stubGlobal('navigator', { platform: 'Win32' })
    vi.stubGlobal('window', { location: { origin: 'https://admin.go2me.vip' } })
    vi.stubGlobal('btoa', (value: string) => Buffer.from(value, 'binary').toString('base64'))

    const script = buildConfigScript({
      apiKey: 'sk-test',
      baseUrl: 'https://api.go2me.vip/v1'
    })

    expect(script.filename).toBe('configure-go2me-codex.cmd')
    expect(script.content).toContain('powershell.exe -NoProfile -ExecutionPolicy Bypass -EncodedCommand ')
    expect(script.content).toContain('pause')

    const encodedCommand = script.content.match(/-EncodedCommand ([A-Za-z0-9+/=]+)/)?.[1]
    expect(encodedCommand).toBeDefined()

    const decodedScript = Buffer.from(encodedCommand!, 'base64').toString('utf16le')
    expect(decodedScript).toContain('model_provider = "go2me"')
    expect(decodedScript).toContain('model_reasoning_effort = "medium"')
    expect(decodedScript).toContain('[Environment]::GetFolderPath("UserProfile")')
    expect(decodedScript).toContain('Set-Content -LiteralPath $ConfigFile -Encoding UTF8')
    expect(decodedScript).toContain('Set-Content -LiteralPath $AuthFile -Encoding UTF8')

    vi.unstubAllGlobals()
  })
})

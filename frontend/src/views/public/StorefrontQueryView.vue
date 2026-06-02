<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <main class="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
      <header class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-3xl font-bold tracking-normal">查询中心</h1>
          <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">邮箱验证码通过后会在本浏览器缓存查询状态，之后可直接查看订单、API Key 和用量。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button class="btn btn-primary inline-flex items-center justify-center px-4 py-2" @click="downloadInstallScript">
            安装脚本
          </button>
          <RouterLink to="/storefront" class="btn btn-secondary inline-flex items-center justify-center px-4 py-2">返回商城</RouterLink>
        </div>
      </header>

      <section v-if="cachedSession" class="card mb-5 border border-primary-100 p-5 dark:border-primary-900/40">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-lg font-bold">已登录查询中心</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ cachedSession.email }}</p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-primary px-5 py-2.5" :disabled="loading" @click="queryCachedEmail">刷新订单和用量</button>
            <button class="btn btn-secondary px-5 py-2.5" @click="clearCachedSession">退出查询中心</button>
          </div>
        </div>
      </section>

      <section class="card p-5">
        <div class="grid gap-4 md:grid-cols-[1fr_auto_1fr] md:items-end">
          <label class="block">
            <span class="mb-1 block text-sm font-medium">邮箱</span>
            <input v-model.trim="email" type="email" class="input" placeholder="you@example.com">
          </label>
          <button class="btn btn-secondary px-4 py-3" :disabled="sending || codeCooldown > 0 || !email" @click="sendCode">
            {{ sending ? '发送中...' : codeCooldown > 0 ? `${codeCooldown}s 后重发` : '发送验证码' }}
          </button>
          <label class="block">
            <span class="mb-1 block text-sm font-medium">验证码</span>
            <input v-model.trim="code" class="input" placeholder="6 位验证码">
          </label>
        </div>
        <div class="mt-4 flex flex-wrap gap-3">
          <button class="btn btn-primary px-5 py-2.5" :disabled="loading || !email || !code" @click="queryEmail">按邮箱查询</button>
        </div>
        <label class="mt-4 flex items-start gap-2 text-sm text-gray-500 dark:text-gray-400">
          <input v-model="rememberQuery" type="checkbox" class="mt-1 rounded border-gray-300 text-primary-600 focus:ring-primary-500">
          <span>在这台设备记住 7 天。仅建议在私人设备上开启；关闭时只在当前浏览器会话内保留查询状态。</span>
        </label>
      </section>

      <section class="card mt-5 p-5">
        <label class="block">
          <span class="mb-1 block text-sm font-medium">API Key</span>
          <input v-model.trim="apiKey" type="password" class="input" placeholder="sk-...">
        </label>
        <button class="btn btn-primary mt-4 px-5 py-2.5" :disabled="loading || !apiKey" @click="queryKey">按 Key 查询</button>
      </section>

      <p v-if="message" class="mt-4 text-sm text-gray-500 dark:text-gray-400">{{ message }}</p>

      <section class="mt-6 grid gap-4">
        <article v-for="item in items" :key="`${item.order_no}-${item.api_key_id}`" class="card p-5">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 class="text-lg font-bold">{{ item.product_name || 'API Key' }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ item.order_no || '非商城订单' }}</p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <button
                v-if="item.api_key"
                class="btn btn-secondary px-3 py-1.5 text-xs"
                @click="downloadOrderConfigScript(item)"
              >
                配置脚本
              </button>
              <span class="rounded-md bg-gray-100 px-2 py-1 text-xs font-medium dark:bg-dark-800">{{ item.delivery_status || item.key_status }}</span>
            </div>
          </div>
          <dl class="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <div><dt>API Key</dt><dd>{{ item.api_key_masked || '-' }}</dd></div>
            <div><dt>额度</dt><dd>{{ money(item.quota) }}</dd></div>
            <div><dt>已用额度</dt><dd>{{ money(item.quota_used) }}</dd></div>
            <div><dt>账户余额</dt><dd>{{ money(item.balance) }}</dd></div>
            <div><dt>总成本</dt><dd>{{ money(item.total_cost) }}</dd></div>
            <div><dt>Tokens</dt><dd>{{ item.input_tokens }} / {{ item.output_tokens }}</dd></div>
            <div><dt>有效期</dt><dd>{{ formatDate(item.expires_at) }}</dd></div>
            <div><dt>最近使用</dt><dd>{{ formatDate(item.last_used_at) }}</dd></div>
            <div><dt>支付时间</dt><dd>{{ formatDate(item.paid_at) }}</dd></div>
          </dl>
        </article>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { storefrontAPI, type StoreUsageItem } from '@/api/storefront'
import { downloadConfigScript } from '@/utils/configScriptDownload'

const CACHE_KEY = 'storefront.query.session.v1'
const SESSION_KEY = 'storefront.query.session.temp.v1'
const INSTALL_SCRIPT = `$ErrorActionPreference = "Stop"

$StoreId = "9PLM9XGG6VKS"

if (Get-Command winget -ErrorAction SilentlyContinue) {
  winget install --source msstore --id $StoreId --accept-source-agreements --accept-package-agreements
} else {
  Start-Process "https://get.microsoft.com/installer/download/$StoreId?cid=website_cta_psi"
}
`

interface CachedSession {
  email: string
  queryToken: string
  savedAt: number
}

const email = ref('')
const code = ref('')
const apiKey = ref('')
const message = ref('')
const sending = ref(false)
const codeCooldown = ref(0)
const loading = ref(false)
const items = ref<StoreUsageItem[]>([])
const cachedSession = ref<CachedSession | null>(null)
const rememberQuery = ref(false)
let codeCooldownTimer: ReturnType<typeof setInterval> | null = null

function money(value: number) {
  return `$${Number(value || 0).toFixed(4)}`
}

function formatDate(value?: string) {
  return value ? new Date(value).toLocaleString() : '-'
}

function saveFile(filename: string, content: string, mimeType = 'text/plain;charset=utf-8') {
  const blob = new Blob([content], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

function startCodeCooldown(seconds = 60) {
  codeCooldown.value = Math.max(0, Math.ceil(seconds))
  if (codeCooldownTimer) {
    clearInterval(codeCooldownTimer)
  }
  codeCooldownTimer = setInterval(() => {
    codeCooldown.value = Math.max(0, codeCooldown.value - 1)
    if (codeCooldown.value <= 0 && codeCooldownTimer) {
      clearInterval(codeCooldownTimer)
      codeCooldownTimer = null
    }
  }, 1000)
}

function retryAfterFromError(err: any) {
  const value = Number(err?.metadata?.retry_after)
  return Number.isFinite(value) && value > 0 ? value : 60
}

function loadCachedSession() {
  try {
    const raw = window.localStorage.getItem(CACHE_KEY) || window.sessionStorage.getItem(SESSION_KEY)
    if (!raw) return
    const parsed = JSON.parse(raw) as CachedSession
    if (!parsed?.email || !parsed?.queryToken) return
    cachedSession.value = parsed
    email.value = parsed.email
    rememberQuery.value = window.localStorage.getItem(CACHE_KEY) === raw
  } catch {
    window.localStorage.removeItem(CACHE_KEY)
    window.sessionStorage.removeItem(SESSION_KEY)
  }
}

function cacheSession(next: CachedSession) {
  cachedSession.value = next
  const serialized = JSON.stringify(next)
  if (rememberQuery.value) {
    window.localStorage.setItem(CACHE_KEY, serialized)
    window.sessionStorage.removeItem(SESSION_KEY)
  } else {
    window.sessionStorage.setItem(SESSION_KEY, serialized)
    window.localStorage.removeItem(CACHE_KEY)
  }
}

function clearCachedSession() {
  cachedSession.value = null
  items.value = []
  code.value = ''
  window.localStorage.removeItem(CACHE_KEY)
  window.sessionStorage.removeItem(SESSION_KEY)
  message.value = '已退出查询中心。'
}

function downloadInstallScript() {
  saveFile('install-codex.ps1', INSTALL_SCRIPT, 'text/x-powershell;charset=utf-8')
}

function downloadOrderConfigScript(item: StoreUsageItem) {
  if (!item.api_key) {
    message.value = '该订单暂无可下载的 API Key 配置脚本。'
    return
  }
  downloadConfigScript({
    apiKey: item.api_key,
    baseUrl: window.location.origin,
    platform: null
  })
}

async function sendCode() {
  sending.value = true
  message.value = ''
  try {
    await storefrontAPI.sendQueryCode(email.value)
    startCodeCooldown()
    message.value = '验证码已发送，请检查邮箱。'
  } catch (err: any) {
    if (err?.reason === 'STORE_QUERY_CODE_TOO_FREQUENT') {
      startCodeCooldown(retryAfterFromError(err))
      message.value = '发送过于频繁，请稍后再试。'
    } else {
      message.value = err?.reason === 'STORE_EMAIL_NOT_FOUND' ? '无法找到该邮箱任何有效数据。' : (err?.message || '验证码发送失败')
    }
  } finally {
    sending.value = false
  }
}

async function queryEmail() {
  loading.value = true
  message.value = ''
  try {
    const verify = await storefrontAPI.verifyQueryCode(email.value, code.value)
    const token = verify.data.query_token
    cacheSession({ email: email.value, queryToken: token, savedAt: Date.now() })
    const result = await storefrontAPI.queryByEmail(email.value, token)
    items.value = Array.isArray(result.data.items) ? result.data.items : []
    message.value = items.value.length ? '' : '没有找到记录。'
  } catch (err: any) {
    message.value = err?.message || '查询失败'
  } finally {
    loading.value = false
  }
}

async function queryCachedEmail() {
  if (!cachedSession.value) return
  loading.value = true
  message.value = ''
  try {
    const result = await storefrontAPI.queryByEmail(cachedSession.value.email, cachedSession.value.queryToken)
    items.value = Array.isArray(result.data.items) ? result.data.items : []
    message.value = items.value.length ? '' : '没有找到记录。'
  } catch (err: any) {
    clearCachedSession()
    message.value = err?.message || '缓存状态已失效，请重新验证邮箱。'
  } finally {
    loading.value = false
  }
}

async function queryKey() {
  loading.value = true
  message.value = ''
  try {
    const result = await storefrontAPI.queryByKey(apiKey.value)
    items.value = Array.isArray(result.data.items) ? result.data.items : []
    message.value = items.value.length ? '' : '没有找到记录。'
  } catch (err: any) {
    message.value = err?.message || '查询失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadCachedSession()
  if (cachedSession.value) {
    queryCachedEmail()
  }
})

onUnmounted(() => {
  if (codeCooldownTimer) {
    clearInterval(codeCooldownTimer)
    codeCooldownTimer = null
  }
})
</script>

<style scoped>
dt {
  font-size: 0.75rem;
  color: rgb(107 114 128);
}

dd {
  margin-top: 0.25rem;
  overflow-wrap: anywhere;
  font-weight: 600;
}
</style>

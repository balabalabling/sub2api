<template>
  <div class="min-h-screen bg-[#f6f9f7] text-slate-800 dark:bg-dark-950 dark:text-white">
    <main class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
      <header class="mb-6 overflow-hidden rounded-2xl border border-white/70 bg-white/85 p-6 shadow-sm dark:border-dark-700 dark:bg-dark-900/90 sm:p-8">
        <div class="grid gap-6 lg:grid-cols-[1fr_340px] lg:items-center">
          <div>
            <div class="mb-4 flex flex-wrap items-center gap-2">
              <span class="rounded-full bg-emerald-50 px-3 py-1 text-xs font-bold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">官方商城</span>
              <span class="rounded-full bg-slate-100 px-3 py-1 text-xs font-bold text-slate-600 dark:bg-dark-800 dark:text-slate-300">订阅套餐 / API Key / 人工商品</span>
            </div>
            <h1 class="text-3xl font-extrabold tracking-normal text-slate-950 dark:text-white sm:text-4xl">Code With AI Store</h1>
            <p class="mt-3 max-w-2xl text-sm leading-6 text-slate-500 dark:text-gray-400">
              {{ t('storefront.description') }} 支付后可在订单查询中心查看交付状态、API Key、额度用量，并下载安装与配置脚本。
            </p>
            <div class="mt-5 flex flex-wrap gap-3">
              <RouterLink to="/storefront/query" class="btn btn-primary inline-flex items-center justify-center px-5 py-2.5">
                {{ t('storefront.queryOrders') }}
              </RouterLink>
              <a href="#storefront-faq" class="btn btn-secondary inline-flex items-center justify-center px-5 py-2.5">购买说明</a>
            </div>
          </div>
          <div class="rounded-2xl bg-slate-50 p-4 dark:bg-dark-800">
            <div class="grid grid-cols-2 gap-2">
              <div class="rounded-xl bg-white p-4 dark:bg-dark-900">
                <div class="text-xs text-slate-500 dark:text-gray-400">交付</div>
                <div class="mt-1 text-lg font-extrabold text-slate-950 dark:text-white">自动/人工</div>
              </div>
              <div class="rounded-xl bg-white p-4 dark:bg-dark-900">
                <div class="text-xs text-slate-500 dark:text-gray-400">查询</div>
                <div class="mt-1 text-lg font-extrabold text-slate-950 dark:text-white">邮箱验证码</div>
              </div>
              <div class="rounded-xl bg-white p-4 dark:bg-dark-900">
                <div class="text-xs text-slate-500 dark:text-gray-400">脚本</div>
                <div class="mt-1 text-lg font-extrabold text-slate-950 dark:text-white">一键下载</div>
              </div>
              <div class="rounded-xl bg-white p-4 dark:bg-dark-900">
                <div class="text-xs text-slate-500 dark:text-gray-400">套餐</div>
                <div class="mt-1 text-lg font-extrabold text-slate-950 dark:text-white">自动上架</div>
              </div>
            </div>
          </div>
        </div>
      </header>

      <div v-if="loading" class="flex justify-center py-20">
        <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
      </div>

      <div v-else-if="loadError" class="card p-8">
        <h2 class="text-lg font-bold text-gray-900 dark:text-white">{{ t('storefront.loadFailed') }}</h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ loadError }}</p>
        <div class="mt-5 flex flex-wrap gap-3">
          <button class="btn btn-primary px-5 py-2.5" @click="loadProducts">{{ t('common.refresh') }}</button>
          <RouterLink to="/storefront/query" class="btn btn-secondary px-5 py-2.5">{{ t('storefront.queryOrders') }}</RouterLink>
        </div>
      </div>

      <div v-else class="grid gap-6 lg:grid-cols-[1fr_380px]">
        <section>
          <div class="mb-4 flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h2 class="text-lg font-semibold">{{ t('storefront.products') }}</h2>
              <p class="text-sm text-gray-500 dark:text-gray-400">订阅套餐和普通商品统一展示，选择后在右侧确认购买信息。</p>
            </div>
            <span v-if="hasCachedEmail" class="text-xs font-medium text-primary-600 dark:text-primary-400">
              已使用缓存邮箱：{{ email }}
            </span>
          </div>
          <div v-if="products.length === 0" class="card p-8 text-center text-gray-500 dark:text-gray-400">
            {{ t('storefront.empty') }}
          </div>
          <div class="grid gap-4 md:grid-cols-2">
            <button
              v-for="product in products"
              :key="`${product.source}:${product.id}`"
              type="button"
              class="flex min-h-[292px] flex-col rounded-2xl border bg-white p-5 text-left shadow-sm transition dark:bg-dark-900"
              :class="selectedProduct?.source === product.source && selectedProduct?.id === product.id ? 'border-emerald-600 ring-4 ring-emerald-600/10' : 'border-slate-200 hover:-translate-y-0.5 hover:border-slate-300 hover:shadow-md dark:border-dark-700 dark:hover:border-dark-600'"
              :disabled="product.source === 'store_product' && isSoldOut(product)"
              @click="selectProduct(product)"
            >
              <div class="mb-3 flex items-center justify-between gap-3">
                <div class="flex flex-wrap items-center gap-2">
                  <span :class="['rounded-full px-2.5 py-1 text-xs font-bold', sourceBadgeClass(product)]">
                    {{ sourceLabel(product) }}
                  </span>
                  <span v-if="productBadge(product)" :class="['rounded-full px-2.5 py-1 text-xs font-bold', productBadgeClass(product)]">
                    {{ productBadge(product) }}
                  </span>
                </div>
                <span v-if="product.source === 'subscription_plan'" class="shrink-0 text-xs font-bold text-emerald-700 dark:text-emerald-300">
                  {{ product.validity_days }}{{ validityUnitLabel(product.validity_unit) }}
                </span>
                <span v-else-if="isSoldOut(product)" class="text-xs font-medium text-red-500">{{ t('storefront.soldOut') }}</span>
              </div>
              <h3 class="text-xl font-extrabold text-slate-950 dark:text-white">{{ product.name }}</h3>
              <p class="mt-2 line-clamp-2 min-h-[48px] text-sm leading-6 text-slate-500 dark:text-gray-400">{{ product.description }}</p>
              <div class="mt-4 grid grid-cols-2 overflow-hidden rounded-xl border border-slate-100 bg-slate-50 text-xs dark:border-dark-700 dark:bg-dark-800">
                <div class="border-b border-r border-slate-100 p-3 dark:border-dark-700">
                  <span class="block text-slate-400 dark:text-gray-500">到手内容</span>
                  <span class="mt-1 block font-extrabold text-slate-900 dark:text-gray-100">{{ deliverySummary(product) }}</span>
                </div>
                <div class="border-b border-slate-100 p-3 dark:border-dark-700">
                  <span class="block text-slate-400 dark:text-gray-500">交付方式</span>
                  <span class="mt-1 block font-extrabold text-slate-900 dark:text-gray-100">{{ deliveryModeLabel(product) }}</span>
                </div>
                <div class="border-r border-slate-100 p-3 dark:border-dark-700">
                  <span class="block text-slate-400 dark:text-gray-500">{{ product.source === 'subscription_plan' ? '适用分组' : '库存' }}</span>
                  <span class="mt-1 block font-extrabold text-slate-900 dark:text-gray-100">{{ secondaryMetric(product) }}</span>
                </div>
                <div class="p-3">
                  <span class="block text-slate-400 dark:text-gray-500">{{ product.source === 'subscription_plan' ? 'Key 额度' : '商品类型' }}</span>
                  <span class="mt-1 block font-extrabold text-slate-900 dark:text-gray-100">{{ finalMetric(product) }}</span>
                </div>
              </div>
              <div class="mt-auto flex items-end justify-between gap-3 pt-4">
                <div class="text-2xl font-extrabold text-emerald-800 dark:text-emerald-300">
                  {{ formatMoney(product.price, product.currency) }}
                </div>
                <span
                  class="rounded-lg px-3 py-2 text-sm font-semibold"
                  :class="isSoldOut(product) ? 'bg-gray-100 text-gray-400 dark:bg-dark-800' : 'bg-emerald-50 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300'"
                >
                  {{ cardActionLabel(product) }}
                </span>
              </div>
            </button>
          </div>
        </section>

        <aside class="h-fit rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900 lg:sticky lg:top-6">
          <div class="flex items-start justify-between gap-3">
            <div>
              <h2 class="text-lg font-semibold">{{ t('storefront.checkout') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">确认商品、邮箱和支付方式。</p>
            </div>
            <span v-if="selectedProduct" class="rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-gray-300">
              {{ sourceLabel(selectedProduct) }}
            </span>
          </div>
          <div v-if="selectedProduct" class="mt-4 rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
            <div class="font-medium">{{ selectedProduct.name }}</div>
            <div class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ selectedProduct.description }}</div>
            <div class="mt-3 grid grid-cols-2 gap-2 text-xs text-gray-500 dark:text-gray-400">
              <div>
                <span class="block text-gray-400 dark:text-gray-500">价格</span>
                <span class="font-semibold text-primary-600 dark:text-primary-400">{{ formatMoney(selectedProduct.price, selectedProduct.currency) }}</span>
              </div>
              <div>
                <span class="block text-gray-400 dark:text-gray-500">内容</span>
                <span class="font-semibold text-gray-700 dark:text-gray-200">{{ deliverySummary(selectedProduct) }}</span>
              </div>
            </div>
          </div>
          <form class="mt-5 space-y-4" @submit.prevent="createOrder">
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{{ t('storefront.email') }}</span>
              <input v-model.trim="email" type="email" required class="input" placeholder="you@example.com">
            </label>
            <div v-if="selectedProduct?.source === 'subscription_plan' && !hasCachedEmail" class="rounded-lg border border-dashed border-gray-200 p-3 dark:border-dark-700">
              <div class="grid gap-2 sm:grid-cols-[1fr_auto]">
                <input ref="emailCodeInput" v-model.trim="emailCode" class="input" :placeholder="t('storefront.emailCodePlaceholder')" maxlength="6">
                <button type="button" class="btn btn-secondary whitespace-nowrap px-4" :disabled="sendingCode || codeCooldown > 0 || !email" @click="sendEmailCode">
                  {{ sendingCode ? t('storefront.sendingCode') : codeCooldown > 0 ? t('storefront.resendIn', { seconds: codeCooldown }) : t('storefront.sendCode') }}
                </button>
              </div>
              <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('storefront.subscriptionEmailVerifyHint') }}</p>
            </div>
            <button v-if="selectedProduct?.source !== 'subscription_plan'" class="btn btn-primary w-full py-3" :disabled="submitting || !selectedProduct || isSoldOut(selectedProduct)">
              {{ submitting ? t('storefront.creatingOrder') : cardActionLabel(selectedProduct) }}
            </button>
            <button
              v-if="selectedProduct?.source === 'subscription_plan'"
              type="submit"
              class="btn btn-primary w-full py-3"
              :disabled="submitting || !email || (!hasCachedEmail && !emailCode)"
            >
              {{ submitting ? t('storefront.creatingOrder') : cardActionLabel(selectedProduct) }}
            </button>
            <p v-if="selectedProduct?.source === 'subscription_plan'" class="text-sm text-gray-500 dark:text-gray-400">{{ t('storefront.subscriptionCheckoutHint') }}</p>
            <p v-if="message" class="text-sm text-gray-500 dark:text-gray-400">{{ message }}</p>
          </form>
        </aside>
      </div>

      <div v-if="payment" class="fixed inset-0 z-50 grid place-items-center bg-black/40 px-4">
        <div class="w-full max-w-md rounded-lg bg-white p-5 shadow-xl dark:bg-dark-900">
          <div class="flex items-center justify-between gap-4">
            <h2 class="text-lg font-bold">{{ t('storefront.alipayPay') }}</h2>
            <button class="btn btn-secondary px-3 py-1" @click="payment = null">{{ t('common.close') }}</button>
          </div>
          <div class="mt-4 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('storefront.paymentCreated') }}</p>
            <a v-if="payment.payment.pay_url" :href="payment.payment.pay_url" target="_blank" rel="noreferrer" class="btn btn-primary mt-4 block text-center py-3">
              {{ t('storefront.openCashier') }}
            </a>
            <div v-if="payment.payment.qr_code" class="mt-4 break-all rounded bg-gray-50 p-3 text-xs dark:bg-dark-800">
              {{ payment.payment.qr_code }}
            </div>
            <RouterLink to="/storefront/query" class="btn btn-primary mt-3 block text-center py-3">查看订单</RouterLink>
            <button class="btn btn-secondary mt-3 w-full py-3" @click="copyQueryEmail">复制查询邮箱</button>
            <RouterLink to="/storefront/query" class="btn btn-secondary mt-3 block text-center py-3">下载脚本</RouterLink>
          </div>
        </div>
      </div>

      <section id="storefront-faq" class="mt-8 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="mb-5">
          <h2 class="text-xl font-extrabold text-slate-950 dark:text-white">购买说明</h2>
          <p class="mt-1 text-sm text-slate-500 dark:text-gray-400">下单、交付、查询和脚本相关的常见问题。</p>
        </div>
        <div class="grid gap-3 md:grid-cols-2">
          <div v-for="faq in faqs" :key="faq.question" class="rounded-xl bg-slate-50 p-4 dark:bg-dark-800">
            <h3 class="text-sm font-bold text-slate-950 dark:text-white">{{ faq.question }}</h3>
            <p class="mt-2 text-sm leading-6 text-slate-500 dark:text-gray-400">{{ faq.answer }}</p>
          </div>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { storefrontAPI, type StorefrontProduct, type StoreOrderResult } from '@/api/storefront'

const { t } = useI18n()
const QUERY_CACHE_KEY = 'storefront.query.session.v1'
const QUERY_SESSION_KEY = 'storefront.query.session.temp.v1'
const products = ref<StorefrontProduct[]>([])
const selectedProduct = ref<StorefrontProduct | null>(null)
const email = ref('')
const emailCode = ref('')
const loading = ref(false)
const submitting = ref(false)
const sendingCode = ref(false)
const codeCooldown = ref(0)
const message = ref('')
const loadError = ref('')
const payment = ref<StoreOrderResult | null>(null)
const cachedQueryToken = ref('')
const emailCodeInput = ref<HTMLInputElement | null>(null)
let codeCooldownTimer: ReturnType<typeof setInterval> | null = null

const faqs = [
  { question: '支付后多久交付？', answer: 'API Key 和订阅套餐通常会在支付成功后自动交付；人工商品会按商品说明处理。' },
  { question: '如何查询订单？', answer: '使用下单邮箱接收验证码后进入订单查询中心，可查看订单、API Key、额度和用量。' },
  { question: '安装脚本和配置脚本有什么区别？', answer: '安装脚本用于安装客户端或工具；配置脚本会写入具体订单对应的 API Key。' },
  { question: '生成新 Key 和充值已有 Key 怎么选？', answer: '首次购买建议生成新 Key；已有同分组 Key 时可选择充值，额度会增加并按规则更新有效期。' }
]

const hasCachedEmail = computed(() => !!email.value && !!cachedQueryToken.value)

function formatMoney(price: number, currency: string) {
  return `${currency || 'CNY'} ${Number(price || 0).toFixed(2)}`
}

function productTypeLabel(type: string) {
  const labels: Record<string, string> = {
    api_key: 'API Key',
    account: t('admin.store.products.productTypes.account'),
    sms: t('admin.store.products.productTypes.sms'),
    manual: t('admin.store.products.productTypes.manual'),
    subscription_plan: t('admin.store.products.productTypes.subscription_plan')
  }
  return labels[type] || type
}

function sourceLabel(product: StorefrontProduct) {
  return product.source === 'subscription_plan' ? t('storefront.subscriptionSource') : productTypeLabel(product.product_type)
}

function sourceBadgeClass(product: StorefrontProduct) {
  return product.source === 'subscription_plan'
    ? 'bg-emerald-50 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300'
    : 'bg-slate-100 text-slate-700 dark:bg-dark-800 dark:text-slate-300'
}

function validityUnitLabel(unit?: string) {
  return unit === 'month' || unit === 'months' ? t('storefront.months') : unit === 'year' || unit === 'years' ? t('storefront.years') : t('storefront.days')
}

function keyQuotaLabel(quota?: number) {
  return !quota ? t('storefront.unlimitedKeyQuota') : t('storefront.keyQuota', { quota: Number(quota).toFixed(2) })
}

function deliveryModeLabel(product: StorefrontProduct) {
  if (product.source === 'subscription_plan') return '自动生成或充值'
  return product.delivery_mode === 'manual' ? '人工交付' : '自动交付'
}

function deliverySummary(product: StorefrontProduct) {
  if (product.source === 'subscription_plan') {
    return `${product.validity_days || 0}${validityUnitLabel(product.validity_unit)} · ${keyQuotaLabel(product.key_quota_usd)}`
  }
  if (product.product_type === 'api_key') return 'API Key'
  if (product.product_type === 'account') return '账号信息'
  if (product.product_type === 'sms') return '短信/接码服务'
  return '人工交付商品'
}

function productBadge(product: StorefrontProduct) {
  if (product.source === 'subscription_plan') return '推荐套餐'
  if (isSoldOut(product)) return '已售罄'
  if (product.delivery_mode === 'auto') return '自动交付'
  if (product.stock_mode === 'tracked') return '库存有限'
  return '人工服务'
}

function productBadgeClass(product: StorefrontProduct) {
  if (isSoldOut(product)) return 'bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-300'
  if (product.source === 'subscription_plan') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (product.delivery_mode === 'auto') return 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
  return 'bg-violet-50 text-violet-700 dark:bg-violet-900/30 dark:text-violet-300'
}

function secondaryMetric(product: StorefrontProduct) {
  if (product.source === 'subscription_plan') return product.group_name || product.group_platform || 'Group'
  if (product.stock_mode === 'tracked') return `${product.stock_count || 0} 件`
  return '不限库存'
}

function finalMetric(product: StorefrontProduct) {
  if (product.source === 'subscription_plan') return keyQuotaLabel(product.key_quota_usd)
  return productTypeLabel(product.product_type)
}

function cardActionLabel(product: StorefrontProduct | null) {
  if (!product) return t('storefront.alipayPay')
  if (isSoldOut(product)) return t('storefront.soldOut')
  return product.source === 'subscription_plan' ? '购买套餐' : '立即购买'
}

function isSoldOut(product: StorefrontProduct | null) {
  return !!product && product.stock_mode === 'tracked' && (product.stock_count || 0) <= 0
}

function selectProduct(product: StorefrontProduct) {
  selectedProduct.value = product
  message.value = ''
}

function readCachedQuerySession() {
  try {
    const raw = window.localStorage.getItem(QUERY_CACHE_KEY) || window.sessionStorage.getItem(QUERY_SESSION_KEY)
    if (!raw) return
    const parsed = JSON.parse(raw) as { email?: string; queryToken?: string; savedAt?: number }
    if (!parsed.email || !parsed.queryToken) return
    email.value = parsed.email
    cachedQueryToken.value = parsed.queryToken
  } catch {
    cachedQueryToken.value = ''
  }
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
  const raw = err?.metadata?.retry_after
  const value = Number(raw)
  return Number.isFinite(value) && value > 0 ? value : 60
}

async function loadProducts() {
  loading.value = true
  loadError.value = ''
  message.value = ''
  try {
    const { data } = await storefrontAPI.listProducts()
    const list = Array.isArray(data) ? data : []
    products.value = list
    selectedProduct.value = list.find((item) => item.source === 'store_product' && !isSoldOut(item)) || list[0] || null
  } catch (err: any) {
    products.value = []
    selectedProduct.value = null
    loadError.value = err?.message || t('storefront.loadFailed')
  } finally {
    loading.value = false
  }
}

async function sendEmailCode() {
  if (!email.value) return
  sendingCode.value = true
  message.value = ''
  try {
    await storefrontAPI.sendQueryCode(email.value)
    startCodeCooldown()
    message.value = t('storefront.emailCodeSent')
    await nextTick()
    emailCodeInput.value?.focus()
  } catch (err: any) {
    if (err?.reason === 'STORE_QUERY_CODE_TOO_FREQUENT') {
      startCodeCooldown(retryAfterFromError(err))
      message.value = t('storefront.emailCodeTooFrequent')
    } else {
      message.value = err?.reason === 'STORE_EMAIL_NOT_FOUND' ? t('storefront.emailNoValidData') : (err?.message || t('storefront.emailCodeFailed'))
    }
  } finally {
    sendingCode.value = false
  }
}

async function verifyEmailIfNeeded() {
  if (selectedProduct.value?.source !== 'subscription_plan' || hasCachedEmail.value) return true
  if (!email.value || !emailCode.value) return false
  const verify = await storefrontAPI.verifyQueryCode(email.value, emailCode.value)
  const token = verify.data?.query_token || ''
  if (token) {
    cachedQueryToken.value = token
    window.sessionStorage.setItem(QUERY_SESSION_KEY, JSON.stringify({ email: email.value, queryToken: token, savedAt: Date.now() }))
  }
  return true
}

async function createOrder() {
  if (!selectedProduct.value) return
  submitting.value = true
  message.value = ''
  try {
    if (!await verifyEmailIfNeeded()) {
      message.value = t('storefront.emailCodeRequired')
      return
    }
    const { data } = await storefrontAPI.createOrder({
      email: email.value,
      product_id: selectedProduct.value.source === 'store_product' ? selectedProduct.value.id : undefined,
      source: selectedProduct.value.source,
      plan_id: selectedProduct.value.source === 'subscription_plan' ? (selectedProduct.value.plan_id || selectedProduct.value.id) : undefined,
      query_token: selectedProduct.value.source === 'subscription_plan' ? cachedQueryToken.value : undefined,
      payment_type: 'alipay',
      return_url: `${window.location.origin}/storefront/query`
    })
    payment.value = data
    message.value = t('storefront.orderCreated', { orderNo: data.store_order.order_no })
  } catch (err: any) {
    message.value = err?.message || t('storefront.createOrderFailed')
  } finally {
    submitting.value = false
  }
}

async function copyQueryEmail() {
  if (!payment.value?.store_order?.email) return
  try {
    await navigator.clipboard.writeText(payment.value.store_order.email)
    message.value = '查询邮箱已复制。'
  } catch {
    message.value = payment.value.store_order.email
  }
}

onMounted(() => {
  readCachedQuerySession()
  void loadProducts()
})

onUnmounted(() => {
  if (codeCooldownTimer) {
    clearInterval(codeCooldownTimer)
    codeCooldownTimer = null
  }
})
</script>

<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <main class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
      <header class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-3xl font-bold tracking-normal">API Key Store</h1>
          <p class="mt-2 max-w-2xl text-sm text-gray-500 dark:text-gray-400">
            {{ t('storefront.description') }}
          </p>
        </div>
        <RouterLink to="/storefront/query" class="btn btn-secondary inline-flex items-center justify-center px-4 py-2">
          {{ t('storefront.queryOrders') }}
        </RouterLink>
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

      <div v-else class="grid gap-6 lg:grid-cols-[1fr_360px]">
        <section>
          <h2 class="mb-4 text-lg font-semibold">{{ t('storefront.products') }}</h2>
          <div v-if="products.length === 0" class="card p-8 text-center text-gray-500 dark:text-gray-400">
            {{ t('storefront.empty') }}
          </div>
          <div class="grid gap-4 md:grid-cols-2">
            <button
              v-for="product in products"
              :key="`${product.source}:${product.id}`"
              type="button"
              class="card min-h-[172px] border p-5 text-left transition"
              :class="selectedProduct?.source === product.source && selectedProduct?.id === product.id ? 'border-primary-500 ring-2 ring-primary-500/20' : 'border-transparent hover:border-gray-300 dark:hover:border-dark-600'"
              :disabled="product.source === 'store_product' && isSoldOut(product)"
              @click="selectProduct(product)"
            >
              <div class="mb-3 flex items-center justify-between gap-3">
                <span class="rounded-md bg-gray-100 px-2 py-1 text-xs font-medium uppercase text-gray-600 dark:bg-dark-800 dark:text-gray-300">
                  {{ sourceLabel(product) }}
                </span>
                <span v-if="product.source === 'subscription_plan'" class="text-xs font-medium text-primary-600 dark:text-primary-400">
                  {{ product.validity_days }}{{ validityUnitLabel(product.validity_unit) }}
                </span>
                <span v-else-if="isSoldOut(product)" class="text-xs font-medium text-red-500">{{ t('storefront.soldOut') }}</span>
              </div>
              <h3 class="text-lg font-bold">{{ product.name }}</h3>
              <p class="mt-2 line-clamp-3 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ product.description }}</p>
              <div v-if="product.source === 'subscription_plan'" class="mt-3 grid grid-cols-2 gap-2 text-xs text-gray-500 dark:text-gray-400">
                <div>{{ product.group_name || product.group_platform || 'Group' }}</div>
                <div>{{ keyQuotaLabel(product.key_quota_usd) }}</div>
              </div>
              <div class="mt-4 text-2xl font-bold text-primary-600 dark:text-primary-400">
                {{ formatMoney(product.price, product.currency) }}
              </div>
            </button>
          </div>
        </section>

        <aside class="card h-fit p-5">
          <h2 class="text-lg font-semibold">{{ t('storefront.checkout') }}</h2>
          <div v-if="selectedProduct" class="mt-4 rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
            <div class="font-medium">{{ selectedProduct.name }}</div>
            <div class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ formatMoney(selectedProduct.price, selectedProduct.currency) }}</div>
          </div>
          <form class="mt-5 space-y-4" @submit.prevent="createOrder">
            <label class="block">
              <span class="mb-1 block text-sm font-medium">{{ t('storefront.email') }}</span>
              <input v-model.trim="email" type="email" required class="input" placeholder="you@example.com">
            </label>
            <div v-if="selectedProduct?.source === 'subscription_plan' && !hasCachedEmail" class="rounded-lg border border-dashed border-gray-200 p-3 dark:border-dark-700">
              <div class="flex gap-2">
                <input v-model.trim="emailCode" class="input" :placeholder="t('storefront.emailCodePlaceholder')" maxlength="6">
                <button type="button" class="btn btn-secondary whitespace-nowrap px-4" :disabled="sendingCode || !email" @click="sendEmailCode">
                  {{ sendingCode ? t('storefront.sendingCode') : t('storefront.sendCode') }}
                </button>
              </div>
              <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('storefront.subscriptionEmailVerifyHint') }}</p>
            </div>
            <button v-if="selectedProduct?.source !== 'subscription_plan'" class="btn btn-primary w-full py-3" :disabled="submitting || !selectedProduct || isSoldOut(selectedProduct)">
              {{ submitting ? t('storefront.creatingOrder') : t('storefront.alipayPay') }}
            </button>
            <button
              v-if="selectedProduct?.source === 'subscription_plan'"
              type="submit"
              class="btn btn-primary w-full py-3"
              :disabled="submitting || !email || (!hasCachedEmail && !emailCode)"
            >
              {{ submitting ? t('storefront.creatingOrder') : t('storefront.alipayPay') }}
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
            <RouterLink to="/storefront/query" class="btn btn-secondary mt-3 block text-center py-3">{{ t('storefront.queryLater') }}</RouterLink>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
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
const message = ref('')
const loadError = ref('')
const payment = ref<StoreOrderResult | null>(null)
const cachedQueryToken = ref('')

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

function validityUnitLabel(unit?: string) {
  return unit === 'month' || unit === 'months' ? t('storefront.months') : unit === 'year' || unit === 'years' ? t('storefront.years') : t('storefront.days')
}

function keyQuotaLabel(quota?: number) {
  return !quota ? t('storefront.unlimitedKeyQuota') : t('storefront.keyQuota', { quota: Number(quota).toFixed(2) })
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
    message.value = t('storefront.emailCodeSent')
  } catch (err: any) {
    message.value = err?.reason === 'STORE_EMAIL_NOT_FOUND' ? t('storefront.emailNoValidData') : (err?.message || t('storefront.emailCodeFailed'))
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

onMounted(() => {
  readCachedQuerySession()
  void loadProducts()
})
</script>

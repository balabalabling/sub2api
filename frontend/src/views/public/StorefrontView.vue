<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <main class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
      <header class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-3xl font-bold tracking-normal">API Key Store</h1>
          <p class="mt-2 max-w-2xl text-sm text-gray-500 dark:text-gray-400">
            选择商品并使用支付宝付款。API Key 商品会自动发货到邮箱，账号、短信和人工商品会进入发货队列。
          </p>
        </div>
        <RouterLink to="/storefront/query" class="btn btn-secondary inline-flex items-center justify-center px-4 py-2">
          查询订单和用量
        </RouterLink>
      </header>

      <div v-if="loading" class="flex justify-center py-20">
        <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
      </div>

      <div v-else class="grid gap-6 lg:grid-cols-[1fr_360px]">
        <section>
          <h2 class="mb-4 text-lg font-semibold">商品</h2>
          <div v-if="products.length === 0" class="card p-8 text-center text-gray-500 dark:text-gray-400">
            暂无可购买商品
          </div>
          <div class="grid gap-4 md:grid-cols-2">
            <button
              v-for="product in products"
              :key="product.id"
              type="button"
              class="card min-h-[172px] border p-5 text-left transition"
              :class="selectedProduct?.id === product.id ? 'border-primary-500 ring-2 ring-primary-500/20' : 'border-transparent hover:border-gray-300 dark:hover:border-dark-600'"
              :disabled="isSoldOut(product)"
              @click="selectedProduct = product"
            >
              <div class="mb-3 flex items-center justify-between gap-3">
                <span class="rounded-md bg-gray-100 px-2 py-1 text-xs font-medium uppercase text-gray-600 dark:bg-dark-800 dark:text-gray-300">
                  {{ productTypeLabel(product.product_type) }}
                </span>
                <span v-if="isSoldOut(product)" class="text-xs font-medium text-red-500">售罄</span>
              </div>
              <h3 class="text-lg font-bold">{{ product.name }}</h3>
              <p class="mt-2 line-clamp-3 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ product.description }}</p>
              <div class="mt-4 text-2xl font-bold text-primary-600 dark:text-primary-400">
                {{ formatMoney(product.price, product.currency) }}
              </div>
            </button>
          </div>
        </section>

        <aside class="card h-fit p-5">
          <h2 class="text-lg font-semibold">下单</h2>
          <div v-if="selectedProduct" class="mt-4 rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
            <div class="font-medium">{{ selectedProduct.name }}</div>
            <div class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ formatMoney(selectedProduct.price, selectedProduct.currency) }}</div>
          </div>
          <form class="mt-5 space-y-4" @submit.prevent="createOrder">
            <label class="block">
              <span class="mb-1 block text-sm font-medium">接收邮箱</span>
              <input v-model.trim="email" type="email" required class="input" placeholder="you@example.com">
            </label>
            <button class="btn btn-primary w-full py-3" :disabled="submitting || !selectedProduct || isSoldOut(selectedProduct)">
              {{ submitting ? '创建订单中...' : '支付宝支付' }}
            </button>
            <p v-if="message" class="text-sm text-gray-500 dark:text-gray-400">{{ message }}</p>
          </form>
        </aside>
      </div>

      <div v-if="payment" class="fixed inset-0 z-50 grid place-items-center bg-black/40 px-4">
        <div class="w-full max-w-md rounded-lg bg-white p-5 shadow-xl dark:bg-dark-900">
          <div class="flex items-center justify-between gap-4">
            <h2 class="text-lg font-bold">支付宝支付</h2>
            <button class="btn btn-secondary px-3 py-1" @click="payment = null">关闭</button>
          </div>
          <div class="mt-4 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
            <p class="text-sm text-gray-500 dark:text-gray-400">订单已创建，请完成支付。支付成功后系统会自动处理发货。</p>
            <a v-if="payment.payment.pay_url" :href="payment.payment.pay_url" target="_blank" rel="noreferrer" class="btn btn-primary mt-4 block text-center py-3">
              打开支付宝收银台
            </a>
            <div v-if="payment.payment.qr_code" class="mt-4 break-all rounded bg-gray-50 p-3 text-xs dark:bg-dark-800">
              {{ payment.payment.qr_code }}
            </div>
            <RouterLink to="/storefront/query" class="btn btn-secondary mt-3 block text-center py-3">稍后查询订单</RouterLink>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { storefrontAPI, type StoreProduct, type StoreOrderResult } from '@/api/storefront'

const products = ref<StoreProduct[]>([])
const selectedProduct = ref<StoreProduct | null>(null)
const email = ref('')
const loading = ref(false)
const submitting = ref(false)
const message = ref('')
const payment = ref<StoreOrderResult | null>(null)

function formatMoney(price: number, currency: string) {
  return `${currency || 'CNY'} ${Number(price || 0).toFixed(2)}`
}

function productTypeLabel(type: string) {
  const labels: Record<string, string> = {
    api_key: 'API Key',
    account: '拼车账号',
    sms: '短信业务',
    manual: '人工发货'
  }
  return labels[type] || type
}

function isSoldOut(product: StoreProduct | null) {
  return !!product && product.stock_mode === 'tracked' && product.stock_count <= 0
}

async function loadProducts() {
  loading.value = true
  try {
    const { data } = await storefrontAPI.listProducts()
    const list = Array.isArray(data) ? data : []
    products.value = list
    selectedProduct.value = list.find((item) => !isSoldOut(item)) || list[0] || null
  } catch (err: any) {
    products.value = []
    selectedProduct.value = null
    message.value = err?.message || '商品加载失败'
  } finally {
    loading.value = false
  }
}

async function createOrder() {
  if (!selectedProduct.value) return
  submitting.value = true
  message.value = ''
  try {
    const { data } = await storefrontAPI.createOrder({
      email: email.value,
      product_id: selectedProduct.value.id,
      payment_type: 'alipay',
      return_url: `${window.location.origin}/storefront/query`
    })
    payment.value = data
    message.value = `订单 ${data.store_order.order_no} 已创建`
  } catch (err: any) {
    message.value = err?.message || '创建订单失败'
  } finally {
    submitting.value = false
  }
}

onMounted(loadProducts)
</script>

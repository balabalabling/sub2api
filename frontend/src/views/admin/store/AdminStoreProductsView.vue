<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('admin.store.products.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.store.products.description') }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="loadProducts">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <button class="btn btn-primary" @click="openCreate">
            <Icon name="plus" size="md" class="mr-1" />
            {{ t('admin.store.products.createProduct') }}
          </button>
        </div>
      </header>

      <section class="rounded-lg border border-primary-100 bg-primary-50 p-4 text-sm text-primary-800 dark:border-primary-900/50 dark:bg-primary-900/20 dark:text-primary-200">
        <p class="font-semibold">普通商品在这里管理，订阅套餐仍在“支付管理 -> 订阅套餐”管理。</p>
        <p class="mt-1">商品设置为 active + public 后会显示在公开商城；订阅套餐上架后会自动进入商城，不需要重复写入普通商品。</p>
      </section>

      <section class="flex flex-wrap items-center gap-3 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <input v-model.trim="searchQuery" class="input min-w-56 flex-1" :placeholder="t('admin.store.products.searchPlaceholder')" />
        <select v-model="typeFilter" class="input w-40">
          <option value="">{{ t('admin.store.products.allTypes') }}</option>
          <option v-for="type in productTypes" :key="type" :value="type">{{ productTypeLabel(type) }}</option>
        </select>
        <select v-model="statusFilter" class="input w-36">
          <option value="">{{ t('admin.store.products.allStatuses') }}</option>
          <option v-for="status in statuses" :key="status" :value="status">{{ t(`admin.store.products.statuses.${status}`) }}</option>
        </select>
        <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
          <input v-model="publicOnlyFilter" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          {{ t('admin.store.products.publicOnly') }}
        </label>
      </section>

      <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
        <div v-if="loading" class="flex justify-center py-16">
          <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
        </div>
        <div v-else-if="filteredProducts.length === 0" class="py-16 text-center text-gray-500 dark:text-gray-400">
          {{ t('admin.store.products.empty') }}
        </div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.store.products.fields.name') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.store.products.fields.productType') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.store.products.fields.price') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.store.products.fields.status') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500">{{ t('admin.store.products.fields.stock') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase text-gray-500">{{ t('admin.store.products.fields.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
              <tr v-for="product in filteredProducts" :key="product.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                <td class="px-4 py-4">
                  <div class="font-medium text-gray-900 dark:text-white">{{ product.name }}</div>
                  <div class="mt-1 line-clamp-2 max-w-md text-sm text-gray-500 dark:text-gray-400">{{ product.description }}</div>
                </td>
                <td class="px-4 py-4">
                  <span class="badge bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200">
                    {{ productTypeLabel(product.product_type) }}
                  </span>
                  <div class="mt-1 text-xs text-gray-500">{{ t(`admin.store.products.deliveryModes.${product.delivery_mode}`) }}</div>
                </td>
                <td class="px-4 py-4 text-sm font-medium text-gray-900 dark:text-white">
                  {{ formatMoney(product.price, product.currency) }}
                </td>
                <td class="px-4 py-4">
                  <div class="flex flex-col gap-1">
                    <span :class="['badge w-fit', storefrontVisibilityClass(product)]">{{ storefrontVisibilityLabel(product) }}</span>
                    <span class="text-xs text-gray-500">
                      {{ t(`admin.store.products.statuses.${product.status}`) }} · {{ t(`admin.store.products.visibilities.${product.visibility}`) }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-600 dark:text-gray-300">
                  {{ stockLabel(product) }}
                </td>
                <td class="px-4 py-4">
                  <div class="flex items-center justify-end gap-1">
                    <button class="rounded-lg p-1.5 text-gray-500 hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-primary-900/20" :title="toggleTitle(product)" @click="toggleActive(product)">
                      <Icon :name="isStorefrontVisible(product) ? 'eyeOff' : 'eye'" size="sm" />
                    </button>
                    <button class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200" :title="t('common.edit')" @click="openEdit(product)">
                      <Icon name="edit" size="sm" />
                    </button>
                    <button class="rounded-lg p-1.5 text-gray-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400" :title="t('common.delete')" @click="deleteProduct(product)">
                      <Icon name="trash" size="sm" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <BaseDialog :show="dialogOpen" :title="editingProduct ? t('admin.store.products.editProduct') : t('admin.store.products.createProduct')" width="wide" @close="closeDialog">
      <form id="store-product-form" class="space-y-5" @submit.prevent="saveProduct">
        <div class="grid gap-5 lg:grid-cols-[1fr_320px]">
          <div class="grid gap-4 md:grid-cols-2">
            <label class="block">
              <span class="input-label">{{ t('admin.store.products.fields.productType') }}</span>
              <select v-model="form.product_type" class="input" @change="applyProductTypeDefaults">
                <option v-for="type in productTypes" :key="type" :value="type">{{ productTypeLabel(type) }}</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.store.products.fields.name') }}</span>
              <input v-model.trim="form.name" class="input" required />
            </label>
            <label class="block md:col-span-2">
              <span class="input-label">{{ t('admin.store.products.fields.description') }}</span>
              <textarea v-model="form.description" class="input min-h-24" rows="3"></textarea>
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.store.products.fields.price') }}</span>
              <input v-model.number="form.price" type="number" min="0.01" step="0.01" class="input" required />
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.store.products.fields.currency') }}</span>
              <input v-model.trim="form.currency" class="input uppercase" maxlength="10" />
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.store.products.fields.status') }}</span>
              <select v-model="form.status" class="input">
                <option v-for="status in statuses" :key="status" :value="status">{{ t(`admin.store.products.statuses.${status}`) }}</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.store.products.fields.visibility') }}</span>
              <select v-model="form.visibility" class="input">
                <option value="public">{{ t('admin.store.products.visibilities.public') }}</option>
                <option value="hidden">{{ t('admin.store.products.visibilities.hidden') }}</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.store.products.fields.sortOrder') }}</span>
              <input v-model.number="form.sort_order" type="number" step="1" class="input" />
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.store.products.fields.deliveryMode') }}</span>
              <select v-model="form.delivery_mode" class="input">
                <option value="auto">{{ t('admin.store.products.deliveryModes.auto') }}</option>
                <option value="manual">{{ t('admin.store.products.deliveryModes.manual') }}</option>
              </select>
            </label>
            <template v-if="form.product_type === 'api_key'">
              <label class="block">
                <span class="input-label">{{ t('admin.store.products.fields.groupId') }}</span>
                <select v-model.number="apiKeyConfig.group_id" class="input">
                  <option :value="0">{{ t('admin.store.products.noGroup') }}</option>
                  <option v-for="group in groups" :key="group.id" :value="group.id">
                    {{ group.name }} · {{ group.platform }}
                  </option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.store.products.fields.quota') }}</span>
                <input v-model.number="apiKeyConfig.quota" type="number" min="0" step="0.01" class="input" />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.store.products.fields.expiresInDays') }}</span>
                <input v-model.number="apiKeyConfig.expires_in_days" type="number" min="1" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.store.products.fields.rateLimit5h') }}</span>
                <input v-model.number="apiKeyConfig.rate_limit_5h" type="number" min="0" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.store.products.fields.rateLimit1d') }}</span>
                <input v-model.number="apiKeyConfig.rate_limit_1d" type="number" min="0" step="1" class="input" />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.store.products.fields.rateLimit7d') }}</span>
                <input v-model.number="apiKeyConfig.rate_limit_7d" type="number" min="0" step="1" class="input" />
              </label>
            </template>
            <label class="block">
              <span class="input-label">{{ t('admin.store.products.fields.stockMode') }}</span>
              <select v-model="form.stock_mode" class="input">
                <option value="unlimited">{{ t('admin.store.products.stockModes.unlimited') }}</option>
                <option value="tracked">{{ t('admin.store.products.stockModes.tracked') }}</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.store.products.fields.stockCount') }}</span>
              <input v-model.number="form.stock_count" type="number" min="0" step="1" class="input" :disabled="form.stock_mode === 'unlimited'" />
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.store.products.fields.saleStartAt') }}</span>
              <input v-model="saleStartLocal" type="datetime-local" class="input" />
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.store.products.fields.saleEndAt') }}</span>
              <input v-model="saleEndLocal" type="datetime-local" class="input" />
            </label>
            <label class="block md:col-span-2">
              <span class="input-label">{{ t('admin.store.products.fields.deliveryConfig') }}</span>
              <textarea v-model="deliveryConfigText" class="input min-h-32 font-mono text-sm" rows="5" spellcheck="false"></textarea>
              <span class="mt-1 block text-xs text-gray-500">
                {{ form.product_type === 'api_key' ? t('admin.store.products.deliveryConfigAdvancedHint') : t('admin.store.products.deliveryConfigHint') }}
              </span>
            </label>
          </div>
          <aside class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="mb-3 flex items-center justify-between gap-3">
              <h3 class="text-sm font-bold text-gray-900 dark:text-white">商城卡片预览</h3>
              <span :class="['rounded-md px-2 py-1 text-xs font-medium', storefrontVisibilityClass(formPreview)]">{{ storefrontVisibilityLabel(formPreview) }}</span>
            </div>
            <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
              <div class="mb-3 flex items-center justify-between gap-2">
                <span class="rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-gray-300">{{ productTypeLabel(form.product_type) }}</span>
                <span v-if="form.stock_mode === 'tracked' && Number(form.stock_count || 0) <= 0" class="text-xs font-medium text-red-500">售罄</span>
              </div>
              <h4 class="text-lg font-bold text-gray-900 dark:text-white">{{ form.name || '商品名称' }}</h4>
              <p class="mt-2 line-clamp-3 min-h-[72px] text-sm leading-6 text-gray-500 dark:text-gray-400">{{ form.description || '商品描述会显示在这里。' }}</p>
              <dl class="mt-4 grid grid-cols-2 gap-2 text-xs text-gray-500 dark:text-gray-400">
                <div>
                  <dt>交付方式</dt>
                  <dd>{{ t(`admin.store.products.deliveryModes.${form.delivery_mode}`) }}</dd>
                </div>
                <div>
                  <dt>库存</dt>
                  <dd>{{ form.stock_mode === 'unlimited' ? t('admin.store.products.stockModes.unlimited') : `${Number(form.stock_count || 0)} 件` }}</dd>
                </div>
              </dl>
              <div class="mt-4 flex items-end justify-between gap-3">
                <span class="text-2xl font-bold text-primary-600 dark:text-primary-400">{{ formatMoney(form.price, form.currency) }}</span>
                <span class="rounded-lg bg-primary-50 px-3 py-2 text-sm font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">立即购买</span>
              </div>
            </div>
            <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">只有 active + public 的商品会在 /storefront 对用户公开展示。</p>
          </aside>
        </div>
      </form>

      <template #footer>
        <button class="btn btn-secondary" @click="closeDialog">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="saving" form="store-product-form" type="submit">
          {{ saving ? t('common.loading') : t('common.save') }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import adminAPI from '@/api/admin'
import adminStoreAPI, {
  type AdminStoreProduct,
  type AdminStoreProductInput,
  type StoreDeliveryMode,
  type StoreProductStatus,
  type StoreProductType,
  type StoreStockMode,
  type StoreVisibility
} from '@/api/admin/store'
import type { AdminGroup } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

const productTypes: StoreProductType[] = ['api_key', 'account', 'sms', 'manual']
const statuses: StoreProductStatus[] = ['draft', 'active', 'inactive']

const products = ref<AdminStoreProduct[]>([])
const groups = ref<AdminGroup[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogOpen = ref(false)
const editingProduct = ref<AdminStoreProduct | null>(null)
const deliveryConfigText = ref('{}')
const saleStartLocal = ref('')
const saleEndLocal = ref('')
const searchQuery = ref('')
const typeFilter = ref('')
const statusFilter = ref('')
const publicOnlyFilter = ref(false)

const apiKeyConfig = reactive({
  group_id: 0,
  quota: 0,
  expires_in_days: 30,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0
})

const form = reactive<AdminStoreProductInput>({
  product_type: 'api_key',
  name: '',
  description: '',
  price: 1,
  currency: 'CNY',
  status: 'draft',
  visibility: 'public',
  sort_order: 0,
  stock_mode: 'unlimited',
  stock_count: 0,
  delivery_mode: 'auto',
  delivery_config: {},
  sale_start_at: null,
  sale_end_at: null
})

const filteredProducts = computed(() => {
  const q = searchQuery.value.toLowerCase()
  return products.value.filter((product) => {
    if (typeFilter.value && product.product_type !== typeFilter.value) return false
    if (statusFilter.value && product.status !== statusFilter.value) return false
    if (publicOnlyFilter.value && !(product.status === 'active' && product.visibility === 'public')) return false
    if (!q) return true
    return product.name.toLowerCase().includes(q) || product.description.toLowerCase().includes(q)
  })
})

const formPreview = computed(() => ({
  status: form.status,
  visibility: form.visibility
}))

function resetForm() {
  Object.assign(form, {
    product_type: 'api_key' as StoreProductType,
    name: '',
    description: '',
    price: 1,
    currency: 'CNY',
    status: 'draft' as StoreProductStatus,
    visibility: 'public' as StoreVisibility,
    sort_order: 0,
    stock_mode: 'unlimited' as StoreStockMode,
    stock_count: 0,
    delivery_mode: 'auto' as StoreDeliveryMode,
    delivery_config: {},
    sale_start_at: null,
    sale_end_at: null
  })
  deliveryConfigText.value = '{}'
  saleStartLocal.value = ''
  saleEndLocal.value = ''
  Object.assign(apiKeyConfig, {
    group_id: 0,
    quota: 0,
    expires_in_days: 30,
    rate_limit_5h: 0,
    rate_limit_1d: 0,
    rate_limit_7d: 0
  })
}

function applyProductTypeDefaults() {
  if (['account', 'sms', 'manual'].includes(form.product_type)) {
    form.delivery_mode = 'manual'
  } else if (form.product_type === 'api_key') {
    form.delivery_mode = 'auto'
  }
}

function formatMoney(price: number, currency: string) {
  return `${currency || 'CNY'} ${Number(price || 0).toFixed(2)}`
}

function productTypeLabel(type: string) {
  return t(`admin.store.products.productTypes.${type}`)
}

function isStorefrontVisible(product: Pick<AdminStoreProduct, 'status' | 'visibility'> | { status: StoreProductStatus; visibility: StoreVisibility }) {
  return product.status === 'active' && product.visibility === 'public'
}

function storefrontVisibilityLabel(product: Pick<AdminStoreProduct, 'status' | 'visibility'> | { status: StoreProductStatus; visibility: StoreVisibility }) {
  return isStorefrontVisible(product) ? '商城可见' : '未公开'
}

function storefrontVisibilityClass(product: Pick<AdminStoreProduct, 'status' | 'visibility'> | { status: StoreProductStatus; visibility: StoreVisibility }) {
  return isStorefrontVisible(product)
    ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
    : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
}

function stockLabel(product: AdminStoreProduct) {
  if (product.stock_mode === 'unlimited') return t('admin.store.products.stockModes.unlimited')
  return t('admin.store.products.stockCountValue', { count: product.stock_count })
}

function toggleTitle(product: AdminStoreProduct) {
  return isStorefrontVisible(product) ? t('admin.store.products.unpublish') : t('admin.store.products.publish')
}

function toLocalInput(value?: string | null) {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function fromLocalInput(value: string) {
  return value ? new Date(value).toISOString() : null
}

async function loadProducts() {
  loading.value = true
  try {
    const { data } = await adminStoreAPI.listProducts()
    products.value = Array.isArray(data) ? data : []
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.store.products.failedToLoad'))
  } finally {
    loading.value = false
  }
}

async function loadGroups() {
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch {
    groups.value = []
  }
}

function openCreate() {
  editingProduct.value = null
  resetForm()
  dialogOpen.value = true
}

function openEdit(product: AdminStoreProduct) {
  editingProduct.value = product
  Object.assign(form, {
    product_type: product.product_type,
    name: product.name,
    description: product.description,
    price: product.price,
    currency: product.currency,
    status: product.status,
    visibility: product.visibility,
    sort_order: product.sort_order,
    stock_mode: product.stock_mode,
    stock_count: product.stock_count,
    delivery_mode: product.delivery_mode,
    delivery_config: product.delivery_config || {},
    sale_start_at: product.sale_start_at || null,
    sale_end_at: product.sale_end_at || null
  })
  deliveryConfigText.value = JSON.stringify(product.delivery_config || {}, null, 2)
  syncApiKeyConfigFromDeliveryConfig(product.delivery_config || {})
  saleStartLocal.value = toLocalInput(product.sale_start_at)
  saleEndLocal.value = toLocalInput(product.sale_end_at)
  dialogOpen.value = true
}

function syncApiKeyConfigFromDeliveryConfig(config: Record<string, unknown>) {
  Object.assign(apiKeyConfig, {
    group_id: Number(config.group_id || 0),
    quota: Number(config.quota || 0),
    expires_in_days: Number(config.expires_in_days || 30),
    rate_limit_5h: Number(config.rate_limit_5h || 0),
    rate_limit_1d: Number(config.rate_limit_1d || 0),
    rate_limit_7d: Number(config.rate_limit_7d || 0)
  })
}

function closeDialog() {
  dialogOpen.value = false
  editingProduct.value = null
}

function buildPayload(): AdminStoreProductInput | null {
  let deliveryConfig: Record<string, unknown>
  try {
    const parsed = JSON.parse(deliveryConfigText.value || '{}')
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new Error('object required')
    }
    deliveryConfig = parsed
  } catch {
    appStore.showError(t('admin.store.products.invalidJson'))
    return null
  }
  if (form.product_type === 'api_key') {
    deliveryConfig = {
      ...deliveryConfig,
      group_id: Number(apiKeyConfig.group_id || 0),
      quota: Number(apiKeyConfig.quota || 0),
      expires_in_days: Number(apiKeyConfig.expires_in_days || 30),
      rate_limit_5h: Number(apiKeyConfig.rate_limit_5h || 0),
      rate_limit_1d: Number(apiKeyConfig.rate_limit_1d || 0),
      rate_limit_7d: Number(apiKeyConfig.rate_limit_7d || 0)
    }
  }
  return {
    ...form,
    currency: (form.currency || 'CNY').toUpperCase(),
    stock_count: form.stock_mode === 'unlimited' ? 0 : Number(form.stock_count || 0),
    delivery_config: deliveryConfig,
    sale_start_at: fromLocalInput(saleStartLocal.value),
    sale_end_at: fromLocalInput(saleEndLocal.value)
  }
}

async function saveProduct() {
  const payload = buildPayload()
  if (!payload) return
  saving.value = true
  try {
    if (editingProduct.value) {
      await adminStoreAPI.updateProduct(editingProduct.value.id, payload)
      appStore.showSuccess(t('admin.store.products.updateSuccess'))
    } else {
      await adminStoreAPI.createProduct(payload)
      appStore.showSuccess(t('admin.store.products.createSuccess'))
    }
    closeDialog()
    await loadProducts()
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.store.products.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function toggleActive(product: AdminStoreProduct) {
  const willPublish = !isStorefrontVisible(product)
  const payload: AdminStoreProductInput = {
    ...product,
    status: willPublish ? 'active' : 'inactive',
    visibility: willPublish ? 'public' : product.visibility
  }
  try {
    await adminStoreAPI.updateProduct(product.id, payload)
    appStore.showSuccess(t('admin.store.products.updateSuccess'))
    await loadProducts()
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.store.products.saveFailed'))
  }
}

async function deleteProduct(product: AdminStoreProduct) {
  if (!window.confirm(t('admin.store.products.deleteConfirm', { name: product.name }))) return
  try {
    await adminStoreAPI.deleteProduct(product.id)
    appStore.showSuccess(t('admin.store.products.deleteSuccess'))
    await loadProducts()
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.store.products.failedToDelete'))
  }
}

onMounted(() => {
  loadProducts()
  loadGroups()
})
</script>

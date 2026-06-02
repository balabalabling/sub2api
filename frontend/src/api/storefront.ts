import { apiClient } from './client'

export type StorefrontProductSource = 'store_product' | 'subscription_plan'

export interface StorefrontProduct {
  source: StorefrontProductSource
  id: number
  product_type: 'api_key' | 'account' | 'sms' | 'manual' | 'subscription_plan'
  name: string
  description: string
  price: number
  currency: string
  status?: string
  visibility?: string
  sort_order?: number
  stock_mode?: string
  stock_count?: number
  delivery_mode?: string
  delivery_config?: Record<string, any>
  plan_id?: number
  group_id?: number
  group_name?: string
  group_platform?: string
  validity_days?: number
  validity_unit?: string
  key_quota_usd?: number
}

export interface StoreOrderResult {
  store_order: {
    id: number
    order_no: string
    email: string
    delivery_status: string
  }
  payment: {
    order_id: number
    pay_amount: number
    status: string
    result_type?: string
    payment_type: string
    pay_url?: string
    qr_code?: string
    expires_at: string
  }
}

export interface StoreUsageItem {
  order_no?: string
  product_type?: string
  product_name?: string
  delivery_status?: string
  created_at?: string
  paid_at?: string
  delivered_at?: string
  api_key_id?: number
  api_key?: string
  api_key_masked?: string
  key_status?: string
  quota: number
  quota_used: number
  expires_at?: string
  last_used_at?: string
  balance: number
  input_tokens: number
  output_tokens: number
  total_cost: number
}

export const storefrontAPI = {
  listProducts() {
    return apiClient.get<StorefrontProduct[]>('/storefront/products')
  },

  createOrder(data: { email: string; product_id?: number; source?: StorefrontProductSource; plan_id?: number; query_token?: string; payment_type?: string; return_url?: string; is_mobile?: boolean }) {
    return apiClient.post<StoreOrderResult>('/storefront/orders', data)
  },

  sendQueryCode(email: string) {
    return apiClient.post('/storefront/query/send-code', { email })
  },

  verifyQueryCode(email: string, code: string) {
    return apiClient.post<{ query_token: string }>('/storefront/query/verify-code', { email, code })
  },

  queryByEmail(email: string, queryToken: string) {
    return apiClient.get<{ items: StoreUsageItem[] }>('/storefront/usage', {
      params: { email, query_token: queryToken }
    })
  },

  queryByKey(key: string) {
    return apiClient.get<{ items: StoreUsageItem[] }>('/storefront/usage', {
      params: { key }
    })
  }
}

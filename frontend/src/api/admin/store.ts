import { apiClient } from '../client'

export type StoreProductType = 'api_key' | 'account' | 'sms' | 'manual'
export type StoreProductStatus = 'draft' | 'active' | 'inactive'
export type StoreVisibility = 'public' | 'hidden'
export type StoreStockMode = 'unlimited' | 'tracked'
export type StoreDeliveryMode = 'auto' | 'manual'

export interface AdminStoreProduct {
  id: number
  product_type: StoreProductType
  name: string
  description: string
  price: number
  currency: string
  status: StoreProductStatus
  visibility: StoreVisibility
  sort_order: number
  stock_mode: StoreStockMode
  stock_count: number
  delivery_mode: StoreDeliveryMode
  delivery_config: Record<string, unknown>
  sale_start_at?: string | null
  sale_end_at?: string | null
  created_at?: string
  updated_at?: string
}

export interface AdminStoreProductInput {
  product_type: StoreProductType
  name: string
  description: string
  price: number
  currency: string
  status: StoreProductStatus
  visibility: StoreVisibility
  sort_order: number
  stock_mode: StoreStockMode
  stock_count: number
  delivery_mode: StoreDeliveryMode
  delivery_config: Record<string, unknown>
  sale_start_at?: string | null
  sale_end_at?: string | null
}

export const adminStoreAPI = {
  listProducts() {
    return apiClient.get<AdminStoreProduct[]>('/admin/store/products')
  },

  createProduct(data: AdminStoreProductInput) {
    return apiClient.post<AdminStoreProduct>('/admin/store/products', data)
  },

  updateProduct(id: number, data: AdminStoreProductInput) {
    return apiClient.put<AdminStoreProduct>(`/admin/store/products/${id}`, data)
  },

  deleteProduct(id: number) {
    return apiClient.delete(`/admin/store/products/${id}`)
  }
}

export default adminStoreAPI

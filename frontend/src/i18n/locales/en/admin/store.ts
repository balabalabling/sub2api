export default {
  store: {
    products: {
      title: 'Store Products',
      description: 'Manage API Key, account, SMS and manually delivered store products',
      createProduct: 'Create Product',
      editProduct: 'Edit Product',
      searchPlaceholder: 'Search product name or description...',
      allTypes: 'All Types',
      allStatuses: 'All Statuses',
      publicOnly: 'Public and active only',
      empty: 'No store products yet',
      publish: 'Publish',
      unpublish: 'Unpublish',
      noGroup: 'No group',
      deliveryConfigHint: 'Enter a JSON object, for example: template=default',
      deliveryConfigAdvancedHint: 'Common API Key delivery fields can be filled above; use this JSON area for advanced options.',
      invalidJson: 'Delivery config must be a valid JSON object',
      stockCountValue: 'Stock {count}',
      createSuccess: 'Product created successfully',
      updateSuccess: 'Product updated successfully',
      deleteSuccess: 'Product deleted successfully',
      saveFailed: 'Failed to save product',
      failedToLoad: 'Failed to load products',
      failedToDelete: 'Failed to delete product',
      deleteConfirm: 'Delete product "{name}"? This action cannot be undone.',
      fields: {
        productType: 'Product Type',
        name: 'Name',
        description: 'Description',
        price: 'Price',
        currency: 'Currency',
        status: 'Status',
        visibility: 'Visibility',
        sortOrder: 'Sort Order',
        stockMode: 'Stock Mode',
        stockCount: 'Stock Count',
        stock: 'Stock',
        deliveryMode: 'Delivery Mode',
        deliveryConfig: 'Delivery Config',
        groupId: 'API Key Group',
        quota: 'Key Quota',
        expiresInDays: 'Valid Days',
        rateLimit5h: '5h Limit',
        rateLimit1d: '1d Limit',
        rateLimit7d: '7d Limit',
        saleStartAt: 'Sale Starts',
        saleEndAt: 'Sale Ends',
        actions: 'Actions'
      },
      productTypes: {
        api_key: 'API Key',
        account: 'Account',
        sms: 'SMS / Verification',
        manual: 'Manual Delivery',
        subscription_plan: 'Subscription Plan'
      },
      statuses: {
        draft: 'Draft',
        active: 'Active',
        inactive: 'Inactive'
      },
      visibilities: {
        public: 'Public',
        hidden: 'Hidden'
      },
      stockModes: {
        unlimited: 'Unlimited',
        tracked: 'Tracked'
      },
      deliveryModes: {
        auto: 'Auto',
        manual: 'Manual'
      }
    }
  },
}

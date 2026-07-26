export default {
  store: {
    products: {
      title: '商城商品管理',
      description: '管理 API Key、账号、短信接码和人工交付等普通商城商品',
      createProduct: '创建商品',
      editProduct: '编辑商品',
      searchPlaceholder: '搜索商品名称或描述...',
      allTypes: '全部类型',
      allStatuses: '全部状态',
      publicOnly: '仅看公开上架',
      empty: '暂无商城商品',
      publish: '上架',
      unpublish: '下架',
      noGroup: '不绑定分组',
      deliveryConfigHint: '请输入 JSON 对象，例如：template=default',
      deliveryConfigAdvancedHint: 'API Key 的常用交付字段可在上方填写，也可以继续在这里补充高级 JSON 配置。',
      invalidJson: '交付配置必须是有效的 JSON 对象',
      stockCountValue: '库存 {count}',
      createSuccess: '商品创建成功',
      updateSuccess: '商品更新成功',
      deleteSuccess: '商品删除成功',
      saveFailed: '保存商品失败',
      failedToLoad: '加载商品失败',
      failedToDelete: '删除商品失败',
      deleteConfirm: '确定要删除商品「{name}」吗？此操作不可撤销。',
      fields: {
        productType: '商品类型',
        name: '商品名称',
        description: '商品描述',
        price: '价格',
        currency: '币种',
        status: '状态',
        visibility: '可见性',
        sortOrder: '排序',
        stockMode: '库存模式',
        stockCount: '库存数量',
        stock: '库存',
        deliveryMode: '交付模式',
        deliveryConfig: '交付配置',
        groupId: 'API Key 分组',
        quota: 'Key 额度',
        expiresInDays: '有效天数',
        rateLimit5h: '5 小时限额',
        rateLimit1d: '1 天限额',
        rateLimit7d: '7 天限额',
        saleStartAt: '开售时间',
        saleEndAt: '停售时间',
        actions: '操作'
      },
      productTypes: {
        api_key: 'API Key',
        account: '账号',
        sms: '短信/接码',
        manual: '手动交付',
        subscription_plan: '订阅套餐'
      },
      statuses: {
        draft: '草稿',
        active: '上架',
        inactive: '下架'
      },
      visibilities: {
        public: '公开',
        hidden: '隐藏'
      },
      stockModes: {
        unlimited: '不限库存',
        tracked: '跟踪库存'
      },
      deliveryModes: {
        auto: '自动',
        manual: '人工'
      }
    }
  },
}

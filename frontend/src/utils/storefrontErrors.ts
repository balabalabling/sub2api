const REASON_MESSAGES: Record<string, string> = {
  INVALID_EMAIL: '请输入有效的邮箱地址。',
  STORE_EMAIL_NOT_FOUND: '无法找到该邮箱任何有效数据。',
  STORE_QUERY_CODE_TOO_FREQUENT: '发送过于频繁，请稍后再试。',
  INVALID_CODE: '验证码无效或已过期，请重新输入。',
  INVALID_QUERY_TOKEN: '查询状态已失效，请重新验证邮箱。',
  OUT_OF_STOCK: '商品库存不足，请选择其他商品。',
  PRODUCT_NOT_FOUND: '商品不存在或已下架。',
  PLAN_NOT_AVAILABLE: '套餐不存在或暂不可购买。',
  GROUP_NOT_FOUND: '套餐对应分组暂不可用。',
  INVALID_INPUT: '提交的信息不完整，请检查后重试。',
  INVALID_RETURN_URL: '支付回跳地址无效，请刷新页面后重试。',
}

const MESSAGE_PATTERNS: Array<[RegExp, string]> = [
  [/invalid email/i, REASON_MESSAGES.INVALID_EMAIL],
  [/no valid data found for this email/i, REASON_MESSAGES.STORE_EMAIL_NOT_FOUND],
  [/invalid or expired code/i, REASON_MESSAGES.INVALID_CODE],
  [/invalid code/i, REASON_MESSAGES.INVALID_CODE],
  [/invalid query token/i, REASON_MESSAGES.INVALID_QUERY_TOKEN],
  [/please wait before requesting a new code/i, REASON_MESSAGES.STORE_QUERY_CODE_TOO_FREQUENT],
  [/product is out of stock/i, REASON_MESSAGES.OUT_OF_STOCK],
  [/product not found/i, REASON_MESSAGES.PRODUCT_NOT_FOUND],
  [/plan not found or not for sale/i, REASON_MESSAGES.PLAN_NOT_AVAILABLE],
  [/subscription group is no longer available/i, REASON_MESSAGES.GROUP_NOT_FOUND],
  [/return_url must target the canonical internal payment result page/i, REASON_MESSAGES.INVALID_RETURN_URL],
  [/return_url must/i, REASON_MESSAGES.INVALID_RETURN_URL],
]

export function storefrontErrorMessage(error: unknown, fallback: string): string {
  const err = error as { reason?: string; message?: string } | null | undefined
  const reason = typeof err?.reason === 'string' ? err.reason.trim() : ''
  if (reason && REASON_MESSAGES[reason]) {
    return REASON_MESSAGES[reason]
  }

  const message = typeof err?.message === 'string' ? err.message.trim() : ''
  if (message) {
    const matched = MESSAGE_PATTERNS.find(([pattern]) => pattern.test(message))
    if (matched) {
      return matched[1]
    }
  }

  return fallback
}

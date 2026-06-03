import { describe, expect, it } from 'vitest'
import { nextTick, reactive, ref } from 'vue'
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import { useDocumentTitle } from '../useDocumentTitle'

function createRoute(title: string): RouteLocationNormalizedLoaded {
  return reactive({
    fullPath: '/storefront',
    meta: { title },
  }) as RouteLocationNormalizedLoaded
}

describe('useDocumentTitle', () => {
  it('updates the browser tab title when site name is loaded after the route title', async () => {
    const route = createRoute('API Key Store')
    const siteName = ref('Sub2API')

    useDocumentTitle(route, siteName)
    await nextTick()

    expect(document.title).toBe('API Key Store - Sub2API')

    siteName.value = 'PinCC'
    await nextTick()

    expect(document.title).toBe('API Key Store - PinCC')
  })
})

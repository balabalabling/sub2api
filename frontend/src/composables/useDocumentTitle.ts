import type { RouteLocationNormalizedLoaded } from 'vue-router'
import { watch, type Ref } from 'vue'
import { resolveDocumentTitle } from '@/router/title'

export function updateDocumentTitle(route: RouteLocationNormalizedLoaded, siteName?: string): void {
  document.title = resolveDocumentTitle(route.meta.title, siteName, route.meta.titleKey as string)
}

export function useDocumentTitle(route: RouteLocationNormalizedLoaded, siteName: Ref<string>): void {
  watch(
    () => [route.fullPath, route.meta.title, route.meta.titleKey, siteName.value],
    () => updateDocumentTitle(route, siteName.value),
    { immediate: true }
  )
}

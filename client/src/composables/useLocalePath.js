import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import {
  DEFAULT_LOCALE,
  isLocale,
  localeFromRoute,
  localePath as buildLocalePath,
  splitLocalePath,
} from '../i18n/locales'

export function useLocalePath() {
  const route = useRoute()
  const router = useRouter()
  const { locale } = useI18n()

  const currentLocale = computed(() => localeFromRoute(route) || locale.value || DEFAULT_LOCALE)

  function localePath(path = '/') {
    return buildLocalePath(currentLocale.value, path)
  }

  function switchLocale(next) {
    if (!isLocale(next)) return
    const from = currentLocale.value
    if (next === from) return
    try {
      localStorage.setItem('aipoly_locale', next)
    } catch {
      /* ignore */
    }
    locale.value = next
    const { path, search } = splitLocalePath(route.fullPath)
    const target = buildLocalePath(next, path)
    router.push(search ? `${target}?${search}` : target)
  }

  return { currentLocale, localePath, switchLocale }
}

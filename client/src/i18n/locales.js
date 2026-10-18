export const SUPPORT_LOCALES = ['en', 'cn']
export const DEFAULT_LOCALE = 'en'

export const LOCALE_META = {
  en: { code: 'en', label: 'English', short: 'EN', flag: '🇺🇸' },
  cn: { code: 'cn', label: '中文', short: '中文', flag: '🇨🇳' },
}

export function isLocale(value) {
  return SUPPORT_LOCALES.includes(value)
}

export function detectLocale() {
  try {
    const saved = localStorage.getItem('aipoly_locale')
    if (isLocale(saved)) return saved
  } catch {
    /* ignore */
  }
  return DEFAULT_LOCALE
}

/** Strip leading locale from a path, return { locale, path } */
export function splitLocalePath(fullPath) {
  const [pathname, search = ''] = (fullPath || '/').split('?')
  const hashIdx = pathname.indexOf('#')
  const raw = hashIdx >= 0 ? pathname.slice(0, hashIdx) : pathname
  const parts = raw.split('/').filter(Boolean)
  if (parts.length && isLocale(parts[0])) {
    const locale = parts[0]
    const rest = '/' + parts.slice(1).join('/')
    const path = rest === '/' ? '/' : rest.replace(/\/$/, '') || '/'
    return { locale, path, search }
  }
  return { locale: null, path: raw === '/' ? '/' : raw.replace(/\/$/, '') || '/', search }
}

/**
 * Build a path for a locale.
 * Default locale (en) has no prefix: '/', '/subscribe', '/app'
 * Other locales keep prefix: '/cn', '/cn/subscribe'
 */
export function localePath(locale, path = '/') {
  const loc = isLocale(locale) ? locale : DEFAULT_LOCALE
  let p = path || '/'
  if (!p.startsWith('/')) p = '/' + p

  const parts = p.split('/').filter(Boolean)
  if (parts.length && isLocale(parts[0])) {
    p = '/' + parts.slice(1).join('/')
    if (p !== '/' && p.endsWith('/')) p = p.slice(0, -1)
    if (!p) p = '/'
  }

  if (loc === DEFAULT_LOCALE) {
    return p === '/' ? '/' : p
  }
  if (p === '/') return `/${loc}`
  return `/${loc}${p}`
}

/** Resolve active locale from a route-like object */
export function localeFromRoute(route) {
  if (route?.meta?.locale && isLocale(route.meta.locale)) {
    return route.meta.locale
  }
  const param = route?.params?.locale
  if (typeof param === 'string' && isLocale(param)) return param
  const { locale } = splitLocalePath(route?.path || '/')
  return locale || DEFAULT_LOCALE
}

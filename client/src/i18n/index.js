import { createI18n } from 'vue-i18n'
import { DEFAULT_LOCALE, detectLocale } from './locales'
import cn from './messages/cn'
import en from './messages/en'

const i18n = createI18n({
  legacy: false,
  locale: detectLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages: { cn, en },
})

export default i18n

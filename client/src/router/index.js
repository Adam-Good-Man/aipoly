import { createRouter, createWebHistory } from 'vue-router'
import { useWallet } from '../composables/useWallet'
import { detectLocale, isLocale, localePath, DEFAULT_LOCALE, localeFromRoute } from '../i18n/locales'
import i18n from '../i18n'
import PublicLayout from '../layouts/PublicLayout.vue'
import MainLayout from '../layouts/MainLayout.vue'
import Portal from '../views/Portal.vue'
import Subscribe from '../views/Subscribe.vue'
import Login from '../views/Login.vue'
import Dashboard from '../views/Dashboard.vue'
import Trades from '../views/Trades.vue'
import Events from '../views/Events.vue'
import EventDetail from '../views/EventDetail.vue'

function publicChildren() {
  return [
    { path: '', component: Portal },
    { path: 'subscribe', component: Subscribe },
  ]
}

function appChildren() {
  return [
    { path: '', component: Dashboard },
    { path: 'trades', component: Trades },
    { path: 'events', component: Events },
    { path: 'events/:id', component: EventDetail },
  ]
}

const routes = [
  // Default locale (en) — no language prefix
  {
    path: '/',
    component: PublicLayout,
    meta: { public: true, locale: DEFAULT_LOCALE },
    children: publicChildren(),
  },
  {
    path: '/login',
    component: Login,
    meta: { guest: true, locale: DEFAULT_LOCALE },
  },
  {
    path: '/app',
    component: MainLayout,
    meta: { requiresAuth: true, locale: DEFAULT_LOCALE },
    children: appChildren(),
  },

  // Explicit /en → strip prefix (en is default)
  {
    path: '/en',
    redirect: '/',
  },
  {
    path: '/en/:pathMatch(.*)*',
    redirect: (to) => {
      const rest = to.params.pathMatch
      const suffix = Array.isArray(rest) ? rest.join('/') : rest || ''
      return suffix ? `/${suffix}` : '/'
    },
  },

  // Chinese (and any future non-default) — prefixed
  {
    path: '/cn',
    component: PublicLayout,
    meta: { public: true, locale: 'cn' },
    children: publicChildren(),
  },
  {
    path: '/cn/login',
    component: Login,
    meta: { guest: true, locale: 'cn' },
  },
  {
    path: '/cn/app',
    component: MainLayout,
    meta: { requiresAuth: true, locale: 'cn' },
    children: appChildren(),
  },

  {
    path: '/:pathMatch(.*)*',
    redirect: (to) => {
      const segs = to.path.split('/').filter(Boolean)
      if (segs[0] && !isLocale(segs[0])) {
        return localePath(detectLocale(), to.fullPath)
      }
      return localePath(detectLocale(), '/')
    },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior() {
    return { top: 0 }
  },
})

router.beforeEach((to) => {
  const loc = localeFromRoute(to)
  i18n.global.locale.value = loc
  if (typeof document !== 'undefined') {
    document.documentElement.lang = loc === 'cn' ? 'zh-CN' : 'en'
  }
  try {
    localStorage.setItem('aipoly_locale', loc)
  } catch {
    /* ignore */
  }

  const { authenticated } = useWallet()

  if (to.meta.requiresAuth && !authenticated.value) {
    return {
      path: localePath(loc, '/login'),
      query: { redirect: to.fullPath },
    }
  }
  if (to.meta.guest && authenticated.value) {
    const redirect = to.query.redirect
    if (typeof redirect === 'string' && redirect.startsWith('/')) {
      return redirect
    }
    return { path: localePath(loc, '/app') }
  }
})

export default router

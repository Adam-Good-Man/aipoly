<template>
  <div
    class="public-shell min-h-screen flex flex-col"
    :class="isPortalHome ? 'text-stone-800 bg-[#080c0a]' : 'text-stone-800 bg-[#eef1ef]'"
  >
    <header
      class="z-30 transition-colors duration-300 backdrop-blur-md"
      :class="[
        headerClass,
        isPortalHome ? 'fixed top-0 left-0 right-0' : 'sticky top-0',
      ]"
      style="padding-top: env(safe-area-inset-top)"
    >
      <div class="max-w-6xl mx-auto px-4 sm:px-5 h-14 sm:h-16 flex items-center justify-between gap-2 sm:gap-4 min-w-0">
        <router-link :to="localePath('/')" class="flex items-baseline gap-2 group shrink-0 min-w-0">
          <span
            class="font-display text-xl sm:text-2xl tracking-tight transition-colors"
            :class="lightNav
              ? 'text-[#14201a] group-hover:text-[#1f6b45]'
              : 'text-[#e8f5e4] group-hover:text-[#c8e6c0]'"
          >
            {{ t('common.brand') }}
          </span>
        </router-link>
        <nav class="flex items-center gap-0.5 sm:gap-2 min-w-0 shrink">
          <router-link
            :to="localePath('/')"
            class="px-2 sm:px-3 py-1.5 text-sm transition-colors"
            :class="lightNav
              ? 'text-stone-500 hover:text-stone-900'
              : 'text-stone-400 hover:text-white'"
            :active-class="lightNav ? '!text-[#1f6b45] !font-medium' : '!text-[#c8e6c0] !font-medium'"
          >
            {{ t('common.home') }}
          </router-link>
          <router-link
            :to="localePath('/subscribe')"
            class="px-2 sm:px-3 py-1.5 text-sm transition-colors"
            :class="lightNav
              ? 'text-stone-500 hover:text-stone-900'
              : 'text-stone-400 hover:text-white'"
            :active-class="lightNav ? '!text-[#1f6b45] !font-medium' : '!text-[#c8e6c0] !font-medium'"
          >
            {{ t('common.subscribe') }}
          </router-link>
          <LocaleSwitcher class="ml-1 sm:inline-flex" :light="lightNav" />
          <PublicWalletBar class="ml-1 sm:ml-2" :light="lightNav" />
        </nav>
      </div>
    </header>

    <main class="flex-1 relative min-w-0">
      <router-view />
    </main>

    <footer
      class="border-t py-8 sm:py-10 text-center text-xs px-4"
      :class="isPortalHome
        ? 'border-white/10 bg-[#0a100e] text-stone-500'
        : 'border-stone-200 bg-white text-stone-500'"
      style="padding-bottom: max(2rem, env(safe-area-inset-bottom))"
    >
      <p
        class="font-display text-lg mb-2"
        :class="isPortalHome ? 'text-[#e8f5e4]' : 'text-stone-800'"
      >
        {{ t('common.brand') }}
      </p>
      <p class="leading-relaxed max-w-md mx-auto">
        {{ t('footer.risk') }}
      </p>
    </footer>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import PublicWalletBar from '../components/PublicWalletBar.vue'
import LocaleSwitcher from '../components/LocaleSwitcher.vue'
import { useLocalePath } from '../composables/useLocalePath'

const route = useRoute()
const { t } = useI18n()
const { localePath } = useLocalePath()

const isPortalHome = computed(() => {
  const p = route.path.replace(/^\/(cn|en)/, '') || '/'
  return p === '/' || p === ''
})

/** page band under header: 'dark' | 'light' */
const bandTone = ref('dark')

/**
 * 暗色内容区 → 暗色调 nav（深底浅字）
 * 亮色内容区 → 亮色调 nav（浅底深字）
 */
const lightNav = computed(() => {
  if (!isPortalHome.value) return true
  return bandTone.value === 'light'
})

const headerClass = computed(() => {
  if (lightNav.value) {
    return 'border-b border-stone-200/80 bg-[#eef1ef]/95 shadow-sm'
  }
  // Dark bands: solid dark bar so nav stays readable over hero/imagery
  return 'border-b border-white/10 bg-[#0a100e]/92 shadow-[0_8px_24px_rgba(0,0,0,0.35)]'
})

function syncNavTone() {
  if (!isPortalHome.value) {
    bandTone.value = 'light'
    return
  }
  const probeY = 40
  const nodes = document.querySelectorAll('[data-nav-tone]')
  let tone = 'dark'
  for (const node of nodes) {
    const rect = node.getBoundingClientRect()
    if (rect.top <= probeY && rect.bottom > probeY) {
      tone = node.getAttribute('data-nav-tone') === 'light' ? 'light' : 'dark'
      break
    }
  }
  bandTone.value = tone
}

watch(isPortalHome, () => {
  requestAnimationFrame(syncNavTone)
})

onMounted(() => {
  window.addEventListener('scroll', syncNavTone, { passive: true })
  window.addEventListener('resize', syncNavTone, { passive: true })
  requestAnimationFrame(syncNavTone)
})

onUnmounted(() => {
  window.removeEventListener('scroll', syncNavTone)
  window.removeEventListener('resize', syncNavTone)
})
</script>

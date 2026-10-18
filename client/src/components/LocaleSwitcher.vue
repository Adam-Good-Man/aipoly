<template>
  <el-dropdown trigger="click" @command="onSelect">
    <button
      type="button"
      class="lang-trigger inline-flex items-center gap-1.5 px-2 py-1.5 text-xs transition-colors outline-none"
      :class="light
        ? 'text-stone-600 hover:text-stone-900 border border-stone-300/80 bg-white/80 rounded-md'
        : 'text-stone-200 hover:text-white border border-white/20 bg-black/25 rounded-md'"
      :aria-label="current.label"
    >
      <svg class="w-3.5 h-3.5 opacity-80 shrink-0" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.6" />
        <path d="M3 12h18M12 3c2.5 2.8 3.8 5.8 3.8 9s-1.3 6.2-3.8 9c-2.5-2.8-3.8-5.8-3.8-9S9.5 5.8 12 3z" stroke="currentColor" stroke-width="1.6" />
      </svg>
      <span class="text-sm leading-none" aria-hidden="true">{{ current.flag }}</span>
      <span class="font-medium tracking-wide">{{ current.short }}</span>
      <svg class="w-3 h-3 opacity-60 shrink-0" viewBox="0 0 12 12" fill="none" aria-hidden="true">
        <path d="M3 4.5L6 7.5L9 4.5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    </button>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item
          v-for="opt in options"
          :key="opt.code"
          :command="opt.code"
          :class="{ 'is-active-locale': opt.code === currentLocale }"
        >
          <span class="inline-flex items-center gap-2 min-w-[7.5rem]">
            <span class="text-base leading-none">{{ opt.flag }}</span>
            <span>{{ opt.label }}</span>
            <span
              v-if="opt.code === currentLocale"
              class="ml-auto text-[#1f6b45] text-[10px]"
            >●</span>
          </span>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup>
import { computed } from 'vue'
import { LOCALE_META, SUPPORT_LOCALES } from '../i18n/locales'
import { useLocalePath } from '../composables/useLocalePath'

defineProps({
  light: { type: Boolean, default: false },
})

const { currentLocale, switchLocale } = useLocalePath()

const options = SUPPORT_LOCALES.map((code) => LOCALE_META[code])

const current = computed(() => LOCALE_META[currentLocale.value] || LOCALE_META.en)

function onSelect(code) {
  switchLocale(code)
}
</script>

<style scoped>
:deep(.is-active-locale) {
  background: rgba(31, 107, 69, 0.08);
}
</style>

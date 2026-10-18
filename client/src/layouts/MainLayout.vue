<template>
  <el-container class="min-h-screen">
    <el-aside width="220px" class="bg-slate-900 text-white flex flex-col">
      <div class="p-5 border-b border-slate-700">
        <router-link :to="localePath('/')" class="block">
          <h1 class="text-xl font-bold text-emerald-400">{{ t('common.brand') }}</h1>
        </router-link>
        <p class="text-xs text-slate-400 mt-1">{{ t('app.backend') }}</p>
      </div>
      <el-menu
        :default-active="route.path"
        router
        background-color="#0f172a"
        text-color="#94a3b8"
        active-text-color="#34d399"
        class="border-none flex-1"
      >
        <el-menu-item :index="localePath('/app')">
          <el-icon><DataAnalysis /></el-icon>
          <span>{{ t('app.dashboard') }}</span>
        </el-menu-item>
        <el-menu-item :index="localePath('/app/trades')">
          <el-icon><TrendCharts /></el-icon>
          <span>{{ t('app.trades') }}</span>
        </el-menu-item>
        <el-menu-item :index="localePath('/app/events')">
          <el-icon><Bell /></el-icon>
          <span>{{ t('app.events') }}</span>
        </el-menu-item>
      </el-menu>

      <div class="p-4 border-t border-slate-700 text-xs text-slate-500">
        <div class="text-emerald-400 mb-1">{{ t('app.loggedIn') }} · {{ walletLabel }}</div>
        <code class="text-slate-400 break-all">{{ shortAddress }}</code>
        <router-link
          :to="localePath('/subscribe')"
          class="block mt-3 text-slate-400 hover:text-emerald-400 transition-colors"
        >
          {{ t('app.upgrade') }}
        </router-link>
      </div>
    </el-aside>

    <el-container>
      <el-header class="bg-white shadow-sm flex items-center justify-between px-6 gap-4">
        <div class="flex items-center gap-3 min-w-0">
          <span class="text-lg font-medium text-slate-700 shrink-0">{{ pageTitle }}</span>
          <el-tag v-if="matchBot" type="success" size="small" effect="plain">与交易机器人一致</el-tag>
          <el-tag v-else-if="matchBot === false" type="warning" size="small" effect="plain">与 .env 钱包不一致</el-tag>
        </div>
        <div class="flex items-center gap-2 sm:gap-3 shrink-0">
          <el-tag size="small" effect="plain" type="info" class="!border-slate-300 !text-slate-600">
            {{ planLabel }}
          </el-tag>
          <LocaleSwitcher light />
          <el-button size="small" @click="handleLogout">{{ t('common.logout') }}</el-button>
          <el-button type="primary" plain :loading="triggering" @click="handleTrigger">
            <el-icon class="mr-1"><Refresh /></el-icon>
            {{ t('app.triggerAi') }}
          </el-button>
        </div>
      </el-header>
      <el-main class="p-6">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed, ref, watch, provide } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { triggerTrading, getAccountProfile } from '../api'
import { useWallet } from '../composables/useWallet'
import { useLocalePath } from '../composables/useLocalePath'
import LocaleSwitcher from '../components/LocaleSwitcher.vue'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const { localePath } = useLocalePath()
const triggering = ref(false)
const matchBot = ref(null)
const profile = ref(null)

const { address, shortAddress, walletLabel, disconnect } = useWallet()

provide('walletAddress', address)
provide('accountProfile', profile)

const planLabel = computed(() => {
  const p = localStorage.getItem('aipoly_plan') || 'Free'
  return `Plan · ${p}`
})

const pageTitle = computed(() => {
  const base = route.path.replace(/^\/(cn|en)/, '') || '/'
  if (base.startsWith('/app/events/')) return t('app.events')
  const map = {
    '/app': t('app.dashboard'),
    '/app/trades': t('app.trades'),
    '/app/events': t('app.events'),
  }
  return map[base] || t('common.brand')
})

async function refreshProfile() {
  if (!address.value) return
  try {
    const { data } = await getAccountProfile(address.value)
    profile.value = data.profile
    matchBot.value = data.matches_bot ?? null
  } catch {
    matchBot.value = null
  }
}

async function handleLogout() {
  disconnect()
  router.replace(localePath('/login'))
}

async function handleTrigger() {
  triggering.value = true
  try {
    await triggerTrading()
    ElMessage.success('OK')
  } catch {
    ElMessage.error('Failed')
  } finally {
    triggering.value = false
  }
}

watch(address, (addr) => {
  if (addr) refreshProfile()
}, { immediate: true })
</script>

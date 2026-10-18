<template>
  <div class="flex items-center gap-1.5 sm:gap-2 shrink-0">
    <template v-if="authenticated">
      <span class="hidden md:inline text-xs font-mono text-stone-500">{{ shortAddress }}</span>
      <el-button
        size="small"
        :class="light
          ? '!bg-[#14201a] !border-[#14201a] !text-white'
          : '!bg-[#1a2e22] !border-[#2d4a38] !text-[#c8e6c0]'"
        @click="goApp"
      >
        <span class="sm:hidden">{{ t('common.enterAppShort') }}</span>
        <span class="hidden sm:inline">{{ t('common.enterApp') }}</span>
      </el-button>
    </template>
    <template v-else-if="connected && !signed">
      <el-button size="small" :loading="signing" class="!bg-[#1f6b45] !border-[#1f6b45] !text-white" @click="handleSign">
        {{ t('common.signIn') }}
      </el-button>
    </template>
    <template v-else>
      <el-dropdown trigger="click" @command="onConnect">
        <el-button
          size="small"
          :class="light
            ? '!bg-[#1f6b45] !border-[#1f6b45] !text-white !font-medium'
            : '!bg-[#c8e6c0] !border-[#c8e6c0] !text-[#0c1210] !font-medium'"
        >
          {{ t('common.connectWallet') }}
        </el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item v-if="hasInjectedWallet" command="injected" :disabled="connecting">
              {{ t('common.injectedWallet') }}
            </el-dropdown-item>
            <el-dropdown-item command="wc" :disabled="connecting">
              {{ t('common.walletConnect') }}
            </el-dropdown-item>
            <el-dropdown-item divided command="login">
              {{ t('common.fullLogin') }}
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </template>
  </div>
</template>

<script setup>
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { useWallet } from '../composables/useWallet'
import { useLocalePath } from '../composables/useLocalePath'

defineProps({
  light: { type: Boolean, default: false },
})

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const { localePath } = useLocalePath()

const {
  shortAddress,
  connected,
  signed,
  authenticated,
  connecting,
  signing,
  hasInjectedWallet,
  connectInjected,
  connectWalletConnect,
  signIn,
} = useWallet()

function goApp() {
  router.push(localePath('/app'))
}

function goLogin() {
  router.push({
    path: localePath('/login'),
    query: { redirect: localePath('/app') },
  })
}

async function onConnect(cmd) {
  if (cmd === 'login') {
    goLogin()
    return
  }
  try {
    if (cmd === 'injected') {
      await connectInjected()
    } else {
      await connectWalletConnect()
    }
    ElMessage.success(t('common.signIn'))
  } catch (e) {
    ElMessage.error(e.message || 'Error')
  }
}

async function handleSign() {
  try {
    await signIn()
    const redirect = route.query.redirect
    if (typeof redirect === 'string' && redirect.startsWith('/')) {
      router.push(redirect)
    } else {
      router.push(localePath('/app'))
    }
  } catch (e) {
    ElMessage.error(e.message || 'Error')
  }
}
</script>

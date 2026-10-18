<template>
  <div class="min-h-screen bg-gradient-to-br from-slate-900 via-slate-800 to-emerald-950 flex flex-col">
    <header class="px-6 py-5 flex items-center justify-between gap-3">
      <div class="flex items-center gap-3">
        <router-link :to="localePath('/')" class="text-2xl font-bold text-emerald-400 font-display">aiPoly</router-link>
        <span class="text-slate-400 text-sm">{{ t('login.tagline') }}</span>
      </div>
      <div class="flex items-center gap-3">
        <LocaleSwitcher />
        <router-link :to="localePath('/subscribe')" class="text-sm text-slate-400 hover:text-emerald-400 transition-colors">
          {{ t('common.subscribe') }}
        </router-link>
      </div>
    </header>

    <main class="flex-1 flex items-center justify-center px-4 pb-12">
      <div class="w-full max-w-md bg-white rounded-2xl shadow-xl p-8">
        <WalletLogin
          :connected="connected"
          :signed="signed"
          :connecting="connecting"
          :signing="signing"
          :login-step="loginStep"
          :has-injected-wallet="hasInjectedWallet"
          :address="address"
          :short-address="shortAddress"
          :wallet-error="walletError"
          :bot-signer="botSigner"
          @connect-injected="handleConnectInjected"
          @connect-wc="handleConnectWC"
          @sign="handleSign"
          @switch-account="handleSwitchAccount"
          @disconnect="disconnect"
        />
      </div>
    </main>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getWallet } from '../api'
import { useWallet } from '../composables/useWallet'
import { useLocalePath } from '../composables/useLocalePath'
import WalletLogin from '../components/WalletLogin.vue'
import LocaleSwitcher from '../components/LocaleSwitcher.vue'

const router = useRouter()
const { t } = useI18n()
const { localePath } = useLocalePath()
const botSigner = ref('')

const {
  address,
  shortAddress,
  connected,
  signed,
  authenticated,
  connecting,
  signing,
  loginStep,
  error: walletError,
  hasInjectedWallet,
  connectionMode,
  connectInjected,
  connectWalletConnect,
  switchInjectedAccount,
  signIn,
  disconnect,
} = useWallet()

async function fetchBotSigner() {
  try {
    const { data } = await getWallet()
    botSigner.value = data.signer_address || ''
  } catch {
    botSigner.value = ''
  }
}

function goDashboard() {
  const redirect = router.currentRoute.value.query.redirect
  if (typeof redirect === 'string' && redirect.startsWith('/')) {
    router.replace(redirect)
    return
  }
  router.replace(localePath('/app'))
}

async function handleConnectInjected() {
  try {
    const addr = await connectInjected()
    ElMessage.success(`已选择 ${addr.slice(0, 6)}...${addr.slice(-4)}，请签名`)
  } catch (e) {
    ElMessage.error(e.message || '连接失败')
  }
}

async function handleConnectWC() {
  try {
    const addr = await connectWalletConnect()
    ElMessage.success(`已连接 ${addr.slice(0, 6)}...${addr.slice(-4)}，请签名`)
  } catch (e) {
    ElMessage.error(e.message || '连接失败')
  }
}

async function handleSwitchAccount() {
  try {
    if (connectionMode.value === 'walletconnect') {
      await disconnect()
      const addr = await connectWalletConnect()
      ElMessage.success(`已切换至 ${addr.slice(0, 6)}...${addr.slice(-4)}`)
      return
    }
    const addr = await switchInjectedAccount()
    ElMessage.success(`已切换至 ${addr.slice(0, 6)}...${addr.slice(-4)}`)
  } catch (e) {
    ElMessage.error(e.message || '切换失败')
  }
}

async function handleSign() {
  try {
    await signIn()
    ElMessage.success('登录成功')
    goDashboard()
  } catch (e) {
    ElMessage.error(e.message || '签名失败')
  }
}

onMounted(async () => {
  await fetchBotSigner()
  if (authenticated.value) goDashboard()
})
</script>

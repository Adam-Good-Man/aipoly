import { ref, computed, onMounted, onUnmounted } from 'vue'
import EthereumProvider from '@walletconnect/ethereum-provider'

const POLYGON_CHAIN = 137
const POLYGON_HEX = '0x89'
const SIG_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000 // 7 days

function loadPersistedSession() {
  const addr = (localStorage.getItem('aipoly_wallet') || '').toLowerCase()
  const mode = localStorage.getItem('aipoly_wallet_mode') || ''
  if (!addr || !loadStoredSignature(addr)) {
    return { address: '', mode: '' }
  }
  return { address: addr, mode }
}

const persisted = loadPersistedSession()
const address = ref(persisted.address)
const connectionMode = ref(persisted.mode)
const isSigned = ref(!!loadStoredSignature(persisted.address))
const connecting = ref(false)
const signing = ref(false)
const loginStep = ref(persisted.address ? 'done' : 'idle') // idle | connect | sign | done
const error = ref('')

let wcProvider = null
let injectedHandlers = null

function getInjected() {
  if (typeof window === 'undefined') return null
  const eth = window.ethereum
  if (!eth) return null
  if (eth.providers?.length) {
    return eth.providers.find((p) => p.isMetaMask) || eth.providers[0]
  }
  return eth
}

function buildSignMessage(addr) {
  const ts = Date.now()
  return {
    message: [
      'Welcome to aiPoly!',
      '',
      'Sign this message to verify wallet ownership.',
      'This will not trigger a blockchain transaction or cost gas.',
      '',
      `Wallet: ${addr}`,
      `Timestamp: ${ts}`,
    ].join('\n'),
    timestamp: ts,
  }
}

function loadStoredSignature(addr) {
  const storedAddr = localStorage.getItem('aipoly_wallet_sig_addr') || ''
  const sig = localStorage.getItem('aipoly_wallet_sig') || ''
  const message = localStorage.getItem('aipoly_wallet_sig_message') || ''
  const ts = Number(localStorage.getItem('aipoly_wallet_sig_ts') || 0)
  if (!sig || !message || !storedAddr) return null
  if (storedAddr.toLowerCase() !== addr.toLowerCase()) return null
  if (ts && Date.now() - ts > SIG_MAX_AGE_MS) return null
  return { signature: sig, message, timestamp: ts }
}

function saveSignature(addr, message, signature, timestamp) {
  localStorage.setItem('aipoly_wallet_sig_addr', addr.toLowerCase())
  localStorage.setItem('aipoly_wallet_sig', signature)
  localStorage.setItem('aipoly_wallet_sig_message', message)
  localStorage.setItem('aipoly_wallet_sig_ts', String(timestamp))
}

function clearSignature() {
  localStorage.removeItem('aipoly_wallet_sig_addr')
  localStorage.removeItem('aipoly_wallet_sig')
  localStorage.removeItem('aipoly_wallet_sig_message')
  localStorage.removeItem('aipoly_wallet_sig_ts')
}

async function ensurePolygon(provider) {
  try {
    await provider.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: POLYGON_HEX }],
    })
  } catch (e) {
    if (e?.code === 4902) {
      await provider.request({
        method: 'wallet_addEthereumChain',
        params: [{
          chainId: POLYGON_HEX,
          chainName: 'Polygon Mainnet',
          nativeCurrency: { name: 'MATIC', symbol: 'MATIC', decimals: 18 },
          rpcUrls: ['https://polygon-rpc.com'],
          blockExplorerUrls: ['https://polygonscan.com'],
        }],
      })
    } else {
      throw e
    }
  }
}

/** 强制弹出 MetaMask / Rabby 账户选择器 */
async function requestAccountSelection(provider) {
  try {
    await provider.request({
      method: 'wallet_requestPermissions',
      params: [{ eth_accounts: {} }],
    })
  } catch (e) {
    if (e?.code === 4001 || e?.code === 'ACTION_REJECTED') {
      throw new Error('已取消选择账户')
    }
    // 部分钱包不支持，继续用 eth_requestAccounts
  }

  let accounts = await provider.request({ method: 'eth_accounts' })
  if (!accounts?.length) {
    accounts = await provider.request({ method: 'eth_requestAccounts' })
  }
  if (!accounts?.length) {
    throw new Error('未获取到钱包地址')
  }
  return accounts
}

async function requestSignature(provider, addr) {
  const { message, timestamp } = buildSignMessage(addr)
  const signature = await provider.request({
    method: 'personal_sign',
    params: [message, addr],
  })
  if (!signature) {
    throw new Error('签名被拒绝')
  }
  saveSignature(addr, message, signature, timestamp)
  return signature
}

function bindInjectedListeners(provider) {
  unbindInjectedListeners()
  const onAccounts = (accounts) => {
    const next = accounts?.[0]?.toLowerCase() || ''
    if (!next) {
      clearSession()
      return
    }
    if (next !== address.value) {
      clearSignature()
      isSigned.value = false
      setAddress(next, 'injected', false)
      loginStep.value = 'sign'
    }
  }
  const onChain = () => {}
  provider.on('accountsChanged', onAccounts)
  provider.on('chainChanged', onChain)
  injectedHandlers = { provider, onAccounts, onChain }
}

function unbindInjectedListeners() {
  if (!injectedHandlers) return
  const { provider, onAccounts, onChain } = injectedHandlers
  provider.removeListener?.('accountsChanged', onAccounts)
  provider.removeListener?.('chainChanged', onChain)
  injectedHandlers = null
}

function setAddress(addr, mode, persist = false) {
  address.value = addr.toLowerCase()
  connectionMode.value = mode || ''
  if (persist && addr) {
    localStorage.setItem('aipoly_wallet', address.value)
    if (mode) localStorage.setItem('aipoly_wallet_mode', mode)
  }
}

function clearSession() {
  address.value = ''
  connectionMode.value = ''
  loginStep.value = 'idle'
  isSigned.value = false
  localStorage.removeItem('aipoly_wallet')
  localStorage.removeItem('aipoly_wallet_mode')
  clearSignature()
}

function getActiveProvider() {
  if (connectionMode.value === 'walletconnect' && wcProvider) return wcProvider
  return getInjected()
}

async function initWalletConnect() {
  const projectId = import.meta.env.VITE_WALLETCONNECT_PROJECT_ID
  if (!projectId) {
    throw new Error('请在 client/.env 配置 VITE_WALLETCONNECT_PROJECT_ID')
  }
  if (wcProvider) {
    try {
      await wcProvider.disconnect()
    } catch {
      // ignore
    }
    wcProvider = null
  }
  wcProvider = await EthereumProvider.init({
    projectId,
    chains: [POLYGON_CHAIN],
    optionalChains: [POLYGON_CHAIN],
    showQrModal: true,
    metadata: {
      name: 'aiPoly',
      description: 'Polymarket 自动化交易',
      url: window.location.origin,
      icons: [`${window.location.origin}/vite.svg`],
    },
  })
  wcProvider.on('accountsChanged', (accounts) => {
    const next = accounts?.[0]?.toLowerCase() || ''
    if (!next) clearSession()
    else if (next !== address.value) {
      clearSignature()
      isSigned.value = false
      setAddress(next, 'walletconnect', false)
      loginStep.value = 'sign'
    }
  })
  wcProvider.on('disconnect', () => {
    if (connectionMode.value === 'walletconnect') clearSession()
  })
  return wcProvider
}

export function useWallet() {
  const signed = computed(() => isSigned.value && !!address.value)

  const authenticated = computed(() => signed.value)
  const connected = computed(() => !!address.value)
  const hasInjectedWallet = computed(() => !!getInjected())

  const shortAddress = computed(() => {
    if (!address.value) return ''
    return `${address.value.slice(0, 6)}...${address.value.slice(-4)}`
  })

  const walletLabel = computed(() => {
    if (connectionMode.value === 'injected') return '浏览器插件'
    if (connectionMode.value === 'walletconnect') return 'WalletConnect'
    return ''
  })

  /** Step 1: 弹出账户选择器，仅连接不签名 */
  async function connectAccounts(mode, { forcePicker = true } = {}) {
    if (mode === 'injected') {
      const injected = getInjected()
      if (!injected) {
        error.value = '未检测到浏览器钱包插件，请安装 MetaMask 或使用 WalletConnect 扫码'
        throw new Error(error.value)
      }
      connecting.value = true
      loginStep.value = 'connect'
      error.value = ''
      try {
        clearSignature()
        isSigned.value = false
        const accounts = forcePicker
          ? await requestAccountSelection(injected)
          : await injected.request({ method: 'eth_requestAccounts' })
        await ensurePolygon(injected)
        bindInjectedListeners(injected)
        setAddress(accounts[0], 'injected', false)
        loginStep.value = 'sign'
        return accounts[0]
      } finally {
        connecting.value = false
      }
    }

    connecting.value = true
    loginStep.value = 'connect'
    error.value = ''
    try {
      clearSignature()
      isSigned.value = false
      const provider = await initWalletConnect()
      await provider.enable()
      const accounts = provider.accounts || []
      if (!accounts.length) throw new Error('未获取到钱包地址')
      setAddress(accounts[0], 'walletconnect', false)
      loginStep.value = 'sign'
      return accounts[0]
    } finally {
      connecting.value = false
    }
  }

  /** 重新选择浏览器插件内的其他账户 */
  async function switchInjectedAccount() {
    const injected = getInjected()
    if (!injected) throw new Error('未检测到浏览器钱包')
    connecting.value = true
    error.value = ''
    try {
      clearSignature()
      isSigned.value = false
      const accounts = await requestAccountSelection(injected)
      await ensurePolygon(injected)
      bindInjectedListeners(injected)
      setAddress(accounts[0], 'injected', false)
      loginStep.value = 'sign'
      return accounts[0]
    } finally {
      connecting.value = false
    }
  }

  /** Step 2: 签名验证 */
  async function signIn() {
    if (!address.value) throw new Error('请先连接钱包')
    const provider = getActiveProvider()
    if (!provider) throw new Error('钱包 provider 不可用')

    signing.value = true
    loginStep.value = 'sign'
    error.value = ''
    try {
      await requestSignature(provider, address.value)
      setAddress(address.value, connectionMode.value, true)
      isSigned.value = true
      loginStep.value = 'done'
    } catch (e) {
      if (e?.code === 4001 || e?.code === 'ACTION_REJECTED') {
        throw new Error('签名已取消，需完成签名才能登录')
      }
      throw e
    } finally {
      signing.value = false
    }
  }

  async function connectInjected() {
    return connectAccounts('injected')
  }

  async function connectWalletConnect() {
    return connectAccounts('walletconnect')
  }

  async function disconnect() {
    if (connectionMode.value === 'walletconnect' && wcProvider) {
      try {
        await wcProvider.disconnect()
      } catch {
        // ignore
      }
      wcProvider = null
    }
    unbindInjectedListeners()
    clearSession()
  }

  async function tryRestoreSession() {
    if (!address.value || !loadStoredSignature(address.value)) {
      clearSession()
      return
    }
    const mode = connectionMode.value
    if (mode === 'injected') {
      const injected = getInjected()
      if (!injected) {
        clearSession()
        return
      }
      try {
        const accounts = await injected.request({ method: 'eth_accounts' })
        if (!accounts?.length || accounts[0].toLowerCase() !== address.value) {
          clearSession()
          return
        }
        bindInjectedListeners(injected)
        isSigned.value = true
        loginStep.value = 'done'
      } catch {
        clearSession()
      }
    } else if (mode === 'walletconnect') {
      isSigned.value = true
      loginStep.value = 'done'
    }
  }

  onMounted(() => {
    tryRestoreSession()
  })

  onUnmounted(() => {
    unbindInjectedListeners()
  })

  return {
    address,
    shortAddress,
    connected,
    signed,
    authenticated,
    connecting,
    signing,
    loginStep,
    error,
    hasInjectedWallet,
    walletLabel,
    connectionMode,
    connectAccounts,
    switchInjectedAccount,
    signIn,
    connectInjected,
    connectWalletConnect,
    disconnect,
  }
}

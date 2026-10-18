<template>
  <div class="flex flex-col items-center text-center">
    <el-icon :size="40" class="text-emerald-500 mb-3"><Wallet /></el-icon>
    <h2 class="text-lg font-semibold text-slate-700 mb-1">登录 Polymarket 钱包</h2>
    <p class="text-slate-500 mb-5 text-sm">
      连接钱包 → 签名验证 → 进入后台查看余额与交易
    </p>

    <div class="w-full mb-6 text-left bg-slate-50 rounded-xl p-4 border border-slate-100">
      <div class="flex items-center gap-3 mb-3">
        <span class="w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold shrink-0" :class="stepClass(1)">1</span>
        <div>
          <div class="text-sm font-medium text-slate-700">选择账户并连接</div>
          <div class="text-xs text-slate-400">会弹出 MetaMask / Rabby，请手动选择要用的账户</div>
        </div>
        <el-icon v-if="stepOneDone" class="ml-auto text-emerald-500"><CircleCheck /></el-icon>
      </div>
      <div class="flex items-center gap-3">
        <span class="w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold shrink-0" :class="stepClass(2)">2</span>
        <div>
          <div class="text-sm font-medium text-slate-700">签名验证</div>
          <div class="text-xs text-slate-400">确认签名请求，证明钱包归属</div>
        </div>
        <el-icon v-if="signed" class="ml-auto text-emerald-500"><CircleCheck /></el-icon>
      </div>
    </div>

    <el-alert v-if="botSigner" type="info" :closable="false" class="w-full mb-4 text-left">
      <template #title>Polymarket 交易账户（推荐连接此地址）</template>
      <p class="text-xs mt-1">
        <code class="break-all">{{ botSigner }}</code>
      </p>
    </el-alert>

    <!-- 已连接，待选账户确认 / 签名 -->
    <template v-if="connected && !signed">
      <div class="w-full mb-4 p-4 bg-white border border-slate-200 rounded-xl text-left">
        <div class="text-xs text-slate-500 mb-1">当前选中账户</div>
        <code class="text-sm break-all text-slate-800">{{ address }}</code>
        <p v-if="botSigner && address?.toLowerCase() !== botSigner.toLowerCase()" class="text-xs text-amber-600 mt-2">
          此地址与服务器交易钱包不一致，可能看不到 Polymarket 余额。
        </p>
      </div>

      <div class="flex flex-col gap-2 w-full">
        <el-button type="primary" size="large" :loading="signing" @click="$emit('sign')">
          第 2 步：签名验证
        </el-button>
        <el-button size="large" :loading="connecting" @click="$emit('switch-account')">
          选择其他账户
        </el-button>
        <el-button size="large" plain @click="$emit('disconnect')">取消</el-button>
      </div>
      <p class="text-xs text-slate-400 mt-3">
        点「选择其他账户」会再次弹出钱包，可切换到 {{ botSigner ? '服务器钱包' : '其他地址' }}。
      </p>
    </template>

    <!-- 初始：选择连接方式 -->
    <template v-else>
      <div class="flex flex-col sm:flex-row gap-3 w-full justify-center">
        <el-button
          v-if="hasInjectedWallet"
          type="primary"
          size="large"
          :loading="connecting"
          @click="$emit('connect-injected')"
        >
          浏览器钱包登录
        </el-button>
        <el-button
          :type="hasInjectedWallet ? 'default' : 'primary'"
          size="large"
          :loading="connecting"
          @click="$emit('connect-wc')"
        >
          WalletConnect 扫码
        </el-button>
      </div>
      <p v-if="hasInjectedWallet" class="text-slate-400 text-xs mt-3">
        点击后会弹出 MetaMask / Rabby，请在弹窗中选择账户（不会自动使用当前账户）
      </p>
      <p v-else class="text-slate-400 text-xs mt-3">
        未检测到插件？可安装 MetaMask 或使用扫码登录
      </p>
    </template>

    <p v-if="walletError" class="text-red-500 text-sm mt-4">{{ walletError }}</p>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  connected: Boolean,
  signed: Boolean,
  connecting: Boolean,
  signing: Boolean,
  loginStep: String,
  hasInjectedWallet: Boolean,
  address: String,
  shortAddress: String,
  walletError: String,
  botSigner: String,
})

defineEmits(['connect-injected', 'connect-wc', 'sign', 'switch-account', 'disconnect'])

const stepOneDone = computed(() =>
  props.signed || props.connected || ['sign', 'done'].includes(props.loginStep)
)

function stepClass(n) {
  if (n === 1) {
    if (props.connected) return 'bg-emerald-500 text-white'
    if (props.loginStep === 'connect') return 'bg-blue-500 text-white'
    return 'bg-slate-200 text-slate-600'
  }
  if (props.signed) return 'bg-emerald-500 text-white'
  if (props.loginStep === 'sign') return 'bg-blue-500 text-white'
  return 'bg-slate-200 text-slate-600'
}
</script>

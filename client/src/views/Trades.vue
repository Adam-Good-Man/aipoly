<template>
  <div>
    <div class="flex gap-3 mb-4 items-center justify-between">
      <div class="flex items-center gap-2">
        <el-radio-group v-model="tab" size="small">
          <el-radio-button value="ai">AI 自动下注</el-radio-button>
          <el-radio-button value="chain">Polymarket 链上</el-radio-button>
        </el-radio-group>
        <span class="text-xs text-slate-500">
          {{ tab === 'ai' ? `本地 AI 决策记录（共 ${aiTotal} 条）` : `链上成交（共 ${chainTotal} 条，按每笔买卖计）` }}
        </span>
      </div>
      <el-button size="small" :loading="aiLoading || chainLoading" @click="refresh">刷新</el-button>
    </div>

    <el-alert v-if="aiFetchError && tab === 'ai'" type="error" :closable="false" class="mb-4" :title="aiFetchError" />
    <el-alert v-if="chainFetchError && tab === 'chain'" type="error" :closable="false" class="mb-4" :title="chainFetchError" />
    <el-alert
      v-if="tab === 'chain' && !walletAddress && !chainFetchError"
      type="info"
      :closable="false"
      class="mb-4"
      title="链上记录需要已连接的钱包地址；AI 自动下注记录在「AI 自动下注」标签页。"
    />
    <el-alert
      v-if="tab === 'ai' && !aiLoading && aiTotal === 0 && !aiFetchError"
      type="info"
      :closable="false"
      class="mb-4"
      title="AI 自动下注来自本地数据库（机器人每次决策写入）；Polymarket 链上 tab 是钱包在 Polygon 上的成交，两者数据源不同。"
    />

    <!-- AI bets from local DB -->
    <div v-show="tab === 'ai'" class="bg-white rounded-xl shadow-sm border border-slate-100" v-loading="aiLoading">
      <el-table
        v-if="aiTotal > 0 || aiLoading"
        :data="aiTrades"
        stripe
        size="small"
        empty-text="暂无 AI 下注记录"
        :row-key="aiTradeRowKey"
        :expand-row-keys="aiExpandedRows"
        @expand-change="onAiExpandChange"
      >
        <el-table-column type="expand" width="42">
          <template #default="{ row }">
            <div class="px-2 py-3">
              <div class="rounded-lg border border-slate-100 bg-slate-50/80 p-4">
                <div class="flex flex-wrap items-center gap-2 mb-3 text-xs text-slate-500">
                  <span>下注 <strong class="text-slate-700">${{ formatMoney(row.amount) }}</strong></span>
                  <span>· {{ formatTime(row.created_at) }}</span>
                  <span v-if="row.price">· @ {{ (row.price * 100).toFixed(1) }}¢</span>
                  <el-tag size="small" type="success" effect="plain">AI</el-tag>
                  <el-tag size="small" :type="statusType(row.status, row)" effect="plain">{{ statusLabel(row.status, row) }}</el-tag>
                  <span v-if="row.self_win_rate">· 预估胜率 {{ row.self_win_rate.toFixed(0) }}%</span>
                </div>
                <TradeReasonCell :row="aiRowForCell(row)" />
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="market_title" label="市场" min-width="168" show-overflow-tooltip />
        <el-table-column prop="outcome" label="方向" width="64" />
        <el-table-column label="金额" width="72">
          <template #default="{ row }">${{ formatMoney(row.amount) }}</template>
        </el-table-column>
        <el-table-column label="结果" width="72">
          <template #default="{ row }">
            <el-tag size="small" :type="tradeResultType(row)" effect="plain">
              {{ tradeResultLabel(row) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="盈亏" width="88">
          <template #default="{ row }">
            <span v-if="row.status === 'won' || row.status === 'lost'" :class="row.pnl >= 0 ? 'text-emerald-600' : 'text-red-500'">
              {{ row.pnl >= 0 ? '+' : '' }}${{ formatMoney(row.pnl) }}
            </span>
            <span v-else class="text-slate-400">—</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag size="small" :type="statusType(row.status, row)">{{ statusLabel(row.status, row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="128" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="text-xs text-slate-600">{{ formatTime(row.created_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="理由摘要" min-width="140">
          <template #default="{ row }">
            <div class="cursor-pointer group" @click="toggleAiExpand(row)">
              <TradeReasonCell v-if="hasAiReason(row)" :row="aiRowForCell(row)" compact />
              <span v-else class="text-xs text-slate-300">—</span>
              <span v-if="hasAiReason(row)" class="text-emerald-600 text-xs opacity-0 group-hover:opacity-100 transition-opacity">展开 ↓</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="胜率" width="64" align="right">
          <template #default="{ row }">
            <span class="text-xs text-slate-600">{{ row.self_win_rate ? row.self_win_rate.toFixed(0) + '%' : '—' }}</span>
          </template>
        </el-table-column>
      </el-table>
      <div v-else class="p-8 text-center text-slate-400 text-sm">暂无 AI 下注记录</div>
      <div v-if="aiTotal > 0" class="px-5 py-3 border-t border-slate-100 flex justify-end">
        <el-pagination
          v-model:current-page="aiPage"
          v-model:page-size="aiPageSize"
          :page-sizes="[10, 20, 50]"
          :total="aiTotal"
          layout="total, sizes, prev, pager, next"
          small
          background
        />
      </div>
    </div>

    <!-- Polymarket chain activity -->
    <div v-show="tab === 'chain'" class="bg-white rounded-xl shadow-sm border border-slate-100" v-loading="chainLoading">
      <el-table
        v-if="activity.length || chainLoading"
        :data="paginatedActivity"
        stripe
        size="small"
        empty-text="暂无 Polymarket 交易记录"
        :row-key="chainRowKey"
        :expand-row-keys="chainExpandedRows"
        @expand-change="onChainExpandChange"
      >
        <el-table-column type="expand" width="42">
          <template #default="{ row }">
            <div class="px-2 py-3">
              <div class="rounded-lg border border-slate-100 bg-slate-50/80 p-4">
                <div class="flex flex-wrap items-center gap-2 mb-3 text-xs text-slate-500">
                  <span v-if="row.usdc_size">金额 <strong class="text-slate-700">${{ row.usdc_size?.toFixed(2) }}</strong></span>
                  <span v-if="row.size">· {{ row.size?.toFixed(2) }} 份</span>
                  <span v-if="row.price">· @ {{ (row.price * 100).toFixed(1) }}¢</span>
                  <span>· {{ formatTs(row.timestamp) }}</span>
                  <el-tag v-if="row.is_ai_trade" size="small" type="success" effect="plain">AI</el-tag>
                  <el-tag v-else size="small" type="info" effect="plain">手动</el-tag>
                </div>
                <TradeReasonCell v-if="row.is_ai_trade" :row="row" />
                <span v-else class="text-xs text-slate-400 italic">手动下注，无 AI 分析</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="title" label="市场" min-width="168" show-overflow-tooltip />
        <el-table-column prop="outcome" label="方向" width="64" />
        <el-table-column label="类型" width="72">
          <template #default="{ row }">
            <el-tag :type="row.side === 'BUY' ? 'success' : 'warning'" size="small">{{ row.side || row.type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="金额" width="72">
          <template #default="{ row }">${{ row.usdc_size?.toFixed(2) }}</template>
        </el-table-column>
        <el-table-column label="价格" width="72">
          <template #default="{ row }">{{ row.price ? (row.price * 100).toFixed(1) + '¢' : '—' }}</template>
        </el-table-column>
        <el-table-column label="来源" width="72">
          <template #default="{ row }">
            <el-tag v-if="row.is_ai_trade" type="success" size="small" effect="plain">AI</el-tag>
            <el-tag v-else size="small" type="info" effect="plain">手动</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="128" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="text-xs text-slate-600">{{ formatTs(row.timestamp) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="理由摘要" min-width="140">
          <template #default="{ row }">
            <div class="cursor-pointer group" @click="toggleChainExpand(row)">
              <TradeReasonCell v-if="row.is_ai_trade && hasChainReason(row)" :row="row" compact />
              <span v-else-if="row.is_ai_trade" class="text-xs text-slate-400">AI（无理由）</span>
              <span v-else class="text-xs text-slate-300">—</span>
              <span v-if="row.is_ai_trade && hasChainReason(row)" class="text-emerald-600 text-xs opacity-0 group-hover:opacity-100 transition-opacity">展开 ↓</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="Tx" width="64" align="right">
          <template #default="{ row }">
            <a
              v-if="row.transaction_hash"
              :href="'https://polygonscan.com/tx/' + row.transaction_hash"
              target="_blank"
              rel="noopener"
              class="text-emerald-600 text-xs hover:underline"
              @click.stop
            >查看</a>
          </template>
        </el-table-column>
      </el-table>
      <div v-else class="p-8 text-center text-slate-400 text-sm">暂无 Polymarket 交易记录</div>
      <div v-if="chainTotal > 0" class="px-5 py-3 border-t border-slate-100 flex justify-end">
        <el-pagination
          v-model:current-page="chainPage"
          v-model:page-size="chainPageSize"
          :page-sizes="[10, 20, 50]"
          :total="chainTotal"
          layout="total, sizes, prev, pager, next"
          small
          background
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getAccountActivity, getTrades } from '../api'
import { useWallet } from '../composables/useWallet'
import TradeReasonCell from '../components/TradeReasonCell.vue'

const { address: walletAddress } = useWallet()
const tab = ref('ai')
const aiLoading = ref(false)
const chainLoading = ref(false)
const aiFetchError = ref('')
const chainFetchError = ref('')
const activity = ref([])
const chainTotal = ref(0)
const aiTrades = ref([])
const aiTotal = ref(0)
const aiPage = ref(1)
const aiPageSize = ref(10)
const aiExpandedRows = ref([])
const chainPage = ref(1)
const chainPageSize = ref(10)
const chainExpandedRows = ref([])

const paginatedActivity = computed(() => activity.value)

function formatTs(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString('zh-CN')
}

function formatTime(iso) {
  if (!iso) return '-'
  return new Date(iso).toLocaleString('zh-CN')
}

function formatMoney(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  return n.toFixed(2)
}

function statusLabel(s, row) {
  if (s === 'lost' && isStopLoss(row)) return '止损'
  const map = { open: '持仓中', won: '赢', lost: '输', canceled: '取消', pending: '待处理' }
  return map[s] || s
}

function statusType(s, row) {
  if (s === 'lost' && isStopLoss(row)) return 'danger'
  const map = { open: 'warning', won: 'success', lost: 'danger', canceled: 'info' }
  return map[s] || 'info'
}

function isStopLoss(row) {
  return Boolean(row?.order_error && String(row.order_error).includes('stop_loss'))
}

function tradeResultLabel(row) {
  if (row.status === 'won') return '盈利'
  if (row.status === 'lost') return isStopLoss(row) ? '止损亏损' : '亏损'
  if (row.status === 'open' || row.status === 'pending') return '未知'
  return '—'
}

function tradeResultType(row) {
  if (row.status === 'won') return 'success'
  if (row.status === 'lost') return 'danger'
  if (row.status === 'open' || row.status === 'pending') return 'info'
  return 'info'
}

function aiRowForCell(row) {
  return {
    is_ai_trade: true,
    market_reason: row.market_reason,
    outcome_reason: row.outcome_reason,
    reasoning: row.reasoning,
    outcome: row.outcome,
    self_win_rate: row.self_win_rate,
  }
}

function hasAiReason(row) {
  return !!(row.market_reason || row.outcome_reason || row.reasoning)
}

function hasChainReason(row) {
  return !!(row.market_reason || row.outcome_reason || row.reasoning)
}

function aiTradeRowKey(row) {
  return row.id || `${row.market_title}-${row.created_at}`
}

function chainRowKey(row) {
  return row.transaction_hash || `${row.title}-${row.timestamp}-${row.usdc_size}`
}

function toggleAiExpand(row) {
  const key = aiTradeRowKey(row)
  if (aiExpandedRows.value.includes(key)) {
    aiExpandedRows.value = aiExpandedRows.value.filter(k => k !== key)
  } else {
    aiExpandedRows.value = [...aiExpandedRows.value, key]
  }
}

function onAiExpandChange(row, expandedRows) {
  const key = aiTradeRowKey(row)
  const expanded = Array.isArray(expandedRows)
    ? expandedRows.some(r => aiTradeRowKey(r) === key)
    : !!expandedRows
  if (expanded) {
    if (!aiExpandedRows.value.includes(key)) aiExpandedRows.value = [...aiExpandedRows.value, key]
  } else {
    aiExpandedRows.value = aiExpandedRows.value.filter(k => k !== key)
  }
}

function toggleChainExpand(row) {
  const key = chainRowKey(row)
  if (chainExpandedRows.value.includes(key)) {
    chainExpandedRows.value = chainExpandedRows.value.filter(k => k !== key)
  } else {
    chainExpandedRows.value = [...chainExpandedRows.value, key]
  }
}

function onChainExpandChange(row, expandedRows) {
  const key = chainRowKey(row)
  const expanded = Array.isArray(expandedRows)
    ? expandedRows.some(r => chainRowKey(r) === key)
    : !!expandedRows
  if (expanded) {
    if (!chainExpandedRows.value.includes(key)) chainExpandedRows.value = [...chainExpandedRows.value, key]
  } else {
    chainExpandedRows.value = chainExpandedRows.value.filter(k => k !== key)
  }
}

function apiErrorMessage(e) {
  const msg = e?.response?.data?.error || e?.message || '请求失败'
  if (e?.response?.status === 404) {
    return 'API 接口不存在，请重启后端：make run-api'
  }
  return msg
}

async function loadAiTrades() {
  aiLoading.value = true
  aiFetchError.value = ''
  try {
    const { data } = await getTrades({
      page: aiPage.value,
      limit: aiPageSize.value,
    })
    aiTrades.value = Array.isArray(data?.data) ? data.data : []
    aiTotal.value = typeof data?.total === 'number' ? data.total : aiTrades.value.length
    aiExpandedRows.value = []
  } catch (e) {
    aiFetchError.value = apiErrorMessage(e)
    aiTrades.value = []
    aiTotal.value = 0
    throw e
  } finally {
    aiLoading.value = false
  }
}

async function loadChainActivity() {
  if (!walletAddress.value) {
    activity.value = []
    chainTotal.value = 0
    return
  }
  chainLoading.value = true
  chainFetchError.value = ''
  try {
    const { data } = await getAccountActivity(walletAddress.value, {
      limit: chainPageSize.value,
      offset: (chainPage.value - 1) * chainPageSize.value,
    })
    activity.value = Array.isArray(data?.data) ? data.data : []
    chainTotal.value = typeof data?.total === 'number' ? data.total : activity.value.length
    chainExpandedRows.value = []
  } catch (e) {
    chainFetchError.value = apiErrorMessage(e)
    activity.value = []
    chainTotal.value = 0
    throw e
  } finally {
    chainLoading.value = false
  }
}

async function refresh() {
  const results = await Promise.allSettled([loadAiTrades(), loadChainActivity()])
  const messages = []
  if (results[0].status === 'rejected' && aiFetchError.value) messages.push(aiFetchError.value)
  if (results[1].status === 'rejected' && chainFetchError.value) messages.push(chainFetchError.value)
  if (messages.length) ElMessage.error(messages.join('；'))
}

watch(walletAddress, () => {
  loadChainActivity().catch(() => {})
})

watch(chainPageSize, () => {
  chainPage.value = 1
  loadChainActivity().catch(() => {})
})

watch(chainPage, () => {
  loadChainActivity().catch(() => {})
})

watch(aiPage, () => {
  loadAiTrades().catch(() => {})
})

watch(aiPageSize, () => {
  aiPage.value = 1
  loadAiTrades().catch(() => {})
})

watch(tab, (value) => {
  if (value === 'ai' && aiTotal.value === 0 && !aiLoading.value) {
    loadAiTrades().catch(() => {})
  }
})

onMounted(() => {
  refresh()
})
</script>

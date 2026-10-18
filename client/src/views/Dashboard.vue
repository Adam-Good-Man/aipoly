<template>
  <div v-loading="initialLoading">
    <el-alert v-if="fetchError" type="error" :closable="false" class="mb-4" :title="fetchError" />

    <!-- Always-visible AI cycle diagnostics -->
    <div
      v-if="tradingInfo"
      class="mb-4 rounded-lg border border-slate-200 bg-slate-50 px-4 py-3"
    >
      <div class="text-sm font-medium text-slate-800 mb-2">AI 扫描进度</div>
      <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 text-center">
        <div>
          <div class="text-xl font-semibold text-slate-900">{{ lastCycle.screen_rounds ?? 0 }}</div>
          <div class="text-xs text-slate-500">初筛轮数</div>
        </div>
        <div>
          <div class="text-xl font-semibold text-slate-900">{{ lastCycle.items_screened ?? 0 }}</div>
          <div class="text-xs text-slate-500">遍历 items</div>
        </div>
        <div>
          <div class="text-xl font-semibold text-slate-900">{{ lastCycle.pool_size ?? 0 }}</div>
          <div class="text-xs text-slate-500">候选池</div>
        </div>
        <div>
          <div class="text-xl font-semibold text-slate-900">{{ lastCycle.shortlist_size ?? 0 }}</div>
          <div class="text-xs text-slate-500">深析 shortlist</div>
        </div>
        <div>
          <div class="text-xl font-semibold text-slate-900">{{ lastCycle.ai_candidates ?? 0 }}</div>
          <div class="text-xs text-slate-500">AI 候选</div>
        </div>
        <div>
          <div class="text-xl font-semibold text-slate-900">{{ lastCycle.trades_placed ?? 0 }}</div>
          <div class="text-xs text-slate-500">本轮下单</div>
        </div>
      </div>
      <p v-if="hasRealCycleTime(lastCycle.at)" class="text-xs text-slate-500 mt-2">
        最近一轮：{{ formatCycleTime(lastCycle.at) }}
        <span v-if="lastCycle.skipped"> · {{ lastCycle.skipped }}</span>
        <span v-else> · 正常</span>
        · 每 {{ tradingInfo.interval_minutes }} 分钟扫描 · 最低胜率 {{ tradingInfo.min_win_rate }}%
      </p>
      <p v-else class="text-xs text-slate-400 mt-2">尚未完成一轮扫描（重启后需等首轮跑完才会有数字）</p>
    </div>

    <el-alert
      v-if="tradingInfo?.last_cycle?.ai_paused || tradingInfo?.ai_paused"
      type="info"
      :closable="false"
      class="mb-4"
    >
      <template #title>AI 已暂停（可用资金不足）</template>
      <p class="text-sm">
        连续资金不足后已停止调用大模型，避免空耗 Token。结算 / 止损卖出 / 赎回释放现金后会自动恢复扫描。
        <span v-if="lastCycle.deployable != null">
          当前可动用约 ${{ Number(lastCycle.deployable).toFixed(2) }}
          <span v-if="tradingInfo.min_deployable_to_scan">
            （门槛 ${{ Number(tradingInfo.min_deployable_to_scan).toFixed(2) }}）
          </span>
        </span>
      </p>
    </el-alert>

    <el-alert
      v-if="tradingInfo?.last_cycle?.skipped"
      type="warning"
      :closable="true"
      class="mb-4"
    >
      <template #title>AI 扫描未下单</template>
      <p class="text-sm">
        {{ formatCycleTime(tradingInfo.last_cycle.at) }} · {{ tradingInfo.last_cycle.skipped }}
      </p>
    </el-alert>

    <el-alert
      v-if="settlementNotice.length"
      type="success"
      :closable="true"
      class="mb-4"
      @close="settlementNotice = []"
    >
      <template #title>有持仓已结算，资金已更新</template>
      <ul class="text-sm mt-1 list-disc pl-4">
        <li v-for="t in settlementNotice" :key="t.id">
          {{ t.market_title }} · {{ t.outcome }} ·
          <span :class="t.status === 'won' ? 'text-emerald-600' : 'text-red-500'">
            {{ settleStatusText(t) }} {{ t.pnl >= 0 ? '+' : '' }}${{ t.pnl?.toFixed(2) }}
          </span>
        </li>
      </ul>
    </el-alert>

    <el-alert
      v-if="matchBot === false"
      type="warning"
      :closable="false"
      class="mb-4"
    >
      <template #title>
        当前连接的钱包与服务器交易钱包不一致
      </template>
      <p class="text-sm">
        服务器交易钱包 (POLYMARKET_PRIVATE_KEY)：<code class="text-xs">{{ botSigner || '—' }}</code><br />
        要查看有余额和交易记录的 Polymarket 账户，请断开当前钱包并连接上面的地址。
        AI 自动下单始终使用服务器钱包。
      </p>
    </el-alert>

    <el-alert
      v-if="profile && profile.polymarket_registered === false"
      type="info"
      :closable="false"
      class="mb-4"
      title="此钱包在 Polymarket 上没有注册账户或无任何交易/持仓记录。"
    />

    <!-- Account -->
    <div class="bg-white rounded-xl p-5 shadow-sm border border-slate-100 mb-6">
      <div class="flex justify-between items-center mb-4">
        <h3 class="text-base font-semibold text-slate-700 flex items-center gap-2">
          <el-icon class="text-emerald-500"><Wallet /></el-icon>
          Polymarket 账户
          <el-tag size="small" type="success" effect="plain">链上真实数据</el-tag>
        </h3>
        <el-button size="small" :loading="refreshing" @click="fetchData(true)">刷新</el-button>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
        <div>
          <div class="text-xs text-slate-500 mb-1">连接钱包 (EOA)</div>
          <code class="text-xs bg-slate-50 px-2 py-1 rounded break-all">{{ walletAddress }}</code>
        </div>
        <div>
          <div class="text-xs text-slate-500 mb-1">Polymarket 交易钱包</div>
          <code class="text-xs bg-emerald-50 text-emerald-800 px-2 py-1 rounded break-all">{{ profile?.proxy_wallet || '—' }}</code>
        </div>
        <div v-if="profile?.display_name || profile?.pseudonym">
          <div class="text-xs text-slate-500 mb-1">Polymarket 昵称</div>
          <span>{{ profile.display_name || profile.pseudonym }}</span>
        </div>
      </div>
    </div>

    <!-- Stats -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
      <div
        v-for="card in assetCards"
        :key="card.key"
        class="stat-card relative rounded-xl p-5 shadow-sm border"
        :class="card.boxClass"
      >
        <StatHelp :text="card.tip" />
        <div class="text-sm text-slate-500 mb-1 pr-6">{{ card.label }}</div>
        <div class="text-2xl font-bold" :class="card.valueClass">{{ card.value }}</div>
        <div class="text-xs text-slate-400 mt-1">{{ card.sub }}</div>
      </div>
    </div>

    <!-- 持仓战绩：全历史统计（Polymarket 链上数据分页拉全） -->
    <div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4 mb-6">
      <div
        v-for="card in perfCards"
        :key="card.key"
        class="stat-card relative rounded-xl p-5 shadow-sm border"
        :class="card.boxClass"
      >
        <StatHelp :text="card.tip" />
        <div class="text-sm text-slate-500 mb-1 pr-6">{{ card.label }}</div>
        <div class="text-2xl font-bold" :class="card.valueClass">{{ card.value }}</div>
        <div class="text-xs text-slate-400 mt-1">{{ card.sub }}</div>
      </div>
    </div>
    <div v-if="lastSynced" class="text-xs text-slate-400 mb-4 -mt-2">
      上次同步：{{ lastSynced }} · 每 60 秒自动检查结算
      <span v-if="tradingInfo"> · AI 每 {{ tradingInfo.interval_minutes }} 分钟自动扫描（仅 {{ tradingInfo.max_market_days }} 天内结算的市场）</span>
    </div>

    <!-- 当前持仓（含下注金额、时间与 AI 理由） -->
    <div class="bg-white rounded-xl shadow-sm border border-slate-100 mb-6">
      <div class="p-5 border-b border-slate-100 flex justify-between items-center">
        <h3 class="text-base font-semibold text-slate-700">
          当前持仓
          <span v-if="totalOpen" class="text-sm font-normal text-slate-400 ml-1">共 {{ totalOpen }} 笔</span>
        </h3>
        <router-link :to="localePath('/app/trades')" class="text-sm text-emerald-600 hover:underline">查看全部成交</router-link>
      </div>
      <el-table
        v-if="positions.length"
        :data="paginatedPositions"
        stripe
        size="small"
        :row-key="positionRowKey"
        :expand-row-keys="expandedRows"
        :default-sort="{ prop: 'entry_timestamp', order: 'descending' }"
        @sort-change="onPositionSortChange"
        @expand-change="onExpandChange"
      >
        <el-table-column type="expand" width="42">
          <template #default="{ row }">
            <div class="px-2 py-3">
              <div class="rounded-lg border border-slate-100 bg-slate-50/80 p-4">
                <div class="flex flex-wrap items-center gap-2 mb-3 text-xs text-slate-500">
                  <span v-if="row.entry_amount">下注 <strong class="text-slate-700">${{ row.entry_amount.toFixed(2) }}</strong></span>
                  <span v-if="row.entry_timestamp">· {{ formatTs(row.entry_timestamp) }}</span>
                  <el-tag v-if="row.is_ai_trade" size="small" type="success" effect="plain">AI</el-tag>
                  <el-tag v-else size="small" type="info" effect="plain">手动</el-tag>
                </div>
                <TradeReasonCell :row="row" />
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="title" label="市场" min-width="168" show-overflow-tooltip />
        <el-table-column prop="outcome" label="方向" width="64" />
        <el-table-column label="状态" width="88">
          <template #default="{ row }">
            <el-tag size="small" :type="settlementTagType(row.settlement_status)" effect="plain">
              {{ settlementLabel(row.settlement_status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="结果" width="72">
          <template #default="{ row }">
            <el-tag size="small" :type="resultTagType(row)" effect="plain">
              {{ resultLabel(row) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          prop="cash_pnl"
          label="盈亏"
          width="96"
          sortable="custom"
          :sort-orders="['descending', 'ascending']"
        >
          <template #default="{ row }">
            <span v-if="row.result_known" :class="row.result === 'loss' ? 'text-red-500' : 'text-emerald-600'">
              {{ formatPositionPnl(row.cash_pnl) }}
            </span>
            <span v-else class="text-slate-400">
              {{ formatPositionPnl(row.cash_pnl) }}
              <span class="text-xs">(浮)</span>
            </span>
          </template>
        </el-table-column>
        <el-table-column
          prop="entry_amount"
          label="下注"
          width="72"
          sortable="custom"
          :sort-orders="['descending', 'ascending']"
        >
          <template #default="{ row }">
            <span v-if="row.entry_amount">${{ row.entry_amount.toFixed(2) }}</span>
            <span v-else class="text-slate-300">—</span>
          </template>
        </el-table-column>
        <el-table-column
          prop="entry_timestamp"
          label="时间"
          width="128"
          show-overflow-tooltip
          sortable="custom"
          :sort-orders="['descending', 'ascending']"
        >
          <template #default="{ row }">
            <span class="text-xs text-slate-600">{{ row.entry_timestamp ? formatTs(row.entry_timestamp) : '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="理由摘要" min-width="140">
          <template #default="{ row }">
            <div class="cursor-pointer group" @click="toggleExpand(row)">
              <TradeReasonCell v-if="hasReason(row)" :row="row" compact />
              <span v-else class="text-xs text-slate-300">—</span>
              <span v-if="hasReason(row)" class="text-emerald-600 text-xs opacity-0 group-hover:opacity-100 transition-opacity">展开 ↓</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="仓位" width="120" align="right">
          <template #default="{ row }">
            <div class="text-xs text-slate-600">{{ row.size?.toFixed(1) }} @ {{ (row.avg_price * 100).toFixed(1) }}¢</div>
            <div class="text-xs text-slate-400">现 {{ (row.cur_price * 100).toFixed(1) }}¢</div>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="totalOpen > 0" class="px-5 py-3 border-t border-slate-100 flex justify-end">
        <el-pagination
          v-model:current-page="positionPage"
          v-model:page-size="positionPageSize"
          :page-sizes="[5, 10, 20, 50]"
          :total="totalOpen"
          layout="total, sizes, prev, pager, next"
          small
          background
        />
      </div>
      <div v-else class="p-8 text-center text-slate-400 text-sm">暂无持仓</div>
    </div>
  </div>
</template>

<script setup>
import { ref, inject, watch, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getAccountProfile, getAccountPositions, getDashboard } from '../api'
import TradeReasonCell from '../components/TradeReasonCell.vue'
import StatHelp from '../components/StatHelp.vue'
import { useLocalePath } from '../composables/useLocalePath'

const { localePath } = useLocalePath()
const walletAddress = inject('walletAddress')
const initialLoading = ref(true)
const refreshing = ref(false)
const loadedOnce = ref(false)
const fetchError = ref('')
const profile = ref(null)
const clobBalance = ref(0)
const clobReady = ref(false)
const matchBot = ref(null)
const botSigner = ref('')
const positions = ref([])
const totalOpen = ref(0)
const expandedRows = ref([])
const lastSynced = ref('')
const settlementNotice = ref([])
const prevTotalBalance = ref(null)
const prevPositionCount = ref(null)
const tradingInfo = ref(null)
const performance = ref(null)
const positionPage = ref(1)
const positionPageSize = ref(10)
const positionSort = ref({ prop: 'entry_timestamp', order: 'descending' })
let pollTimer = null

const lastCycle = computed(() => tradingInfo.value?.last_cycle || {})

const assetCards = computed(() => [
  {
    key: 'total',
    label: '账户总资产',
    value: `$${formatMoney(totalBalance.value)}`,
    sub: '持仓 + CLOB 现金 · 与 polymarket.com 一致',
    tip: '当前持仓市值加上 CLOB 可用现金。用于对照官网 Portfolio 总资产；不含未同步的链下记账。',
    boxClass: 'bg-white border-slate-100',
    valueClass: 'text-emerald-700',
  },
  {
    key: 'portfolio',
    label: '持仓总价值',
    value: `$${formatMoney(profile.value?.portfolio_value)}`,
    sub: '当前未平仓头寸',
    tip: '仅统计当前未平仓头寸的市值（按最新价估算），不含 CLOB 现金。',
    boxClass: 'bg-white border-slate-100',
    valueClass: 'text-slate-800',
  },
  {
    key: 'cash',
    label: 'CLOB 现金 (USDC)',
    value: `$${formatMoney(clobBalance.value)}`,
    sub: clobReady.value ? '可下单余额' : '需连接与 .env 相同的钱包',
    tip: 'Polymarket CLOB 中可用于下单的 USDC。若连接钱包与机器人签名地址不一致，可能无法读取，显示会偏少。',
    boxClass: 'bg-white border-slate-100',
    valueClass: 'text-slate-800',
  },
  {
    key: 'open_n',
    label: '持仓数量',
    value: String(performance.value?.open_count ?? profile.value?.open_positions ?? 0),
    sub: '当前未平仓 · 全量拉取',
    tip: '当前仍未结算/未平仓的独立持仓个数（按市场+方向计），不是成交笔数。',
    boxClass: 'bg-white border-slate-100',
    valueClass: 'text-blue-600',
  },
])

const perfCards = computed(() => {
  const p = performance.value
  const closed = p?.closed_count ?? 0
  return [
    {
      key: 'history',
      label: '历史持仓',
      value: String(p?.position_count ?? p?.total_count ?? 0),
      sub: `独立持仓 = 进行中 + 已赢 + 已输 · 已平仓 ${closed}`,
      tip: `这里统计的是「独立持仓」个数（同一市场+同一方向只算 1 次），等于：进行中 + 已赢 + 已输。

与 Polymarket 官网 Portfolio → History 不一致是正常的：官网 History 按「每笔买卖成交（fill）」计数，加仓/减仓/分批成交都会多出很多行；本页按持仓去重。要对齐官网 History，请看「交易」页的链上成交，而不是本数字。

另外极小仓位（尘埃仓）可能被 Data API 过滤，也会略少于官网列表。`,
      boxClass: 'bg-white border-slate-100',
      valueClass: 'text-slate-800',
    },
    {
      key: 'open',
      label: '进行中',
      value: String(p?.open_count ?? 0),
      sub: '全历史 · 未结算',
      tip: '尚未结算（或尚未记为已赢/已输）的持仓数。浮盈亏仍会反映在累计盈亏中。',
      boxClass: 'bg-white border-slate-100',
      valueClass: 'text-amber-600',
    },
    {
      key: 'won',
      label: '已赢',
      value: String(p?.won_count ?? 0),
      sub: '全历史 · 已结算盈利或持平',
      tip: '已结算且结果为盈利或持平的独立持仓数（含接近打平的结算）。',
      boxClass: 'bg-white border-slate-100',
      valueClass: 'text-emerald-600',
    },
    {
      key: 'lost',
      label: '已输',
      value: String(p?.lost_count ?? 0),
      sub: '全历史 · 已结算亏损',
      tip: '已结算亏损，或止损成功卖出记为已输的独立持仓数。卖不掉时不会伪造「已输」。',
      boxClass: 'bg-white border-slate-100',
      valueClass: 'text-red-500',
    },
    {
      key: 'wr',
      label: '胜率',
      value: p?.win_rate_known ? `${p.win_rate.toFixed(1)}%` : '—',
      sub: p?.win_rate_known ? `已结算 ${p.settled_count} 笔` : '暂无已结算持仓',
      tip: '胜率 = 已赢 /（已赢 + 已输）。只统计已结算持仓；进行中不计入分母。',
      boxClass: 'bg-white border-slate-100',
      valueClass: 'text-slate-800',
    },
    {
      key: 'pnl',
      label: '累计盈亏',
      value: formatSignedMoney(p?.total_pnl),
      sub: '全历史 · 已实现 + 当前浮盈亏',
      tip: '已平仓/已结算的已实现盈亏，加上当前进行中持仓的浮动盈亏。不等于「充值 − 提现」；手续费、赎回时滞也会造成差额。',
      boxClass: 'bg-white border-slate-100',
      valueClass: (p?.total_pnl ?? 0) >= 0 ? 'text-emerald-600' : 'text-red-500',
    },
  ]
})

function hasRealCycleTime(iso) {
  if (!iso) return false
  const t = Date.parse(iso)
  return Number.isFinite(t) && t > Date.parse('2000-01-01')
}
const sortedPositions = computed(() => {
  const list = [...positions.value]
  const { prop, order } = positionSort.value
  if (!prop || !order) return list
  return list.sort((a, b) => comparePositionRows(a, b, prop, order))
})

const paginatedPositions = computed(() => sortedPositions.value)

const totalBalance = computed(() => {
  const portfolio = profile.value?.portfolio_value ?? 0
  const cash = clobReady.value ? (clobBalance.value ?? 0) : 0
  return portfolio + cash
})

function formatMoney(v) {
  return (v ?? 0).toFixed(2)
}

function formatSignedMoney(v) {
  const n = v ?? 0
  if (Math.abs(n) < 0.01) return '$0.00'
  return `${n >= 0 ? '+' : '-'}$${Math.abs(n).toFixed(2)}`
}

function settleStatusText(t) {
  if (t.status === 'won') return '赢'
  if (t.order_error && String(t.order_error).includes('stop_loss')) return '止损'
  return '输'
}

function formatTs(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString('zh-CN')
}

function formatCycleTime(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('zh-CN')
}

function settlementLabel(status) {
  switch (status) {
    case 'redeemable': return '可赎回'
    case 'resolved_won': return '已赢'
    case 'resolved_lost': return '已输'
    default: return '进行中'
  }
}

function settlementTagType(status) {
  switch (status) {
    case 'redeemable': return 'warning'
    case 'resolved_won': return 'success'
    case 'resolved_lost': return 'danger'
    default: return 'info'
  }
}

function resultLabel(row) {
  if (!row.result_known) return '未知'
  if (row.result === 'profit') return '盈利'
  if (row.result === 'breakeven') return '持平'
  return '亏损'
}

function resultTagType(row) {
  if (!row.result_known) return 'info'
  if (row.result === 'profit') return 'success'
  if (row.result === 'breakeven') return 'success'
  return 'danger'
}

function formatPositionPnl(pnl) {
  const v = pnl ?? 0
  if (Math.abs(v) < 0.01) return '$0.00'
  return `${v >= 0 ? '+' : ''}$${v.toFixed(2)}`
}

function hasReason(row) {
  return row.is_ai_trade && (row.market_reason || row.outcome_reason || row.reasoning)
}

function positionRowKey(row) {
  return row.condition_id || `${row.title}-${row.outcome}`
}

function positionSortValue(row, prop) {
  switch (prop) {
    case 'cash_pnl':
      return row.cash_pnl ?? 0
    case 'entry_amount':
      return row.entry_amount > 0 ? row.entry_amount : null
    case 'entry_timestamp':
      return row.entry_timestamp > 0 ? row.entry_timestamp : null
    default:
      return null
  }
}

function comparePositionRows(a, b, prop, order) {
  const va = positionSortValue(a, prop)
  const vb = positionSortValue(b, prop)
  const aEmpty = va == null
  const bEmpty = vb == null
  if (aEmpty && bEmpty) return 0
  if (aEmpty) return 1
  if (bEmpty) return -1
  const dir = order === 'ascending' ? 1 : -1
  if (va < vb) return -dir
  if (va > vb) return dir
  return 0
}

function onPositionSortChange({ prop, order }) {
  positionSort.value = { prop: prop || 'entry_timestamp', order: order || 'descending' }
  positionPage.value = 1
}

function toggleExpand(row) {
  const key = positionRowKey(row)
  if (expandedRows.value.includes(key)) {
    expandedRows.value = expandedRows.value.filter(k => k !== key)
  } else {
    expandedRows.value = [...expandedRows.value, key]
  }
}

function onExpandChange(row, expanded) {
  const key = positionRowKey(row)
  if (expanded) {
    if (!expandedRows.value.includes(key)) expandedRows.value = [...expandedRows.value, key]
  } else {
    expandedRows.value = expandedRows.value.filter(k => k !== key)
  }
}

function apiErrorMessage(e) {
  const msg = e?.response?.data?.error || e?.message || '请求失败'
  if (e?.response?.status === 404) {
    return 'API 接口不存在，请重启后端：make run-api'
  }
  return msg
}

async function loadPositions(sync = false) {
  if (!walletAddress?.value) return
  const { data } = await getAccountPositions(walletAddress.value, {
    limit: positionPageSize.value,
    offset: (positionPage.value - 1) * positionPageSize.value,
    ...(sync ? { sync: 1 } : {}),
  })
  positions.value = data.data || []
  totalOpen.value = data.total_open ?? positions.value.length
  performance.value = data.performance || null
  if (data.clob_balance != null && data.clob_ready) {
    clobBalance.value = data.clob_balance
    clobReady.value = true
  }
  return data
}

async function fetchData(manual = false, sync = false) {
  if (!walletAddress?.value) return
  if (!loadedOnce.value) {
    initialLoading.value = true
  } else if (manual) {
    refreshing.value = true
  }
  fetchError.value = ''
  const errors = []
  try {
    const [profRes, posRes, dashRes] = await Promise.allSettled([
      getAccountProfile(walletAddress.value, manual),
      loadPositions(sync || manual),
      getDashboard(),
    ])

    if (dashRes.status === 'fulfilled') {
      tradingInfo.value = dashRes.value.data.trading || null
    }

    if (profRes.status === 'fulfilled') {
      profile.value = profRes.value.data.profile
      matchBot.value = profRes.value.data.matches_bot ?? null
      botSigner.value = profRes.value.data.bot_signer || ''
      clobBalance.value = profRes.value.data.clob_balance ?? 0
      clobReady.value = !!profRes.value.data.clob_ready
      if (profRes.value.data.recently_resolved?.length) {
        settlementNotice.value = profRes.value.data.recently_resolved
      }
    } else {
      errors.push(apiErrorMessage(profRes.reason))
    }

    if (posRes.status === 'fulfilled') {
      const posData = posRes.value
      if (posData?.redeemed?.length) {
        const ok = posData.redeemed.filter(r => !r.error)
        const failed = posData.redeemed.filter(r => r.error)
        if (ok.length) {
          ElMessage.success(`已自动赎回 ${ok.length} 个持仓，资金已回 CLOB 余额`)
        }
        if (failed.length && (manual || sync)) {
          ElMessage.warning(`赎回失败 ${failed.length} 个：${failed[0].error?.slice(0, 80)}`)
        }
      }
    } else {
      errors.push(apiErrorMessage(posRes.reason))
      positions.value = []
      totalOpen.value = 0
      performance.value = null
    }

    if (errors.length && !profile.value) {
      fetchError.value = errors.join('；')
      if (manual || !loadedOnce.value) ElMessage.error(fetchError.value)
    } else if (errors.length) {
      fetchError.value = '部分数据加载失败：' + errors.join('；')
    }

    if (profRes.status === 'fulfilled' || posRes.status === 'fulfilled') {
      lastSynced.value = new Date().toLocaleString('zh-CN')
      loadedOnce.value = true

      const balanceAfter = totalBalance.value
      const posCountAfter = totalOpen.value || positions.value.length
      if (prevTotalBalance.value != null && Math.abs(balanceAfter - prevTotalBalance.value) >= 0.01) {
        if (sync && !manual) {
          ElMessage.success(`账户资金已更新：$${balanceAfter.toFixed(2)}`)
        }
      }
      if (prevPositionCount.value != null && posCountAfter < prevPositionCount.value && sync) {
        ElMessage.info('有持仓已结算，数据已同步')
      }
      prevTotalBalance.value = balanceAfter
      prevPositionCount.value = posCountAfter
    }
  } catch (e) {
    console.error(e)
    fetchError.value = apiErrorMessage(e)
    if (manual || !loadedOnce.value) ElMessage.error(fetchError.value)
  } finally {
    initialLoading.value = false
    refreshing.value = false
  }
}

watch(positionPageSize, () => {
  positionPage.value = 1
  loadPositions(false).catch(() => {})
})

watch(positionPage, () => {
  loadPositions(false).catch(() => {})
})

watch(walletAddress, (addr) => {
  if (addr) {
    prevTotalBalance.value = null
    prevPositionCount.value = null
    positionPage.value = 1
    fetchData(false, false)
  }
}, { immediate: true })

onMounted(() => {
  pollTimer = setInterval(() => {
    if (walletAddress?.value && loadedOnce.value) {
      fetchData(false, true)
    }
  }, 60000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

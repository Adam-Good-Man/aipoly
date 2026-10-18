<template>
  <div v-loading="loading">
    <div class="mb-4">
      <el-button text type="primary" @click="goBack">
        <el-icon class="mr-1"><ArrowLeft /></el-icon>
        返回列表
      </el-button>
    </div>

    <el-alert v-if="fetchError" type="error" :closable="false" class="mb-4" :title="fetchError" />

    <template v-if="event">
      <div class="bg-white rounded-xl shadow-sm border border-slate-100 p-6 mb-4">
        <div class="flex flex-wrap items-start gap-2 mb-3">
          <h2 class="text-xl font-semibold text-slate-800 leading-snug">{{ event.title }}</h2>
          <el-tag v-if="event.is_fast" type="danger" size="small" effect="dark">快速</el-tag>
          <el-tag size="small" effect="plain">{{ event.category || '其他' }}</el-tag>
        </div>

        <div class="flex flex-wrap gap-4 text-sm text-slate-500 mb-4">
          <span>成交量 <strong class="text-slate-700">${{ formatVolume(event.volume) }}</strong></span>
          <span>流动性 <strong class="text-slate-700">${{ formatVolume(event.liquidity) }}</strong></span>
          <span v-if="event.market_count">{{ event.market_count }} 个子市场</span>
          <span v-if="event.end_date">截止 {{ formatDate(event.end_date) }}</span>
        </div>

        <div v-if="event.tags?.length" class="flex flex-wrap gap-1.5 mb-4">
          <el-tag v-for="tag in event.tags" :key="tag" size="small" type="info" effect="plain">{{ tag }}</el-tag>
        </div>

        <p class="text-sm text-slate-600 leading-relaxed whitespace-pre-line">{{ event.description }}</p>

        <div class="mt-4">
          <a
            v-if="event.polymarket_url"
            :href="event.polymarket_url"
            target="_blank"
            rel="noopener"
            class="text-sm text-emerald-600 hover:underline"
          >
            在 Polymarket 打开 ↗
          </a>
        </div>
      </div>

      <div class="bg-white rounded-xl shadow-sm border border-slate-100">
        <div class="px-5 py-4 border-b border-slate-100">
          <h3 class="text-base font-semibold text-slate-700">子市场 ({{ event.markets?.length || 0 }})</h3>
        </div>
        <el-table v-if="event.markets?.length" :data="event.markets" stripe size="small">
          <el-table-column prop="question" label="问题" min-width="220" show-overflow-tooltip />
          <el-table-column label="选项 / 价格" min-width="180">
            <template #default="{ row }">
              <div v-for="(o, i) in row.outcomes" :key="i" class="text-xs text-slate-600">
                {{ o.outcome }} · {{ formatPrice(o.price) }}
              </div>
            </template>
          </el-table-column>
          <el-table-column label="成交量" width="96">
            <template #default="{ row }">${{ formatVolume(row.volume) }}</template>
          </el-table-column>
          <el-table-column label="24h 变动" width="88">
            <template #default="{ row }">
              <span v-if="row.one_day_price_change" :class="row.one_day_price_change >= 0 ? 'text-emerald-600' : 'text-red-500'">
                {{ (row.one_day_price_change * 100).toFixed(1) }}%
              </span>
              <span v-else class="text-slate-300">—</span>
            </template>
          </el-table-column>
          <el-table-column label="截止" width="128" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="text-xs">{{ row.end_date ? formatDate(row.end_date) : '—' }}</span>
            </template>
          </el-table-column>
        </el-table>
        <div v-else class="p-8 text-center text-slate-400 text-sm">暂无活跃子市场</div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import { getEvent } from '../api'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const fetchError = ref('')
const event = ref(null)

function formatVolume(v) {
  const n = Number(v)
  if (!Number.isFinite(n) || n <= 0) return '0'
  if (n >= 1e6) return (n / 1e6).toFixed(2) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K'
  return n.toFixed(0)
}

function formatPrice(p) {
  const n = Number(p)
  if (!Number.isFinite(n)) return '—'
  return (n * 100).toFixed(1) + '¢'
}

function formatDate(d) {
  if (!d) return '—'
  return new Date(d).toLocaleString('zh-CN')
}

function goBack() {
  router.push({ name: 'Events', params: { locale: route.params.locale } })
}

async function loadEvent() {
  const id = route.params.id
  if (!id) return
  loading.value = true
  fetchError.value = ''
  event.value = null
  try {
    const { data } = await getEvent(id)
    event.value = data?.data || null
    if (!event.value) fetchError.value = '事件不存在'
  } catch (e) {
    fetchError.value = e?.response?.data?.error || e?.message || '加载失败'
  } finally {
    loading.value = false
  }
}

watch(() => route.params.id, loadEvent)
onMounted(loadEvent)
</script>

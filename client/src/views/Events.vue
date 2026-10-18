<template>
  <div>
    <div class="flex flex-wrap gap-3 mb-4 items-center justify-between">
      <div class="flex items-center gap-2">
        <el-switch v-model="fastOnly" active-text="仅快速事件" @change="onFilterChange" />
        <span v-if="live" class="text-xs text-emerald-600 flex items-center gap-1">
          <span class="inline-block w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
          实时 · Polymarket
        </span>
      </div>
      <el-button size="small" :loading="loading" @click="fetchEvents">刷新</el-button>
    </div>

    <div class="flex flex-wrap gap-2 mb-4">
      <button
        v-for="cat in displayCategories"
        :key="cat.key || 'all'"
        type="button"
        class="px-3 py-1.5 rounded-full text-sm border transition-colors"
        :class="activeCategory === cat.key
          ? 'bg-emerald-600 text-white border-emerald-600'
          : 'bg-white text-slate-600 border-slate-200 hover:border-emerald-300'"
        @click="selectCategory(cat.key)"
      >
        {{ cat.label }}
        <span v-if="cat.count != null" class="opacity-70 ml-1">({{ cat.count }})</span>
      </button>
    </div>

    <el-alert v-if="fetchError" type="error" :closable="false" class="mb-4" :title="fetchError" />

    <div class="bg-white rounded-xl shadow-sm border border-slate-100" v-loading="loading">
      <div v-if="events.length || loading">
        <div
          v-for="event in events"
          :key="event.id"
          class="px-4 py-3 border-b border-slate-100 last:border-b-0 hover:bg-slate-50 cursor-pointer transition-colors group"
          @click="openDetail(event.id)"
        >
          <div class="flex items-center gap-2 min-w-0">
            <h3 class="font-medium text-slate-800 truncate min-w-0 flex-1">{{ event.title }}</h3>
            <el-tag v-if="event.is_fast" type="danger" size="small" effect="plain" class="shrink-0">快速</el-tag>
            <el-tag size="small" effect="plain" class="shrink-0">{{ event.category || '其他' }}</el-tag>
            <span class="text-xs text-slate-400 shrink-0 hidden sm:inline">
              {{ event.market_count || 0 }} 市场 · 量 ${{ formatVolume(event.volume) }}
            </span>
            <span class="text-emerald-600 text-xs opacity-0 group-hover:opacity-100 transition-opacity shrink-0 ml-auto">
              查看详情 →
            </span>
          </div>
          <p v-if="event.summary" class="text-xs text-slate-500 truncate mt-1">{{ event.summary }}</p>
        </div>
      </div>
      <div v-else class="p-8 text-center text-slate-400 text-sm">暂无市场事件，请检查网络或稍后重试</div>

      <div v-if="total > 0" class="px-5 py-3 border-t border-slate-100 flex justify-end">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50]"
          :total="total"
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
import { useRoute, useRouter } from 'vue-router'
import { getEvents } from '../api'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const fetchError = ref('')
const events = ref([])
const categories = ref([])
const live = ref(false)
const fastOnly = ref(false)
const activeCategory = ref('')
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)

const defaultCategories = [
  { key: '', label: '全部', count: null },
  { key: '体育', label: '体育', count: null },
  { key: '政治', label: '政治', count: null },
  { key: '加密货币', label: '加密货币', count: null },
  { key: '电竞', label: '电竞', count: null },
  { key: '娱乐', label: '娱乐', count: null },
  { key: '其他', label: '其他', count: null },
]

const displayCategories = computed(() => {
  if (categories.value.length) return categories.value
  return defaultCategories
})

function formatVolume(v) {
  const n = Number(v)
  if (!Number.isFinite(n) || n <= 0) return '0'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K'
  return n.toFixed(0)
}

function openDetail(id) {
  router.push({
    name: 'EventDetail',
    params: { locale: route.params.locale, id },
  })
}

function selectCategory(key) {
  if (activeCategory.value === key) return
  activeCategory.value = key
  page.value = 1
  fetchEvents()
}

function onFilterChange() {
  page.value = 1
  fetchEvents()
}

async function fetchEvents() {
  loading.value = true
  fetchError.value = ''
  try {
    const params = {
      page: page.value,
      limit: pageSize.value,
      live: 'true',
    }
    if (activeCategory.value) params.category = activeCategory.value
    if (fastOnly.value) params.fast = 'true'
    const { data } = await getEvents(params)
    events.value = Array.isArray(data?.data) ? data.data : []
    total.value = typeof data?.total === 'number' ? data.total : events.value.length
    live.value = !!data?.live
    if (Array.isArray(data?.categories) && data.categories.length) {
      categories.value = data.categories
    }
  } catch (e) {
    fetchError.value = e?.response?.data?.error || e?.message || '加载失败'
    events.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

watch(page, () => fetchEvents())
watch(pageSize, () => {
  page.value = 1
  fetchEvents()
})

onMounted(fetchEvents)
</script>

<template>
  <div :class="compact ? 'text-xs leading-relaxed' : 'text-sm leading-relaxed'">
    <template v-if="row.is_ai_trade && hasReason">
      <template v-if="compact">
        <p v-if="previewText" class="text-slate-600 line-clamp-2 m-0">{{ previewText }}</p>
      </template>
      <template v-else>
        <div v-if="sections.market" class="text-slate-700 mb-3">
          <div class="inline-block text-emerald-700 font-medium bg-emerald-50 px-1.5 py-0.5 rounded text-xs mb-1">为何选此市场</div>
          <p class="m-0 whitespace-pre-line">{{ sections.market }}</p>
        </div>
        <div v-if="sections.outcome" class="text-slate-700 mb-3">
          <div class="inline-block text-blue-700 font-medium bg-blue-50 px-1.5 py-0.5 rounded text-xs mb-1">为何选 {{ row.outcome || '该方向' }}</div>
          <p class="m-0 whitespace-pre-line">{{ sections.outcome }}</p>
        </div>
        <div v-if="sections.sources" class="text-slate-700 mb-3">
          <div class="inline-block text-amber-800 font-medium bg-amber-50 px-1.5 py-0.5 rounded text-xs mb-1">查阅资料</div>
          <p class="m-0 whitespace-pre-line text-slate-600">{{ sections.sources }}</p>
        </div>
        <div v-if="sections.process" class="text-slate-700 mb-3">
          <div class="inline-block text-violet-800 font-medium bg-violet-50 px-1.5 py-0.5 rounded text-xs mb-1">分析过程</div>
          <p class="m-0 whitespace-pre-line text-slate-600">{{ sections.process }}</p>
        </div>
        <div v-if="sections.fill" class="text-slate-700 mb-3">
          <div class="inline-block text-rose-800 font-medium bg-rose-50 px-1.5 py-0.5 rounded text-xs mb-1">成交说明</div>
          <p class="m-0 whitespace-pre-line text-slate-600">{{ sections.fill }}</p>
        </div>
        <div
          v-if="!sections.market && !sections.outcome && !sections.sources && !sections.process && row.reasoning"
          class="text-slate-600 whitespace-pre-line"
        >
          {{ row.reasoning }}
        </div>
        <div v-if="row.self_win_rate" class="text-xs text-slate-400 mt-2 pt-2 border-t border-slate-100">
          AI 预估胜率 {{ row.self_win_rate.toFixed(0) }}%
        </div>
      </template>
    </template>
    <template v-else-if="row.is_ai_trade">
      <span :class="compact ? 'text-slate-400' : 'text-slate-500'">AI 下单（理由未记录）</span>
    </template>
    <template v-else>
      <span :class="compact ? 'text-slate-400 italic text-xs' : 'text-slate-400 italic'">手动下注，无 AI 分析</span>
    </template>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  row: { type: Object, required: true },
  compact: { type: Boolean, default: false },
})

const hasReason = computed(() => {
  const r = props.row
  return !!(r.market_reason || r.outcome_reason || r.reasoning)
})

function extractSections(text) {
  const raw = (text || '').trim()
  let market = ''
  let outcome = ''
  let sources = ''
  let process = ''
  let fill = ''
  if (!raw) return { market, outcome, sources, process, fill }

  const mk = '【为何选此市场】'
  const ok = '【为何选 '
  const src = '【查阅资料】'
  const proc = '【分析过程】'
  const fillTag = '【成交说明】'

  if (raw.includes(mk)) {
    let rest = raw.slice(raw.indexOf(mk) + mk.length)
    const okIdx = rest.indexOf('\n' + ok)
    if (okIdx >= 0) {
      market = rest.slice(0, okIdx).trim()
      rest = rest.slice(okIdx + 1)
      const close = rest.indexOf('】')
      if (close >= 0) {
        outcome = rest.slice(close + 1).trim()
      }
    } else {
      market = rest.trim()
    }
  }

  const cutTrailing = (s) => {
    for (const tag of [src, proc, fillTag, mk, '\n' + ok]) {
      const i = s.indexOf(tag)
      if (i >= 0) s = s.slice(0, i)
    }
    return s.trim()
  }
  if (outcome) outcome = cutTrailing(outcome)

  if (raw.includes(src)) {
    let rest = raw.slice(raw.indexOf(src) + src.length)
    const next = rest.search(/\n【/)
    sources = (next >= 0 ? rest.slice(0, next) : rest).trim()
  }
  if (raw.includes(proc)) {
    let rest = raw.slice(raw.indexOf(proc) + proc.length)
    const next = rest.search(/\n【/)
    process = (next >= 0 ? rest.slice(0, next) : rest).trim()
  }
  if (raw.includes(fillTag)) {
    let rest = raw.slice(raw.indexOf(fillTag) + fillTag.length)
    const next = rest.search(/\n【/)
    fill = (next >= 0 ? rest.slice(0, next) : rest).trim()
  }

  return { market, outcome, sources, process, fill }
}

const sections = computed(() => {
  const r = props.row
  const fromReasoning = extractSections(r.reasoning || '')
  return {
    market: fromReasoning.market || r.market_reason || '',
    outcome: fromReasoning.outcome || r.outcome_reason || '',
    sources: fromReasoning.sources || '',
    process: fromReasoning.process || '',
    fill: fromReasoning.fill || '',
  }
})

const previewText = computed(() => {
  const s = sections.value
  const parts = [s.market, s.outcome, s.sources].filter(Boolean)
  if (parts.length) return parts.join(' · ')
  return props.row.reasoning || ''
})
</script>

<style scoped>
.line-clamp-2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>

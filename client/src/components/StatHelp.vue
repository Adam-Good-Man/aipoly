<template>
  <el-popover placement="top-end" :width="300" trigger="click" :teleported="true">
    <template #reference>
      <button
        type="button"
        class="stat-help absolute top-3 right-3 z-10 inline-flex items-center justify-center w-5 h-5 rounded-full border border-slate-300 text-slate-400 hover:text-slate-700 hover:border-slate-500 bg-white/90 transition-colors"
        aria-label="说明"
        @click.stop
      >
        <span class="text-[11px] font-semibold leading-none">?</span>
      </button>
    </template>
    <p class="text-sm text-slate-600 leading-relaxed whitespace-pre-line">{{ text }}</p>
  </el-popover>
</template>

<script setup>
defineProps({
  text: { type: String, required: true },
})
</script>

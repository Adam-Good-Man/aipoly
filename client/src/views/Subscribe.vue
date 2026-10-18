<template>
  <section class="relative overflow-hidden min-h-[calc(100vh-8rem)] bg-[#eef1ef]">
    <div class="subscribe-wash absolute inset-0 pointer-events-none" aria-hidden="true" />

    <div class="relative z-10 max-w-6xl mx-auto px-4 sm:px-5 pt-10 sm:pt-20 pb-16 sm:pb-20">
      <div
        class="max-w-2xl"
        v-motion
        :initial="{ opacity: 0, y: 20 }"
        :enter="{ opacity: 1, y: 0, transition: { duration: 550 } }"
      >
        <h1 class="font-display text-3xl sm:text-5xl text-[#14201a] tracking-tight">{{ t('subscribe.title') }}</h1>
        <p class="mt-4 text-stone-600 text-base sm:text-lg leading-relaxed">
          {{ t('subscribe.body') }}
        </p>
      </div>

      <div class="mt-8 sm:mt-12 grid gap-4 sm:gap-5 md:grid-cols-3">
        <article
          v-for="(plan, i) in plans"
          :key="plan.id"
          class="plan-card flex flex-col p-6"
          :class="plan.featured ? 'plan-featured' : ''"
          v-motion
          :initial="{ opacity: 0, y: 24 }"
          :enter="{ opacity: 1, y: 0, transition: { duration: 500, delay: 80 + i * 80 } }"
        >
          <div class="flex items-baseline justify-between gap-2">
            <h2 class="font-display text-2xl text-[#14201a]">{{ plan.name }}</h2>
            <el-tag v-if="plan.featured" size="small" effect="dark" class="!bg-[#1f6b45] !border-0">
              {{ t('subscribe.recommended') }}
            </el-tag>
          </div>
          <p class="mt-3">
            <span class="font-display text-3xl text-[#1f6b45]">{{ plan.price }}</span>
            <span v-if="plan.period" class="text-stone-500 text-sm ml-1">{{ plan.period }}</span>
          </p>
          <ul class="mt-5 space-y-2 text-sm text-stone-600 flex-1">
            <li v-for="f in plan.features" :key="f" class="flex gap-2">
              <span class="text-[#1f6b45]">·</span>
              <span>{{ f }}</span>
            </li>
          </ul>
          <el-button
            class="mt-6 !h-11"
            :class="plan.featured
              ? '!bg-[#1f6b45] !border-[#1f6b45] !text-white !font-semibold'
              : '!bg-white !border-stone-300 !text-stone-800'"
            :loading="busyPlan === plan.id"
            @click="choosePlan(plan)"
          >
            {{ plan.cta }}
          </el-button>
        </article>
      </div>

      <p class="mt-10 text-xs text-stone-500 max-w-xl">
        {{ t('subscribe.note') }}
      </p>
      <div v-if="authenticated" class="mt-6">
        <el-button class="!bg-[#14201a] !border-[#14201a] !text-white" @click="goApp">
          {{ t('common.enterApp') }}
        </el-button>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { useWallet } from '../composables/useWallet'
import { useLocalePath } from '../composables/useLocalePath'

const router = useRouter()
const { t } = useI18n()
const { localePath } = useLocalePath()
const { authenticated } = useWallet()
const busyPlan = ref('')

const plans = computed(() => [
  {
    id: 'free',
    name: 'Free',
    price: '$0',
    period: '',
    featured: false,
    cta: t('subscribe.freeCta'),
    features: [t('subscribe.freeF1'), t('subscribe.freeF2'), t('subscribe.freeF3')],
  },
  {
    id: 'pro',
    name: 'Pro',
    price: '$29',
    period: '/mo',
    featured: true,
    cta: t('subscribe.proCta'),
    features: [t('subscribe.proF1'), t('subscribe.proF2'), t('subscribe.proF3')],
  },
  {
    id: 'elite',
    name: 'Elite',
    price: '$99',
    period: '/mo',
    featured: false,
    cta: t('subscribe.eliteCta'),
    features: [t('subscribe.eliteF1'), t('subscribe.eliteF2'), t('subscribe.eliteF3')],
  },
])

function choosePlan(plan) {
  if (!authenticated.value) {
    router.push({ path: localePath('/login'), query: { redirect: localePath('/subscribe') } })
    return
  }
  busyPlan.value = plan.id
  localStorage.setItem('aipoly_plan', plan.id === 'free' ? 'Free' : plan.name)
  if (plan.id === 'free') {
    busyPlan.value = ''
    router.push(localePath('/app'))
    return
  }
  ElMessage.info('Coming soon')
  busyPlan.value = ''
}

function goApp() {
  router.push(localePath('/app'))
}
</script>

<style scoped>
.subscribe-wash {
  background:
    radial-gradient(ellipse 60% 50% at 80% 0%, rgba(31, 107, 69, 0.1), transparent 55%),
    linear-gradient(180deg, #f5f7f5, #eef1ef);
}

.plan-card {
  background: #fff;
  border: 1px solid rgba(20, 32, 26, 0.1);
}

.plan-featured {
  border-color: rgba(31, 107, 69, 0.45);
  box-shadow: 0 16px 40px rgba(20, 32, 26, 0.08);
}
</style>

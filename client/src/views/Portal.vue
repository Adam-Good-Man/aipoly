<template>
  <div class="portal-page text-stone-700">
    <!-- Hero: true full-bleed carousel, copy on the image -->
    <section class="hero relative isolate overflow-hidden" data-nav-tone="dark">
      <div class="absolute inset-0 z-0">
        <Swiper
          class="hero-swiper h-full w-full"
          :modules="heroModules"
          :slides-per-view="1"
          :loop="true"
          :effect="'fade'"
          :fade-effect="{ crossFade: true }"
          :speed="900"
          :autoplay="{ delay: 4500, disableOnInteraction: false }"
          :allow-touch-move="true"
          :pagination="{ clickable: true }"
          @slide-change="onHeroSlide"
        >
          <SwiperSlide v-for="slide in heroSlides" :key="slide.src">
            <div class="hero-slide-inner">
              <img :src="slide.src" :alt="slide.alt" />
            </div>
          </SwiperSlide>
        </Swiper>
        <div class="hero-veil" aria-hidden="true" />
      </div>

      <div class="relative z-10 max-w-6xl mx-auto px-4 sm:px-5 hero-copy flex items-end sm:items-center">
        <div
          class="max-w-xl w-full pb-6 sm:pb-0"
          v-motion
          :initial="{ opacity: 0, y: 28 }"
          :enter="{ opacity: 1, y: 0, transition: { duration: 700, ease: 'easeOut' } }"
        >
          <p class="text-[0.65rem] sm:text-xs tracking-[0.28em] sm:tracking-[0.32em] uppercase text-[#9bc49a] mb-2 sm:mb-4">
            {{ t('portal.eyebrow') }}
          </p>
          <h1 class="font-display text-[clamp(2.8rem,12vw,6.5rem)] leading-[0.9] tracking-tight text-[#f2f7f3] drop-shadow-[0_2px_24px_rgba(0,0,0,0.45)]">
            aiPoly
          </h1>
          <p class="mt-3 sm:mt-6 text-[0.95rem] sm:text-lg text-stone-100/90 leading-relaxed max-w-md drop-shadow-[0_1px_12px_rgba(0,0,0,0.4)]">
            {{ t('portal.subtitle') }}
          </p>
          <div class="mt-5 sm:mt-8 flex flex-col sm:flex-row flex-wrap gap-3 w-full sm:w-auto">
            <el-button
              size="large"
              class="!h-11 sm:!h-12 !px-6 sm:!px-7 !w-full sm:!w-auto !bg-[#c8e6c0] !border-[#c8e6c0] !text-[#0c1210] !font-semibold"
              :loading="connecting || signing"
              @click="primaryCta"
            >
              {{ primaryLabel }}
            </el-button>
            <el-button
              size="large"
              class="!h-11 sm:!h-12 !px-6 !w-full sm:!w-auto !bg-white/10 !border-white/40 !text-white backdrop-blur-sm !m-0"
              @click="scrollTo('how')"
            >
              {{ t('portal.seeHow') }}
            </el-button>
          </div>
          <p class="mt-5 sm:mt-8 text-xs sm:text-sm text-stone-200/85">
            {{ heroSlides[heroIndex]?.caption || t('portal.captionFallback') }}
          </p>
        </div>
      </div>
    </section>

    <!-- Marquee — continuous ticker, always fills the bar -->
    <div class="marquee-bar" aria-hidden="true" data-nav-tone="dark">
      <div class="marquee-viewport">
        <div class="marquee-track">
          <div v-for="n in 6" :key="n" class="marquee-group">
            <span v-for="(item, i) in marqueeItems" :key="`${n}-${i}`" class="marquee-item">
              <span class="marquee-spark" aria-hidden="true">✦</span>
              <span>{{ item }}</span>
              <span class="marquee-dot">·</span>
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- Product: image as section atmosphere + icon points -->
    <section class="relative overflow-hidden" data-nav-tone="dark">
      <div class="absolute inset-0 product-band" aria-hidden="true">
        <img src="/marketing/portal-terminal.png" alt="" class="product-band-img" />
        <div class="product-band-mask" />
      </div>
      <div class="relative z-10 max-w-6xl mx-auto px-4 sm:px-5 py-14 sm:py-20 lg:py-28 grid lg:grid-cols-2 gap-10 lg:gap-12 items-center">
        <div
          v-motion
          :initial="{ opacity: 0, y: 28 }"
          :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 550 } }"
        >
          <p class="eyebrow eyebrow--on-dark">{{ t('portal.productEyebrow') }}</p>
          <h2 class="section-title section-title--light">{{ t('portal.productTitle') }}</h2>
          <p class="mt-5 text-stone-200 leading-relaxed max-w-md">
            {{ t('portal.productBody') }}
          </p>
          <ul class="mt-8 space-y-3">
            <li
              v-for="(item, i) in productPoints"
              :key="item.text"
              class="product-point"
              v-motion
              :initial="{ opacity: 0, x: -12 }"
              :visibleOnce="{ opacity: 1, x: 0, transition: { duration: 400, delay: 120 + i * 90 } }"
            >
              <span class="product-point-icon" v-html="item.icon" aria-hidden="true" />
              <span class="leading-relaxed text-sm text-[#e8f0e9]">{{ item.text }}</span>
            </li>
          </ul>
        </div>
        <div
          class="product-float"
          v-motion
          :initial="{ opacity: 0, scale: 0.94, y: 16 }"
          :visibleOnce="{ opacity: 1, scale: 1, y: 0, transition: { duration: 650, delay: 100 } }"
        >
          <img src="/marketing/portal-ui-dashboard.png" alt="" class="product-float-main" />
          <img src="/marketing/portal-ui-reason.png" alt="" class="product-float-card" />
          <div class="product-float-badge" aria-hidden="true">
            <span class="pulse-dot" />
            AI live
          </div>
        </div>
      </div>
    </section>

    <!-- Features as editorial strips (image = stage, not thumbnail card) -->
    <section class="portal-section bg-[#f7f9f7]" data-nav-tone="light">
      <div class="max-w-6xl mx-auto px-4 sm:px-5">
        <div
          class="max-w-2xl mb-8 sm:mb-12"
          v-motion
          :initial="{ opacity: 0, y: 20 }"
          :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 500 } }"
        >
          <p class="eyebrow">{{ t('portal.featuresEyebrow') }}</p>
          <h2 class="section-title">{{ t('portal.featuresTitle') }}</h2>
        </div>

        <div class="space-y-10 lg:space-y-14">
          <article
            v-for="(f, i) in features"
            :key="f.title"
            class="feature-strip"
            :class="i % 2 === 1 ? 'feature-strip--flip' : ''"
            v-motion
            :initial="{ opacity: 0, y: 32 }"
            :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 550, delay: i * 60 } }"
          >
            <div class="feature-copy">
              <span class="font-display text-4xl text-[#1f6b45]/35">{{ String(i + 1).padStart(2, '0') }}</span>
              <h3 class="mt-2 font-display text-2xl sm:text-3xl text-[#14201a]">{{ f.title }}</h3>
              <p class="mt-3 text-stone-600 leading-relaxed max-w-sm">{{ f.body }}</p>
            </div>
            <div class="feature-visual">
              <img :src="f.img" :alt="f.title" />
            </div>
          </article>
        </div>
      </div>
    </section>

    <!-- How it works: native steps — no empty PNG placeholders -->
    <section id="how" class="portal-section relative overflow-hidden" data-nav-tone="light">
      <div class="how-wash absolute inset-0 pointer-events-none" aria-hidden="true" />
      <div class="relative z-10 max-w-6xl mx-auto px-4 sm:px-5">
        <div class="flex flex-col lg:flex-row lg:items-end lg:justify-between gap-4 mb-8 sm:mb-12">
          <div>
            <p class="eyebrow">{{ t('portal.howEyebrow') }}</p>
            <h2 class="section-title max-w-xl">{{ t('portal.howTitle') }}</h2>
          </div>
          <p class="text-sm text-stone-500 max-w-xs lg:text-right">{{ t('portal.howHint') }}</p>
        </div>

        <div class="how-rail" aria-hidden="true">
          <div class="how-rail-line" />
        </div>

        <ol class="how-list grid sm:grid-cols-2 lg:grid-cols-4 gap-5 sm:gap-6 lg:gap-5">
          <li
            v-for="(step, i) in steps"
            :key="step.title"
            class="how-step"
            v-motion
            :initial="{ opacity: 0, y: 28 }"
            :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 500, delay: i * 100 } }"
          >
            <div class="how-head">
              <span class="how-num font-display">{{ String(i + 1).padStart(2, '0') }}</span>
              <div class="how-icon" v-html="step.icon" aria-hidden="true" />
              <h3 class="how-title">{{ step.title }}</h3>
            </div>
            <p class="how-desc">{{ step.desc }}</p>
            <p class="how-tag">{{ step.tag }}</p>
          </li>
        </ol>
      </div>
    </section>

    <!-- Algorithm -->
    <section class="portal-section bg-[#14201a] text-stone-200 relative overflow-hidden" data-nav-tone="dark">
      <div class="algo-glow absolute inset-0 pointer-events-none" aria-hidden="true" />
      <img
        class="algo-side-art absolute right-0 top-1/2 -translate-y-1/2 w-[42%] max-w-lg opacity-[0.22] pointer-events-none hidden lg:block"
        src="/marketing/portal-hero-neural.png"
        alt=""
        aria-hidden="true"
      />
      <div class="relative z-10 max-w-6xl mx-auto px-4 sm:px-5">
        <p class="text-xs tracking-[0.28em] uppercase text-[#8fbf9a] mb-3">{{ t('portal.algoEyebrow') }}</p>
        <h2 class="font-display text-[clamp(1.65rem,5vw,2.25rem)] sm:text-4xl text-white max-w-2xl leading-tight">
          {{ t('portal.algoTitle') }}
        </h2>
        <div class="mt-8 sm:mt-12 grid sm:grid-cols-2 lg:grid-cols-3 gap-5 sm:gap-6">
          <article
            v-for="(block, i) in algoBlocks"
            :key="block.title"
            class="algo-block"
            v-motion
            :initial="{ opacity: 0, y: 24 }"
            :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 500, delay: i * 90 } }"
          >
            <div class="algo-icon" v-html="block.icon" aria-hidden="true" />
            <h3 class="mt-4 text-white text-lg font-medium">{{ block.title }}</h3>
            <p class="mt-3 text-sm text-stone-400 leading-relaxed">{{ block.body }}</p>
            <ul class="mt-4 space-y-2 text-sm text-[#8fbf9a]">
              <li v-for="item in block.items" :key="item" class="algo-chip">{{ item }}</li>
            </ul>
          </article>
        </div>
        <div
          class="flow-strip mt-12 sm:mt-14"
          v-motion
          :initial="{ opacity: 0, y: 12 }"
          :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 500, delay: 200 } }"
        >
          <template v-for="(node, i) in flowNodes" :key="node">
            <span class="flow-node">
              <span class="flow-node-dot" aria-hidden="true" />
              {{ node }}
            </span>
            <span v-if="i < flowNodes.length - 1" class="flow-arrow" aria-hidden="true">→</span>
          </template>
        </div>
        <p class="mt-4 text-xs text-stone-500 max-w-2xl">
          {{ t('portal.algoRisk') }}
        </p>
      </div>
    </section>

    <!-- Samples: full-width grid with tone icons -->
    <section class="portal-section bg-[#f7f9f7]" data-nav-tone="light">
      <div class="max-w-6xl mx-auto px-4 sm:px-5">
        <div class="max-w-2xl mb-8 sm:mb-10">
          <p class="eyebrow">{{ t('portal.samplesEyebrow') }}</p>
          <h2 class="section-title">{{ t('portal.samplesTitle') }}</h2>
        </div>
        <div class="sample-grid">
          <article
            v-for="(s, i) in samples"
            :key="s.title"
            class="sample-panel"
            :class="`sample-panel--${s.tone}`"
            v-motion
            :initial="{ opacity: 0, y: 20 }"
            :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 450, delay: i * 80 } }"
          >
            <div class="sample-top">
              <span class="sample-tone-icon" v-html="s.icon" aria-hidden="true" />
              <div class="flex flex-wrap gap-x-3 gap-y-1 text-xs text-stone-500">
                <span>{{ s.meta }}</span>
                <span class="sample-side">{{ s.side }}</span>
                <span>{{ s.rate }}</span>
              </div>
            </div>
            <h3 class="mt-3 text-base font-medium text-[#14201a]">{{ s.title }}</h3>
            <p class="mt-4 text-sm text-stone-600 leading-relaxed flex-1">{{ s.reason }}</p>
            <p class="mt-5 text-xs text-stone-400">{{ t('portal.sampleDisclaimer') }}</p>
          </article>
        </div>
      </div>
    </section>

    <!-- Audience + pricing -->
    <section class="portal-section relative overflow-hidden" data-nav-tone="light">
      <div class="absolute inset-y-0 right-0 w-1/2 opacity-[0.08] pointer-events-none hidden lg:block" aria-hidden="true">
        <img src="/marketing/portal-hero-candles.png" alt="" class="w-full h-full object-cover" />
      </div>
      <div class="relative z-10 max-w-6xl mx-auto px-4 sm:px-5 grid lg:grid-cols-2 gap-10 lg:gap-14">
        <div
          v-motion
          :initial="{ opacity: 0, y: 20 }"
          :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 500 } }"
        >
          <p class="eyebrow">{{ t('portal.audienceEyebrow') }}</p>
          <h2 class="section-title">{{ t('portal.audienceTitle') }}</h2>
          <ul class="mt-7 space-y-3">
            <li
              v-for="(a, i) in audience"
              :key="a.title"
              class="audience-row"
              v-motion
              :initial="{ opacity: 0, x: -10 }"
              :visibleOnce="{ opacity: 1, x: 0, transition: { duration: 400, delay: i * 80 } }"
            >
              <span class="audience-icon" v-html="a.icon" aria-hidden="true" />
              <div>
                <strong class="text-[#14201a] font-medium text-sm">{{ a.title }}</strong>
                <p class="mt-1 text-stone-600 text-sm leading-relaxed">{{ a.body }}</p>
              </div>
            </li>
          </ul>
        </div>
        <div
          v-motion
          :initial="{ opacity: 0, y: 20 }"
          :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 500, delay: 80 } }"
        >
          <p class="eyebrow">{{ t('portal.payEyebrow') }}</p>
          <h2 class="section-title">{{ t('portal.payTitle') }}</h2>
          <p class="mt-5 text-stone-600 text-sm leading-relaxed">
            {{ t('portal.payBody') }}
          </p>
          <ul class="price-list mt-7">
            <li
              v-for="(p, i) in priceTeaser"
              :key="p.name"
              class="price-row"
              :class="{ 'price-row--featured': p.featured }"
              v-motion
              :initial="{ opacity: 0, y: 12 }"
              :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 400, delay: 100 + i * 70 } }"
            >
              <div class="price-row-main">
                <span class="price-icon" aria-hidden="true">{{ p.icon }}</span>
                <span class="price-name">{{ p.name }}</span>
                <span v-if="p.featured" class="price-badge">{{ t('portal.recommended') }}</span>
              </div>
              <div class="price-amount font-display">
                {{ p.price }}<span v-if="p.period" class="price-period">{{ p.period }}</span>
              </div>
              <p class="price-note">{{ p.note }}</p>
            </li>
          </ul>
          <div class="mt-7 flex flex-col sm:flex-row flex-wrap gap-3">
            <el-button
              class="!h-11 !px-6 !w-full sm:!w-auto !bg-[#1f6b45] !border-[#1f6b45] !text-white !font-semibold"
              @click="goSubscribe"
            >
              {{ t('portal.viewPlans') }}
            </el-button>
            <el-button
              class="!h-11 !px-5 !w-full sm:!w-auto !bg-white !border-stone-300 !text-stone-800 !m-0"
              :loading="connecting || signing"
              @click="primaryCta"
            >
              {{ primaryLabel }}
            </el-button>
          </div>
        </div>
      </div>
    </section>

    <!-- FAQ -->
    <section class="portal-section bg-[#f7f9f7] border-t border-stone-200/80" data-nav-tone="light">
      <div class="max-w-3xl mx-auto px-4 sm:px-5">
        <div
          class="faq-head"
          v-motion
          :initial="{ opacity: 0, y: 16 }"
          :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 450 } }"
        >
          <span class="faq-mark" aria-hidden="true">?</span>
          <p class="eyebrow text-center">{{ t('portal.faqEyebrow') }}</p>
          <h2 class="section-title text-center mx-auto">{{ t('portal.faqTitle') }}</h2>
        </div>
        <div class="mt-8 sm:mt-10 space-y-2">
          <details
            v-for="(faq, i) in faqs"
            :key="faq.q"
            class="faq-item"
            v-motion
            :initial="{ opacity: 0, y: 10 }"
            :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 350, delay: i * 50 } }"
          >
            <summary>
              <span class="faq-q-icon" aria-hidden="true">+</span>
              {{ faq.q }}
            </summary>
            <p>{{ faq.a }}</p>
          </details>
        </div>
      </div>
    </section>

    <!-- Closing -->
    <section class="portal-section relative overflow-hidden" data-nav-tone="dark">
      <div class="absolute inset-0 closing-band" aria-hidden="true">
        <img src="/marketing/portal-hero-atmosphere.png" alt="" />
        <div class="closing-band-mask" />
      </div>
      <div
        class="relative z-10 max-w-3xl mx-auto px-4 sm:px-5 text-center py-6 sm:py-8"
        v-motion
        :initial="{ opacity: 0, y: 24 }"
        :visibleOnce="{ opacity: 1, y: 0, transition: { duration: 550 } }"
      >
        <h2 class="font-display text-[clamp(1.75rem,6vw,3rem)] sm:text-5xl text-white leading-tight">
          {{ t('portal.closeTitle') }}
        </h2>
        <p class="mt-4 text-sm sm:text-base text-stone-300 leading-relaxed">
          {{ t('portal.closeBody') }}
        </p>
        <div class="mt-8 flex flex-col sm:flex-row flex-wrap justify-center gap-3">
          <el-button
            size="large"
            class="!h-11 sm:!h-12 !px-8 !w-full sm:!w-auto !bg-[#c8e6c0] !border-[#c8e6c0] !text-[#0c1210] !font-semibold"
            :loading="connecting || signing"
            @click="primaryCta"
          >
            {{ primaryLabel }}
          </el-button>
          <el-button
            size="large"
            class="!h-11 sm:!h-12 !px-6 !w-full sm:!w-auto !bg-transparent !border-white/40 !text-white !m-0"
            @click="goSubscribe"
          >
            {{ t('portal.closePlans') }}
          </el-button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Swiper, SwiperSlide } from 'swiper/vue'
import { Autoplay, Pagination, EffectFade } from 'swiper/modules'
import 'swiper/css'
import 'swiper/css/pagination'
import 'swiper/css/effect-fade'
import { useWallet } from '../composables/useWallet'
import { useLocalePath } from '../composables/useLocalePath'

const router = useRouter()
const { t, tm, locale } = useI18n()
const { localePath } = useLocalePath()
const {
  authenticated,
  connected,
  signed,
  connecting,
  signing,
  hasInjectedWallet,
  connectInjected,
  connectWalletConnect,
  signIn,
} = useWallet()

const heroModules = [Autoplay, EffectFade, Pagination]
const heroIndex = ref(0)

function onHeroSlide(swiper) {
  heroIndex.value = swiper.realIndex ?? swiper.activeIndex ?? 0
}

const heroSlides = computed(() => [
  { src: '/marketing/portal-hero-atmosphere.png', alt: '', caption: t('portal.heroCap4') },
  { src: '/marketing/portal-hero-candles.png', alt: '', caption: t('portal.heroCap2') },
  { src: '/marketing/portal-hero-curve.png', alt: '', caption: t('portal.heroCap1') },
  { src: '/marketing/portal-hero-neural.png', alt: '', caption: t('portal.heroCap3') },
  { src: '/marketing/portal-terminal.png', alt: '', caption: t('portal.heroCap2') },
])

const marqueeItems = computed(() => {
  const raw = tm('portal.marquee')
  return Array.isArray(raw) ? raw.filter(Boolean) : []
})

const iconScan = `<svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg"><circle cx="32" cy="32" r="18" stroke="currentColor" stroke-width="1.5" opacity=".35"/><path d="M32 14v4M32 46v4M14 32h4M46 32h4" stroke="currentColor" stroke-width="1.5"/><circle cx="32" cy="32" r="8" stroke="currentColor" stroke-width="1.75"/><path d="M38 38l8 8" stroke="currentColor" stroke-width="1.75" stroke-linecap="round"/></svg>`
const iconBrain = `<svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M28 18c-6 0-10 4-10 10 0 2-.5 4-2 5.5C14 35 13 38 15 42c2 4 6 6 11 6h4M36 18c6 0 10 4 10 10 0 2 .5 4 2 5.5C50 35 51 38 49 42c-2 4-6 6-11 6h-4" stroke="currentColor" stroke-width="1.75" stroke-linecap="round"/><path d="M32 18v30M28 28h8M26 36h12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" opacity=".7"/></svg>`
const iconTrade = `<svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M14 44V34M24 44V24M34 44V30M44 44V18" stroke="currentColor" stroke-width="3" stroke-linecap="round"/><path d="M12 48h40" stroke="currentColor" stroke-width="1.5" opacity=".4"/><path d="M40 16l8 2-2 8" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"/></svg>`
const iconShield = `<svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M32 12l18 6v12c0 12-8 20-18 24-10-4-18-12-18-24V18l18-6z" stroke="currentColor" stroke-width="1.75" stroke-linejoin="round"/><path d="M22 34l7 7 13-14" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"/></svg>`
const iconDoc = `<svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M20 12h18l10 10v30H20V12z" stroke="currentColor" stroke-width="1.75" stroke-linejoin="round"/><path d="M38 12v10h10M26 30h12M26 38h16M26 46h10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>`
const iconWallet = `<svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg"><rect x="12" y="18" width="40" height="30" rx="4" stroke="currentColor" stroke-width="1.75"/><path d="M12 26h40M40 36h8" stroke="currentColor" stroke-width="1.75" stroke-linecap="round"/><circle cx="44" cy="36" r="2" fill="currentColor"/></svg>`
const iconChart = `<svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M14 46V28M26 46V18M38 46V32M50 46V22" stroke="currentColor" stroke-width="3" stroke-linecap="round"/><path d="M12 50h40" stroke="currentColor" stroke-width="1.5" opacity=".4"/></svg>`
const iconNews = `<svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg"><rect x="14" y="14" width="36" height="36" rx="3" stroke="currentColor" stroke-width="1.75"/><path d="M22 24h20M22 32h20M22 40h12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>`
const iconList = `<svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M24 20h24M24 32h24M24 44h16" stroke="currentColor" stroke-width="1.75" stroke-linecap="round"/><circle cx="16" cy="20" r="2.5" fill="currentColor"/><circle cx="16" cy="32" r="2.5" fill="currentColor"/><circle cx="16" cy="44" r="2.5" fill="currentColor"/></svg>`
const iconBuy = `<svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M32 46V18M22 28l10-10 10 10" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M16 48h32" stroke="currentColor" stroke-width="1.5" opacity=".4"/></svg>`
const iconSkip = `<svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg"><circle cx="32" cy="32" r="18" stroke="currentColor" stroke-width="1.75"/><path d="M22 22l20 20M42 22L22 42" stroke="currentColor" stroke-width="1.75" stroke-linecap="round"/></svg>`

const productPoints = computed(() => [
  { text: t('portal.productPoint1'), icon: iconScan },
  { text: t('portal.productPoint2'), icon: iconDoc },
  { text: t('portal.productPoint3'), icon: iconWallet },
])

const features = computed(() => [
  { title: t('portal.feature1Title'), body: t('portal.feature1Body'), img: '/marketing/portal-ui-dashboard.png' },
  { title: t('portal.feature2Title'), body: t('portal.feature2Body'), img: '/marketing/portal-ui-reason.png' },
  { title: t('portal.feature3Title'), body: t('portal.feature3Body'), img: '/marketing/portal-ui-wallet.png' },
])

const steps = computed(() => [
  { title: t('portal.step1Title'), desc: t('portal.step1Desc'), tag: t('portal.step1Tag'), icon: iconScan },
  { title: t('portal.step2Title'), desc: t('portal.step2Desc'), tag: t('portal.step2Tag'), icon: iconBrain },
  { title: t('portal.step3Title'), desc: t('portal.step3Desc'), tag: t('portal.step3Tag'), icon: iconTrade },
  { title: t('portal.step4Title'), desc: t('portal.step4Desc'), tag: t('portal.step4Tag'), icon: iconShield },
])

const algoBlocks = computed(() => [
  { title: t('portal.algo1Title'), body: t('portal.algo1Body'), items: ['PriceHistory', 'OrderBook', 'RecentTrades'], icon: iconChart },
  { title: t('portal.algo2Title'), body: t('portal.algo2Body'), items: ['RSSHub', 'IntelGaps'], icon: iconNews },
  { title: t('portal.algo3Title'), body: t('portal.algo3Body'), items: ['MIN_WIN_RATE', 'STOP_LOSS_STRATEGY'], icon: iconShield },
])

const flowNodes = computed(() => (
  locale.value === 'cn'
    ? ['市场池', '初筛', '深度分析', '胜率门槛', '下单 / 跳过']
    : ['Pool', 'Screen', 'Analyze', 'Gate', 'Trade / Skip']
))

const samples = computed(() => [
  {
    meta: t('portal.sample1Meta'),
    side: t('portal.sample1Side'),
    rate: t('portal.sample1Rate'),
    title: t('portal.sample1Title'),
    reason: t('portal.sample1Reason'),
    tone: 'buy',
    icon: iconBuy,
  },
  {
    meta: t('portal.sample2Meta'),
    side: t('portal.sample2Side'),
    rate: t('portal.sample2Rate'),
    title: t('portal.sample2Title'),
    reason: t('portal.sample2Reason'),
    tone: 'skip',
    icon: iconSkip,
  },
  {
    meta: t('portal.sample3Meta'),
    side: t('portal.sample3Side'),
    rate: t('portal.sample3Rate'),
    title: t('portal.sample3Title'),
    reason: t('portal.sample3Reason'),
    tone: 'buy',
    icon: iconBuy,
  },
])

const audience = computed(() => [
  { title: t('portal.audience1Title'), body: t('portal.audience1Body'), icon: iconScan },
  { title: t('portal.audience2Title'), body: t('portal.audience2Body'), icon: iconList },
  { title: t('portal.audience3Title'), body: t('portal.audience3Body'), icon: iconShield },
])

const priceTeaser = computed(() => [
  { name: 'Free', price: '$0', period: '', featured: false, note: t('portal.payNoteFree'), icon: '◇' },
  { name: 'Pro', price: '$29', period: '/mo', featured: true, note: t('portal.payNotePro'), icon: '◆' },
  { name: 'Elite', price: '$99', period: '/mo', featured: false, note: t('portal.payNoteElite'), icon: '✦' },
])

const faqs = computed(() => [
  { q: t('portal.faq1Q'), a: t('portal.faq1A') },
  { q: t('portal.faq2Q'), a: t('portal.faq2A') },
  { q: t('portal.faq3Q'), a: t('portal.faq3A') },
  { q: t('portal.faq4Q'), a: t('portal.faq4A') },
])

const primaryLabel = computed(() => {
  if (authenticated.value) return t('common.enterApp')
  if (connected.value && !signed.value) return t('common.signIn')
  return t('common.connectWallet')
})

function scrollTo(id) {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function goSubscribe() {
  router.push(localePath('/subscribe'))
}

async function primaryCta() {
  if (authenticated.value) {
    router.push(localePath('/app'))
    return
  }
  if (connected.value && !signed.value) {
    try {
      await signIn()
      router.push(localePath('/app'))
    } catch (e) {
      ElMessage.error(e.message || 'Error')
    }
    return
  }
  try {
    if (hasInjectedWallet.value) await connectInjected()
    else await connectWalletConnect()
  } catch (e) {
    ElMessage.error(e.message || 'Error')
    router.push({ path: localePath('/login'), query: { redirect: localePath('/app') } })
  }
}
</script>

<style scoped>
.portal-page {
  background: #eef1ef;
  overflow-x: clip;
}

/* —— Hero: true full-viewport bleed —— */
.hero {
  min-height: 100dvh;
  min-height: 100svh;
}

.hero-copy {
  min-height: 100dvh;
  min-height: 100svh;
  padding-top: calc(4.5rem + env(safe-area-inset-top));
  padding-bottom: 4.5rem;
}

.hero-swiper,
.hero-slide-inner {
  height: 100%;
  width: 100%;
  min-height: 100%;
}

.hero-slide-inner {
  position: relative;
  background: #080c0a;
}

.hero-slide-inner img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center;
  display: block;
  animation: heroDrift 28s ease-in-out infinite alternate;
}

@media (prefers-reduced-motion: reduce) {
  .hero-slide-inner img,
  .marquee-track,
  .product-float-main,
  .product-float-card,
  .pulse-dot,
  .algo-icon,
  .how-icon,
  .faq-mark,
  .flow-node-dot {
    animation: none !important;
  }
  .marquee-track {
    transform: none !important;
  }
}

/* Soft readability veil — keep image visible edge-to-edge, no light split */
.hero-veil {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  background:
    linear-gradient(180deg, rgba(8, 12, 10, 0.45) 0%, rgba(8, 12, 10, 0.15) 35%, rgba(8, 12, 10, 0.55) 72%, rgba(8, 12, 10, 0.88) 100%),
    linear-gradient(90deg, rgba(8, 12, 10, 0.55) 0%, rgba(8, 12, 10, 0.2) 42%, rgba(8, 12, 10, 0.08) 100%);
}

.hero :deep(.swiper-pagination) {
  bottom: max(1.25rem, env(safe-area-inset-bottom));
  z-index: 2;
}

.hero :deep(.swiper-pagination-bullet) {
  background: #fff;
  opacity: 0.35;
}

.hero :deep(.swiper-pagination-bullet-active) {
  opacity: 1;
  background: #c8e6c0;
}

@keyframes heroDrift {
  from { transform: scale(1.04); }
  to { transform: scale(1.12) translate(-1%, 0.6%); }
}

.marquee-bar {
  background: #0a100e;
  border-top: 1px solid rgba(200, 230, 192, 0.1);
  border-bottom: 1px solid rgba(200, 230, 192, 0.1);
  overflow: hidden;
}

.marquee-viewport {
  overflow: hidden;
  width: 100%;
  mask-image: linear-gradient(90deg, transparent, #000 4%, #000 96%, transparent);
  -webkit-mask-image: linear-gradient(90deg, transparent, #000 4%, #000 96%, transparent);
}

.marquee-track {
  display: flex;
  width: max-content;
  animation: marquee 48s linear infinite;
  will-change: transform;
}

.marquee-group {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 2.5rem;
  padding: 0.9rem 2.5rem 0.9rem 0;
  white-space: nowrap;
}

.marquee-item {
  display: inline-flex;
  align-items: center;
  gap: 0.65rem;
  font-size: 0.875rem;
  color: #a8b5ad;
  flex-shrink: 0;
}

.marquee-spark {
  color: rgba(143, 191, 154, 0.85);
  font-size: 0.65rem;
}

.marquee-dot {
  color: rgba(143, 191, 154, 0.7);
  margin-left: 1.85rem;
}

@keyframes marquee {
  from { transform: translateX(0); }
  to { transform: translateX(-16.6667%); } /* 6 identical groups */
}

.portal-section {
  padding: 3.25rem 0;
}

@media (min-width: 640px) {
  .portal-section {
    padding: 5rem 0;
  }
}

@media (min-width: 1024px) {
  .portal-section {
    padding: 6rem 0;
  }
}

.eyebrow {
  font-size: 0.65rem;
  letter-spacing: 0.24em;
  text-transform: uppercase;
  color: #1f6b45;
  margin-bottom: 0.75rem;
}

.eyebrow--on-dark {
  color: #c8e6c0;
}

@media (min-width: 640px) {
  .eyebrow {
    font-size: 0.7rem;
    letter-spacing: 0.28em;
  }
}

.section-title {
  font-family: var(--font-display);
  font-size: clamp(1.55rem, 5.5vw, 2.45rem);
  line-height: 1.15;
  color: #14201a;
  max-width: 32rem;
}

.section-title--light {
  color: #f2f7f3;
}

/* Product band: photo as environment */
.product-band-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center right;
  transform: scale(1.05);
}

.product-band-mask {
  position: absolute;
  inset: 0;
  background:
    linear-gradient(180deg, rgba(10, 16, 14, 0.88) 0%, rgba(10, 16, 14, 0.78) 100%),
    linear-gradient(90deg, rgba(10, 16, 14, 0.9) 0%, rgba(10, 16, 14, 0.55) 100%);
}

@media (min-width: 1024px) {
  .product-band-mask {
    background:
      linear-gradient(90deg, rgba(10, 16, 14, 0.94) 0%, rgba(10, 16, 14, 0.82) 42%, rgba(10, 16, 14, 0.35) 100%),
      linear-gradient(180deg, rgba(10, 16, 14, 0.2), rgba(10, 16, 14, 0.55));
  }
}

.product-point {
  display: flex;
  align-items: flex-start;
  gap: 0.85rem;
  padding: 0.75rem 0.85rem;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(200, 230, 192, 0.12);
}

.product-point-icon {
  width: 1.75rem;
  height: 1.75rem;
  color: #c8e6c0;
  flex-shrink: 0;
  margin-top: 0.1rem;
}

.product-point-icon :deep(svg) {
  width: 100%;
  height: 100%;
}

.product-float {
  position: relative;
  min-height: 16rem;
}

.product-float-main {
  width: 100%;
  display: block;
  border: 1px solid rgba(200, 230, 192, 0.15);
  box-shadow: 0 24px 50px rgba(0, 0, 0, 0.35);
  animation: floatY 7s ease-in-out infinite;
}

.product-float-card {
  position: absolute;
  width: 46%;
  right: -4%;
  bottom: -8%;
  border: 1px solid rgba(200, 230, 192, 0.2);
  box-shadow: 0 16px 36px rgba(0, 0, 0, 0.4);
  animation: floatY 8.5s ease-in-out infinite reverse;
}

.product-float-badge {
  position: absolute;
  left: 0.75rem;
  top: 0.75rem;
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.35rem 0.65rem;
  font-size: 0.7rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: #c8e6c0;
  background: rgba(10, 16, 14, 0.72);
  border: 1px solid rgba(200, 230, 192, 0.25);
  backdrop-filter: blur(6px);
}

.pulse-dot {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 50%;
  background: #6fdc8c;
  box-shadow: 0 0 0 0 rgba(111, 220, 140, 0.55);
  animation: pulseDot 1.8s ease-out infinite;
}

@keyframes floatY {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-8px); }
}

@keyframes pulseDot {
  0% { box-shadow: 0 0 0 0 rgba(111, 220, 140, 0.55); }
  70% { box-shadow: 0 0 0 8px rgba(111, 220, 140, 0); }
  100% { box-shadow: 0 0 0 0 rgba(111, 220, 140, 0); }
}

@media (max-width: 1023px) {
  .product-float {
    margin-top: 0.5rem;
  }
  .product-float-card {
    width: 42%;
    right: 0;
    bottom: -6%;
  }
}

/* Feature strips: image + copy as one composition */
.feature-strip {
  display: grid;
  gap: 1.25rem;
  align-items: center;
}

@media (min-width: 768px) {
  .feature-strip {
    grid-template-columns: 0.9fr 1.1fr;
    gap: 2.5rem;
  }
  .feature-strip--flip .feature-copy {
    order: 2;
  }
  .feature-strip--flip .feature-visual {
    order: 1;
  }
}

.feature-visual {
  position: relative;
  overflow: hidden;
  aspect-ratio: 16 / 10;
  background: #dfe6e1;
  mask-image: linear-gradient(180deg, #000 70%, transparent 100%);
  -webkit-mask-image: linear-gradient(180deg, #000 70%, transparent 100%);
}

.feature-visual img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: top center;
  display: block;
}

.feature-strip--flip .feature-visual {
  mask-image: linear-gradient(180deg, #000 70%, transparent 100%);
}

.how-wash {
  background:
    radial-gradient(ellipse 60% 50% at 50% 0%, rgba(31, 107, 69, 0.08), transparent 65%);
}

.how-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.how-step {
  position: relative;
  padding-top: 0;
  border-top: 1px solid rgba(31, 107, 69, 0.2);
  padding-top: 1rem;
}

.how-head {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-width: 0;
}

.how-num {
  flex: none;
  font-size: 1.15rem;
  line-height: 1;
  color: #1f6b45;
  letter-spacing: 0.02em;
  min-width: 1.6rem;
}

.how-icon {
  flex: none;
  width: 1.75rem;
  height: 1.75rem;
  color: #1f6b45;
  margin: 0;
  animation: iconPulse 4.5s ease-in-out infinite;
}

.how-icon :deep(svg) {
  width: 100%;
  height: 100%;
  display: block;
}

.how-title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 550;
  line-height: 1.25;
  color: #14201a;
  min-width: 0;
}

.how-desc {
  margin: 0.65rem 0 0;
  font-size: 0.875rem;
  line-height: 1.6;
  color: #57534e;
}

.how-tag {
  margin: 0.7rem 0 0;
  font-size: 0.7rem;
  letter-spacing: 0.06em;
  color: #1f6b45;
}

@media (min-width: 640px) {
  .how-num {
    font-size: 1.25rem;
  }
  .how-icon {
    width: 1.9rem;
    height: 1.9rem;
  }
  .how-title {
    font-size: 1.1rem;
  }
}

@media (min-width: 1024px) {
  .how-step {
    border-top-width: 2px;
    border-top-color: #1f6b45;
  }
}

.how-rail {
  display: none;
}

@media (min-width: 1024px) {
  .how-rail {
    display: block;
    position: relative;
    height: 1px;
    margin: 0 1.5rem 1.75rem;
  }
  .how-rail-line {
    height: 1px;
    background: linear-gradient(90deg, transparent, rgba(31, 107, 69, 0.35), transparent);
  }
}

.algo-glow {
  background: radial-gradient(ellipse 50% 40% at 80% 20%, rgba(31, 107, 69, 0.2), transparent 60%);
}

.algo-block {
  border: 1px solid rgba(200, 230, 192, 0.12);
  background: rgba(255, 255, 255, 0.03);
  padding: 1.25rem 1.15rem 1.35rem;
  transition: border-color 0.25s ease, transform 0.25s ease;
}

.algo-block:hover {
  border-color: rgba(200, 230, 192, 0.28);
  transform: translateY(-2px);
}

.algo-icon {
  width: 2.5rem;
  height: 2.5rem;
  color: #8fbf9a;
  animation: iconPulse 4s ease-in-out infinite;
}

.algo-icon :deep(svg) {
  width: 100%;
  height: 100%;
}

.algo-chip {
  display: inline-block;
  margin-right: 0.4rem;
  margin-bottom: 0.35rem;
  padding: 0.2rem 0.5rem;
  border: 1px solid rgba(143, 191, 154, 0.28);
  background: rgba(143, 191, 154, 0.08);
  font-size: 0.75rem;
}

.flow-strip {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.45rem 0.55rem;
}

.flow-node {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  border: 1px solid rgba(200, 230, 192, 0.25);
  padding: 0.4rem 0.7rem;
  background: rgba(200, 230, 192, 0.06);
  font-size: 0.8rem;
  color: #c8e6c0;
}

.flow-node-dot {
  width: 0.35rem;
  height: 0.35rem;
  border-radius: 50%;
  background: #8fbf9a;
  animation: pulseDot 2.2s ease-out infinite;
}

.flow-arrow {
  color: rgba(143, 191, 154, 0.55);
}

@keyframes iconPulse {
  0%, 100% { opacity: 0.85; transform: scale(1); }
  50% { opacity: 1; transform: scale(1.04); }
}

.sample-grid {
  display: grid;
  gap: 1.25rem;
}

@media (min-width: 768px) {
  .sample-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 1.5rem;
  }
}

.sample-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 1.25rem 1.1rem 1.15rem;
  border-top: 2px solid #1f6b45;
  background: rgba(255, 255, 255, 0.72);
  border-right: 1px solid rgba(20, 32, 26, 0.06);
  border-bottom: 1px solid rgba(20, 32, 26, 0.06);
  border-left: 1px solid rgba(20, 32, 26, 0.06);
  transition: transform 0.25s ease, box-shadow 0.25s ease;
}

.sample-panel:hover {
  transform: translateY(-3px);
  box-shadow: 0 12px 28px rgba(20, 32, 26, 0.08);
}

.sample-panel--skip {
  border-top-color: #8a7a55;
}

.sample-top {
  display: flex;
  align-items: flex-start;
  gap: 0.65rem;
}

.sample-tone-icon {
  width: 1.75rem;
  height: 1.75rem;
  color: #1f6b45;
  flex-shrink: 0;
}

.sample-panel--skip .sample-tone-icon {
  color: #8a7a55;
}

.sample-tone-icon :deep(svg) {
  width: 100%;
  height: 100%;
}

.sample-side {
  color: #1f6b45;
  font-weight: 500;
}

.sample-panel--skip .sample-side {
  color: #8a7a55;
}

@media (min-width: 640px) {
  .sample-panel {
    padding: 1.4rem 1.25rem 1.25rem;
  }
}

.audience-row {
  display: flex;
  gap: 0.9rem;
  align-items: flex-start;
  padding: 0.9rem 0.95rem;
  background: rgba(255, 255, 255, 0.7);
  border: 1px solid rgba(20, 32, 26, 0.08);
}

.audience-icon {
  width: 2rem;
  height: 2rem;
  color: #1f6b45;
  flex-shrink: 0;
}

.audience-icon :deep(svg) {
  width: 100%;
  height: 100%;
}

.price-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.price-row {
  display: grid;
  grid-template-columns: 1fr auto;
  grid-template-rows: auto auto;
  column-gap: 0.75rem;
  row-gap: 0.15rem;
  align-items: baseline;
  padding: 0.85rem 0;
  border-bottom: 1px solid rgba(20, 32, 26, 0.1);
}

.price-row:first-child {
  border-top: 1px solid rgba(20, 32, 26, 0.1);
}

.price-row--featured {
  background: rgba(31, 107, 69, 0.06);
  margin: 0 -0.65rem;
  padding-left: 0.65rem;
  padding-right: 0.65rem;
  border-bottom-color: rgba(31, 107, 69, 0.18);
}

.price-row-main {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  grid-column: 1;
  grid-row: 1;
}

.price-icon {
  color: #1f6b45;
  font-size: 0.85rem;
  line-height: 1;
}

.price-name {
  font-size: 0.95rem;
  font-weight: 600;
  color: #14201a;
}

.price-badge {
  font-size: 0.65rem;
  letter-spacing: 0.04em;
  color: #1f6b45;
  border: 1px solid rgba(31, 107, 69, 0.35);
  padding: 0.1rem 0.4rem;
  line-height: 1.4;
}

.price-amount {
  grid-column: 2;
  grid-row: 1 / span 2;
  align-self: center;
  font-size: 1.65rem;
  line-height: 1;
  color: #14201a;
  white-space: nowrap;
}

.price-row--featured .price-amount {
  color: #1f6b45;
}

.price-period {
  font-family: var(--font-sans);
  font-size: 0.75rem;
  color: #78716c;
  margin-left: 0.15rem;
}

.price-note {
  grid-column: 1;
  grid-row: 2;
  margin: 0;
  font-size: 0.75rem;
  color: #78716c;
  line-height: 1.4;
}

.faq-head {
  position: relative;
}

.faq-mark {
  display: block;
  margin: 0 auto 0.75rem;
  width: 2.75rem;
  height: 2.75rem;
  line-height: 2.75rem;
  text-align: center;
  font-family: var(--font-display);
  font-size: 1.6rem;
  color: #1f6b45;
  border: 1px solid rgba(31, 107, 69, 0.25);
  background: rgba(31, 107, 69, 0.06);
  animation: iconPulse 3.5s ease-in-out infinite;
}

.faq-item {
  border-bottom: 1px solid rgba(20, 32, 26, 0.1);
  padding: 0.95rem 0;
}

.faq-item summary {
  cursor: pointer;
  font-weight: 500;
  color: #14201a;
  list-style: none;
  display: flex;
  align-items: flex-start;
  gap: 0.65rem;
}

.faq-q-icon {
  flex-shrink: 0;
  width: 1.25rem;
  height: 1.25rem;
  margin-top: 0.1rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 0.85rem;
  color: #1f6b45;
  border: 1px solid rgba(31, 107, 69, 0.3);
  transition: transform 0.2s ease;
}

.faq-item[open] .faq-q-icon {
  transform: rotate(45deg);
}

.faq-item summary::-webkit-details-marker {
  display: none;
}

.faq-item[open] summary {
  color: #1f6b45;
}

.faq-item p {
  margin-top: 0.65rem;
  margin-left: 1.9rem;
  font-size: 0.875rem;
  line-height: 1.6;
  color: #57534e;
}

.closing-band img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.closing-band-mask {
  position: absolute;
  inset: 0;
  background: linear-gradient(180deg, rgba(10, 16, 14, 0.75), rgba(10, 16, 14, 0.88));
}
</style>

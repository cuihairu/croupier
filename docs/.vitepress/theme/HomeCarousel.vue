<!--
  首页界面轮播（用户令：dashboard 真实截图 + 手机原型图混排展示放文档首页，
  而非深页单独节）。当前素材：10 张真实运行栈截图（3200×1800，16:9 统一）；
  手机原型图仓库无资产（不生成代餐），素材到位后在 slides 数组追加即可。
  挂载点：theme/index.ts 经 home-features-before 插槽注入（hero 之下、特性之上）。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useData } from 'vitepress';

import shot01 from '../../screenshots/ui/01-login.png';
import shot02 from '../../screenshots/ui/02-console-home.png';
import shot03 from '../../screenshots/ui/03-functions.png';
import shot04a from '../../screenshots/ui/04a-invoke-form.png';
import shot04c from '../../screenshots/ui/04c-invoke-result.png';
import shot05 from '../../screenshots/ui/05-invoke-approval-required.png';
import shot06 from '../../screenshots/ui/06-approvals.png';
import shot07 from '../../screenshots/ui/07-approval-detail.png';
import shot08 from '../../screenshots/ui/08-ops-nodes.png';
import shot10 from '../../screenshots/ui/10-execution-logs.png';

interface Slide {
  src: string;
  caption: string;
}

const slides: Slide[] = [
  { src: shot01, caption: '登录' },
  { src: shot02, caption: '工作台总览' },
  { src: shot03, caption: '函数目录' },
  { src: shot04a, caption: '函数执行 · 参数表单' },
  { src: shot04c, caption: '函数执行 · 结果' },
  { src: shot05, caption: '高危操作 · 触发审批' },
  { src: shot06, caption: '审批列表' },
  { src: shot07, caption: '审批详情' },
  { src: shot08, caption: '节点维护' },
  { src: shot10, caption: '执行留痕' },
];

const AUTOPLAY_MS = 4500;

const index = ref(0);
const total = slides.length;
const paused = ref(false);
const current = computed(() => slides[index.value]);

let timer: ReturnType<typeof setInterval> | null = null;

function stopTimer(): void {
  if (timer !== null) {
    clearInterval(timer);
    timer = null;
  }
}

function go(next: number): void {
  index.value = ((next % total) + total) % total;
}

onMounted(() => {
  timer = setInterval(() => {
    if (!paused.value) go(index.value + 1);
  }, AUTOPLAY_MS);
});

onBeforeUnmount(stopTimer);

const { site } = useData();
const detailLink = `${site.value.base}guide/interface-preview`;
</script>

<template>
  <section
    class="home-carousel"
    aria-roledescription="轮播"
    aria-label="界面预览"
    @mouseenter="paused = true"
    @mouseleave="paused = false"
    @focusin="paused = true"
    @focusout="paused = false"
    @keydown.left="go(index - 1)"
    @keydown.right="go(index + 1)"
  >
    <header class="hc-head">
      <h2 class="hc-title">界面预览</h2>
      <a class="hc-more" :href="detailLink">分屏详解 →</a>
    </header>

    <div class="hc-frame">
      <Transition name="hc-fade" mode="out-in">
        <img
          :key="current.src"
          class="hc-img"
          :src="current.src"
          :alt="`Croupier 控制台：${current.caption}`"
          loading="lazy"
          decoding="async"
        />
      </Transition>

      <button
        type="button"
        class="hc-nav hc-prev"
        :aria-label="`上一张：${slides[(index - 1 + total) % total].caption}`"
        @click="go(index - 1)"
      >
        <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M14.5 5.5 8 12l6.5 6.5" />
        </svg>
      </button>
      <button
        type="button"
        class="hc-nav hc-next"
        :aria-label="`下一张：${slides[(index + 1) % total].caption}`"
        @click="go(index + 1)"
      >
        <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M9.5 5.5 16 12l-6.5 6.5" />
        </svg>
      </button>

      <span class="hc-caption">{{ current.caption }}</span>
    </div>

    <div class="hc-dots" role="tablist" :aria-label="`共 ${total} 张`">
      <button
        v-for="(slide, i) in slides"
        :key="slide.src"
        type="button"
        role="tab"
        :aria-selected="i === index"
        :aria-label="`第 ${i + 1} 张：${slide.caption}`"
        :class="['hc-dot', { active: i === index }]"
        @click="go(i)"
      />
    </div>
  </section>
</template>

<style scoped>
.home-carousel {
  max-width: 1152px;
  margin: 0 auto;
  padding: 0 24px 8px;
}

.hc-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 12px;
}

.hc-title {
  margin: 0;
  padding: 0;
  border: none;
  font-size: 20px;
  font-weight: 600;
  line-height: 1.4;
  color: var(--vp-c-text-1);
  letter-spacing: -0.02em;
}

.hc-more {
  font-size: 14px;
  font-weight: 500;
  color: var(--vp-c-brand-1);
  text-decoration: none;
}

.hc-more:hover {
  text-decoration: underline;
}

.hc-frame {
  position: relative;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  border-radius: 12px;
  border: 1px solid var(--vp-c-divider);
  background-color: var(--vp-c-bg-soft);
  box-shadow: var(--vp-shadow-1);
  outline: none;
}

.hc-img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.hc-nav {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 50%;
  background-color: var(--vp-button-alt-bg);
  color: var(--vp-c-text-1);
  cursor: pointer;
  opacity: 0;
  transition:
    opacity 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease;
}

.hc-prev {
  left: 12px;
}

.hc-next {
  right: 12px;
}

.hc-frame:hover .hc-nav,
.hc-nav:focus-visible {
  opacity: 1;
}

/* 触屏无 hover：箭头常显（ coarse 指针媒体查询） */
@media (pointer: coarse) {
  .hc-nav {
    opacity: 0.85;
  }
}

.hc-nav:hover {
  color: var(--vp-c-brand-1);
  border-color: var(--vp-c-brand-1);
}

.hc-nav:focus-visible {
  outline: 2px solid var(--vp-c-brand-1);
  outline-offset: 2px;
}

.hc-caption {
  position: absolute;
  left: 50%;
  bottom: 10px;
  transform: translateX(-50%);
  max-width: 80%;
  padding: 2px 12px;
  border-radius: 999px;
  font-size: 12.5px;
  line-height: 1.7;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: #fff;
  background-color: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(4px);
}

.hc-dots {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  margin-top: 10px;
}

.hc-dot {
  width: 8px;
  height: 8px;
  padding: 0;
  border: none;
  border-radius: 999px;
  background-color: var(--vp-c-text-3);
  opacity: 0.55;
  cursor: pointer;
  transition:
    width 0.2s ease,
    background-color 0.2s ease,
    opacity 0.2s ease;
}

.hc-dot:hover {
  opacity: 0.9;
}

.hc-dot.active {
  width: 20px;
  background-color: var(--vp-c-brand-1);
  opacity: 1;
}

.hc-dot:focus-visible {
  outline: 2px solid var(--vp-c-brand-1);
  outline-offset: 2px;
}

/* crossfade；reduced-motion 下免动画直接切换 */
.hc-fade-enter-active,
.hc-fade-leave-active {
  transition: opacity 0.25s ease;
}

.hc-fade-enter-from,
.hc-fade-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .hc-fade-enter-active,
  .hc-fade-leave-active {
    transition: none;
  }
}

@media (max-width: 768px) {
  .home-carousel {
    padding: 0 16px 4px;
  }

  .hc-nav {
    width: 32px;
    height: 32px;
  }
}
</style>

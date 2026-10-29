<script setup lang="ts">
import { withBase } from "vitepress";

const props = withDefaults(defineProps<{ lang?: "en" | "zh" }>(), {
  lang: "en",
});
</script>

<template>
  <figure class="architecture-diagram">
    <a
      :href="withBase(`/architecture/system-${props.lang}.svg`)"
      :aria-label="
        props.lang === 'zh'
          ? '打开完整 SVG 架构图'
          : 'Open the full SVG architecture diagram'
      "
    >
      <picture>
        <source
          media="(max-width: 768px)"
          :srcset="withBase(`/architecture/system-${props.lang}-mobile.svg`)"
        />
        <img
          :src="withBase(`/architecture/system-${props.lang}.svg`)"
          :alt="
            props.lang === 'zh'
              ? 'Croupier 当前架构：控制台、Server、四类 Agent、SDK、共享状态与独立分析管道'
              : 'Croupier current architecture: console, Server, four Agent roles, SDKs, shared state and separate analytics pipeline'
          "
          width="1200"
          height="1050"
          loading="lazy"
        />
      </picture>
    </a>
    <figcaption>
      {{
        props.lang === "zh"
          ? "2026-10-28 源码核对 · 点击打开完整 SVG"
          : "Source review: 2026-10-28 · Open the full SVG"
      }}
    </figcaption>
  </figure>
</template>

<style scoped>
.architecture-diagram {
  margin: 24px 0;
}
.architecture-diagram a {
  display: block;
  border-radius: 16px;
}
.architecture-diagram a:focus-visible {
  outline: 3px solid var(--vp-c-brand-1);
  outline-offset: 4px;
}
.architecture-diagram img {
  display: block;
  width: 100%;
  height: auto;
  border-radius: 16px;
}
.architecture-diagram figcaption {
  margin-top: 10px;
  font-size: 13px;
  color: var(--vp-c-text-2);
  text-align: center;
}
</style>

import { h } from "vue";
import DefaultTheme from "vitepress/theme-without-fonts";
import HomeCarousel from "./HomeCarousel.vue";
import "../styles/index.scss";
import "./custom.css";

// 首页界面轮播（用户令）：hero 之下、特性之上（home-features-before 插槽
// 只在 layout: home 页生效，仓库仅 docs/index.md 一页 home）。
export default {
  ...DefaultTheme,
  Layout: () =>
    h(DefaultTheme.Layout, null, {
      "home-features-before": () => h(HomeCarousel),
    }),
};

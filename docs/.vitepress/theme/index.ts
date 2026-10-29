import DefaultTheme from "vitepress/theme-without-fonts";
import HomeCarousel from "./HomeCarousel.vue";
import ArchitectureDiagram from "./ArchitectureDiagram.vue";
import "../styles/index.scss";
import "./custom.css";

// 首页界面轮播不再走 Layout 插槽：按 2026-10-22 用户令，「界面预览」条目
// 移到「What is Croupier / 什么是 Croupier」分组下方，由 index.md /
// zh/index.md 的 <HomeCarousel /> 直接落位。此处仅做全局注册供 markdown 使用。
export default {
  ...DefaultTheme,
  enhanceApp({ app }) {
    app.component("HomeCarousel", HomeCarousel);
    app.component("ArchitectureDiagram", ArchitectureDiagram);
  },
};

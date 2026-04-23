import { defineClientConfig } from "vuepress/client";
import Mermaid from "/Users/cui/Workspaces/croupier/server/docs/node_modules/.pnpm/@vuepress+plugin-markdown-chart@2.0.0-rc.121_markdown-it@14.1.0_mermaid@11.12.2_vuepres_20ef36c8539f0d5e4be972c681e4f00e/node_modules/@vuepress/plugin-markdown-chart/lib/client/components/Mermaid.js";

export default defineClientConfig({
  enhance: ({ app }) => {
    app.component("Mermaid", Mermaid);
  },
});

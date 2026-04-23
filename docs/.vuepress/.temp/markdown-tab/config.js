import { CodeTabs } from "/Users/cui/Workspaces/croupier/server/docs/node_modules/.pnpm/@vuepress+plugin-markdown-tab@2.0.0-rc.121_markdown-it@14.1.0_vuepress@2.0.0-rc.26_@vue_031ea175494189e5cb9e28a416dcd728/node_modules/@vuepress/plugin-markdown-tab/lib/client/components/CodeTabs.js";
import { Tabs } from "/Users/cui/Workspaces/croupier/server/docs/node_modules/.pnpm/@vuepress+plugin-markdown-tab@2.0.0-rc.121_markdown-it@14.1.0_vuepress@2.0.0-rc.26_@vue_031ea175494189e5cb9e28a416dcd728/node_modules/@vuepress/plugin-markdown-tab/lib/client/components/Tabs.js";
import "/Users/cui/Workspaces/croupier/server/docs/node_modules/.pnpm/@vuepress+plugin-markdown-tab@2.0.0-rc.121_markdown-it@14.1.0_vuepress@2.0.0-rc.26_@vue_031ea175494189e5cb9e28a416dcd728/node_modules/@vuepress/plugin-markdown-tab/lib/client/styles/vars.css";

export default {
  enhance: ({ app }) => {
    app.component("CodeTabs", CodeTabs);
    app.component("Tabs", Tabs);
  },
};

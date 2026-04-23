import { GitContributors } from "/Users/cui/Workspaces/croupier/server/docs/node_modules/.pnpm/@vuepress+plugin-git@2.0.0-rc.121_vuepress@2.0.0-rc.26_@vuepress+bundler-vite@2.0.0-rc._09190ed965066235b9f8c37401974eee/node_modules/@vuepress/plugin-git/lib/client/components/GitContributors.js";
import { GitChangelog } from "/Users/cui/Workspaces/croupier/server/docs/node_modules/.pnpm/@vuepress+plugin-git@2.0.0-rc.121_vuepress@2.0.0-rc.26_@vuepress+bundler-vite@2.0.0-rc._09190ed965066235b9f8c37401974eee/node_modules/@vuepress/plugin-git/lib/client/components/GitChangelog.js";

export default {
  enhance: ({ app }) => {
    app.component("GitContributors", GitContributors);
    app.component("GitChangelog", GitChangelog);
  },
};

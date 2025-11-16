import React from 'react';
import ComponentCreator from '@docusaurus/ComponentCreator';

export default [
  {
    path: '/croupier/analytics',
    component: ComponentCreator('/croupier/analytics', '1aa'),
    routes: [
      {
        path: '/croupier/analytics',
        component: ComponentCreator('/croupier/analytics', '7a6'),
        routes: [
          {
            path: '/croupier/analytics',
            component: ComponentCreator('/croupier/analytics', '6b6'),
            routes: [
              {
                path: '/croupier/analytics/data-collection-architecture',
                component: ComponentCreator('/croupier/analytics/data-collection-architecture', 'ce9'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/analytics/game-metrics-overview',
                component: ComponentCreator('/croupier/analytics/game-metrics-overview', '1ac'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/analytics/instrumentation-spec-cn',
                component: ComponentCreator('/croupier/analytics/instrumentation-spec-cn', '605'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/analytics/opentelemetry-integration',
                component: ComponentCreator('/croupier/analytics/opentelemetry-integration', '904'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/analytics/playbooks/board-table-cn',
                component: ComponentCreator('/croupier/analytics/playbooks/board-table-cn', 'd78'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/analytics/playbooks/card-ccg-cn',
                component: ComponentCreator('/croupier/analytics/playbooks/card-ccg-cn', '3ec'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/analytics/playbooks/idle-cn',
                component: ComponentCreator('/croupier/analytics/playbooks/idle-cn', '4a3'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/analytics/playbooks/tower-defense-cn',
                component: ComponentCreator('/croupier/analytics/playbooks/tower-defense-cn', '937'),
                exact: true,
                sidebar: "docs"
              }
            ]
          }
        ]
      }
    ]
  },
  {
    path: '/croupier/ops',
    component: ComponentCreator('/croupier/ops', '729'),
    routes: [
      {
        path: '/croupier/ops',
        component: ComponentCreator('/croupier/ops', '356'),
        routes: [
          {
            path: '/croupier/ops',
            component: ComponentCreator('/croupier/ops', 'd08'),
            routes: [
              {
                path: '/croupier/ops/remote-access-web',
                component: ComponentCreator('/croupier/ops/remote-access-web', 'e97'),
                exact: true,
                sidebar: "docs"
              }
            ]
          }
        ]
      }
    ]
  },
  {
    path: '/croupier/',
    component: ComponentCreator('/croupier/', '586'),
    exact: true
  },
  {
    path: '/croupier/',
    component: ComponentCreator('/croupier/', '048'),
    routes: [
      {
        path: '/croupier/',
        component: ComponentCreator('/croupier/', '738'),
        routes: [
          {
            path: '/croupier/',
            component: ComponentCreator('/croupier/', 'db5'),
            routes: [
              {
                path: '/croupier/api',
                component: ComponentCreator('/croupier/api', '393'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/ARCHITECTURE',
                component: ComponentCreator('/croupier/ARCHITECTURE', '08f'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/assignments',
                component: ComponentCreator('/croupier/assignments', '93a'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/complete-game-roles-design',
                component: ComponentCreator('/croupier/complete-game-roles-design', '18d'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/config',
                component: ComponentCreator('/croupier/config', 'bdc'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/control-capabilities',
                component: ComponentCreator('/croupier/control-capabilities', '577'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/CPP_SDK_ANALYSIS',
                component: ComponentCreator('/croupier/CPP_SDK_ANALYSIS', '9cb'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/CPP_SDK_ANALYSIS_SUMMARY',
                component: ComponentCreator('/croupier/CPP_SDK_ANALYSIS_SUMMARY', 'd25'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/CPP_SDK_BUILD_OPTIMIZATION',
                component: ComponentCreator('/croupier/CPP_SDK_BUILD_OPTIMIZATION', '2c3'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/CPP_SDK_DEEP_ANALYSIS',
                component: ComponentCreator('/croupier/CPP_SDK_DEEP_ANALYSIS', '4dc'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/CPP_SDK_DIRECTORY_INDEX',
                component: ComponentCreator('/croupier/CPP_SDK_DIRECTORY_INDEX', 'f13'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/CPP_SDK_DOCS_INDEX',
                component: ComponentCreator('/croupier/CPP_SDK_DOCS_INDEX', 'bc5'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/CPP_SDK_QUICK_REFERENCE',
                component: ComponentCreator('/croupier/CPP_SDK_QUICK_REFERENCE', '26a'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/deployment',
                component: ComponentCreator('/croupier/deployment', 'b1a'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/directory-structure',
                component: ComponentCreator('/croupier/directory-structure', 'd20'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/e2e-example',
                component: ComponentCreator('/croupier/e2e-example', '4bc'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/FUNCTION_MANAGEMENT_ARCHITECTURE_ANALYSIS',
                component: ComponentCreator('/croupier/FUNCTION_MANAGEMENT_ARCHITECTURE_ANALYSIS', 'e47'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/FUNCTION_MANAGEMENT_EXECUTIVE_SUMMARY',
                component: ComponentCreator('/croupier/FUNCTION_MANAGEMENT_EXECUTIVE_SUMMARY', '594'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/FUNCTION_MANAGEMENT_QUICK_REFERENCE',
                component: ComponentCreator('/croupier/FUNCTION_MANAGEMENT_QUICK_REFERENCE', '83d'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/FUNCTION_MANAGEMENT_README',
                component: ComponentCreator('/croupier/FUNCTION_MANAGEMENT_README', 'b55'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/game-roles-design',
                component: ComponentCreator('/croupier/game-roles-design', 'ad4'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/generator',
                component: ComponentCreator('/croupier/generator', 'e9f'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/HOT_RELOAD_SOLUTIONS',
                component: ComponentCreator('/croupier/HOT_RELOAD_SOLUTIONS', '827'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/HOTRELOAD_BEST_PRACTICES',
                component: ComponentCreator('/croupier/HOTRELOAD_BEST_PRACTICES', 'a72'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/HOTRELOAD_IMPLEMENTATION_SUMMARY',
                component: ComponentCreator('/croupier/HOTRELOAD_IMPLEMENTATION_SUMMARY', '031'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/http-adapter',
                component: ComponentCreator('/croupier/http-adapter', '281'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/metrics',
                component: ComponentCreator('/croupier/metrics', '69f'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/providers-manifest',
                component: ComponentCreator('/croupier/providers-manifest', 'b32'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/SDK_HOTRELOAD_SUPPORT',
                component: ComponentCreator('/croupier/SDK_HOTRELOAD_SUPPORT', 'd96'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/sdk-development',
                component: ComponentCreator('/croupier/sdk-development', '310'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/security',
                component: ComponentCreator('/croupier/security', 'b2b'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/tracing',
                component: ComponentCreator('/croupier/tracing', 'a7c'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/ui-and-views',
                component: ComponentCreator('/croupier/ui-and-views', '872'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/VCPKG_OPTIMIZATION',
                component: ComponentCreator('/croupier/VCPKG_OPTIMIZATION', '1b8'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/VIRTUAL_OBJECT_DESIGN',
                component: ComponentCreator('/croupier/VIRTUAL_OBJECT_DESIGN', 'da6'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/VIRTUAL_OBJECT_QUICK_REFERENCE',
                component: ComponentCreator('/croupier/VIRTUAL_OBJECT_QUICK_REFERENCE', 'c5f'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/croupier/wire-and-di',
                component: ComponentCreator('/croupier/wire-and-di', 'f74'),
                exact: true,
                sidebar: "docs"
              }
            ]
          }
        ]
      }
    ]
  },
  {
    path: '*',
    component: ComponentCreator('*'),
  },
];

const tsParser = require('@typescript-eslint/parser');
const tsPlugin = require('@typescript-eslint/eslint-plugin');
const reactHooksPlugin = require('eslint-plugin-react-hooks');
const jsxA11yPlugin = require('eslint-plugin-jsx-a11y');

module.exports = [
  {
    ignores: [
      'lambda/**',
      'scripts/**',
      'config/**',
      '.history/**',
      'public/**',
      'dist/**',
      '**/.umi/**',
      '**/.umi-production/**',
      'src/services/generated/**',
      'mock/**',
    ],
  },
  {
    files: ['**/*.{js,jsx,ts,tsx}'],
    plugins: {
      '@typescript-eslint': tsPlugin,
      'react-hooks': reactHooksPlugin,
      'jsx-a11y': jsxA11yPlugin,
    },
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',
        ecmaFeatures: { jsx: true },
      },
      globals: {
        page: 'readonly',
        REACT_APP_ENV: 'readonly',
      },
    },
    rules: {
      // TypeScript strict rules - no any types allowed
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      // React hooks rules
      'react-hooks/rules-of-hooks': 'error',
      'react-hooks/exhaustive-deps': 'warn',
      // Accessibility (CR-FIX-19): unambiguous markup-level a11y defects are
      // errors; context-dependent judgement calls (autofocus, redundant alt)
      // stay warnings until a sweep settles them.
      ...jsxA11yPlugin.configs.recommended.rules,
      // 6.10.2 loads minimatch through require(); the workspace-wide security
      // override pins minimatch >=11 (ESM-only), so these two rules crash
      // ESLint at lint time (not a style choice). Re-enable when upstream
      // ships a release that imports minimatch compatibly.
      'jsx-a11y/label-has-associated-control': 'off',
      'jsx-a11y/control-has-associated-label': 'off',
      'jsx-a11y/no-autofocus': 'warn',
      'jsx-a11y/media-has-caption': 'warn',
      'jsx-a11y/mouse-events-have-key-events': 'warn',
      'jsx-a11y/no-noninteractive-element-interactions': 'warn',
      'jsx-a11y/no-static-element-interactions': 'warn',
      'jsx-a11y/click-events-have-key-events': 'warn',
    },
  },
];

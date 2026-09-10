export default async () => {
  return {
    rootDir: '.',
    testEnvironment: 'jsdom',
    // 默认 babel-istanbul provider 已损坏（@babel/core 8 × minimatch 10 的
    // CJS 互操作：instrument 阶段报 "minimatch is not a function"），
    // 固定 v8 provider（语句/分支口径一致）
    coverageProvider: 'v8',
    testMatch: ['**/?(*.)+(spec|test).[jt]s?(x)'],
    testPathIgnorePatterns: ['/node_modules/', '<rootDir>/e2e/'],
    transform: {
      '^.+\\.(t|j)sx?$': [
        'ts-jest',
        {
          tsconfig: '<rootDir>/tsconfig.jest.json',
        },
      ],
    },
    transformIgnorePatterns: [
      '/node_modules/(?!(?:@rjsf|@x0k|@ant-design|antd|lodash-es|@faker-js)/|\\.pnpm/(?:@rjsf\\+|@x0k\\+|@ant-design\\+|antd@|lodash-es@|@faker-js\\+))',
    ],
    moduleNameMapper: {
      '^@/(.*)$': '<rootDir>/src/$1',
      '^@@/(.*)$': '<rootDir>/tests/umi/$1',
      '\\.(css|less|scss|sass)$': '<rootDir>/tests/umi/styleMock.js',
    },
    testEnvironmentOptions: {
      url: 'http://localhost:8000',
    },
    setupFiles: ['./tests/setupTests.jsx'],
    setupFilesAfterEnv: ['@testing-library/jest-dom'],
    globals: {
      localStorage: null,
    },
  };
};

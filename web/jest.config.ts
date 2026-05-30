export default async () => {
  return {
    rootDir: '.',
    testEnvironment: 'jsdom',
    testMatch: ['**/?(*.)+(spec|test).[jt]s?(x)'],
    transform: {
      '^.+\\.(t|j)sx?$': [
        'ts-jest',
        {
          tsconfig: '<rootDir>/tsconfig.jest.json',
        },
      ],
    },
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

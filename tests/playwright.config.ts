import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: '.',
  testMatch: 'e2e_flow.spec.ts',
  timeout: 45_000,
  use: {
    baseURL: process.env.BASE_URL || 'http://web',
    locale: 'zh-CN',
  },
})

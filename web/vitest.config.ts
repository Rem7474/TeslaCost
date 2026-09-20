import { defineConfig } from 'vitest/config'
import path from 'path'

// Unit tests target framework-free modules (utils, composables), so no Vue or Tailwind plugin is loaded.
export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
    setupFiles: ['./vitest.setup.ts'],
  },
})

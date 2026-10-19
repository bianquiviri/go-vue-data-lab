import { fileURLToPath } from 'node:url'
import { mergeConfig, defineConfig } from 'vitest/config'
import viteConfig from './vite.config.ts'

export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      environment: 'jsdom',
      globals: false,
      setupFiles: ['./src/__tests__/setup.ts'],
      include: ['src/**/__tests__/**/*.spec.ts'],
      root: fileURLToPath(new URL('./', import.meta.url)),
      coverage: {
        provider: 'v8',
        reporter: ['text-summary', 'text', 'html', 'lcov'],
        reportsDirectory: './coverage',
        include: ['src/**/*.{ts,vue}'],
        exclude: [
          // bootstrap de la app (cubierto por inspección, no por unit tests)
          'src/main.ts',
          // infraestructura de tests y fixtures
          'src/__tests__/**',
          'src/**/__tests__/**',
          // componentes shadcn-vue generados por CLI (código de terceros)
          'src/components/ui/**',
        ],
        thresholds: {
          statements: 80,
          branches: 80,
          functions: 80,
          lines: 80,
        },
      },
    },
  }),
)

import { defineConfig } from 'vitest/config';

export default defineConfig({
  esbuild: {
    target: 'node20',
  },
  test: {
    environment: 'node',
    testTimeout: 30_000,
    hookTimeout: 30_000,
    env: {
      IDIP_BASE_URL: process.env.IDIP_BASE_URL ?? 'http://127.0.0.1:8080',
      IDIP_KEY: process.env.IDIP_KEY ?? 'change-me-in-production',
      IDIP_WEBCLIENT_URL: process.env.IDIP_WEBCLIENT_URL ?? '',
    },
  },
});

import { defineConfig } from 'vite';
import { regressionRunnerPlugin } from './vite-plugin-regression-runner';

/**
 * 开发时浏览器走同源代理，满足 server-go：
 * - IDIP 仅允许内网来源（经代理后 RemoteAddr 为 127.0.0.1）
 * - 回归用例 IDP-003/006 等需 POST /api/v1/auth/guest 建 MySQL 玩家行（须同时代理 /api）
 */
export default defineConfig({
  plugins: [regressionRunnerPlugin()],
  server: {
    port: 5174,
    proxy: {
      '/idip': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
});

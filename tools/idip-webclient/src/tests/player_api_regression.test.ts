/**
 * 玩家 API 回归（P1 整包下载字段）— 对照 doc/测试用例.md API-GAMES-001
 */
import { describe, expect, it } from 'vitest';

const baseUrl = (process.env.IDIP_BASE_URL ?? 'http://127.0.0.1:8080').replace(/\/$/, '');

describe(`Player API regression (${baseUrl})`, () => {
  it('API-GAMES-001 GET /api/v1/games returns configVersion and download fields', async (ctx) => {
    const q = new URLSearchParams({
      appVersion: '1.0.0',
      platform: 'android',
      channel: 'ChannelType_GooglePlay',
    });
    const res = await fetch(`${baseUrl}/api/v1/games?${q}`);
    if (res.status === 503 || res.status === 502) {
      ctx.skip(`games API unavailable: ${res.status}`);
    }
    expect(res.status).toBe(200);
    const body = (await res.json()) as {
      code?: number;
      data?: {
        configVersion?: string;
        serverTime?: string;
        games?: Array<{
          gameId?: string;
          entryUrl?: string;
          downloadUrl?: string;
          packageBytes?: number;
          downloadSha256?: string;
        }>;
      };
    };
    expect(body.code).toBe(0);
    const cv = body.data?.configVersion ?? '';
    expect(cv).toMatch(/^[a-f0-9]{64}$/i);
    expect(body.data?.serverTime).toBeTruthy();

    const games = body.data?.games ?? [];
    const g001 = games.find((g) => (g.gameId ?? '').toLowerCase() === 'g001');
    if (!g001) {
      ctx.skip('g001 not in games list');
    }
    if (g001.entryUrl) {
      expect(g001.entryUrl).toMatch(/^h5\//);
    }
    if (g001.downloadUrl) {
      expect(g001.downloadUrl).toMatch(/^h5\//);
      expect(g001.downloadUrl.length).toBeGreaterThan(0);
      if (g001.packageBytes != null && g001.packageBytes > 0) {
        expect(g001.downloadSha256).toMatch(/^[a-f0-9]{64}$/i);
      }
    }
  });
});

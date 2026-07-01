import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const FIXTURE_DIR = join(dirname(fileURLToPath(import.meta.url)), 'fixtures');

export const VITEST_GAME_ID = 'vitest-h5-001';
export const VITEST_GAME_DIR = 'vitest-game1';

export function loadH5ZipFixture(name: 'vitest-game1.zip' | 'vitest-badname.zip'): File {
  const buf = readFileSync(join(FIXTURE_DIR, name));
  const blob = new Blob([buf], { type: 'application/zip' });
  return new File([blob], name, { type: 'application/zip' });
}

export function h5UploadMeta(overrides?: Partial<Record<string, unknown>>) {
  return {
    gameId: VITEST_GAME_ID,
    minigameVersion: '1.0.0.1',
    name: 'Vitest H5 Game',
    entryType: 'h5',
    status: 'offline',
    sort: 999,
    ...overrides,
  };
}

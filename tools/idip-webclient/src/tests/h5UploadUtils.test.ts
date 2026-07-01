import { describe, expect, it } from 'vitest';
import {
  detectH5FolderRootName,
  findGameRowByDirName,
  parseMinigameVersionFromEntryUrl,
  parseMinigameVersionFromPackageFileName,
  suggestNextMinigameVersion,
  wireGameIdMetaReload,
} from '../games/h5UploadUtils';
import type { IdipGameRow } from '../api/types';

describe('h5UploadUtils', () => {
  it('detects folder root from webkitRelativePath', () => {
    const files = [
      { name: 'index.html', webkitRelativePath: 'web-mobile-planefight/index.html' },
      { name: 'main.js', webkitRelativePath: 'web-mobile-planefight/main.js' },
    ] as File[];
    expect(detectH5FolderRootName(files)).toBe('web-mobile-planefight');
  });

  it('parses version from tar.gz file name', () => {
    expect(parseMinigameVersionFromPackageFileName('web-mobile-planefight_v1.0.0.2.tar.gz')).toBe(
      '1.0.0.2',
    );
    expect(parseMinigameVersionFromPackageFileName('D:/pkg/game1_v2.3.4.5.zip')).toBe('2.3.4.5');
    expect(parseMinigameVersionFromPackageFileName('game1.zip')).toBeNull();
  });

  it('parses version from entryUrl', () => {
    expect(parseMinigameVersionFromEntryUrl('h5/foo/index.html?v=1.0.0.2')).toBe('1.0.0.2');
    expect(parseMinigameVersionFromEntryUrl('h5/foo/index.html')).toBeNull();
  });

  it('reloads meta on gameId blur', async () => {
    document.body.innerHTML = `
      <div id="meta">
        <input data-meta-key="gameId" id="gid" />
        <input data-meta-key="minigameVersion" />
        <input data-meta-key="name" />
        <select data-meta-key="status"><option value="online">online</option><option value="offline">offline</option></select>
      </div>
    `;
    const metaForm = document.getElementById('meta')!;
    const gameIdInput = document.getElementById('gid') as HTMLInputElement;
    const row: IdipGameRow = {
      gameId: 'g005',
      name: '雷霆战机',
      entryType: 'h5',
      entryUrl: 'h5/web-mobile-planefight/index.html?v=1.0.0.2',
      sort: 1,
      status: 'online',
    };
    wireGameIdMetaReload({
      gameIdInput,
      metaForm,
      getRows: () => [row],
      ensureRowsLoaded: async () => [row],
    });
    gameIdInput.value = 'g005';
    gameIdInput.dispatchEvent(new FocusEvent('blur'));
    await new Promise((r) => setTimeout(r, 0));
    expect((metaForm.querySelector('[data-meta-key="name"]') as HTMLInputElement).value).toBe(
      '雷霆战机',
    );
    expect((metaForm.querySelector('[data-meta-key="minigameVersion"]') as HTMLInputElement).value).toBe(
      '1.0.0.3',
    );
  });

  it('matches game row by folder dir name', () => {
    const rows: IdipGameRow[] = [
      {
        gameId: 'g005',
        name: '雷霆战机',
        entryType: 'h5',
        entryUrl: 'h5/web-mobile-planefight/index.html?v=1.0.0.2',
        sort: 1,
      },
    ];
    expect(findGameRowByDirName(rows, 'web-mobile-planefight')?.gameId).toBe('g005');
    expect(suggestNextMinigameVersion('1.0.0.2')).toBe('1.0.0.3');
  });
});

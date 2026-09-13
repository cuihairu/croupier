import { request } from '@umijs/max';
import type {
  BindingSelectorSyncReport,
  Diagnostic,
  PageSpecDraftSummary,
  PageVersionItem,
} from '@/types/dashboard';
import {
  bulkPublishPages,
  bulkUnpublishPages,
  getPageDraft,
  getPageVersion,
  listPageDrafts,
  listPageVersions,
  previewPageDraft,
  publishPageDraft,
  regeneratePageDraft,
  rollbackPageDraft,
  savePageDraft,
  syncPageSelectors,
  unpublishPage,
  validatePageDraft,
  type PageSavePayload,
} from './pages';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

// 最小合法 composite PageSpec（PageSavePayload = PageSpec & { draftRevision }）
const pagePayload: PageSavePayload = {
  pageKey: 'players',
  title: { 'zh-CN': '玩家', 'en-US': 'Players' },
  category: { key: 'ops', labels: { 'zh-CN': '运营', 'en-US': 'Ops' } },
  bindings: [],
  type: 'composite',
  composite: { sections: [] },
  draftRevision: 2,
};

const draftSummary: PageSpecDraftSummary = {
  pageKey: 'players',
  type: 'composite',
  title: { 'zh-CN': '玩家' },
  category: { key: 'ops', labels: { 'zh-CN': '运营' } },
  status: 'draft',
  draftRevision: 2,
  updatedAt: '2026-09-01T00:00:00Z',
};

describe('pages draft/version API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  describe('listPageDrafts', () => {
    it('GETs the base URL with filters and returns the items array', async () => {
      mockedRequest.mockResolvedValue({ items: [draftSummary] });

      const rows = await listPageDrafts({ resourceKey: 'player', status: 'draft' });

      expect(rows).toEqual([draftSummary]);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages', {
        method: 'GET',
        params: { resourceKey: 'player', status: 'draft' },
      });
    });

    it('calls without params when none are given', async () => {
      mockedRequest.mockResolvedValue({ items: [] });

      await listPageDrafts();

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages', {
        method: 'GET',
        params: undefined,
      });
    });

    it('falls back to an empty list when items is missing', async () => {
      mockedRequest.mockResolvedValue({});

      await expect(listPageDrafts()).resolves.toEqual([]);
    });

    it('falls back to an empty list when the response body is empty', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await expect(listPageDrafts()).resolves.toEqual([]);
    });
  });

  describe('getPageDraft', () => {
    it('GETs the draft and URL-encodes path separators in the page key', async () => {
      const draft = { ...pagePayload, status: 'draft', updatedAt: '2026-09-01T00:00:00Z' };
      mockedRequest.mockResolvedValue(draft);

      const resp = await getPageDraft('ops/a b');

      expect(resp).toEqual(draft);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/ops%2Fa%20b', {
        method: 'GET',
      });
    });
  });

  describe('savePageDraft', () => {
    it('PUTs the payload to the page key endpoint', async () => {
      mockedRequest.mockResolvedValue({ pageKey: 'players', draftRevision: 3 });

      const resp = await savePageDraft(pagePayload);

      expect(resp).toEqual({ pageKey: 'players', draftRevision: 3 });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/players', {
        method: 'PUT',
        data: pagePayload,
      });
    });
  });

  describe('regeneratePageDraft', () => {
    it('POSTs draftRevision to the regenerate endpoint', async () => {
      const diagnostics: Diagnostic[] = [];
      mockedRequest.mockResolvedValue({
        pageKey: 'players',
        draftRevision: 4,
        page: { ...pagePayload, draftRevision: 4 },
        diagnostics,
        quality: 'ready',
      });

      const resp = await regeneratePageDraft('players', 3);

      expect(resp.draftRevision).toBe(4);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/players/regenerate', {
        method: 'POST',
        data: { draftRevision: 3 },
      });
    });
  });

  describe('syncPageSelectors', () => {
    it('POSTs the sync payload untouched', async () => {
      const syncedBindings: BindingSelectorSyncReport[] = [];
      mockedRequest.mockResolvedValue({
        pageKey: 'players',
        dryRun: false,
        applied: true,
        draftRevision: 5,
        syncedBindings,
        remainingDiagnostics: [],
      });

      const resp = await syncPageSelectors('players', {
        draftRevision: 4,
        dryRun: false,
        bindingIds: ['list'],
      });

      expect(resp.applied).toBe(true);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/players/sync-selectors', {
        method: 'POST',
        data: { draftRevision: 4, dryRun: false, bindingIds: ['list'] },
      });
    });
  });

  describe('validatePageDraft', () => {
    it('POSTs to the validate endpoint and returns diagnostics', async () => {
      const diagnostics: Diagnostic[] = [
        { code: 'binding.missing', severity: 'error', message: 'missing binding' },
      ];
      mockedRequest.mockResolvedValue({ valid: false, diagnostics });

      const resp = await validatePageDraft('players');

      expect(resp).toEqual({ valid: false, diagnostics });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/players/validate', {
        method: 'POST',
      });
    });
  });

  describe('previewPageDraft', () => {
    it('POSTs to preview and unwraps response.page', async () => {
      mockedRequest.mockResolvedValue({ page: pagePayload });

      const page = await previewPageDraft('players');

      expect(page).toEqual(pagePayload);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/players/preview', {
        method: 'POST',
      });
    });

    it('throws when the response has no page field', async () => {
      mockedRequest.mockResolvedValue({});

      await expect(previewPageDraft('players')).rejects.toThrow(
        'page preview returned empty page: players',
      );
    });

    it('throws when the response body is empty', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await expect(previewPageDraft('players')).rejects.toThrow(
        'page preview returned empty page: players',
      );
    });
  });

  describe('publishPageDraft', () => {
    it('POSTs draftRevision to the publish endpoint', async () => {
      mockedRequest.mockResolvedValue({
        pageKey: 'players',
        published: true,
        publishedVersion: 7,
      });

      const resp = await publishPageDraft('players', 6);

      expect(resp.publishedVersion).toBe(7);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/players/publish', {
        method: 'POST',
        data: { draftRevision: 6 },
      });
    });
  });

  describe('unpublishPage', () => {
    it('POSTs to the unpublish endpoint', async () => {
      mockedRequest.mockResolvedValue({ pageKey: 'players', published: false });

      const resp = await unpublishPage('players');

      expect(resp.published).toBe(false);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/players/unpublish', {
        method: 'POST',
      });
    });
  });

  describe('listPageVersions', () => {
    it('GETs versions with pagination params', async () => {
      const items: PageVersionItem[] = [
        {
          version: 7,
          status: 'published',
          isCurrentDraft: false,
          isCurrentPublished: true,
          createdAt: '2026-09-01T00:00:00Z',
        },
      ];
      const body = {
        currentDraftRevision: 8,
        currentPublishedVersion: 7,
        total: 1,
        items,
      };
      mockedRequest.mockResolvedValue(body);

      const resp = await listPageVersions('players', { limit: 10, offset: 20 });

      expect(resp).toEqual(body);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/players/versions', {
        method: 'GET',
        params: { limit: 10, offset: 20 },
      });
    });

    it('calls without params when none are given', async () => {
      mockedRequest.mockResolvedValue({ currentDraftRevision: 1, items: [] });

      await listPageVersions('players');

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/players/versions', {
        method: 'GET',
        params: undefined,
      });
    });
  });

  describe('getPageVersion', () => {
    it('GETs one version detail (version coerced through the URL)', async () => {
      const detail = {
        version: 7,
        status: 'published',
        createdAt: '2026-09-01T00:00:00Z',
        page: pagePayload,
      };
      mockedRequest.mockResolvedValue(detail);

      const resp = await getPageVersion('players', 7);

      expect(resp).toEqual(detail);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/players/versions/7', {
        method: 'GET',
      });
    });
  });

  describe('rollbackPageDraft', () => {
    it('POSTs versionId as string plus expectedDraftRevision', async () => {
      mockedRequest.mockResolvedValue({ pageKey: 'players', draftRevision: 9 });

      const resp = await rollbackPageDraft('players', 7, 8);

      expect(resp.draftRevision).toBe(9);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/players/rollback', {
        method: 'POST',
        data: { versionId: '7', expectedDraftRevision: 8 },
      });
    });
  });

  describe('bulk operations', () => {
    it('POSTs bulk-publish and returns the per-page report', async () => {
      const report = {
        total: 2,
        published: ['players'],
        skipped: ['tasks'],
      };
      mockedRequest.mockResolvedValue(report);

      const resp = await bulkPublishPages();

      expect(resp).toEqual(report);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/bulk-publish', {
        method: 'POST',
      });
    });

    it('POSTs bulk-unpublish and returns the per-page report', async () => {
      const report = {
        total: 1,
        unpublished: ['players'],
      };
      mockedRequest.mockResolvedValue(report);

      const resp = await bulkUnpublishPages();

      expect(resp).toEqual(report);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/bulk-unpublish', {
        method: 'POST',
      });
    });
  });
});

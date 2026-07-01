import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock env config before module loads (buildThumbnailUrl reads configResult at module init)
vi.mock('../shared/config/env', () => ({
  configResult: { ok: true, config: { apiBaseUrl: 'http://api.test', apiKey: 'secret' } },
}));

import {
  buildThumbnailCandidates,
  buildSrcSet,
  THUMB_BASE_WIDTHS,
  THUMB_DPRS,
  CARD_SIZES,
} from '../shared/lib/thumbnailCandidates';

const TEST_ID = 'a1b2c3d4-e5f6-7890-abcd-ef1234567890';

describe('buildThumbnailCandidates', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns sorted candidates (smallest width first)', () => {
    const candidates = buildThumbnailCandidates(TEST_ID);
    const widths = candidates.map((c) => c.w);
    expect(widths).toEqual([...widths].sort((a, b) => a - b));
  });

  it('each candidate URL contains the photo ID', () => {
    const candidates = buildThumbnailCandidates(TEST_ID);
    for (const c of candidates) {
      expect(c.url).toContain(TEST_ID);
    }
  });

  it('output widths are base × dpr', () => {
    const candidates = buildThumbnailCandidates(TEST_ID, [200, 400], [1, 2]);
    // 200×1=200, 200×2=400, 400×1=400 (dup!), 400×2=800 → deduplicated → [200, 400, 800]
    expect(candidates.map((c) => c.w)).toEqual([200, 400, 800]);
  });

  it('deduplicates candidates with the same output width (first wins)', () => {
    // 200×3=600 and 600×1=600 — should only appear once
    const candidates = buildThumbnailCandidates(TEST_ID, [200, 600], [1, 2, 3]);
    const widths = candidates.map((c) => c.w);
    const seen = new Set<number>();
    for (const w of widths) {
      expect(seen.has(w)).toBe(false);
      seen.add(w);
    }
  });

  it('no candidate exceeds 2048px output width', () => {
    const candidates = buildThumbnailCandidates(TEST_ID, [1000, 2000], [1, 2, 3]);
    for (const c of candidates) {
      expect(c.w).toBeLessThanOrEqual(2048);
    }
  });

  it('URL contains correct width parameter', () => {
    const candidates = buildThumbnailCandidates(TEST_ID, [300], [1]);
    expect(candidates[0]?.url).toContain('width=300');
  });

  it('URL contains correct dpr parameter', () => {
    const candidates = buildThumbnailCandidates(TEST_ID, [200], [2]);
    expect(candidates[0]?.url).toContain('dpr=2');
  });

  it('API key is NOT included in thumbnail URL', () => {
    const candidates = buildThumbnailCandidates(TEST_ID);
    for (const c of candidates) {
      expect(c.url).not.toContain('secret');
      expect(c.url).not.toContain('apiKey');
      expect(c.url).not.toContain('api_key');
    }
  });

  it('uses defaults (THUMB_BASE_WIDTHS / THUMB_DPRS) when called without explicit args', () => {
    const candidates = buildThumbnailCandidates(TEST_ID);
    // Every candidate must use a base width from THUMB_BASE_WIDTHS
    for (const c of candidates) {
      const usesBase = [...THUMB_BASE_WIDTHS].some((base) =>
        [...THUMB_DPRS].some((dpr) => base * dpr === c.w),
      );
      expect(usesBase).toBe(true);
    }
  });
});

describe('buildSrcSet', () => {
  it('produces "url Xw" entries joined by ", "', () => {
    const candidates = [
      { url: 'http://api.test/a', w: 200 },
      { url: 'http://api.test/b', w: 400 },
    ];
    expect(buildSrcSet(candidates)).toBe(
      'http://api.test/a 200w, http://api.test/b 400w',
    );
  });

  it('returns empty string for empty array', () => {
    expect(buildSrcSet([])).toBe('');
  });

  it('is stable (same input → same output)', () => {
    const candidates = buildThumbnailCandidates(TEST_ID);
    expect(buildSrcSet(candidates)).toBe(buildSrcSet(candidates));
  });
});

describe('CARD_SIZES', () => {
  it('1-column range below 504px viewport', () => {
    // At ≤503px: 1 column; width = 100vw - 2×24px padding = 100vw - 48px
    expect(CARD_SIZES).toContain('max-width: 503px');
    expect(CARD_SIZES).toContain('calc(100vw - 48px)');
  });

  it('2-column range below 740px viewport', () => {
    // At 504–739px: 2 columns; width = (100vw - 48px - 16px gap) / 2 = 50vw - 32px
    expect(CARD_SIZES).toContain('max-width: 739px');
    expect(CARD_SIZES).toContain('calc(50vw - 32px)');
  });

  it('3-column range below 976px viewport', () => {
    // At 740–975px: 3 columns; width = (100vw - 48px - 32px gap) / 3 ≈ 33.33vw - 27px
    expect(CARD_SIZES).toContain('max-width: 975px');
    expect(CARD_SIZES).toContain('calc(33.33vw - 27px)');
  });

  it('4-column formula is calc(25vw - 24px) inside the desktop entry', () => {
    // At ≥976px: 4 columns; base formula = (100vw - 48px - 48px gap) / 4 = 25vw - 24px
    expect(CARD_SIZES).toContain('calc(25vw - 24px)');
  });

  it('desktop source size is capped at 280px', () => {
    // Wide screens produce 5+ columns where 25vw - 24px would exceed 400–500px;
    // the min() cap keeps the browser from fetching a larger candidate than necessary.
    expect(CARD_SIZES).toContain('280px');
    expect(CARD_SIZES).toContain('min(calc(25vw - 24px), 280px)');
  });

  it('wide viewport contract: desktop entry never requests more than 280px', () => {
    // At any viewport ≥976px the effective size is min(25vw-24px, 280px).
    // The default (last) entry must be exactly this expression.
    // Note: split(',') fragments inside min(), so we check endsWith instead.
    expect(CARD_SIZES.endsWith('min(calc(25vw - 24px), 280px)')).toBe(true);
  });
});

import { buildThumbnailUrl } from '../api/thumbnail';

export const THUMB_BASE_WIDTHS = [200, 400, 600] as const;
export const THUMB_DPRS = [1, 2, 3] as const;
export const THUMB_QUALITY = 80;
const THUMB_MAX_W = 2048;

// Grid: repeat(auto-fill, minmax(220px, 1fr)), gap 16px (--space-md), padding 24px each side (--space-lg).
// Column thresholds (viewport): 1 col < 504px, 2 cols < 740px, 3 cols < 976px, 4+ cols ≥ 976px.
// Column width formula: (100vw - 2×padding - (N-1)×gap) / N
export const CARD_SIZES =
  '(max-width: 503px) calc(100vw - 48px), (max-width: 739px) calc(50vw - 32px), (max-width: 975px) calc(33.33vw - 27px), min(calc(25vw - 24px), 280px)';

export interface ThumbnailCandidate {
  url: string;
  /** Actual output pixel width (base × dpr). Used as the srcset width descriptor. */
  w: number;
}

/**
 * Builds deduplicated thumbnail candidates for a srcset.
 * Output widths are base × dpr; duplicates are dropped (first wins).
 */
export function buildThumbnailCandidates(
  photoId: string,
  baseWidths: readonly number[] = THUMB_BASE_WIDTHS,
  dprs: readonly number[] = THUMB_DPRS,
  quality: number = THUMB_QUALITY,
): ThumbnailCandidate[] {
  const seen = new Set<number>();
  const candidates: ThumbnailCandidate[] = [];

  for (const base of baseWidths) {
    for (const dpr of dprs) {
      const outputW = base * dpr;
      if (outputW > THUMB_MAX_W || seen.has(outputW)) continue;
      seen.add(outputW);
      candidates.push({
        url: buildThumbnailUrl({ photoId, width: base, dpr, quality }),
        w: outputW,
      });
    }
  }

  return candidates.sort((a, b) => a.w - b.w);
}

export function buildSrcSet(candidates: ThumbnailCandidate[]): string {
  return candidates.map((c) => `${c.url} ${c.w}w`).join(', ');
}

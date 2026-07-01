import { configResult } from '../config/env';

// Canonical UUID: 8-4-4-4-12 lowercase hex
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export function isCanonicalPhotoId(id: string): boolean {
  return UUID_RE.test(id);
}

interface ThumbnailParams {
  photoId: string;
  width: number;
  dpr?: number;
  quality?: number;
}

export function buildThumbnailUrl(params: ThumbnailParams): string {
  const { photoId, width, dpr, quality } = params;

  if (!UUID_RE.test(photoId)) {
    throw new Error('buildThumbnailUrl: photoId must be a canonical UUID (lowercase 8-4-4-4-12)');
  }
  if (!Number.isInteger(width) || width < 1 || width > 2048) {
    throw new Error('buildThumbnailUrl: width must be an integer in [1, 2048]');
  }
  if (dpr !== undefined && (!Number.isFinite(dpr) || dpr < 1 || dpr > 3)) {
    throw new Error('buildThumbnailUrl: dpr must be a finite number in [1, 3]');
  }
  if (quality !== undefined && (!Number.isInteger(quality) || quality < 1 || quality > 100)) {
    throw new Error('buildThumbnailUrl: quality must be an integer in [1, 100]');
  }

  const baseUrl = configResult.ok ? configResult.config.apiBaseUrl : '';

  const searchParams = new URLSearchParams();
  searchParams.set('width', String(width));
  if (dpr !== undefined) searchParams.set('dpr', String(dpr));
  if (quality !== undefined) searchParams.set('quality', String(quality));

  return `${baseUrl}/photos/${encodeURIComponent(photoId)}/thumbnail?${searchParams.toString()}`;
}

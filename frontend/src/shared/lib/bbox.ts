import type { components } from '../../entities/api/__generated__/schema';

type BoundingBox = components['schemas']['BoundingBox'];

export function clampNorm(v: number): number {
  if (v < 0) return 0;
  if (v > 1) return 1;
  return v;
}

export function isValidBBox(bbox: BoundingBox): boolean {
  const { xMin, yMin, xMax, yMax } = bbox;
  return (
    Number.isFinite(xMin) &&
    Number.isFinite(yMin) &&
    Number.isFinite(xMax) &&
    Number.isFinite(yMax) &&
    xMax > xMin &&
    yMax > yMin
  );
}

/** Maps a normalized [0,1] bbox to SVG rect attributes scaled to (scaleW × scaleH).
 *  Use scaleW=1, scaleH=1 for card 1×1 viewBox; use photo.width/height for viewer native-dimension viewBox. */
export function bboxToSvgRect(
  bbox: BoundingBox,
  scaleW: number,
  scaleH: number,
): { x: number; y: number; w: number; h: number } {
  const x = clampNorm(bbox.xMin) * scaleW;
  const y = clampNorm(bbox.yMin) * scaleH;
  const w = (clampNorm(bbox.xMax) - clampNorm(bbox.xMin)) * scaleW;
  const h = (clampNorm(bbox.yMax) - clampNorm(bbox.yMin)) * scaleH;
  return { x, y, w, h };
}

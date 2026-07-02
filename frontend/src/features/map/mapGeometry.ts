export const WORLD_SIZE = 40;
export const NEAR_RADIUS_METRES = 3;
export const MIN_SCALE = 1;
export const MAX_SCALE = 6;
export const ZOOM_STEP = 1.25;
export const DRAG_THRESHOLD_PX = 4;
export const MAP_PADDING = 28;

export interface ViewState {
  scale: number;
  worldCenterX: number;
  worldCenterY: number;
}

/** world metres → layer-local pixels (at scale=1, before layer transform) */
export function worldToStage(
  worldX: number,
  worldY: number,
  pixelsPerMetre: number,
  padding: number,
): { x: number; y: number } {
  return {
    x: padding + worldX * pixelsPerMetre,
    y: padding + (WORLD_SIZE - worldY) * pixelsPerMetre,
  };
}

/** layer-local pixels → world metres */
export function stageToWorld(
  localX: number,
  localY: number,
  pixelsPerMetre: number,
  padding: number,
): { x: number; y: number } {
  return {
    x: (localX - padding) / pixelsPerMetre,
    y: WORLD_SIZE - (localY - padding) / pixelsPerMetre,
  };
}

/** True iff world coordinates are finite and within [0, WORLD_SIZE] */
export function isValidWorldCoord(x: number, y: number): boolean {
  return (
    Number.isFinite(x) &&
    Number.isFinite(y) &&
    x >= 0 &&
    x <= WORLD_SIZE &&
    y >= 0 &&
    y <= WORLD_SIZE
  );
}

/** True iff (photoX, photoY) is within NEAR_RADIUS_METRES of (selX, selY) — inclusive */
export function isNear(
  photoX: number,
  photoY: number,
  selX: number,
  selY: number,
): boolean {
  return Math.hypot(photoX - selX, photoY - selY) <= NEAR_RADIUS_METRES;
}

/** pixels per metre at scale=1 for a given stage size */
export function computePixelsPerMetre(stageW: number, stageH: number, padding: number): number {
  return Math.max(1, (Math.min(stageW, stageH) - 2 * padding) / WORLD_SIZE);
}

/** total local-pixel size of the plot area (including padding on both sides) */
export function totalPlotSize(pixelsPerMetre: number, padding: number): number {
  return 2 * padding + WORLD_SIZE * pixelsPerMetre;
}

export function clampScale(scale: number): number {
  return Math.max(MIN_SCALE, Math.min(MAX_SCALE, scale));
}

export function initialViewState(): ViewState {
  return { scale: 1, worldCenterX: WORLD_SIZE / 2, worldCenterY: WORLD_SIZE / 2 };
}

export function resetViewState(): ViewState {
  return initialViewState();
}

/**
 * Convert a ViewState + stage dimensions to the Konva Layer's x, y, scale.
 * The world centre point appears at the centre of the stage.
 */
export function viewStateToLayerTransform(
  view: ViewState,
  stageW: number,
  stageH: number,
): { x: number; y: number; scale: number } {
  const ppm = computePixelsPerMetre(stageW, stageH, MAP_PADDING);
  const local = worldToStage(view.worldCenterX, view.worldCenterY, ppm, MAP_PADDING);
  return {
    x: stageW / 2 - local.x * view.scale,
    y: stageH / 2 - local.y * view.scale,
    scale: view.scale,
  };
}

/** Apply a screen-pixel pan delta to the view state, clamping to keep the plot visible. */
export function applyPan(
  view: ViewState,
  layerDx: number,
  layerDy: number,
  stageW: number,
  stageH: number,
): ViewState {
  const ppm = computePixelsPerMetre(stageW, stageH, MAP_PADDING);
  const plotSz = totalPlotSize(ppm, MAP_PADDING);
  const lt = viewStateToLayerTransform(view, stageW, stageH);
  const clamped = clampLayerPos(lt.x + layerDx, lt.y + layerDy, view.scale, stageW, stageH, plotSz);
  return layerTransformToViewState(clamped.x, clamped.y, view.scale, stageW, stageH, ppm);
}

/** Apply zoom around a screen pointer position, clamping scale and position. */
export function applyZoomAtPoint(
  view: ViewState,
  factor: number,
  pointerX: number,
  pointerY: number,
  stageW: number,
  stageH: number,
): ViewState {
  const newScale = clampScale(view.scale * factor);
  const ppm = computePixelsPerMetre(stageW, stageH, MAP_PADDING);
  const plotSz = totalPlotSize(ppm, MAP_PADDING);
  const lt = viewStateToLayerTransform(view, stageW, stageH);
  const localX = (pointerX - lt.x) / view.scale;
  const localY = (pointerY - lt.y) / view.scale;
  const rawX = pointerX - localX * newScale;
  const rawY = pointerY - localY * newScale;
  const clamped = clampLayerPos(rawX, rawY, newScale, stageW, stageH, plotSz);
  return layerTransformToViewState(clamped.x, clamped.y, newScale, stageW, stageH, ppm);
}

/** Apply zoom centred on the stage midpoint (for +/- buttons). */
export function applyZoomAtCenter(
  view: ViewState,
  factor: number,
  stageW: number,
  stageH: number,
): ViewState {
  return applyZoomAtPoint(view, factor, stageW / 2, stageH / 2, stageW, stageH);
}

// ─── Pure internal helpers (also used by tests indirectly) ─────────────────

function clampLayerPos(
  x: number,
  y: number,
  scale: number,
  stageW: number,
  stageH: number,
  plotSize: number,
): { x: number; y: number } {
  const scaledSize = plotSize * scale;
  const cx = scaledSize <= stageW
    ? (stageW - scaledSize) / 2
    : Math.min(0, Math.max(stageW - scaledSize, x));
  const cy = scaledSize <= stageH
    ? (stageH - scaledSize) / 2
    : Math.min(0, Math.max(stageH - scaledSize, y));
  return { x: cx, y: cy };
}

function layerTransformToViewState(
  layerX: number,
  layerY: number,
  scale: number,
  stageW: number,
  stageH: number,
  ppm: number,
): ViewState {
  const centerLocalX = (stageW / 2 - layerX) / scale;
  const centerLocalY = (stageH / 2 - layerY) / scale;
  const w = stageToWorld(centerLocalX, centerLocalY, ppm, MAP_PADDING);
  return {
    scale,
    worldCenterX: Math.max(0, Math.min(WORLD_SIZE, w.x)),
    worldCenterY: Math.max(0, Math.min(WORLD_SIZE, w.y)),
  };
}

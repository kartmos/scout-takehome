import { describe, it, expect } from 'vitest';
import {
  WORLD_SIZE,
  NEAR_RADIUS_METRES,
  MIN_SCALE,
  MAX_SCALE,
  ZOOM_STEP,
  MAP_PADDING,
  worldToStage,
  stageToWorld,
  isValidWorldCoord,
  isNear,
  computePixelsPerMetre,
  totalPlotSize,
  clampScale,
  initialViewState,
  resetViewState,
  viewStateToLayerTransform,
  applyPan,
  applyZoomAtPoint,
  applyZoomAtCenter,
} from '../features/map/mapGeometry';

const STAGE_W = 320;
const STAGE_H = 280;
const PPM = computePixelsPerMetre(STAGE_W, STAGE_H, MAP_PADDING);
const TOLERANCE = 1e-9;

// ── worldToStage / stageToWorld ────────────────────────────────────────────

describe('worldToStage', () => {
  it('(0,0) maps to (padding, padding + 40*ppm)', () => {
    const { x, y } = worldToStage(0, 0, PPM, MAP_PADDING);
    expect(x).toBeCloseTo(MAP_PADDING, 6);
    expect(y).toBeCloseTo(MAP_PADDING + WORLD_SIZE * PPM, 6);
  });

  it('(40,40) maps to (padding+40*ppm, padding)', () => {
    const { x, y } = worldToStage(40, 40, PPM, MAP_PADDING);
    expect(x).toBeCloseTo(MAP_PADDING + WORLD_SIZE * PPM, 6);
    expect(y).toBeCloseTo(MAP_PADDING, 6);
  });

  it('interior point (20,20) maps to (padding+20*ppm, padding+20*ppm)', () => {
    const { x, y } = worldToStage(20, 20, PPM, MAP_PADDING);
    expect(x).toBeCloseTo(MAP_PADDING + 20 * PPM, 6);
    expect(y).toBeCloseTo(MAP_PADDING + 20 * PPM, 6);
  });

  it('stage Y decreases as world Y increases (Y is flipped)', () => {
    const low = worldToStage(0, 5, PPM, MAP_PADDING).y;
    const high = worldToStage(0, 35, PPM, MAP_PADDING).y;
    expect(high).toBeLessThan(low);
  });
});

describe('stageToWorld', () => {
  it('round-trips (0,0) within tolerance', () => {
    const local = worldToStage(0, 0, PPM, MAP_PADDING);
    const back = stageToWorld(local.x, local.y, PPM, MAP_PADDING);
    expect(back.x).toBeCloseTo(0, 6);
    expect(back.y).toBeCloseTo(0, 6);
  });

  it('round-trips (40,40)', () => {
    const local = worldToStage(40, 40, PPM, MAP_PADDING);
    const back = stageToWorld(local.x, local.y, PPM, MAP_PADDING);
    expect(back.x).toBeCloseTo(40, 6);
    expect(back.y).toBeCloseTo(40, 6);
  });

  it('round-trips an interior point (12.5, 33.7)', () => {
    const local = worldToStage(12.5, 33.7, PPM, MAP_PADDING);
    const back = stageToWorld(local.x, local.y, PPM, MAP_PADDING);
    expect(back.x).toBeCloseTo(12.5, 6);
    expect(back.y).toBeCloseTo(33.7, 6);
  });
});

// ── isValidWorldCoord ──────────────────────────────────────────────────────

describe('isValidWorldCoord', () => {
  it('accepts corners', () => {
    expect(isValidWorldCoord(0, 0)).toBe(true);
    expect(isValidWorldCoord(40, 40)).toBe(true);
    expect(isValidWorldCoord(0, 40)).toBe(true);
    expect(isValidWorldCoord(40, 0)).toBe(true);
  });

  it('accepts interior', () => {
    expect(isValidWorldCoord(20, 20)).toBe(true);
    expect(isValidWorldCoord(1.5, 38.9)).toBe(true);
  });

  it('rejects coordinates just outside the boundary', () => {
    expect(isValidWorldCoord(-0.001, 20)).toBe(false);
    expect(isValidWorldCoord(40.001, 20)).toBe(false);
    expect(isValidWorldCoord(20, -0.001)).toBe(false);
    expect(isValidWorldCoord(20, 40.001)).toBe(false);
  });

  it('rejects non-finite values', () => {
    expect(isValidWorldCoord(NaN, 20)).toBe(false);
    expect(isValidWorldCoord(20, Infinity)).toBe(false);
    expect(isValidWorldCoord(-Infinity, 20)).toBe(false);
  });
});

// ── isNear ─────────────────────────────────────────────────────────────────

describe('isNear (NEAR_RADIUS_METRES = 3)', () => {
  it('exact same location is near', () => {
    expect(isNear(10, 10, 10, 10)).toBe(true);
  });

  it('exactly 3 m away is near (inclusive boundary)', () => {
    expect(isNear(13, 10, 10, 10)).toBe(true);
  });

  it('just inside boundary (2.999 m) is near', () => {
    expect(isNear(10 + 2.999, 10, 10, 10)).toBe(true);
  });

  it('just outside boundary (3.001 m) is NOT near', () => {
    expect(isNear(10 + 3.001, 10, 10, 10)).toBe(false);
  });

  it('diagonal 3 m is near', () => {
    const d = 3 / Math.SQRT2;
    expect(isNear(10 + d, 10 + d, 10, 10)).toBe(true);
  });

  it('NEAR_RADIUS_METRES constant is 3', () => {
    expect(NEAR_RADIUS_METRES).toBe(3);
  });
});

// ── clampScale ─────────────────────────────────────────────────────────────

describe('clampScale', () => {
  it('clamps below MIN_SCALE', () => {
    expect(clampScale(0)).toBe(MIN_SCALE);
    expect(clampScale(0.5)).toBe(MIN_SCALE);
  });

  it('clamps above MAX_SCALE', () => {
    expect(clampScale(100)).toBe(MAX_SCALE);
    expect(clampScale(7)).toBe(MAX_SCALE);
  });

  it('passes through in-range values', () => {
    expect(clampScale(1)).toBe(1);
    expect(clampScale(3)).toBe(3);
    expect(clampScale(6)).toBe(MAX_SCALE);
  });
});

// ── initialViewState / resetViewState ──────────────────────────────────────

describe('initialViewState', () => {
  it('has scale=1', () => {
    expect(initialViewState().scale).toBe(1);
  });

  it('centres the world (20, 20)', () => {
    const v = initialViewState();
    expect(v.worldCenterX).toBeCloseTo(WORLD_SIZE / 2, 6);
    expect(v.worldCenterY).toBeCloseTo(WORLD_SIZE / 2, 6);
  });

  it('resetViewState equals initialViewState', () => {
    expect(resetViewState()).toEqual(initialViewState());
  });
});

// ── viewStateToLayerTransform ──────────────────────────────────────────────

describe('viewStateToLayerTransform', () => {
  it('at scale=1, world centre appears at stage centre', () => {
    const view = initialViewState();
    const lt = viewStateToLayerTransform(view, STAGE_W, STAGE_H);
    const ppmLocal = computePixelsPerMetre(STAGE_W, STAGE_H, MAP_PADDING);
    const localCenter = worldToStage(view.worldCenterX, view.worldCenterY, ppmLocal, MAP_PADDING);
    const screenX = lt.x + localCenter.x * lt.scale;
    const screenY = lt.y + localCenter.y * lt.scale;
    expect(screenX).toBeCloseTo(STAGE_W / 2, TOLERANCE);
    expect(screenY).toBeCloseTo(STAGE_H / 2, TOLERANCE);
  });

  it('scale equals view.scale', () => {
    const view = { scale: 2.5, worldCenterX: 10, worldCenterY: 30 };
    const lt = viewStateToLayerTransform(view, STAGE_W, STAGE_H);
    expect(lt.scale).toBe(2.5);
  });
});

// ── applyPan ───────────────────────────────────────────────────────────────

describe('applyPan', () => {
  it('zero pan returns a view within valid bounds', () => {
    const view = initialViewState();
    const result = applyPan(view, 0, 0, STAGE_W, STAGE_H);
    expect(result.scale).toBe(view.scale);
    expect(result.worldCenterX).toBeGreaterThanOrEqual(0);
    expect(result.worldCenterX).toBeLessThanOrEqual(WORLD_SIZE);
    expect(result.worldCenterY).toBeGreaterThanOrEqual(0);
    expect(result.worldCenterY).toBeLessThanOrEqual(WORLD_SIZE);
  });

  it('clamping keeps the plane reachable after extreme right pan', () => {
    const view = initialViewState();
    const panned = applyPan(view, 100_000, 0, STAGE_W, STAGE_H);
    // Layer position after clamp should not allow the plot to go completely off the left
    const lt = viewStateToLayerTransform(panned, STAGE_W, STAGE_H);
    const ppmLocal = computePixelsPerMetre(STAGE_W, STAGE_H, MAP_PADDING);
    const plotSz = totalPlotSize(ppmLocal, MAP_PADDING);
    // Right edge of plot must remain on screen
    expect(lt.x + plotSz * lt.scale).toBeGreaterThanOrEqual(0);
  });

  it('clamping keeps the plane reachable after extreme left pan', () => {
    const view = initialViewState();
    const panned = applyPan(view, -100_000, 0, STAGE_W, STAGE_H);
    const lt = viewStateToLayerTransform(panned, STAGE_W, STAGE_H);
    // Left edge of plot must not have gone past the right side of the stage
    expect(lt.x).toBeLessThanOrEqual(STAGE_W);
  });
});

// ── applyZoomAtPoint ───────────────────────────────────────────────────────

describe('applyZoomAtPoint', () => {
  it('zoom in increases scale', () => {
    const view = initialViewState();
    const zoomed = applyZoomAtPoint(view, ZOOM_STEP, STAGE_W / 2, STAGE_H / 2, STAGE_W, STAGE_H);
    expect(zoomed.scale).toBeGreaterThan(view.scale);
  });

  it('zoom out decreases scale', () => {
    const view = { ...initialViewState(), scale: 3 };
    const zoomed = applyZoomAtPoint(view, 1 / ZOOM_STEP, STAGE_W / 2, STAGE_H / 2, STAGE_W, STAGE_H);
    expect(zoomed.scale).toBeLessThan(view.scale);
  });

  it('clamped to MAX_SCALE when zoom factor is large', () => {
    const view = initialViewState();
    const zoomed = applyZoomAtPoint(view, 1000, STAGE_W / 2, STAGE_H / 2, STAGE_W, STAGE_H);
    expect(zoomed.scale).toBe(MAX_SCALE);
  });

  it('clamped to MIN_SCALE when zoom factor is tiny', () => {
    const view = { ...initialViewState(), scale: MAX_SCALE };
    const zoomed = applyZoomAtPoint(view, 0.0001, STAGE_W / 2, STAGE_H / 2, STAGE_W, STAGE_H);
    expect(zoomed.scale).toBe(MIN_SCALE);
  });
});

describe('applyZoomAtCenter', () => {
  it('zoom in at centre keeps world centre identical', () => {
    const view = initialViewState();
    const zoomed = applyZoomAtCenter(view, ZOOM_STEP, STAGE_W, STAGE_H);
    expect(zoomed.worldCenterX).toBeCloseTo(view.worldCenterX, 4);
    expect(zoomed.worldCenterY).toBeCloseTo(view.worldCenterY, 4);
  });
});

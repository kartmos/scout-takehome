import { describe, it, expect } from 'vitest';
import {
  clampNorm,
  isValidBBox,
  bboxToSvgRect,
} from '../shared/lib/bbox';

describe('clampNorm', () => {
  it('passes through values in [0,1]', () => {
    expect(clampNorm(0)).toBe(0);
    expect(clampNorm(0.5)).toBe(0.5);
    expect(clampNorm(1)).toBe(1);
  });
  it('clamps negative to 0', () => expect(clampNorm(-0.1)).toBe(0));
  it('clamps above 1 to 1', () => expect(clampNorm(1.5)).toBe(1));
});

describe('isValidBBox', () => {
  it('accepts a valid bbox', () => {
    expect(isValidBBox({ xMin: 0.1, yMin: 0.2, xMax: 0.8, yMax: 0.9 })).toBe(true);
  });
  it('rejects when xMax <= xMin', () => {
    expect(isValidBBox({ xMin: 0.5, yMin: 0.2, xMax: 0.5, yMax: 0.9 })).toBe(false);
  });
  it('rejects when yMax <= yMin', () => {
    expect(isValidBBox({ xMin: 0.1, yMin: 0.8, xMax: 0.8, yMax: 0.8 })).toBe(false);
  });
  it('rejects non-finite values', () => {
    expect(isValidBBox({ xMin: NaN, yMin: 0, xMax: 1, yMax: 1 })).toBe(false);
    expect(isValidBBox({ xMin: 0, yMin: 0, xMax: Infinity, yMax: 1 })).toBe(false);
  });
});

describe('bboxToSvgRect', () => {
  it('card 1×1 viewBox — full image bbox', () => {
    const r = bboxToSvgRect({ xMin: 0, yMin: 0, xMax: 1, yMax: 1 }, 1, 1);
    expect(r.x).toBe(0);
    expect(r.y).toBe(0);
    expect(r.w).toBe(1);
    expect(r.h).toBe(1);
  });

  it('card 1×1 viewBox — partial bbox', () => {
    const r = bboxToSvgRect({ xMin: 0.1, yMin: 0.2, xMax: 0.8, yMax: 0.9 }, 1, 1);
    expect(r.x).toBeCloseTo(0.1);
    expect(r.y).toBeCloseTo(0.2);
    expect(r.w).toBeCloseTo(0.7);
    expect(r.h).toBeCloseTo(0.7);
  });

  it('viewer native dimensions 800×600', () => {
    const r = bboxToSvgRect({ xMin: 0.1, yMin: 0.2, xMax: 0.8, yMax: 0.9 }, 800, 600);
    expect(r.x).toBeCloseTo(80);
    expect(r.y).toBeCloseTo(120);
    expect(r.w).toBeCloseTo(560);
    expect(r.h).toBeCloseTo(420);
  });

  it('viewer native dimensions 1024×768', () => {
    const r = bboxToSvgRect({ xMin: 0, yMin: 0, xMax: 0.5, yMax: 0.5 }, 1024, 768);
    expect(r.x).toBe(0);
    expect(r.y).toBe(0);
    expect(r.w).toBeCloseTo(512);
    expect(r.h).toBeCloseTo(384);
  });

  it('clamps out-of-range coords (card scale)', () => {
    const r = bboxToSvgRect({ xMin: -0.1, yMin: -0.2, xMax: 1.2, yMax: 1.5 }, 1, 1);
    expect(r.x).toBe(0);
    expect(r.y).toBe(0);
    expect(r.w).toBe(1);
    expect(r.h).toBe(1);
  });

  it('clamps out-of-range coords (native scale)', () => {
    const r = bboxToSvgRect({ xMin: -0.5, yMin: -0.5, xMax: 1.5, yMax: 1.5 }, 800, 600);
    expect(r.x).toBe(0);
    expect(r.y).toBe(0);
    expect(r.w).toBeCloseTo(800);
    expect(r.h).toBeCloseTo(600);
  });

  it('DPR independence — function has no DPR parameter', () => {
    // Same normalized bbox at any DPR produces the same SVG attribute values;
    // the caller scales to the viewBox, not to device pixels.
    const r = bboxToSvgRect({ xMin: 0.25, yMin: 0.25, xMax: 0.75, yMax: 0.75 }, 400, 300);
    expect(r.x).toBeCloseTo(100);
    expect(r.y).toBeCloseTo(75);
    expect(r.w).toBeCloseTo(200);
    expect(r.h).toBeCloseTo(150);
  });
});

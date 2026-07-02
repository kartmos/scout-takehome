import { useRef, useEffect, useState, useCallback, useMemo } from 'react';
import { Stage, Layer, Circle, Line, Text, Group, Image as KonvaImage } from 'react-konva';
import bgSrc from '../../assets/greenhouse-map-background.png';
import type Konva from 'konva';
import type { components } from '../../entities/api/__generated__/schema';
import { CLASS_COLORS, DEFAULT_CLASS_COLOR } from '../../entities/photo/classColors';
import {
  WORLD_SIZE,
  NEAR_RADIUS_METRES,
  DRAG_THRESHOLD_PX,
  MAP_PADDING,
  type ViewState,
  worldToStage,
  stageToWorld,
  isValidWorldCoord,
  isNear,
  computePixelsPerMetre,
  initialViewState,
  resetViewState,
  viewStateToLayerTransform,
  applyPan,
  applyZoomAtPoint,
  applyZoomAtCenter,
  MIN_SCALE,
  MAX_SCALE,
  ZOOM_STEP,
} from './mapGeometry';
import styles from './GreenhouseMap.module.css';

type Photo = components['schemas']['Photo'];

export interface MapLocation {
  x: number;
  y: number;
}

export interface GreenhouseMapProps {
  mode: 'hidden' | 'compact' | 'expanded';
  onModeChange: (mode: 'hidden' | 'compact' | 'expanded') => void;
  selectedLocation: MapLocation | null;
  onSelectLocation: (loc: MapLocation | null) => void;
  highlightedPhotoId: string | null;
  onHighlightPhoto: (id: string | null) => void;
  classId: string | null;
  minConfidence: number | null;
  mapPhotos: Photo[];
  mapFetching: boolean;
  mapError: boolean;
  onRetry: () => void;
  hasMore: boolean;
}

const GRID_TICKS = [0, 5, 10, 15, 20, 25, 30, 35, 40] as const;

const MARKER_OUTER_RADIUS = 10;
const MARKER_INNER_RADIUS = 5;
const NEAR_CIRCLE_STROKE = 1.5;

// Background image
const BG_OPACITY = 0.6;

// Axis label and legend positioning offsets (in world-space pixels, scaled by invScale)
const AXIS_LABEL_X_OFFSET     = 6;  // half-width of label text, centres it on the tick
const AXIS_LABEL_Y_OFFSET     = 3;  // gap below the bottom plot edge
const AXIS_LABEL_LEFT_OFFSET  = 22; // distance left of the plot left edge
const AXIS_LABEL_TOP_OFFSET   = 4;  // half-height of label text, centres it on the tick
const LEGEND_X_HALF_WIDTH     = 15; // half-width of "metres" text
const LEGEND_Y_OFFSET         = 14; // gap below the axis labels

// Grid
const GRID_STROKE_EDGE  = 'rgba(200,200,200,0.3)';
const GRID_STROKE_INNER = 'rgba(200,200,200,0.12)';
const GRID_STROKE_WIDTH = 0.5;

// Axis labels and scale legend
const AXIS_LABEL_FONT_SIZE = 8;
const AXIS_LABEL_FILL      = 'rgba(180,180,180,0.6)';
const LEGEND_FONT_SIZE     = 7;
const LEGEND_FILL          = 'rgba(150,150,150,0.5)';

// Photo markers
const MARKER_HOVER_SCALE            = 1.3;
const MARKER_FILL_ALPHA             = '33'; // hex opacity suffix (~20%)
const MARKER_FILL_HIGHLIGHTED_ALPHA = '66'; // hex opacity suffix (~40%)
const MARKER_STROKE_HIGHLIGHTED     = 3;
const MARKER_STROKE_HOVERED         = 2;
const MARKER_STROKE_NORMAL          = 1.5;

// Selection indicator
const SELECTION_COLOR            = '#4F8EF7';
const SELECTION_FILL             = 'rgba(79,142,247,0.12)';
const SELECTION_STROKE           = 'rgba(79,142,247,0.55)';
const SELECTION_DOT_RADIUS       = 5;
const SELECTION_DOT_STROKE       = '#fff';
const SELECTION_DOT_STROKE_WIDTH = 1.5;

// ─── Colour helper ───────────────────────────────────────────────────────────

function getMarkerColor(photo: Photo, classId: string | null, minConfidence: number | null): string {
  if (classId !== null) return CLASS_COLORS[classId] ?? DEFAULT_CLASS_COLOR;
  const eligible = photo.predictions
    .filter((p) => minConfidence === null || p.confidence >= minConfidence)
    .sort((a, b) => b.confidence - a.confidence);
  if (eligible.length > 0) return CLASS_COLORS[eligible[0]!.classId] ?? DEFAULT_CLASS_COLOR;
  return DEFAULT_CLASS_COLOR;
}

// ─── Konva canvas sub-component ──────────────────────────────────────────────
//
// Receives callbacks from the shell. Compact mode passes parent callbacks
// (immediate apply). Expanded mode passes draft setters (no apply until
// the action bar's Apply button is pressed).

interface KonvaMapCanvasProps {
  viewState: ViewState;
  onViewChange: (v: ViewState) => void;
  selectedLocation: MapLocation | null;
  onSelectLocation: (loc: MapLocation | null) => void;
  highlightedPhotoId: string | null;
  onHighlightPhoto: (id: string | null) => void;
  classId: string | null;
  minConfidence: number | null;
  mapPhotos: Photo[];
  mapFetching: boolean;
  mapError: boolean;
  onRetry: () => void;
  hasMore: boolean;
}

function KonvaMapCanvas({
  viewState,
  onViewChange,
  selectedLocation,
  onSelectLocation,
  highlightedPhotoId,
  onHighlightPhoto,
  classId,
  minConfidence,
  mapPhotos,
  mapFetching,
  mapError,
  onRetry,
  hasMore,
}: KonvaMapCanvasProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const stageRef = useRef<Konva.Stage>(null);
  const [stageSize, setStageSize] = useState({ w: 320, h: 240 });
  const [hoveredId, setHoveredId] = useState<string | null>(null);
  const [listExpanded, setListExpanded] = useState(false);
  const [backgroundImage, setBackgroundImage] = useState<HTMLImageElement | null>(null);

  const viewRef = useRef(viewState);
  viewRef.current = viewState;
  const stageSizeRef = useRef(stageSize);
  stageSizeRef.current = stageSize;

  const mouseStartRef = useRef<{ x: number; y: number } | null>(null);
  const lastMousePosRef = useRef<{ x: number; y: number } | null>(null);
  const isDraggingRef = useRef(false);

  // Load greenhouse floor-plan background image; failure leaves map functional
  useEffect(() => {
    let cancelled = false;
    const img = new window.Image();
    img.onload = () => {
      if (!cancelled) setBackgroundImage(img);
    };
    img.onerror = () => {};
    img.src = bgSrc;
    return () => {
      cancelled = true;
      img.onload = null;
      img.onerror = null;
    };
  }, []);

  // ResizeObserver — update stage dimensions and re-clamp transform
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (!entry) return;
      const { width, height } = entry.contentRect;
      if (width > 0 && height > 0) {
        const newSize = { w: Math.floor(width), h: Math.floor(height) };
        setStageSize(newSize);
        onViewChange(applyPan(viewRef.current, 0, 0, newSize.w, newSize.h));
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
    // onViewChange intentionally omitted — stable callback via useCallback in parent
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const { w: stageW, h: stageH } = stageSize;
  const ppm = computePixelsPerMetre(stageW, stageH, MAP_PADDING);
  const layerTransform = viewStateToLayerTransform(viewState, stageW, stageH);
  const scale = layerTransform.scale;
  const invScale = scale > 0 ? 1 / scale : 1;

  const validPhotos = useMemo(
    () => mapPhotos.filter((p) => isValidWorldCoord(p.x, p.y)),
    [mapPhotos],
  );
  const omittedCount = mapPhotos.length - validPhotos.length;

  // ── Event helpers ─────────────────────────────────────────────────────────

  function getPointerPos() {
    return stageRef.current?.getPointerPosition() ?? null;
  }

  // ── Pan/drag handling ─────────────────────────────────────────────────────

  const handleMouseDown = useCallback((e: Konva.KonvaEventObject<MouseEvent>) => {
    void e;
    isDraggingRef.current = false;
    const pos = getPointerPos();
    if (!pos) return;
    mouseStartRef.current = pos;
    lastMousePosRef.current = pos;
  }, []);

  const handleMouseMove = useCallback((e: Konva.KonvaEventObject<MouseEvent>) => {
    void e;
    if (!mouseStartRef.current) return;
    const pos = getPointerPos();
    if (!pos) return;
    const dx = pos.x - mouseStartRef.current.x;
    const dy = pos.y - mouseStartRef.current.y;
    if (!isDraggingRef.current && Math.hypot(dx, dy) > DRAG_THRESHOLD_PX) {
      isDraggingRef.current = true;
    }
    if (isDraggingRef.current && lastMousePosRef.current) {
      const moveDx = pos.x - lastMousePosRef.current.x;
      const moveDy = pos.y - lastMousePosRef.current.y;
      const ss = stageSizeRef.current;
      onViewChange(applyPan(viewRef.current, moveDx, moveDy, ss.w, ss.h));
    }
    lastMousePosRef.current = pos;
  }, [onViewChange]);

  const handleMouseUp = useCallback((e: Konva.KonvaEventObject<MouseEvent>) => {
    void e;
    mouseStartRef.current = null;
    lastMousePosRef.current = null;
    // NOTE: isDraggingRef.current is NOT reset here; it must persist until onClick fires
  }, []);

  // ── Background click (fires only when a shape does not cancel bubbling) ───

  const handleStageClick = useCallback((e: Konva.KonvaEventObject<MouseEvent>) => {
    const wasDragging = isDraggingRef.current;
    isDraggingRef.current = false;
    if (wasDragging) return;
    if (e.target !== (stageRef.current as unknown as Konva.Node)) return; // shape click
    const pos = getPointerPos();
    if (!pos) return;
    const t = viewRef.current;
    const ss = stageSizeRef.current;
    const lt = viewStateToLayerTransform(t, ss.w, ss.h);
    const localX = (pos.x - lt.x) / t.scale;
    const localY = (pos.y - lt.y) / t.scale;
    const ppmLocal = computePixelsPerMetre(ss.w, ss.h, MAP_PADDING);
    const world = stageToWorld(localX, localY, ppmLocal, MAP_PADDING);
    if (isValidWorldCoord(world.x, world.y)) {
      onSelectLocation({ x: world.x, y: world.y });
      onHighlightPhoto(null);
    }
  }, [onSelectLocation, onHighlightPhoto]);

  // ── Wheel zoom ────────────────────────────────────────────────────────────

  const handleWheel = useCallback((e: Konva.KonvaEventObject<WheelEvent>) => {
    e.evt.preventDefault();
    const pos = getPointerPos();
    if (!pos) return;
    const factor = e.evt.deltaY < 0 ? ZOOM_STEP : 1 / ZOOM_STEP;
    const ss = stageSizeRef.current;
    onViewChange(applyZoomAtPoint(viewRef.current, factor, pos.x, pos.y, ss.w, ss.h));
  }, [onViewChange]);

  // ── Zoom button handlers ──────────────────────────────────────────────────

  const handleZoomIn = useCallback(() => {
    const ss = stageSizeRef.current;
    onViewChange(applyZoomAtCenter(viewRef.current, ZOOM_STEP, ss.w, ss.h));
  }, [onViewChange]);

  const handleZoomOut = useCallback(() => {
    const ss = stageSizeRef.current;
    onViewChange(applyZoomAtCenter(viewRef.current, 1 / ZOOM_STEP, ss.w, ss.h));
  }, [onViewChange]);

  const handleReset = useCallback(() => {
    onViewChange(resetViewState());
  }, [onViewChange]);

  // ── Marker click ──────────────────────────────────────────────────────────

  function makeMarkerClickHandler(photo: Photo) {
    return (e: Konva.KonvaEventObject<MouseEvent>) => {
      e.cancelBubble = true;
      const wasDragging = isDraggingRef.current;
      isDraggingRef.current = false;
      if (wasDragging) return;
      onSelectLocation({ x: photo.x, y: photo.y });
      onHighlightPhoto(photo.id);
    };
  }

  // ── Accessible DOM list ───────────────────────────────────────────────────

  const handleListSelect = useCallback((photo: Photo) => {
    onSelectLocation({ x: photo.x, y: photo.y });
    onHighlightPhoto(photo.id);
  }, [onSelectLocation, onHighlightPhoto]);

  // ── Derived geometry ──────────────────────────────────────────────────────

  const plotTop = MAP_PADDING;
  const plotLeft = MAP_PADDING;
  const plotSizePx = WORLD_SIZE * ppm;

  return (
    <div className={styles.canvasContainer}>
      {/* Konva canvas */}
      <div ref={containerRef} className={styles.stageWrapper} data-testid="map-stage-wrapper">
        {stageW > 0 && stageH > 0 && (
          <Stage
            ref={stageRef}
            width={stageW}
            height={stageH}
            onMouseDown={handleMouseDown}
            onMouseMove={handleMouseMove}
            onMouseUp={handleMouseUp}
            onClick={handleStageClick}
            onWheel={handleWheel}
            data-testid="konva-stage"
          >
            <Layer
              x={layerTransform.x}
              y={layerTransform.y}
              scaleX={scale}
              scaleY={scale}
            >
              {/* Greenhouse floor-plan background — 60% opacity, follows zoom/pan */}
              {backgroundImage !== null && (
                <KonvaImage
                  image={backgroundImage}
                  x={plotLeft}
                  y={plotTop}
                  width={plotSizePx}
                  height={plotSizePx}
                  opacity={BG_OPACITY}
                  listening={false}
                />
              )}

              {/* Grid lines (vertical + horizontal per tick) */}
              {GRID_TICKS.map((m) => {
                const sx = worldToStage(m, 0, ppm, MAP_PADDING).x;
                const hy = worldToStage(0, m, ppm, MAP_PADDING).y;
                const stroke = (m === 0 || m === WORLD_SIZE)
                  ? GRID_STROKE_EDGE
                  : GRID_STROKE_INNER;
                const sw = GRID_STROKE_WIDTH * invScale;
                return (
                  <Group key={`g${m}`}>
                    <Line
                      points={[sx, plotTop, sx, plotTop + plotSizePx]}
                      stroke={stroke}
                      strokeWidth={sw}
                      listening={false}
                    />
                    <Line
                      points={[plotLeft, hy, plotLeft + plotSizePx, hy]}
                      stroke={stroke}
                      strokeWidth={sw}
                      listening={false}
                    />
                  </Group>
                );
              })}

              {/* Axis labels at inner ticks */}
              {GRID_TICKS.filter((m) => m > 0 && m < WORLD_SIZE).map((m) => {
                const sx = worldToStage(m, 0, ppm, MAP_PADDING).x;
                const hy = worldToStage(0, m, ppm, MAP_PADDING).y;
                const fs = AXIS_LABEL_FONT_SIZE * invScale;
                return (
                  <Group key={`label${m}`}>
                    <Text
                      x={sx - AXIS_LABEL_X_OFFSET * invScale}
                      y={plotTop + plotSizePx + AXIS_LABEL_Y_OFFSET * invScale}
                      text={String(m)}
                      fontSize={fs}
                      fill={AXIS_LABEL_FILL}
                      listening={false}
                    />
                    <Text
                      x={plotLeft - AXIS_LABEL_LEFT_OFFSET * invScale}
                      y={hy - AXIS_LABEL_TOP_OFFSET * invScale}
                      text={String(m)}
                      fontSize={fs}
                      fill={AXIS_LABEL_FILL}
                      listening={false}
                    />
                  </Group>
                );
              })}

              {/* Metres legend */}
              <Text
                x={plotLeft + plotSizePx / 2 - LEGEND_X_HALF_WIDTH * invScale}
                y={plotTop + plotSizePx + LEGEND_Y_OFFSET * invScale}
                text="metres"
                fontSize={LEGEND_FONT_SIZE * invScale}
                fill={LEGEND_FILL}
                listening={false}
              />

              {/* Photo markers — positioned from real x,y */}
              {validPhotos.map((photo) => {
                const local = worldToStage(photo.x, photo.y, ppm, MAP_PADDING);
                const color = getMarkerColor(photo, classId, minConfidence);
                const isHovered = hoveredId === photo.id;
                const isHighlighted = highlightedPhotoId === photo.id;
                const outerR = (isHovered ? MARKER_OUTER_RADIUS * MARKER_HOVER_SCALE : MARKER_OUTER_RADIUS) * invScale;
                return (
                  <Group
                    key={photo.id}
                    onClick={makeMarkerClickHandler(photo)}
                    onMouseEnter={() => setHoveredId(photo.id)}
                    onMouseLeave={() => setHoveredId(null)}
                  >
                    <Circle
                      x={local.x}
                      y={local.y}
                      radius={outerR}
                      fill={isHighlighted ? `${color}${MARKER_FILL_HIGHLIGHTED_ALPHA}` : `${color}${MARKER_FILL_ALPHA}`}
                      stroke={color}
                      strokeWidth={(isHighlighted ? MARKER_STROKE_HIGHLIGHTED : isHovered ? MARKER_STROKE_HOVERED : MARKER_STROKE_NORMAL) * invScale}
                    />
                    <Circle
                      x={local.x}
                      y={local.y}
                      radius={MARKER_INNER_RADIUS * invScale}
                      fill={color}
                      listening={false}
                    />
                  </Group>
                );
              })}

              {/* Selection radius circle + centre dot */}
              {selectedLocation !== null && (() => {
                const local = worldToStage(selectedLocation.x, selectedLocation.y, ppm, MAP_PADDING);
                return (
                  <Group>
                    <Circle
                      x={local.x}
                      y={local.y}
                      radius={NEAR_RADIUS_METRES * ppm}
                      fill={SELECTION_FILL}
                      stroke={SELECTION_STROKE}
                      strokeWidth={NEAR_CIRCLE_STROKE * invScale}
                      listening={false}
                    />
                    <Circle
                      x={local.x}
                      y={local.y}
                      radius={SELECTION_DOT_RADIUS * invScale}
                      fill={SELECTION_COLOR}
                      stroke={SELECTION_DOT_STROKE}
                      strokeWidth={SELECTION_DOT_STROKE_WIDTH * invScale}
                      listening={false}
                    />
                  </Group>
                );
              })()}
            </Layer>
          </Stage>
        )}

        {mapFetching && validPhotos.length === 0 && !mapError && (
          <div className={styles.canvasOverlay} role="status">Loading map data…</div>
        )}
        {mapError && (
          <div className={styles.canvasOverlay} role="alert">
            <p>Failed to load map data.</p>
            <button type="button" className={styles.retryBtn} onClick={onRetry}>Retry</button>
          </div>
        )}
      </div>

      {/* Zoom controls with coordinate readout */}
      <div className={styles.zoomControls} role="group" aria-label="Map zoom controls">
        <span className={styles.coordReadout} aria-live="polite" aria-atomic="true">
          {selectedLocation !== null
            ? `x ${selectedLocation.x.toFixed(1)} m · y ${selectedLocation.y.toFixed(1)} m`
            : 'Location not selected'}
        </span>
        <button
          type="button"
          className={styles.zoomBtn}
          aria-label="Zoom in"
          onClick={handleZoomIn}
          disabled={viewState.scale >= MAX_SCALE}
        >
          +
        </button>
        <button
          type="button"
          className={styles.zoomBtn}
          aria-label="Zoom out"
          onClick={handleZoomOut}
          disabled={viewState.scale <= MIN_SCALE}
        >
          −
        </button>
        <button
          type="button"
          className={styles.zoomBtn}
          aria-label="Reset view"
          onClick={handleReset}
        >
          ⊙
        </button>
      </div>

      {/* Map status line */}
      <p className={styles.mapMeta} aria-live="polite" aria-atomic="true">
        {validPhotos.length} marker{validPhotos.length !== 1 ? 's' : ''}
        {omittedCount > 0 && ` · ${omittedCount} omitted (invalid coords)`}
        {hasMore && ' · Showing first 200 matching photos'}
      </p>

      {/* Accessible DOM marker list — collapsed by default in both modes */}
      <div className={styles.locationListWrapper}>
        <button
          type="button"
          className={styles.listToggle}
          aria-expanded={listExpanded}
          aria-controls="map-location-list"
          onClick={() => setListExpanded((v) => !v)}
        >
          Markers ({validPhotos.length}) {listExpanded ? '▲' : '▼'}
        </button>
        {listExpanded && (
          <ul
            id="map-location-list"
            className={styles.locationList}
            aria-label="Photo locations"
          >
            {validPhotos.map((photo) => {
              const isNearSelected = selectedLocation !== null &&
                isNear(photo.x, photo.y, selectedLocation.x, selectedLocation.y);
              const capturedLabel = new Date(photo.capturedAt).toLocaleString();
              return (
                <li
                  key={photo.id}
                  className={`${styles.locationItem} ${isNearSelected ? styles.locationItemNear : ''}`}
                >
                  <button
                    type="button"
                    className={styles.locationBtn}
                    title={`Captured: ${capturedLabel} · height ${photo.h.toFixed(1)} m · ${photo.predictions.length} prediction(s)`}
                    onClick={() => handleListSelect(photo)}
                  >
                    x&nbsp;{photo.x.toFixed(1)}&nbsp;m, y&nbsp;{photo.y.toFixed(1)}&nbsp;m
                  </button>
                </li>
              );
            })}
            {validPhotos.length === 0 && !mapFetching && !mapError && (
              <li className={styles.locationEmpty}>No photos in this view.</li>
            )}
          </ul>
        )}
      </div>
    </div>
  );
}

// ─── Drawer shell ─────────────────────────────────────────────────────────────

export function GreenhouseMap({
  mode,
  onModeChange,
  selectedLocation,
  onSelectLocation,
  highlightedPhotoId,
  onHighlightPhoto,
  classId,
  minConfidence,
  mapPhotos,
  mapFetching,
  mapError,
  onRetry,
  hasMore,
}: GreenhouseMapProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const expandBtnRef = useRef<HTMLButtonElement>(null);
  const launcherRef = useRef<HTMLButtonElement>(null);
  const modeAfterClose = useRef<'compact' | 'hidden'>('compact');
  // Tracks whether dialog close is triggered by Apply (prevents Cancel overwriting Apply)
  const pendingApplyRef = useRef(false);
  const prevModeRef = useRef(mode);

  // Draft state: local candidate while in expanded mode
  const [draftLocation, setDraftLocation] = useState<MapLocation | null>(null);
  const [draftHighlightedId, setDraftHighlightedId] = useState<string | null>(null);

  const [viewState, setViewState] = useState<ViewState>(initialViewState);

  // Initialize draft from currently applied location when entering expanded mode.
  // Intentionally excludes selectedLocation/highlightedPhotoId from deps so that
  // filter changes during expanded mode do not reset the draft mid-session.
  useEffect(() => {
    if (mode === 'expanded') {
      setDraftLocation(selectedLocation);
      setDraftHighlightedId(highlightedPhotoId);
      modeAfterClose.current = 'compact';
      if (dialogRef.current && !dialogRef.current.open) {
        dialogRef.current.showModal();
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode]);

  // Focus management: return focus to an appropriate control after dialog closes
  useEffect(() => {
    const prev = prevModeRef.current;
    prevModeRef.current = mode;
    if (prev === 'expanded') {
      if (mode === 'compact') expandBtnRef.current?.focus();
      else if (mode === 'hidden') launcherRef.current?.focus();
    }
  }, [mode]);

  const handleViewChange = useCallback((v: ViewState) => setViewState(v), []);

  // Stable draft setters typed to match KonvaMapCanvasProps callbacks
  const handleDraftLocationChange = useCallback((loc: MapLocation | null) => {
    setDraftLocation(loc);
  }, []);
  const handleDraftHighlightChange = useCallback((id: string | null) => {
    setDraftHighlightedId(id);
  }, []);

  // Preview count for the action bar: derived from map data + draft position
  const draftNearCount = useMemo(() => {
    if (draftLocation === null) return 0;
    return mapPhotos.filter(
      (p) => isValidWorldCoord(p.x, p.y) && isNear(p.x, p.y, draftLocation.x, draftLocation.y),
    ).length;
  }, [draftLocation, mapPhotos]);

  // ── Expanded-mode action handlers ─────────────────────────────────────────

  const handleApply = useCallback(() => {
    if (draftLocation === null) return;
    pendingApplyRef.current = true;
    onSelectLocation(draftLocation);
    onHighlightPhoto(draftHighlightedId);
    modeAfterClose.current = 'compact';
    dialogRef.current?.close();
  }, [draftLocation, draftHighlightedId, onSelectLocation, onHighlightPhoto]);

  const handleCancelOrReturn = useCallback(() => {
    modeAfterClose.current = 'compact';
    dialogRef.current?.close();
  }, []);

  const handleHideFromExpanded = useCallback(() => {
    modeAfterClose.current = 'hidden';
    dialogRef.current?.close();
  }, []);

  const handleDialogClose = useCallback(() => {
    pendingApplyRef.current = false;
    onModeChange(modeAfterClose.current);
  }, [onModeChange]);

  // ── Canvas prop sets ──────────────────────────────────────────────────────

  const baseCanvasProps = {
    viewState,
    onViewChange: handleViewChange,
    classId,
    minConfidence,
    mapPhotos,
    mapFetching,
    mapError,
    onRetry,
    hasMore,
  };

  const compactCanvasProps: KonvaMapCanvasProps = {
    ...baseCanvasProps,
    selectedLocation,
    onSelectLocation,
    highlightedPhotoId,
    onHighlightPhoto,
  };

  // Expanded canvas uses draft state instead of applied state
  const expandedCanvasProps: KonvaMapCanvasProps = {
    ...baseCanvasProps,
    selectedLocation: draftLocation,
    onSelectLocation: handleDraftLocationChange,
    highlightedPhotoId: draftHighlightedId,
    onHighlightPhoto: handleDraftHighlightChange,
  };

  // ── Hidden: launcher button ───────────────────────────────────────────────

  if (mode === 'hidden') {
    return (
      <button
        ref={launcherRef}
        type="button"
        className={styles.launcher}
        aria-label={
          selectedLocation !== null
            ? `Show greenhouse map, location x ${selectedLocation.x.toFixed(1)} m y ${selectedLocation.y.toFixed(1)} m active`
            : 'Show greenhouse map'
        }
        onClick={() => onModeChange('compact')}
      >
        Greenhouse map
        {selectedLocation !== null && (
          <span className={styles.launcherBadge} aria-hidden="true">●</span>
        )}
      </button>
    );
  }

  // ── Compact: fixed bottom-right drawer — immediate selection ─────────────

  if (mode === 'compact') {
    return (
      <div className={styles.compact} role="region" aria-label="Greenhouse map">
        <div className={styles.drawerHeader}>
          <span className={styles.drawerTitle}>Greenhouse map</span>
          <button
            ref={expandBtnRef}
            type="button"
            className={styles.headerBtn}
            aria-label="Expand map"
            onClick={() => onModeChange('expanded')}
          >
            Expand
          </button>
          <button
            type="button"
            className={styles.headerBtn}
            aria-label="Hide map"
            onClick={() => onModeChange('hidden')}
          >
            Hide
          </button>
        </div>
        <KonvaMapCanvas {...compactCanvasProps} />
      </div>
    );
  }

  // ── Expanded: native dialog — draft + confirm workflow ───────────────────

  return (
    <dialog
      ref={dialogRef}
      className={styles.expandedDialog}
      aria-label="Greenhouse map"
      onClose={handleDialogClose}
    >
      <div className={styles.drawerHeader}>
        <span className={styles.drawerTitle}>Greenhouse map</span>
        <button
          type="button"
          className={styles.headerBtn}
          aria-label="Return to mini-map"
          onClick={handleCancelOrReturn}
        >
          Mini-map
        </button>
        <button
          type="button"
          className={styles.headerBtn}
          aria-label="Hide map"
          onClick={handleHideFromExpanded}
        >
          Hide
        </button>
      </div>
      <div className={styles.dialogBody}>
        <KonvaMapCanvas {...expandedCanvasProps} />
      </div>
      <div className={styles.actionBar}>
        <button
          type="button"
          className={styles.headerBtn}
          onClick={handleCancelOrReturn}
        >
          Cancel
        </button>
        <span className={styles.previewCount} aria-live="polite" aria-atomic="true">
          {draftLocation !== null
            ? `${draftNearCount} photo${draftNearCount !== 1 ? 's' : ''} within ${NEAR_RADIUS_METRES} m`
            : 'Select a location on the map'}
        </span>
        <button
          type="button"
          className={styles.applyBtn}
          onClick={handleApply}
          disabled={draftLocation === null}
        >
          Apply location
        </button>
      </div>
    </dialog>
  );
}

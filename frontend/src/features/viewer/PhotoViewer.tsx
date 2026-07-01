import { useCallback, useEffect, useRef, useState } from 'react';
import type { components } from '../../entities/api/__generated__/schema';
import { CLASS_COLORS, CLASS_LABELS, DEFAULT_CLASS_COLOR } from '../../entities/photo/classColors';
import { filterPredictions, groupPredictions } from '../../shared/lib/predictions';
import { isValidBBox, bboxToSvgRect } from '../../shared/lib/bbox';
import { useGetPhotoQuery } from '../../shared/api/scoutApi';
import styles from './PhotoViewer.module.css';

type Photo = components['schemas']['Photo'];
type Prediction = components['schemas']['Prediction'];

export interface PhotoViewerProps {
  photos: Photo[];
  initialIndex: number;
  matchingClassId: string | null;
  minConfidence?: number | null;
  triggerEl: HTMLElement | null;
  onClose: () => void;
}

interface NumberedPred {
  pred: Prediction;
  index: number;
}

interface GroupWithRows {
  classId: string;
  count: number;
  rows: NumberedPred[];
}

function buildGroupedRows(predictions: Prediction[]): GroupWithRows[] {
  const groups = groupPredictions(predictions);
  let counter = 0;
  return groups.map((g) => ({
    classId: g.classId,
    count: g.count,
    rows: g.predictions.map((pred) => ({ pred, index: counter++ })),
  }));
}

function ViewerBBoxOverlay({
  allPreds,
  matchingClassId,
  photoW,
  photoH,
  hiddenPreds,
  highlightedPred,
}: {
  allPreds: NumberedPred[];
  matchingClassId: string | null;
  photoW: number;
  photoH: number;
  hiddenPreds: Set<number>;
  highlightedPred: number | null;
}) {
  const labelSize = Math.round(Math.min(photoW, photoH) * 0.03);
  return (
    <svg
      className={styles.overlay}
      viewBox={`0 0 ${photoW} ${photoH}`}
      preserveAspectRatio="xMidYMid meet"
      aria-hidden="true"
    >
      {allPreds.map(({ pred, index }) => {
        if (hiddenPreds.has(index)) return null;
        if (!isValidBBox(pred.bbox)) return null;
        const { x, y, w, h } = bboxToSvgRect(pred.bbox, photoW, photoH);
        if (w <= 0 || h <= 0) return null;
        const color = CLASS_COLORS[pred.classId] ?? DEFAULT_CLASS_COLOR;
        const isMatch = matchingClassId == null || pred.classId === matchingClassId;
        let opacity: number;
        if (highlightedPred !== null) {
          opacity = highlightedPred === index ? 1 : 0.15;
        } else {
          opacity = isMatch ? 1 : 0.3;
        }
        const fillColor =
          highlightedPred === index || (highlightedPred === null && isMatch)
            ? `${color}1A`
            : 'none';
        // Badge dimensions — pill above (or below) the bbox, never touching it.
        const numStr = String(index + 1);
        const badgeH = Math.round(labelSize * 1.35);
        const badgeW = Math.round(numStr.length * labelSize * 0.65 + labelSize * 0.5);
        const badgeGap = Math.round(labelSize * 0.35); // gap between badge and bbox edge
        const badgeX = Math.round(x + 1);
        // Prefer above; fall back to below when bbox is near the top.
        const badgeY = y >= badgeH + badgeGap
          ? Math.round(y - badgeGap - badgeH)
          : Math.round(y + h + badgeGap);
        return (
          <g key={index} opacity={opacity}>
            <rect
              x={x} y={y} width={w} height={h}
              fill="none"
              stroke="rgba(0,0,0,0.55)"
              strokeWidth="3"
              vectorEffect="non-scaling-stroke"
            />
            <rect
              x={x} y={y} width={w} height={h}
              fill={fillColor}
              stroke={color}
              strokeWidth="2"
              vectorEffect="non-scaling-stroke"
            />
            <rect
              x={badgeX}
              y={badgeY}
              width={badgeW}
              height={badgeH}
              rx={Math.round(labelSize * 0.25)}
              fill="rgba(0,0,0,0.78)"
            />
            <text
              x={badgeX + badgeW / 2}
              y={badgeY + Math.round(badgeH * 0.76)}
              textAnchor="middle"
              fontSize={labelSize}
              fontWeight="bold"
              fill="#fff"
            >
              {numStr}
            </text>
          </g>
        );
      })}
    </svg>
  );
}

function ViewerSidebar({
  groupedRows,
  matchingClassId,
  hiddenPreds,
  onToggle,
  onHighlight,
  onResetVisibility,
  isThresholdFiltered,
}: {
  groupedRows: GroupWithRows[];
  matchingClassId: string | null;
  hiddenPreds: Set<number>;
  onToggle: (index: number) => void;
  onHighlight: (index: number | null) => void;
  onResetVisibility: () => void;
  isThresholdFiltered?: boolean;
}) {
  const totalPreds = groupedRows.reduce((sum, g) => sum + g.rows.length, 0);
  if (totalPreds === 0) {
    return (
      <p className={styles.noPredictions}>
        {isThresholdFiltered ? 'No detections at this confidence' : 'No detections'}
      </p>
    );
  }

  const anyHidden = hiddenPreds.size > 0;

  return (
    <div className={styles.predGroups}>
      {anyHidden && (
        <button
          type="button"
          className={styles.showAllBtn}
          onClick={onResetVisibility}
        >
          Show all
        </button>
      )}
      {groupedRows.map((group) => {
        const color = CLASS_COLORS[group.classId] ?? DEFAULT_CLASS_COLOR;
        const label = CLASS_LABELS[group.classId] ?? group.classId;
        const isClassMatch = matchingClassId == null || group.classId === matchingClassId;

        return (
          <div
            key={group.classId}
            className={`${styles.predGroup} ${isClassMatch ? styles.predGroupMatch : ''}`}
          >
            <div className={styles.predGroupHeader} aria-hidden="true">
              <span
                className={styles.predSwatch}
                style={{ background: color }}
                aria-hidden="true"
              />
              <span className={styles.predGroupCount}>{group.count}×</span>
              <span className={styles.predGroupLabel}>{label}</span>
            </div>
            <ul className={styles.predGroupItems}>
              {group.rows.map(({ pred, index }) => {
                const conf = Math.round(pred.confidence * 100);
                const valid = isValidBBox(pred.bbox);
                const hidden = hiddenPreds.has(index);
                const num = index + 1;
                return (
                  <li key={index} className={styles.predGroupItem}>
                    <button
                      type="button"
                      className={`${styles.predToggleBtn} ${hidden ? styles.predToggleBtnHidden : ''} ${!valid ? styles.predToggleBtnInvalid : ''}`}
                      aria-label={
                        valid
                          ? `Toggle detection ${num}, ${label} at ${conf} percent confidence`
                          : `Detection ${num}, ${label} at ${conf} percent confidence, no bounding box`
                      }
                      aria-pressed={valid ? !hidden : undefined}
                      disabled={!valid}
                      onClick={() => valid && onToggle(index)}
                      onMouseEnter={() => { if (valid && !hidden) onHighlight(index); }}
                      onMouseLeave={() => onHighlight(null)}
                      onFocus={() => { if (valid && !hidden) onHighlight(index); }}
                      onBlur={() => onHighlight(null)}
                    >
                      <span className={styles.predNum}>{num}</span>
                      <span className={styles.predConf}>{conf}%</span>
                      {!valid && (
                        <span className={styles.predNoBox} aria-hidden="true">
                          no bbox
                        </span>
                      )}
                    </button>
                  </li>
                );
              })}
            </ul>
          </div>
        );
      })}
    </div>
  );
}

export function PhotoViewer({
  photos,
  initialIndex,
  matchingClassId,
  minConfidence,
  triggerEl,
  onClose,
}: PhotoViewerProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [currentIndex, setCurrentIndex] = useState(() =>
    Math.max(0, Math.min(initialIndex, photos.length - 1)),
  );
  const [imgState, setImgState] = useState<'loading' | 'loaded' | 'error'>('loading');
  const [imgRevision, setImgRevision] = useState(0);
  const [isRetrying, setIsRetrying] = useState(false);
  const retryTokenRef = useRef(0);

  // Per-photo local visibility and transient highlight state — never in Redux.
  const [hiddenPreds, setHiddenPreds] = useState<Set<number>>(new Set());
  const [highlightedPred, setHighlightedPred] = useState<number | null>(null);

  const navPhoto = photos[currentIndex] ?? null;
  const currentPhotoId = navPhoto?.id ?? '';
  const canPrev = currentIndex > 0;
  const canNext = currentIndex < photos.length - 1;

  const activeMinConf = minConfidence ?? null;

  const {
    currentData: freshPhoto,
    isError: isFetchError,
    refetch,
  } = useGetPhotoQuery(currentPhotoId, {
    skip: !currentPhotoId,
    refetchOnMountOrArgChange: true,
  });

  const handleClose = useCallback(() => {
    triggerEl?.focus();
    onClose();
  }, [triggerEl, onClose]);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (!dialog.open) dialog.showModal();
    const onNativeClose = () => {
      triggerEl?.focus();
      onClose();
    };
    dialog.addEventListener('close', onNativeClose);
    return () => {
      dialog.removeEventListener('close', onNativeClose);
    };
  }, [onClose, triggerEl]);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
      if (e.key === 'ArrowLeft' && canPrev) {
        e.preventDefault();
        setCurrentIndex((i) => i - 1);
      } else if (e.key === 'ArrowRight' && canNext) {
        e.preventDefault();
        setCurrentIndex((i) => i + 1);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [canPrev, canNext]);

  // Reset image state and local visibility/highlight when navigating to another photo.
  useEffect(() => {
    retryTokenRef.current++;
    setImgState('loading');
    setImgRevision(0);
    setIsRetrying(false);
    setHiddenPreds(new Set());
    setHighlightedPred(null);
  }, [currentPhotoId]);

  // Reset visibility state when the confidence threshold changes.
  useEffect(() => {
    setHiddenPreds(new Set());
    setHighlightedPred(null);
  }, [activeMinConf]);

  useEffect(() => {
    if (!navPhoto) handleClose();
  }, [navPhoto, handleClose]);

  if (!navPhoto) return null;

  const capturedDate = new Date(navPhoto.capturedAt).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });

  const isApiLoading = !freshPhoto && !isFetchError;
  const isApiErr = !freshPhoto && isFetchError;

  const handleRetry = async () => {
    if (isRetrying) return;
    const token = ++retryTokenRef.current;
    setIsRetrying(true);
    try {
      await refetch();
      if (retryTokenRef.current === token) {
        setImgRevision((r) => r + 1);
        setImgState('loading');
        setIsRetrying(false);
      }
    } catch {
      if (retryTokenRef.current === token) {
        setImgState('error');
        setIsRetrying(false);
      }
    }
  };

  // Filter to visible predictions, then group and number them.
  const visiblePreds = freshPhoto ? filterPredictions(freshPhoto.predictions, activeMinConf) : [];
  const isThresholdFiltered = freshPhoto != null && freshPhoto.predictions.length > 0 && visiblePreds.length === 0;
  const groupedRows = freshPhoto ? buildGroupedRows(visiblePreds) : [];
  const allPreds = groupedRows.flatMap((g) => g.rows);

  const togglePred = (index: number) => {
    setHighlightedPred(null);
    setHiddenPreds((prev) => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  };

  return (
    <dialog
      ref={dialogRef}
      className={styles.dialog}
      aria-modal="true"
      aria-labelledby="viewer-title"
    >
      <div className={styles.header}>
        <h2 id="viewer-title" className={styles.title}>
          <time dateTime={navPhoto.capturedAt}>{capturedDate}</time>
        </h2>
        <button
          type="button"
          className={styles.closeBtn}
          onClick={handleClose}
          aria-label="Close photo viewer"
          autoFocus
        >
          ✕
        </button>
      </div>

      <div className={styles.body}>
        <div className={styles.mediaWrap}>
          {(isRetrying || isApiLoading) && (
            <div className={styles.imgLoading} aria-label="Loading image" role="status" />
          )}
          {!isRetrying && isApiErr && (
            <div className={styles.imgError} role="alert">
              <p className={styles.imgErrorMsg}>Failed to load photo.</p>
              <button type="button" className={styles.retryBtn} onClick={handleRetry}>
                Retry
              </button>
            </div>
          )}
          {!isRetrying && freshPhoto && (
            imgState === 'error' ? (
              <div className={styles.imgError} role="alert">
                <p className={styles.imgErrorMsg}>Image failed to load.</p>
                <button type="button" className={styles.retryBtn} onClick={handleRetry}>
                  Retry
                </button>
              </div>
            ) : (
              <>
                {imgState === 'loading' && (
                  <div className={styles.imgLoading} aria-label="Loading image" role="status" />
                )}
                <img
                  key={`${currentPhotoId}-${imgRevision}`}
                  src={freshPhoto.originalUrl}
                  alt={`Greenhouse photo captured ${capturedDate}`}
                  className={`${styles.img} ${imgState === 'loaded' ? styles.imgVisible : styles.imgHidden}`}
                  onLoad={() => setImgState('loaded')}
                  onError={() => setImgState('error')}
                />
                {imgState === 'loaded' && allPreds.length > 0 && (
                  <ViewerBBoxOverlay
                    allPreds={allPreds}
                    matchingClassId={matchingClassId}
                    photoW={freshPhoto.width}
                    photoH={freshPhoto.height}
                    hiddenPreds={hiddenPreds}
                    highlightedPred={highlightedPred}
                  />
                )}
              </>
            )
          )}
        </div>

        <aside className={styles.sidebar} aria-label="Photo details">
          {freshPhoto != null ? (
            <ViewerSidebar
              groupedRows={groupedRows}
              matchingClassId={matchingClassId}
              hiddenPreds={hiddenPreds}
              onToggle={togglePred}
              onHighlight={setHighlightedPred}
              onResetVisibility={() => setHiddenPreds(new Set())}
              isThresholdFiltered={isThresholdFiltered}
            />
          ) : isApiLoading || isRetrying ? (
            <p
              className={styles.noPredictions}
              role="status"
              aria-label="Loading detections"
            >
              Loading…
            </p>
          ) : null}
        </aside>
      </div>

      <div className={styles.footer}>
        <button
          type="button"
          className={styles.navBtn}
          onClick={() => setCurrentIndex((i) => i - 1)}
          disabled={!canPrev}
          aria-label="Previous photo"
        >
          ← Previous
        </button>
        <span className={styles.pageInfo} aria-live="polite" aria-atomic="true">
          {currentIndex + 1} / {photos.length}
        </span>
        <button
          type="button"
          className={styles.navBtn}
          onClick={() => setCurrentIndex((i) => i + 1)}
          disabled={!canNext}
          aria-label="Next photo"
        >
          Next →
        </button>
      </div>
    </dialog>
  );
}

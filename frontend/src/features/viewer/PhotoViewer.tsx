import { useCallback, useEffect, useRef, useState } from 'react';
import type { components } from '../../entities/api/__generated__/schema';
import { CLASS_COLORS, CLASS_LABELS, DEFAULT_CLASS_COLOR } from '../../entities/photo/classColors';
import { isValidBBox, bboxToSvgRect } from '../../shared/lib/bbox';
import styles from './PhotoViewer.module.css';

type Photo = components['schemas']['Photo'];
type Prediction = components['schemas']['Prediction'];

export interface PhotoViewerProps {
  photos: Photo[];
  initialIndex: number;
  matchingClassId: string | null;
  triggerEl: HTMLElement | null;
  onClose: () => void;
}

function ViewerBBoxOverlay({
  predictions,
  matchingClassId,
  photoW,
  photoH,
}: {
  predictions: Prediction[];
  matchingClassId: string | null;
  photoW: number;
  photoH: number;
}) {
  return (
    // viewBox matches the photo's native aspect ratio; preserveAspectRatio="xMidYMid meet"
    // mirrors object-fit:contain so bbox rects align with the contained image.
    <svg
      className={styles.overlay}
      viewBox={`0 0 ${photoW} ${photoH}`}
      preserveAspectRatio="xMidYMid meet"
      aria-hidden="true"
    >
      {predictions.map((pred, i) => {
        if (!isValidBBox(pred.bbox)) return null;
        const { x, y, w, h } = bboxToSvgRect(pred.bbox, photoW, photoH);
        if (w <= 0 || h <= 0) return null;
        const color = CLASS_COLORS[pred.classId] ?? DEFAULT_CLASS_COLOR;
        const isMatch = matchingClassId == null || pred.classId === matchingClassId;
        return (
          <rect
            key={i}
            x={x}
            y={y}
            width={w}
            height={h}
            fill={isMatch ? `${color}1A` : 'none'}
            stroke={color}
            strokeWidth="3"
            vectorEffect="non-scaling-stroke"
            opacity={isMatch ? 1 : 0.3}
          />
        );
      })}
    </svg>
  );
}

function PredictionList({
  predictions,
  matchingClassId,
}: {
  predictions: Prediction[];
  matchingClassId: string | null;
}) {
  if (predictions.length === 0) {
    return <p className={styles.noPredictions}>No detections</p>;
  }

  const sorted = matchingClassId
    ? [...predictions].sort(
        (a, b) =>
          (b.classId === matchingClassId ? 1 : 0) - (a.classId === matchingClassId ? 1 : 0),
      )
    : predictions;

  return (
    <ul className={styles.predList} aria-label="Detections">
      {sorted.map((pred, i) => {
        const color = CLASS_COLORS[pred.classId] ?? DEFAULT_CLASS_COLOR;
        const label = CLASS_LABELS[pred.classId] ?? pred.classId;
        const conf = Math.round(pred.confidence * 100);
        const isMatch = matchingClassId == null || pred.classId === matchingClassId;
        return (
          <li key={i} className={`${styles.predItem} ${isMatch ? styles.predItemMatch : ''}`}>
            <span
              className={styles.predSwatch}
              style={{ background: color }}
              aria-hidden="true"
            />
            <span className={styles.predClass}>{label}</span>
            <span className={styles.predConf} aria-label={`${conf} percent confidence`}>
              {conf}%
            </span>
          </li>
        );
      })}
    </ul>
  );
}

export function PhotoViewer({
  photos,
  initialIndex,
  matchingClassId,
  triggerEl,
  onClose,
}: PhotoViewerProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [currentIndex, setCurrentIndex] = useState(() =>
    Math.max(0, Math.min(initialIndex, photos.length - 1)),
  );
  const [imgState, setImgState] = useState<'loading' | 'loaded' | 'error'>('loading');
  const [retryKey, setRetryKey] = useState(0);

  const photo = photos[currentIndex] ?? null;
  const canPrev = currentIndex > 0;
  const canNext = currentIndex < photos.length - 1;

  const handleClose = useCallback(() => {
    triggerEl?.focus();
    onClose();
  }, [triggerEl, onClose]);

  // Open modal; listen to native close event (fires on Escape)
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

  // Keyboard navigation (← →); skip when focus is in an input
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

  // Reset image state when navigating to a different photo
  useEffect(() => {
    setImgState('loading');
    setRetryKey(0);
  }, [photo?.id]);

  // Close if the current photo has been removed (page/filter change)
  useEffect(() => {
    if (!photo) handleClose();
  }, [photo, handleClose]);

  if (!photo) return null;

  const capturedDate = new Date(photo.capturedAt).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });

  return (
    <dialog
      ref={dialogRef}
      className={styles.dialog}
      aria-modal="true"
      aria-labelledby="viewer-title"
    >
      <div className={styles.header}>
        <h2 id="viewer-title" className={styles.title}>
          <time dateTime={photo.capturedAt}>{capturedDate}</time>
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
          {imgState === 'error' ? (
            <div className={styles.imgError} role="alert">
              <p className={styles.imgErrorMsg}>Image failed to load.</p>
              <button
                type="button"
                className={styles.retryBtn}
                onClick={() => {
                  setImgState('loading');
                  setRetryKey((k) => k + 1);
                }}
              >
                Retry
              </button>
            </div>
          ) : (
            <>
              {imgState === 'loading' && (
                <div className={styles.imgLoading} aria-label="Loading image" role="status" />
              )}
              <img
                key={retryKey}
                src={photo.originalUrl}
                alt={`Greenhouse photo captured ${capturedDate}`}
                className={`${styles.img} ${imgState === 'loaded' ? styles.imgVisible : styles.imgHidden}`}
                onLoad={() => setImgState('loaded')}
                onError={() => setImgState('error')}
              />
              {imgState === 'loaded' && photo.predictions.length > 0 && (
                <ViewerBBoxOverlay
                  predictions={photo.predictions}
                  matchingClassId={matchingClassId}
                  photoW={photo.width}
                  photoH={photo.height}
                />
              )}
            </>
          )}
        </div>

        <aside className={styles.sidebar} aria-label="Photo details">
          <PredictionList predictions={photo.predictions} matchingClassId={matchingClassId} />
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

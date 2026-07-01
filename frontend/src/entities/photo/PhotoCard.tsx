import { useState } from 'react';
import type { components } from '../api/__generated__/schema';
import { buildThumbnailCandidates, buildSrcSet, CARD_SIZES } from '../../shared/lib/thumbnailCandidates';
import { isCanonicalPhotoId } from '../../shared/api/thumbnail';
import { isValidBBox, bboxToSvgRect } from '../../shared/lib/bbox';
import { CLASS_COLORS, CLASS_LABELS, DEFAULT_CLASS_COLOR } from './classColors';
import styles from './PhotoCard.module.css';

type Photo = components['schemas']['Photo'];
type Prediction = components['schemas']['Prediction'];

interface BBoxOverlayProps {
  predictions: Prediction[];
  matchingClassId: string | null;
}

function BBoxOverlay({ predictions, matchingClassId }: BBoxOverlayProps) {
  return (
    <svg
      className={styles.overlay}
      viewBox="0 0 1 1"
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      {predictions.map((pred, i) => {
        if (!isValidBBox(pred.bbox)) return null;
        const { x, y, w, h } = bboxToSvgRect(pred.bbox, 1, 1);
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
            strokeWidth="2"
            vectorEffect="non-scaling-stroke"
            opacity={isMatch ? 1 : 0.35}
          />
        );
      })}
    </svg>
  );
}

export interface PhotoCardProps {
  photo: Photo;
  matchingClassId?: string | null;
  onSelect?: (photo: Photo, trigger: HTMLButtonElement) => void;
}

export function PhotoCard({ photo, matchingClassId, onSelect }: PhotoCardProps) {
  const [imgError, setImgError] = useState(false);
  const [retryKey, setRetryKey] = useState(0);

  const isInvalidId = !isCanonicalPhotoId(photo.id);
  const candidates = isInvalidId ? [] : buildThumbnailCandidates(photo.id);
  const srcset = buildSrcSet(candidates);
  const src = candidates[0]?.url ?? '';
  const aspectRatio = `${photo.width} / ${photo.height}`;

  const capturedDate = new Date(photo.capturedAt).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });

  const activeClassId = matchingClassId ?? null;
  const sortedPredictions = activeClassId
    ? [...photo.predictions].sort(
        (a, b) =>
          (b.classId === activeClassId ? 1 : 0) - (a.classId === activeClassId ? 1 : 0),
      )
    : photo.predictions;

  return (
    <article className={styles.card}>
      <div className={styles.media} style={{ aspectRatio }}>
        {isInvalidId ? (
          // Invalid metadata: static unavailable, no retry (retry cannot repair bad IDs)
          <div className={styles.imgError}>
            <span className={styles.imgErrorMsg}>Image unavailable</span>
          </div>
        ) : imgError ? (
          // Network failure: show unavailable with retry
          <div className={styles.imgError}>
            <span className={styles.imgErrorMsg}>Image unavailable</span>
            <button
              type="button"
              onClick={() => {
                setImgError(false);
                setRetryKey((k) => k + 1);
              }}
              className={styles.imgRetryBtn}
            >
              Retry
            </button>
          </div>
        ) : (
          <>
            <img
              key={retryKey}
              src={src}
              srcSet={srcset}
              sizes={CARD_SIZES}
              alt={`Greenhouse photo captured ${capturedDate}`}
              width={photo.width}
              height={photo.height}
              loading="lazy"
              decoding="async"
              onError={() => setImgError(true)}
              className={styles.img}
            />
            {photo.predictions.length > 0 && (
              <BBoxOverlay predictions={photo.predictions} matchingClassId={activeClassId} />
            )}
            {onSelect && (
              <button
                type="button"
                className={styles.selectBtn}
                onClick={(e) => onSelect(photo, e.currentTarget)}
                aria-label={`Open photo from ${capturedDate}`}
              />
            )}
          </>
        )}
      </div>

      <footer className={styles.caption}>
        <time className={styles.capturedAt} dateTime={photo.capturedAt}>
          {capturedDate}
        </time>
        {sortedPredictions.length === 0 ? (
          <span className={styles.noPredictions}>No detections</span>
        ) : (
          <ul className={styles.predList} aria-label="Detections">
            {sortedPredictions.map((pred, i) => {
              const color = CLASS_COLORS[pred.classId] ?? DEFAULT_CLASS_COLOR;
              const isMatch = matchingClassId == null || pred.classId === matchingClassId;
              return (
                <li
                  key={i}
                  className={`${styles.predItem} ${isMatch ? styles.predItemMatch : ''}`}
                >
                  <span
                    className={styles.predDot}
                    style={{ background: color }}
                    aria-hidden="true"
                  />
                  <span className={styles.predClass}>
                    {CLASS_LABELS[pred.classId] ?? pred.classId}
                  </span>
                  <span className={styles.predConf}>
                    {Math.round(pred.confidence * 100)}%
                  </span>
                </li>
              );
            })}
          </ul>
        )}
      </footer>
    </article>
  );
}

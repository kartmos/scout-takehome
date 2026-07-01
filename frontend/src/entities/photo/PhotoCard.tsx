import { useState } from 'react';
import type { components } from '../api/__generated__/schema';
import { buildThumbnailCandidates, buildSrcSet, CARD_SIZES } from '../../shared/lib/thumbnailCandidates';
import { isCanonicalPhotoId } from '../../shared/api/thumbnail';
import { isValidBBox, bboxToSvgRect } from '../../shared/lib/bbox';
import { filterPredictions, groupPredictions } from '../../shared/lib/predictions';
import { CLASS_COLORS, CLASS_LABELS, DEFAULT_CLASS_COLOR } from './classColors';
import styles from './PhotoCard.module.css';

type Photo = components['schemas']['Photo'];
type Prediction = components['schemas']['Prediction'];

interface BBoxOverlayProps {
  predictions: Prediction[];
  matchingClassId: string | null;
  photoW: number;
  photoH: number;
}

function BBoxOverlay({ predictions, matchingClassId, photoW, photoH }: BBoxOverlayProps) {
  return (
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
        const opacity = isMatch ? 1 : 0.35;
        const fillColor = isMatch ? `${color}1A` : 'none';
        return (
          <g key={i} opacity={opacity}>
            <rect
              x={x} y={y} width={w} height={h}
              fill="none"
              stroke="rgba(0,0,0,0.55)"
              strokeWidth="1.25"
              vectorEffect="non-scaling-stroke"
            />
            <rect
              x={x} y={y} width={w} height={h}
              fill={fillColor}
              stroke={color}
              strokeWidth="0.75"
              vectorEffect="non-scaling-stroke"
            />
          </g>
        );
      })}
    </svg>
  );
}

export interface PhotoCardProps {
  photo: Photo;
  matchingClassId?: string | null;
  minConfidence?: number | null;
  onSelect?: (photo: Photo, trigger: HTMLButtonElement) => void;
}

export function PhotoCard({ photo, matchingClassId, minConfidence, onSelect }: PhotoCardProps) {
  const [imgError, setImgError] = useState(false);
  const [retryKey, setRetryKey] = useState(0);

  const isInvalidId = !isCanonicalPhotoId(photo.id);
  const candidates = isInvalidId ? [] : buildThumbnailCandidates(photo.id);
  const srcset = buildSrcSet(candidates);
  const src = candidates[0]?.url ?? '';

  const capturedDate = new Date(photo.capturedAt).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });

  const activeClassId = matchingClassId ?? null;
  const activeMinConf = minConfidence ?? null;

  const visiblePredictions = filterPredictions(photo.predictions, activeMinConf);

  // Group and sort alphabetically by display label
  const groups = groupPredictions(visiblePredictions);
  const sortedGroups = [...groups].sort((a, b) => {
    const la = CLASS_LABELS[a.classId] ?? a.classId;
    const lb = CLASS_LABELS[b.classId] ?? b.classId;
    return la.localeCompare(lb);
  });

  const hasVisiblePredictions = sortedGroups.length > 0;
  const isThresholdFiltered = !hasVisiblePredictions && photo.predictions.length > 0;

  return (
    <article className={styles.card}>
      <div className={styles.media}>
        {isInvalidId ? (
          <div className={styles.imgError}>
            <span className={styles.imgErrorMsg}>Image unavailable</span>
          </div>
        ) : imgError ? (
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
            {visiblePredictions.length > 0 && (
              <BBoxOverlay
                predictions={visiblePredictions}
                matchingClassId={activeClassId}
                photoW={photo.width}
                photoH={photo.height}
              />
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
        <ul className={styles.predList} aria-label="Detections">
          {!hasVisiblePredictions ? (
            <li className={styles.noDetections}>
              {isThresholdFiltered ? 'No detections at this confidence' : 'No detections'}
            </li>
          ) : (
            sortedGroups.map((group) => {
              const color = CLASS_COLORS[group.classId] ?? DEFAULT_CLASS_COLOR;
              const isMatch = activeClassId == null || group.classId === activeClassId;
              const label = CLASS_LABELS[group.classId] ?? group.classId;
              const confLabel =
                group.minConf === group.maxConf
                  ? `${group.minConf}%`
                  : `${group.minConf}–${group.maxConf}%`;
              return (
                <li
                  key={group.classId}
                  className={`${styles.predItem} ${isMatch ? styles.predItemMatch : ''}`}
                >
                  <span className={styles.predDot} style={{ background: color }} aria-hidden="true" />
                  <span className={styles.predCount} aria-hidden="true">{group.count}×</span>
                  <span className={styles.predClass}>{label}</span>
                  <span className={styles.predConf}>{confLabel}</span>
                </li>
              );
            })
          )}
        </ul>
      </footer>
    </article>
  );
}

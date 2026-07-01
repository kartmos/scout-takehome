import { useEffect, useId, useRef, useState } from 'react';
import { useAppDispatch, useAppSelector } from '../../app/hooks';
import {
  setClassId,
  setMinConfidence,
  resetFilters,
  CLASS_IDS,
  CLASS_LABELS,
  type ClassId,
} from './filtersSlice';
import styles from './FilterControls.module.css';

export function FilterControls() {
  const dispatch = useAppDispatch();
  const classId = useAppSelector((state) => state.filters.classId);
  const minConfidence = useAppSelector((state) => state.filters.minConfidence);

  const [confidenceInput, setConfidenceInput] = useState('');
  const confidenceId = useId();
  // Tracks the last ratio committed to the store so that a blur followed
  // immediately by a click (before React re-renders) doesn't dispatch twice.
  const committedRef = useRef<number | null>(minConfidence);

  // Sync the text field when minConfidence is cleared or set externally (e.g. resetFilters).
  // Display as integer percentage: 0.7 → "70".
  useEffect(() => {
    committedRef.current = minConfidence;
    setConfidenceInput(minConfidence !== null ? String(Math.round(minConfidence * 100)) : '');
  }, [minConfidence]);

  const applyConfidence = () => {
    const trimmed = confidenceInput.trim();
    if (trimmed === '') {
      if (committedRef.current !== null) {
        committedRef.current = null;
        dispatch(setMinConfidence(null));
      }
      return;
    }
    const pct = Number(trimmed);
    if (Number.isFinite(pct)) {
      const clamped = Math.max(0, Math.min(100, pct));
      const ratio = clamped / 100;
      // Always normalize the visible field to the clamped value, even when Redux
      // already holds the same ratio (e.g. input 150 → display 100, no dispatch).
      setConfidenceInput(String(clamped));
      if (ratio !== committedRef.current) {
        committedRef.current = ratio;
        dispatch(setMinConfidence(ratio));
      }
    }
  };

  const handleReset = () => {
    dispatch(resetFilters());
  };

  const hasActiveFilters = classId !== null || minConfidence !== null;

  return (
    <div className={styles.controls}>
      <fieldset className={styles.fieldset}>
        <legend className={styles.legend}>Class</legend>
        <div className={styles.classButtons}>
          <button
            type="button"
            className={classId === null ? styles.classBtnActive : styles.classBtn}
            onClick={() => dispatch(setClassId(null))}
            aria-pressed={classId === null}
          >
            All
          </button>
          {CLASS_IDS.map((id: ClassId) => (
            <button
              key={id}
              type="button"
              className={classId === id ? styles.classBtnActive : styles.classBtn}
              onClick={() => dispatch(setClassId(id))}
              aria-pressed={classId === id}
            >
              {CLASS_LABELS[id]}
            </button>
          ))}
        </div>
      </fieldset>

      <div className={styles.confidenceGroup}>
        <label htmlFor={confidenceId} className={styles.label}>
          Minimum confidence (%)
        </label>
        <div className={styles.confidenceRow}>
          <input
            id={confidenceId}
            type="number"
            min="0"
            max="100"
            step="1"
            placeholder="0 – 100"
            value={confidenceInput}
            onChange={(e) => setConfidenceInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && applyConfidence()}
            onBlur={applyConfidence}
            className={styles.confidenceInput}
          />
          <span aria-hidden="true" className={styles.pctLabel}>%</span>
          <button type="button" onClick={applyConfidence} className={styles.applyBtn}>
            Apply
          </button>
        </div>
      </div>

      {hasActiveFilters && (
        <div className={styles.activeSummary}>
          <span className={styles.activeSummaryText}>
            {[
              classId !== null ? CLASS_LABELS[classId] : null,
              minConfidence !== null ? `≥${Math.round(minConfidence * 100)}%` : null,
            ]
              .filter(Boolean)
              .join(' · ')}
          </span>
          <button type="button" onClick={handleReset} className={styles.resetBtn}>
            Clear filters
          </button>
        </div>
      )}
    </div>
  );
}

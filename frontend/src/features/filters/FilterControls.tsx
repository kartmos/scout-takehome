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

const PRESET_PERCENTS = [10, 20, 30, 40, 50, 60, 70, 80, 90, 100] as const;

function ratioToPreset(ratio: number | null): string | null {
  if (ratio === null) return 'any';
  const pct = Math.round(ratio * 100);
  if ((PRESET_PERCENTS as readonly number[]).includes(pct)) return String(pct);
  return null;
}

export function FilterControls() {
  const dispatch = useAppDispatch();
  const classId = useAppSelector((state) => state.filters.classId);
  const minConfidence = useAppSelector((state) => state.filters.minConfidence);

  const [selectVal, setSelectVal] = useState<string>(() => ratioToPreset(minConfidence) ?? 'custom');
  const [customInput, setCustomInput] = useState<string>(() =>
    minConfidence !== null && ratioToPreset(minConfidence) == null
      ? String(Math.round(minConfidence * 100))
      : '',
  );
  const selectId = useId();
  const customId = useId();
  // Tracks the last ratio committed to the store to prevent duplicate dispatches.
  const committedRef = useRef<number | null>(minConfidence);

  // Sync UI when Redux state changes externally (e.g., resetFilters or external dispatch).
  useEffect(() => {
    committedRef.current = minConfidence;
    const preset = ratioToPreset(minConfidence);
    if (preset !== null) {
      setSelectVal(preset);
      setCustomInput('');
    } else {
      setSelectVal('custom');
      setCustomInput(minConfidence !== null ? String(Math.round(minConfidence * 100)) : '');
    }
  }, [minConfidence]);

  const handleSelectChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const val = e.target.value;
    setSelectVal(val);
    if (val === 'any') {
      setCustomInput('');
      if (committedRef.current !== null) {
        committedRef.current = null;
        dispatch(setMinConfidence(null));
      }
    } else if (val === 'custom') {
      // Pre-populate from current Redux value when it's already a non-preset.
      if (minConfidence !== null) {
        setCustomInput(String(Math.round(minConfidence * 100)));
      } else {
        setCustomInput('');
      }
    } else {
      const pct = Number(val);
      const ratio = pct / 100;
      setCustomInput('');
      if (ratio !== committedRef.current) {
        committedRef.current = ratio;
        dispatch(setMinConfidence(ratio));
      }
    }
  };

  const applyCustomConfidence = () => {
    const trimmed = customInput.trim();
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
      setCustomInput(String(clamped));
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
        <label htmlFor={selectId} className={styles.label}>
          Minimum confidence
        </label>
        <div className={styles.confidenceRow}>
          <select
            id={selectId}
            value={selectVal}
            onChange={handleSelectChange}
            className={styles.confidenceSelect}
          >
            <option value="any">Any — no filter</option>
            {PRESET_PERCENTS.map((p) => (
              <option key={p} value={String(p)}>
                {p}%
              </option>
            ))}
            <option value="custom">Custom value…</option>
          </select>
          {selectVal === 'custom' && (
            <>
              <input
                id={customId}
                type="number"
                min="0"
                max="100"
                step="1"
                placeholder="0 – 100"
                value={customInput}
                onChange={(e) => setCustomInput(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && applyCustomConfidence()}
                onBlur={applyCustomConfidence}
                className={styles.confidenceInput}
                aria-label="Custom confidence percentage"
              />
              <span aria-hidden="true" className={styles.pctLabel}>%</span>
              <button type="button" onClick={applyCustomConfidence} className={styles.applyBtn}>
                Apply
              </button>
            </>
          )}
        </div>
      </div>

      {hasActiveFilters && (
        <div className={styles.activeFilterGroup}>
          <div className={styles.activeSummary}>
            <span className={styles.activeSummaryText}>
              {[
                classId !== null ? CLASS_LABELS[classId] : null,
                minConfidence !== null ? `≥${Math.round(minConfidence * 100)}%` : null,
              ]
                .filter(Boolean)
                .join(' · ')}
            </span>
          </div>
          <button type="button" onClick={handleReset} className={styles.resetBtn}>
            <span aria-hidden="true">× </span>Clear filters
          </button>
        </div>
      )}
    </div>
  );
}

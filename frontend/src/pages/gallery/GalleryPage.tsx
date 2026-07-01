import { useState } from 'react';
import { useAppDispatch, useAppSelector } from '../../app/hooks';
import { resetFilters } from '../../features/filters/filtersSlice';
import { FilterControls } from '../../features/filters/FilterControls';
import { PhotoViewer } from '../../features/viewer/PhotoViewer';
import { useListPhotosQuery } from '../../shared/api/scoutApi';
import { PhotoCard } from '../../entities/photo/PhotoCard';
import type { ApiError } from '../../shared/lib/apiError';
import type { components } from '../../entities/api/__generated__/schema';
import styles from './GalleryPage.module.css';

type Photo = components['schemas']['Photo'];

const PAGE_SIZE = 24;
const SKELETON_COUNT = 12;

function isApiError(err: unknown): err is ApiError {
  return typeof err === 'object' && err !== null && 'message' in err;
}

function ErrorPanel({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const msg = isApiError(error) ? error.message : 'An unexpected error occurred.';
  const isAuth = isApiError(error) && error.status === 401;
  return (
    <div className={styles.errorPanel} role="alert">
      <p className={styles.errorMsg}>{msg}</p>
      {isAuth ? (
        <p className={styles.errorHint}>Check your API key in the .env file.</p>
      ) : (
        <button type="button" onClick={onRetry} className={styles.retryBtn}>
          Retry
        </button>
      )}
    </div>
  );
}

function EmptyPanel({
  hasFilters,
  onReset,
}: {
  hasFilters: boolean;
  onReset: () => void;
}) {
  return (
    <div className={styles.emptyPanel}>
      <p className={styles.emptyMsg}>
        {hasFilters ? 'No photos match these filters.' : 'No photos found.'}
      </p>
      {hasFilters && (
        <button type="button" onClick={onReset} className={styles.retryBtn}>
          Clear filters
        </button>
      )}
    </div>
  );
}

function SkeletonGrid() {
  return (
    <div className={styles.grid} aria-busy="true" aria-label="Loading photos">
      {Array.from({ length: SKELETON_COUNT }).map((_, i) => (
        <div key={i} className={styles.skeleton} aria-hidden="true" />
      ))}
    </div>
  );
}

export function GalleryPage() {
  const dispatch = useAppDispatch();
  const classId = useAppSelector((state) => state.filters.classId);
  const minConfidence = useAppSelector((state) => state.filters.minConfidence);

  // Atomic filter-change detection: reset pagination on filter change during render
  const [prevClassId, setPrevClassId] = useState<string | null>(classId);
  const [prevMinConf, setPrevMinConf] = useState<number | null>(minConfidence);
  const [pageIndex, setPageIndex] = useState(0);
  const [cursorHistory, setCursorHistory] = useState<(string | undefined)[]>([undefined]);

  let effectivePageIndex = pageIndex;
  let effectiveCursors = cursorHistory;
  if (classId !== prevClassId || minConfidence !== prevMinConf) {
    setPrevClassId(classId);
    setPrevMinConf(minConfidence);
    setPageIndex(0);
    setCursorHistory([undefined]);
    effectivePageIndex = 0;
    effectiveCursors = [undefined];
  }

  const cursor = effectiveCursors[effectivePageIndex];

  // Build args without undefined values — required by exactOptionalPropertyTypes
  const queryArgs: { limit: number; cursor?: string; classId?: string; minConfidence?: number } =
    { limit: PAGE_SIZE };
  if (cursor !== undefined) queryArgs.cursor = cursor;
  if (classId !== null) queryArgs.classId = classId;
  if (minConfidence !== null) queryArgs.minConfidence = minConfidence;

  // currentData is undefined until the active query args have been fulfilled once.
  // Unlike `data`, it never carries results from a previous page or filter set.
  const { currentData, isFetching, isError, error, refetch } =
    useListPhotosQuery(queryArgs);

  // Viewer selection: track by ID so we can detect page/filter invalidation.
  // viewerPhotoIndex === -1 when the photo is no longer in currentData (filter/page change),
  // which naturally collapses isViewerOpen without needing an explicit close effect.
  const [selectedPhotoId, setSelectedPhotoId] = useState<string | null>(null);
  const [viewerTrigger, setViewerTrigger] = useState<HTMLElement | null>(null);

  const viewerPhotoIndex =
    selectedPhotoId && currentData
      ? currentData.items.findIndex((p) => p.id === selectedPhotoId)
      : -1;

  const isViewerOpen = viewerPhotoIndex !== -1;

  const handleSelect = (photo: Photo, trigger: HTMLButtonElement) => {
    setSelectedPhotoId(photo.id);
    setViewerTrigger(trigger);
  };

  const handleViewerClose = () => {
    setSelectedPhotoId(null);
    setViewerTrigger(null);
  };

  const handleNext = () => {
    if (!currentData?.next_token || isFetching) return;
    const token = currentData.next_token;
    setCursorHistory((prev) => {
      const next = [...prev];
      if (next.length <= effectivePageIndex + 1) next.push(token);
      return next;
    });
    setPageIndex(effectivePageIndex + 1);
  };

  const handlePrev = () => {
    if (effectivePageIndex === 0 || isFetching) return;
    setPageIndex(effectivePageIndex - 1);
  };

  const hasActiveFilters = classId !== null || minConfidence !== null;
  // currentData is undefined until the active query args are fulfilled → no stale page shown.
  const showGrid = !!currentData && currentData.items.length > 0;
  const showEmpty = !isError && !!currentData && currentData.items.length === 0;
  // Background-refetch: current args fulfilled; a new fetch of the same key is in progress.
  const isBackgroundFetch = !!currentData && isFetching;
  // Initial fetch: current args not yet fulfilled (no cached result for these args).
  const isInitialFetch = !currentData && isFetching && !isError;

  return (
    <div className={styles.page}>
      <FilterControls />

      <div className={styles.content}>
        {isBackgroundFetch && (
          <div className={styles.fetchingBar} role="status" aria-label="Updating gallery" />
        )}

        {isInitialFetch && <SkeletonGrid />}

        {isError && <ErrorPanel error={error} onRetry={refetch} />}

        {showEmpty && (
          <EmptyPanel
            hasFilters={hasActiveFilters}
            onReset={() => dispatch(resetFilters())}
          />
        )}

        {showGrid && (
          <div
            className={styles.grid}
            aria-busy={isBackgroundFetch}
            aria-label="Photo gallery"
          >
            {currentData.items.map((photo) => (
              <PhotoCard
                key={photo.id}
                photo={photo}
                matchingClassId={classId}
                minConfidence={minConfidence}
                onSelect={handleSelect}
              />
            ))}
          </div>
        )}

        {(showGrid || showEmpty) && (
          <nav className={styles.pagination} aria-label="Gallery pagination">
            <button
              type="button"
              onClick={handlePrev}
              disabled={effectivePageIndex === 0 || isFetching}
              className={styles.pageBtn}
            >
              Previous
            </button>
            <span className={styles.pageInfo} aria-live="polite" aria-atomic="true">
              Page {effectivePageIndex + 1}
              {!currentData?.next_token && currentData ? ' (last)' : ''}
            </span>
            <button
              type="button"
              onClick={handleNext}
              disabled={!currentData?.next_token || isFetching}
              className={styles.pageBtn}
            >
              Next
            </button>
          </nav>
        )}
      </div>

      {isViewerOpen && currentData && (
        <PhotoViewer
          photos={currentData.items}
          initialIndex={viewerPhotoIndex}
          matchingClassId={classId}
          minConfidence={minConfidence}
          triggerEl={viewerTrigger}
          onClose={handleViewerClose}
        />
      )}
    </div>
  );
}

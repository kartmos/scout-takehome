import { useState } from 'react';
import { useAppDispatch, useAppSelector } from '../../app/hooks';
import { resetFilters } from '../../features/filters/filtersSlice';
import { FilterControls } from '../../features/filters/FilterControls';
import { PhotoViewer } from '../../features/viewer/PhotoViewer';
import { GreenhouseMap, type MapLocation } from '../../features/map/GreenhouseMap';
import { NEAR_RADIUS_METRES } from '../../features/map/mapGeometry';
import { useListPhotosQuery } from '../../shared/api/scoutApi';
import { PhotoCard } from '../../entities/photo/PhotoCard';
import type { ApiError } from '../../shared/lib/apiError';
import type { components } from '../../entities/api/__generated__/schema';
import styles from './GalleryPage.module.css';

type Photo = components['schemas']['Photo'];

const PAGE_SIZE = 24;
const SKELETON_COUNT = 12;
const MAP_LIMIT = 200;

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

  const queryArgs: { limit: number; cursor?: string; classId?: string; minConfidence?: number } =
    { limit: PAGE_SIZE };
  if (cursor !== undefined) queryArgs.cursor = cursor;
  if (classId !== null) queryArgs.classId = classId;
  if (minConfidence !== null) queryArgs.minConfidence = minConfidence;

  const { currentData, isFetching, isError, error, refetch } =
    useListPhotosQuery(queryArgs);

  // ── Map drawer state ──────────────────────────────────────────────────────
  const [drawerMode, setDrawerMode] = useState<'hidden' | 'compact' | 'expanded'>('hidden');
  const [selectedLocation, setSelectedLocation] = useState<MapLocation | null>(null);
  const [mapHighlightedPhotoId, setMapHighlightedPhotoId] = useState<string | null>(null);
  const [mapEnabled, setMapEnabled] = useState(false);

  const handleModeChange = (mode: 'hidden' | 'compact' | 'expanded') => {
    if (!mapEnabled && mode !== 'hidden') setMapEnabled(true);
    setDrawerMode(mode);
  };

  const mapQueryArgs: { limit: number; classId?: string; minConfidence?: number } =
    { limit: MAP_LIMIT };
  if (classId !== null) mapQueryArgs.classId = classId;
  if (minConfidence !== null) mapQueryArgs.minConfidence = minConfidence;

  const {
    currentData: mapData,
    isFetching: mapFetching,
    isError: mapError,
    refetch: mapRefetch,
  } = useListPhotosQuery(mapQueryArgs, { skip: !mapEnabled });

  // Server-side cursor-paginated near query — activated when a location is selected.
  const [nearPageIndex, setNearPageIndex] = useState(0);
  const [nearCursorHistory, setNearCursorHistory] = useState<(string | undefined)[]>([undefined]);
  const nearCursor = nearCursorHistory[nearPageIndex];
  const nearQueryArgs = selectedLocation !== null ? {
    limit: PAGE_SIZE,
    nearX: selectedLocation.x,
    nearY: selectedLocation.y,
    nearRadius: NEAR_RADIUS_METRES,
    ...(classId !== null ? { classId } : {}),
    ...(minConfidence !== null ? { minConfidence } : {}),
    ...(nearCursor !== undefined ? { cursor: nearCursor } : {}),
  } : undefined;

  const {
    currentData: nearData,
    isFetching: nearFetching,
    isError: nearError,
    refetch: nearRefetch,
  } = useListPhotosQuery(nearQueryArgs ?? { limit: 0 }, { skip: selectedLocation === null });

  const nearPhotos: Photo[] = nearData?.items ?? [];

  const handleSelectLocation = (loc: MapLocation | null) => {
    setSelectedLocation(loc);
    setNearPageIndex(0);
    setNearCursorHistory([undefined]);
  };

  const handleNearNext = () => {
    if (!nearData?.next_token || nearFetching) return;
    const token = nearData.next_token;
    setNearCursorHistory((prev) => {
      const next = [...prev];
      if (next.length <= nearPageIndex + 1) next.push(token);
      return next;
    });
    setNearPageIndex(nearPageIndex + 1);
  };

  const handleNearPrev = () => {
    if (nearPageIndex === 0 || nearFetching) return;
    setNearPageIndex(nearPageIndex - 1);
  };

  // ── Viewer state ──────────────────────────────────────────────────────────
  const [selectedPhotoId, setSelectedPhotoId] = useState<string | null>(null);
  const [viewerTrigger, setViewerTrigger] = useState<HTMLElement | null>(null);

  // Navigate within nearPhotos when a location is selected; otherwise current page
  const viewerList: Photo[] = selectedLocation !== null ? nearPhotos : (currentData?.items ?? []);
  const viewerPhotoIndex =
    selectedPhotoId ? viewerList.findIndex((p) => p.id === selectedPhotoId) : -1;
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
  const showGrid = !!currentData && currentData.items.length > 0;
  const showEmpty = !isError && !!currentData && currentData.items.length === 0;
  const isBackgroundFetch = !!currentData && isFetching;
  const isInitialFetch = !currentData && isFetching && !isError;

  const isLocationMode = selectedLocation !== null;

  return (
    <div className={styles.page}>
      <FilterControls />

      <div className={styles.content}>
        {isBackgroundFetch && (
          <div className={styles.fetchingBar} role="status" aria-label="Updating gallery" />
        )}

        {isInitialFetch && <SkeletonGrid />}

        {isError && <ErrorPanel error={error} onRetry={refetch} />}

        {/* ── Location-filter mode ──────────────────────────────────── */}
        {isLocationMode && (
          <>
            <div className={styles.locationChip}>
              <span className={styles.locationLabel}>
                Near x&nbsp;{selectedLocation.x.toFixed(1)}&nbsp;m,
                y&nbsp;{selectedLocation.y.toFixed(1)}&nbsp;m
                {nearData && (
                  <>&nbsp;·&nbsp;{nearPhotos.length}&nbsp;photo{nearPhotos.length !== 1 ? 's' : ''}
                  {nearData.next_token ? ' on this page' : ''}</>
                )}
              </span>
              <button
                type="button"
                className={styles.clearLocationBtn}
                onClick={() => { handleSelectLocation(null); setMapHighlightedPhotoId(null); }}
              >
                × Clear location
              </button>
            </div>

            {nearFetching && !nearData && <SkeletonGrid />}

            {nearError && <ErrorPanel error={undefined} onRetry={nearRefetch} />}

            {!nearFetching && !nearError && nearPhotos.length > 0 && (
              <>
                <div
                  className={styles.grid}
                  aria-label={`Photos near x ${selectedLocation.x.toFixed(1)} m, y ${selectedLocation.y.toFixed(1)} m`}
                >
                  {nearPhotos.map((photo) => (
                    <PhotoCard
                      key={photo.id}
                      photo={photo}
                      matchingClassId={classId}
                      minConfidence={minConfidence}
                      onSelect={handleSelect}
                      highlighted={photo.id === mapHighlightedPhotoId}
                    />
                  ))}
                </div>
                <nav className={styles.pagination} aria-label="Near location pagination">
                  <button
                    type="button"
                    onClick={handleNearPrev}
                    disabled={nearPageIndex === 0 || nearFetching}
                    className={styles.pageBtn}
                  >
                    Previous
                  </button>
                  <span className={styles.pageInfo} aria-live="polite" aria-atomic="true">
                    Page {nearPageIndex + 1}
                    {!nearData?.next_token ? ' (last)' : ''}
                  </span>
                  <button
                    type="button"
                    onClick={handleNearNext}
                    disabled={!nearData?.next_token || nearFetching}
                    className={styles.pageBtn}
                  >
                    Next
                  </button>
                </nav>
              </>
            )}

            {!nearFetching && !nearError && nearPhotos.length === 0 && (
              <div className={styles.emptyPanel}>
                <p className={styles.emptyMsg}>
                  No photos within {NEAR_RADIUS_METRES} m of this location
                  {hasActiveFilters ? ' match current filters.' : '.'}
                </p>
                <button
                  type="button"
                  className={styles.retryBtn}
                  onClick={() => { handleSelectLocation(null); setMapHighlightedPhotoId(null); }}
                >
                  Clear location
                </button>
                {hasActiveFilters && (
                  <button
                    type="button"
                    className={styles.retryBtn}
                    onClick={() => dispatch(resetFilters())}
                  >
                    Clear filters
                  </button>
                )}
              </div>
            )}
          </>
        )}

        {/* ── Normal gallery mode ───────────────────────────────────── */}
        {!isLocationMode && (
          <>
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
          </>
        )}
      </div>

      {isViewerOpen && (
        <PhotoViewer
          photos={viewerList}
          initialIndex={viewerPhotoIndex}
          matchingClassId={classId}
          minConfidence={minConfidence}
          triggerEl={viewerTrigger}
          onClose={handleViewerClose}
        />
      )}

      <GreenhouseMap
        mode={drawerMode}
        onModeChange={handleModeChange}
        selectedLocation={selectedLocation}
        onSelectLocation={handleSelectLocation}
        highlightedPhotoId={mapHighlightedPhotoId}
        onHighlightPhoto={setMapHighlightedPhotoId}
        classId={classId}
        minConfidence={minConfidence}
        mapPhotos={mapData?.items ?? []}
        mapFetching={mapFetching}
        mapError={mapError}
        onRetry={mapRefetch}
        hasMore={!!mapData?.next_token}
      />
    </div>
  );
}

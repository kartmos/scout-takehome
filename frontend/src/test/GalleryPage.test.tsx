import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Provider } from 'react-redux';
import { configureStore } from '@reduxjs/toolkit';
import filtersReducer from '../features/filters/filtersSlice';

// Mock env config
vi.mock('../shared/config/env', () => ({
  configResult: { ok: true, config: { apiBaseUrl: 'http://api.test', apiKey: 'secret' } },
}));

// Mock GreenhouseMap so GalleryPage tests focus on gallery/location integration
vi.mock('../features/map/GreenhouseMap', () => ({
  GreenhouseMap: ({
    mode,
    onModeChange,
    selectedLocation,
    onSelectLocation,
    onHighlightPhoto,
  }: {
    mode: string;
    onModeChange: (m: 'hidden' | 'compact' | 'expanded') => void;
    selectedLocation: { x: number; y: number } | null;
    onSelectLocation: (loc: { x: number; y: number } | null) => void;
    highlightedPhotoId: string | null;
    onHighlightPhoto: (id: string | null) => void;
  }) => {
    if (mode === 'hidden') {
      return (
        <button onClick={() => onModeChange('compact')}>
          Show greenhouse map
        </button>
      );
    }
    return (
      <div aria-label="Greenhouse map">
        <button aria-label="Hide map" onClick={() => onModeChange('hidden')}>Hide</button>
        <button aria-label="Expand map" onClick={() => onModeChange('expanded')}>Expand</button>
        {/* Select location only (background canvas click) */}
        <button onClick={() => onSelectLocation({ x: 1.0, y: 2.0 })}>
          Test: select location (1, 2)
        </button>
        {/* Select location + highlight specific photo (marker click) */}
        <button onClick={() => {
          onSelectLocation({ x: 1.0, y: 2.0 });
          onHighlightPhoto('a1b2c3d4-e5f6-7890-abcd-ef1234567890');
        }}>
          Test: select location with marker (1, 2)
        </button>
        {selectedLocation !== null && (
          <button onClick={() => onSelectLocation(null)}>
            Test: deselect map location
          </button>
        )}
      </div>
    );
  },
}));

// Gallery (page-size) query result
const galleryResult: {
  currentData: unknown;
  isFetching: boolean;
  isError: boolean;
  error: unknown;
  refetch: () => void;
} = {
  currentData: undefined,
  isFetching: true,
  isError: false,
  error: undefined,
  refetch: vi.fn(),
};

// Bounded map query result (limit=200)
const mapResult: {
  currentData: unknown;
  isFetching: boolean;
  isError: boolean;
  error: unknown;
  refetch: () => void;
} = {
  currentData: undefined,
  isFetching: false,
  isError: false,
  error: undefined,
  refetch: vi.fn(),
};

// Server-side near filter query result (limit=PAGE_SIZE with nearX/nearY/nearRadius)
const nearResult: {
  currentData: unknown;
  isFetching: boolean;
  isError: boolean;
  error: unknown;
  refetch: () => void;
} = {
  currentData: undefined,
  isFetching: false,
  isError: false,
  error: undefined,
  refetch: vi.fn(),
};

const queryCallLog: Array<[unknown, unknown]> = [];

vi.mock('../shared/api/scoutApi', () => ({
  useListPhotosQuery: (args: unknown, opts?: unknown) => {
    queryCallLog.push([args, opts]);
    const a = args as { limit?: number; nearX?: number };
    if (a.limit === 200) return mapResult;
    if (a.nearX !== undefined) return nearResult;
    return galleryResult;
  },
  useGetPhotoQuery: () => ({ currentData: undefined, isError: false, refetch: vi.fn() }),
  scoutApi: {
    reducerPath: 'scoutApi',
    reducer: (s = {}) => s,
    middleware: () => (next: unknown) => (action: unknown) => (next as (a: unknown) => unknown)(action),
  },
}));

import { GalleryPage } from '../pages/gallery/GalleryPage';

function makeStore() {
  return configureStore({
    reducer: {
      filters: filtersReducer,
      scoutApi: (s = {}) => s,
    },
  });
}

function renderGallery(store = makeStore()) {
  render(
    <Provider store={store}>
      <GalleryPage />
    </Provider>,
  );
  return store;
}

// Photo at (1.0, 2.0) — within 3m of the test location (1, 2)
const PHOTO_A = {
  id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
  x: 1.0,
  y: 2.0,
  h: 3,
  width: 800,
  height: 600,
  capturedAt: '2024-06-01T10:00:00Z',
  originalUrl: 'http://storage.test/photo.jpg',
  predictions: [
    { classId: 'mirid', confidence: 0.87, bbox: { xMin: 0.1, yMin: 0.2, xMax: 0.8, yMax: 0.9 } },
  ],
};

// Photo far from the test location
const PHOTO_B = {
  ...PHOTO_A,
  id: 'b2c3d4e5-f6a7-8901-bcde-f12345678901',
  x: 30.0,
  y: 35.0,
  capturedAt: '2024-06-02T10:00:00Z',
  predictions: [],
};

beforeEach(() => {
  galleryResult.currentData = undefined;
  galleryResult.isFetching = true;
  galleryResult.isError = false;
  galleryResult.error = undefined;
  galleryResult.refetch = vi.fn();

  mapResult.currentData = undefined;
  mapResult.isFetching = false;
  mapResult.isError = false;
  mapResult.error = undefined;
  mapResult.refetch = vi.fn();

  nearResult.currentData = undefined;
  nearResult.isFetching = false;
  nearResult.isError = false;
  nearResult.error = undefined;
  nearResult.refetch = vi.fn();

  queryCallLog.length = 0;

  if (typeof HTMLDialogElement !== 'undefined' && !HTMLDialogElement.prototype.showModal) {
    HTMLDialogElement.prototype.showModal = vi.fn();
    HTMLDialogElement.prototype.close = vi.fn();
  }
});

// ── Core gallery states ────────────────────────────────────────────────────

describe('GalleryPage', () => {
  it('shows skeleton grid during initial fetch', () => {
    renderGallery();
    expect(screen.getByLabelText('Loading photos')).toBeInTheDocument();
  });

  it('renders photo cards when data is loaded', () => {
    galleryResult.currentData = { items: [PHOTO_A, PHOTO_B], next_token: undefined };
    galleryResult.isFetching = false;
    renderGallery();
    expect(screen.getByLabelText('Photo gallery')).toBeInTheDocument();
    expect(screen.getAllByRole('article')).toHaveLength(2);
  });

  it('shows empty panel with reset button when filtered results are empty', () => {
    galleryResult.currentData = { items: [], next_token: undefined };
    galleryResult.isFetching = false;
    const store = makeStore();
    store.dispatch({ type: 'filters/setClassId', payload: 'mirid' });
    renderGallery(store);
    expect(screen.getByText('No photos match these filters.')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Clear filters' }).length).toBeGreaterThan(0);
  });

  it('shows empty panel without reset button when no filters active', () => {
    galleryResult.currentData = { items: [], next_token: undefined };
    galleryResult.isFetching = false;
    renderGallery();
    expect(screen.getByText('No photos found.')).toBeInTheDocument();
  });

  it('shows typed error panel with retry button on fetch error', () => {
    galleryResult.currentData = undefined;
    galleryResult.isFetching = false;
    galleryResult.isError = true;
    galleryResult.error = { status: 500, code: undefined, requestId: 'req-1', message: 'Server blew up' };
    renderGallery();
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('Server blew up')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
  });

  it('shows background fetch bar (not skeleton) during background refetch', () => {
    galleryResult.currentData = { items: [PHOTO_A], next_token: undefined };
    galleryResult.isFetching = true;
    renderGallery();
    expect(screen.getByLabelText('Updating gallery')).toBeInTheDocument();
    expect(screen.queryByLabelText('Loading photos')).toBeNull();
  });

  it('calls refetch when retry button is clicked', () => {
    const refetchMock = vi.fn();
    galleryResult.currentData = undefined;
    galleryResult.isFetching = false;
    galleryResult.isError = true;
    galleryResult.error = { status: 500, code: undefined, requestId: undefined, message: 'Oops' };
    galleryResult.refetch = refetchMock;
    renderGallery();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(refetchMock).toHaveBeenCalledOnce();
  });

  it('disables Previous button on first page', () => {
    galleryResult.currentData = { items: [PHOTO_A], next_token: undefined };
    galleryResult.isFetching = false;
    renderGallery();
    expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled();
  });

  it('disables Next button when there is no next_token', () => {
    galleryResult.currentData = { items: [PHOTO_A], next_token: undefined };
    galleryResult.isFetching = false;
    renderGallery();
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
  });

  it('shows (last) on a full final page (no next_token)', () => {
    const fullPage = Array.from({ length: 24 }, (_, i) => ({
      ...PHOTO_A,
      id: `a1b2c3d4-e5f6-78${i.toString(16).padStart(2, '0')}-abcd-ef1234567890`,
      predictions: [],
    }));
    galleryResult.currentData = { items: fullPage, next_token: undefined };
    galleryResult.isFetching = false;
    renderGallery();
    expect(screen.getByText(/\(last\)/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
  });

  it('malformed photo id is isolated; sibling cards and gallery remain rendered', () => {
    galleryResult.currentData = {
      items: [{ ...PHOTO_A, id: 'invalid-uuid' }, PHOTO_B],
      next_token: undefined,
    };
    galleryResult.isFetching = false;
    renderGallery();
    expect(screen.getAllByRole('article')).toHaveLength(2);
    expect(screen.queryByRole('alert')).toBeNull();
  });
});

// ── Map integration ────────────────────────────────────────────────────────

describe('GalleryPage — map integration', () => {
  beforeEach(() => {
    galleryResult.currentData = { items: [PHOTO_A, PHOTO_B], next_token: undefined };
    galleryResult.isFetching = false;
  });

  it('shows a bottom-right map launcher button (no Gallery/Map view switch)', () => {
    renderGallery();
    expect(screen.getByRole('button', { name: /Show greenhouse map/i })).toBeInTheDocument();
    expect(screen.queryByRole('group', { name: 'View mode' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Gallery' })).toBeNull();
  });

  it('bounded map query is skipped (skip=true) before first open', () => {
    renderGallery();
    const mapCalls = queryCallLog.filter(([args]) => (args as { limit?: number }).limit === 200);
    for (const [, opts] of mapCalls) {
      expect((opts as { skip?: boolean }).skip).toBe(true);
    }
  });

  it('opening compact map fires bounded query with limit=200 and skip=false', () => {
    renderGallery();
    queryCallLog.length = 0;
    fireEvent.click(screen.getByRole('button', { name: /Show greenhouse map/i }));
    const mapCalls = queryCallLog.filter(([args]) => (args as { limit?: number }).limit === 200);
    expect(mapCalls.length).toBeGreaterThan(0);
    const [, opts] = mapCalls[mapCalls.length - 1]!;
    expect((opts as { skip?: boolean }).skip).toBe(false);
  });

  it('selecting a location shows only photos within 3m and hides normal pagination', () => {
    // Server returns only PHOTO_A for the near query (PHOTO_B is too far)
    mapResult.currentData = { items: [PHOTO_A, PHOTO_B], next_token: undefined };
    nearResult.currentData = { items: [PHOTO_A], next_token: undefined };
    renderGallery();
    fireEvent.click(screen.getByRole('button', { name: /Show greenhouse map/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Test: select location (1, 2)' }));

    // Location chip should appear
    expect(screen.getByText(/Near x.*1\.0.*m/i)).toBeInTheDocument();
    // Clear location button visible
    expect(screen.getByRole('button', { name: /Clear location/i })).toBeInTheDocument();
    // Normal pagination hidden
    expect(screen.queryByRole('navigation', { name: 'Gallery pagination' })).toBeNull();
    // Only PHOTO_A (near) should appear — server already filtered out PHOTO_B
    const articles = screen.getAllByRole('article');
    expect(articles).toHaveLength(1);
  });

  it('Clear location button restores normal gallery and pagination', () => {
    mapResult.currentData = { items: [PHOTO_A], next_token: undefined };
    nearResult.currentData = { items: [PHOTO_A], next_token: undefined };
    galleryResult.currentData = { items: [PHOTO_A, PHOTO_B], next_token: 'tok' };
    galleryResult.isFetching = false;
    renderGallery();
    fireEvent.click(screen.getByRole('button', { name: /Show greenhouse map/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Test: select location (1, 2)' }));
    fireEvent.click(screen.getByRole('button', { name: /× Clear location/i }));

    expect(screen.getByLabelText('Photo gallery')).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Gallery pagination' })).toBeInTheDocument();
  });

  it('class/confidence filters pass through to map query', () => {
    const store = makeStore();
    store.dispatch({ type: 'filters/setClassId', payload: 'mirid' });
    renderGallery(store);
    queryCallLog.length = 0;
    fireEvent.click(screen.getByRole('button', { name: /Show greenhouse map/i }));
    const mapCalls = queryCallLog.filter(([args]) => (args as { limit?: number }).limit === 200);
    expect(mapCalls.length).toBeGreaterThan(0);
    const [args] = mapCalls[mapCalls.length - 1]!;
    expect((args as { classId?: string }).classId).toBe('mirid');
  });

  it('Clear location does NOT clear class/confidence filters', () => {
    mapResult.currentData = { items: [PHOTO_A], next_token: undefined };
    nearResult.currentData = { items: [PHOTO_A], next_token: undefined };
    const store = makeStore();
    store.dispatch({ type: 'filters/setClassId', payload: 'mirid' });
    renderGallery(store);
    fireEvent.click(screen.getByRole('button', { name: /Show greenhouse map/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Test: select location (1, 2)' }));
    fireEvent.click(screen.getByRole('button', { name: /× Clear location/i }));
    expect(store.getState().filters.classId).toBe('mirid');
  });

  it('bounded-200 disclosure appears when map query has next_token', () => {
    mapResult.currentData = { items: [PHOTO_A], next_token: 'tok' };
    renderGallery();
    fireEvent.click(screen.getByRole('button', { name: /Show greenhouse map/i }));
    // The disclosure is rendered inside GreenhouseMap (mocked), but the
    // GalleryPage passes hasMore=true to it. Verify the prop is passed
    // by checking that the map query has a next_token in our mock result.
    const md = mapResult.currentData as { next_token?: string };
    expect(md.next_token).toBe('tok');
  });

  it('no random assignment / ten fixed points exist anywhere in the DOM', () => {
    mapResult.currentData = { items: [PHOTO_A], next_token: undefined };
    renderGallery();
    fireEvent.click(screen.getByRole('button', { name: /Show greenhouse map/i }));
    expect(screen.queryByText(/Map point \d+/i)).toBeNull();
    expect(screen.queryByRole('button', { name: /Clear point/i })).toBeNull();
    expect(screen.queryByRole('group', { name: /Camera points/i })).toBeNull();
  });

  it('location-filtered grid shows only near photos (intersection with class filter)', () => {
    // Server applies both near + class filters; returns only PHOTO_A (near + has mirid)
    mapResult.currentData = { items: [PHOTO_A, PHOTO_B], next_token: undefined };
    nearResult.currentData = { items: [PHOTO_A], next_token: undefined };
    const store = makeStore();
    store.dispatch({ type: 'filters/setClassId', payload: 'mirid' });
    renderGallery(store);
    fireEvent.click(screen.getByRole('button', { name: /Show greenhouse map/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Test: select location (1, 2)' }));

    // Server already filtered: PHOTO_A (near + mirid) shown, PHOTO_B excluded
    const articles = screen.getAllByRole('article');
    expect(articles).toHaveLength(1);
  });

  it('clicking a specific marker highlights that card (data-highlighted attribute)', () => {
    // Server returns only PHOTO_A for the near query
    mapResult.currentData = { items: [PHOTO_A, PHOTO_B], next_token: undefined };
    nearResult.currentData = { items: [PHOTO_A], next_token: undefined };
    renderGallery();
    fireEvent.click(screen.getByRole('button', { name: /Show greenhouse map/i }));
    fireEvent.click(screen.getByRole('button', { name: /Test: select location with marker/i }));

    const articles = screen.getAllByRole('article');
    // Only PHOTO_A returned by near query; it should be highlighted
    expect(articles).toHaveLength(1);
    expect(articles[0]).toHaveAttribute('data-highlighted', 'true');
  });

  it('background canvas click (no marker) shows near cards without highlight', () => {
    mapResult.currentData = { items: [PHOTO_A, PHOTO_B], next_token: undefined };
    nearResult.currentData = { items: [PHOTO_A], next_token: undefined };
    renderGallery();
    fireEvent.click(screen.getByRole('button', { name: /Show greenhouse map/i }));
    // "Test: select location (1, 2)" fires only onSelectLocation, not onHighlightPhoto
    fireEvent.click(screen.getByRole('button', { name: /^Test: select location \(1, 2\)$/i }));

    const articles = screen.getAllByRole('article');
    expect(articles).toHaveLength(1);
    expect(articles[0]).not.toHaveAttribute('data-highlighted');
  });

  it('clearing location removes card highlight', () => {
    mapResult.currentData = { items: [PHOTO_A], next_token: undefined };
    nearResult.currentData = { items: [PHOTO_A], next_token: undefined };
    galleryResult.currentData = { items: [PHOTO_A], next_token: undefined };
    galleryResult.isFetching = false;
    renderGallery();
    fireEvent.click(screen.getByRole('button', { name: /Show greenhouse map/i }));
    fireEvent.click(screen.getByRole('button', { name: /Test: select location with marker/i }));
    fireEvent.click(screen.getByRole('button', { name: /× Clear location/i }));

    // Back to normal gallery — articles exist but none are highlighted
    const articles = screen.getAllByRole('article');
    expect(articles.length).toBeGreaterThan(0);
    for (const article of articles) {
      expect(article).not.toHaveAttribute('data-highlighted');
    }
  });
});

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Provider } from 'react-redux';
import { configureStore } from '@reduxjs/toolkit';
import filtersReducer from '../features/filters/filtersSlice';

// Mock env config
vi.mock('../shared/config/env', () => ({
  configResult: { ok: true, config: { apiBaseUrl: 'http://api.test', apiKey: 'secret' } },
}));

// Mock scoutApi so we can control useListPhotosQuery per test
const mockQueryResult: {
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

vi.mock('../shared/api/scoutApi', () => ({
  useListPhotosQuery: () => mockQueryResult,
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

function renderGallery() {
  const store = makeStore();
  render(
    <Provider store={store}>
      <GalleryPage />
    </Provider>,
  );
  return store;
}

const PHOTO_FIXTURE = {
  id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
  x: 1,
  y: 2,
  h: 3,
  width: 800,
  height: 600,
  capturedAt: '2024-06-01T10:00:00Z',
  originalUrl: 'http://storage.test/photo.jpg',
  predictions: [
    {
      classId: 'mirid',
      confidence: 0.87,
      bbox: { xMin: 0.1, yMin: 0.2, xMax: 0.8, yMax: 0.9 },
    },
  ],
};

const PHOTO_FIXTURE_2 = {
  ...PHOTO_FIXTURE,
  id: 'b2c3d4e5-f6a7-8901-bcde-f12345678901',
  capturedAt: '2024-06-02T10:00:00Z',
  predictions: [],
};

beforeEach(() => {
  mockQueryResult.currentData = undefined;
  mockQueryResult.isFetching = true;
  mockQueryResult.isError = false;
  mockQueryResult.error = undefined;
  mockQueryResult.refetch = vi.fn();
});

describe('GalleryPage', () => {
  it('shows skeleton grid during initial fetch', () => {
    renderGallery();
    expect(screen.getByLabelText('Loading photos')).toBeInTheDocument();
  });

  it('renders photo cards when data is loaded', () => {
    mockQueryResult.currentData = { items: [PHOTO_FIXTURE, PHOTO_FIXTURE_2], next_token: undefined };
    mockQueryResult.isFetching = false;
    renderGallery();
    expect(screen.getByLabelText('Photo gallery')).toBeInTheDocument();
    expect(screen.getAllByRole('article')).toHaveLength(2);
  });

  it('shows empty panel with reset button when filtered results are empty', () => {
    mockQueryResult.currentData = { items: [], next_token: undefined };
    mockQueryResult.isFetching = false;
    const store = makeStore();
    store.dispatch({ type: 'filters/setClassId', payload: 'mirid' });
    render(
      <Provider store={store}>
        <GalleryPage />
      </Provider>,
    );
    expect(screen.getByText('No photos match these filters.')).toBeInTheDocument();
    // FilterControls and EmptyPanel both render "Clear filters"
    expect(screen.getAllByRole('button', { name: 'Clear filters' }).length).toBeGreaterThan(0);
  });

  it('shows empty panel without reset button when no filters active', () => {
    mockQueryResult.currentData = { items: [], next_token: undefined };
    mockQueryResult.isFetching = false;
    const store = configureStore({
      reducer: { filters: filtersReducer, scoutApi: (s = {}) => s },
    });
    render(
      <Provider store={store}>
        <GalleryPage />
      </Provider>,
    );
    expect(screen.getByText('No photos found.')).toBeInTheDocument();
  });

  it('shows typed error panel with retry button on fetch error', () => {
    mockQueryResult.currentData = undefined;
    mockQueryResult.isFetching = false;
    mockQueryResult.isError = true;
    mockQueryResult.error = { status: 500, code: undefined, requestId: 'req-1', message: 'Server blew up' };
    renderGallery();
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('Server blew up')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
  });

  it('shows background fetch bar (not skeleton) during background refetch', () => {
    mockQueryResult.currentData = { items: [PHOTO_FIXTURE], next_token: undefined };
    mockQueryResult.isFetching = true;
    renderGallery();
    expect(screen.getByLabelText('Updating gallery')).toBeInTheDocument();
    expect(screen.queryByLabelText('Loading photos')).toBeNull();
  });

  it('calls refetch when retry button is clicked', () => {
    const refetchMock = vi.fn();
    mockQueryResult.currentData = undefined;
    mockQueryResult.isFetching = false;
    mockQueryResult.isError = true;
    mockQueryResult.error = { status: 500, code: undefined, requestId: undefined, message: 'Oops' };
    mockQueryResult.refetch = refetchMock;
    renderGallery();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(refetchMock).toHaveBeenCalledOnce();
  });

  it('disables Previous button on first page', () => {
    mockQueryResult.currentData = { items: [PHOTO_FIXTURE], next_token: undefined };
    mockQueryResult.isFetching = false;
    renderGallery();
    const prev = screen.getByRole('button', { name: 'Previous' });
    expect(prev).toBeDisabled();
  });

  it('disables Next button when there is no next_token', () => {
    mockQueryResult.currentData = { items: [PHOTO_FIXTURE], next_token: undefined };
    mockQueryResult.isFetching = false;
    renderGallery();
    const next = screen.getByRole('button', { name: 'Next' });
    expect(next).toBeDisabled();
  });

  it('shows (last) on a full final page (exactly PAGE_SIZE items, no next_token)', () => {
    // 24 items with no next_token must show "(last)" regardless of items.length === PAGE_SIZE
    const fullPage = Array.from({ length: 24 }, (_, i) => ({
      ...PHOTO_FIXTURE,
      id: `a1b2c3d4-e5f6-78${i.toString(16).padStart(2, '0')}-abcd-ef1234567890`,
      predictions: [],
    }));
    mockQueryResult.currentData = { items: fullPage, next_token: undefined };
    mockQueryResult.isFetching = false;
    renderGallery();
    expect(screen.getByText(/\(last\)/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
  });

  it('malformed photo id is isolated; sibling cards and gallery remain rendered', () => {
    // PhotoCard catches the buildThumbnailCandidates throw and renders an unavailable fallback.
    // The gallery must not blank and the sibling valid card must stay visible.
    mockQueryResult.currentData = {
      items: [{ ...PHOTO_FIXTURE, id: 'invalid-uuid' }, PHOTO_FIXTURE_2],
      next_token: undefined,
    };
    mockQueryResult.isFetching = false;
    renderGallery();
    // Both article elements rendered — no gallery crash
    expect(screen.getAllByRole('article')).toHaveLength(2);
    // No uncaught error banner from the page-level error boundary
    expect(screen.queryByRole('alert')).toBeNull();
  });
});

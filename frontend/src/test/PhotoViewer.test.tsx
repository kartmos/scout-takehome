import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { components } from '../entities/api/__generated__/schema';

// Mock env config
vi.mock('../shared/config/env', () => ({
  configResult: { ok: true, config: { apiBaseUrl: 'http://api.test', apiKey: 'secret' } },
}));

type Photo = components['schemas']['Photo'];

// Mutable mock state — tests update these before render.
const mockRefetch = vi.fn();
const mockGetPhoto: { currentData: Photo | undefined; isError: boolean; refetch: () => void } = {
  currentData: undefined,
  isError: false,
  refetch: mockRefetch,
};

// Per-test implementation hook, reset to identity in beforeEach.
let mockImpl: (id: unknown) => typeof mockGetPhoto = () => mockGetPhoto;

// Track the last photo ID the component queried.
let lastQueriedId: string | null = null;

vi.mock('../shared/api/scoutApi', () => ({
  useGetPhotoQuery: (id: unknown) => {
    if (typeof id === 'string') lastQueriedId = id;
    return mockImpl(id);
  },
}));

import { PhotoViewer } from '../features/viewer/PhotoViewer';

// Stale URLs are what the list page holds; fresh URLs are returned by GET /photos/{id}.
const STALE_URL_A = 'http://storage.test/stale-a.jpg';
const FRESH_URL_A = 'http://storage.test/fresh-a.jpg';
const FRESH_URL_A2 = 'http://storage.test/fresh-a2.jpg';

const PHOTO_LIST_A: Photo = {
  id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
  x: 1,
  y: 2,
  h: 3,
  width: 800,
  height: 600,
  capturedAt: '2024-06-01T10:00:00Z',
  originalUrl: STALE_URL_A,
  predictions: [
    {
      classId: 'mirid',
      confidence: 0.87,
      bbox: { xMin: 0.1, yMin: 0.2, xMax: 0.8, yMax: 0.9 },
    },
  ],
};

const PHOTO_FRESH_A: Photo = { ...PHOTO_LIST_A, originalUrl: FRESH_URL_A };

const PHOTO_LIST_B: Photo = {
  id: 'b2c3d4e5-f6a7-8901-bcde-f12345678901',
  x: 2,
  y: 3,
  h: 4,
  width: 1024,
  height: 768,
  capturedAt: '2024-06-02T10:00:00Z',
  originalUrl: 'http://storage.test/stale-b.jpg',
  predictions: [],
};

const PHOTO_FRESH_B: Photo = { ...PHOTO_LIST_B, originalUrl: 'http://storage.test/fresh-b.jpg' };

const PHOTOS = [PHOTO_LIST_A, PHOTO_LIST_B];

function renderViewer(props: Partial<Parameters<typeof PhotoViewer>[0]> = {}) {
  const onClose = vi.fn();
  const { rerender, ...rest } = render(
    <PhotoViewer
      photos={PHOTOS}
      initialIndex={0}
      matchingClassId={null}
      triggerEl={null}
      onClose={onClose}
      {...props}
    />,
  );
  return { onClose, rerender, ...rest };
}

beforeEach(() => {
  vi.clearAllMocks();
  lastQueriedId = null;
  mockGetPhoto.currentData = PHOTO_FRESH_A;
  mockGetPhoto.isError = false;
  mockGetPhoto.refetch = mockRefetch;
  mockImpl = () => mockGetPhoto;
});

describe('PhotoViewer — fresh URL lifecycle', () => {
  it('requests GET /photos/{id} for the opened photo on mount', () => {
    renderViewer({ initialIndex: 0 });
    expect(lastQueriedId).toBe(PHOTO_LIST_A.id);
  });

  it('uses the fresh presigned URL as img src, not the stale list originalUrl', () => {
    renderViewer({ initialIndex: 0 });
    const img = document.querySelector('img') as HTMLImageElement;
    expect(img.src).toContain('fresh-a');
    expect(img.src).not.toContain('stale');
  });

  it('previous/next navigation requests the newly selected photo ID', async () => {
    const user = userEvent.setup();
    mockImpl = (id) => ({
      ...mockGetPhoto,
      currentData: id === PHOTO_LIST_A.id ? PHOTO_FRESH_A : PHOTO_FRESH_B,
    });
    renderViewer({ initialIndex: 0 });
    await user.click(screen.getByRole('button', { name: 'Next photo' }));
    expect(lastQueriedId).toBe(PHOTO_LIST_B.id);
  });

  it('does not flash previous photo content while next photo is loading', async () => {
    const user = userEvent.setup();
    // B returns no currentData — simulates pending fetch
    mockImpl = (id) => ({
      ...mockGetPhoto,
      currentData: id === PHOTO_LIST_A.id ? PHOTO_FRESH_A : undefined,
      isError: false,
    });
    renderViewer({ initialIndex: 0 });
    await user.click(screen.getByRole('button', { name: 'Next photo' }));
    expect(screen.queryByText('Mirid')).toBeNull();
    expect(screen.getByRole('status', { name: 'Loading image' })).toBeInTheDocument();
  });

  it('does not automatically refetch on initial render', () => {
    renderViewer();
    expect(mockRefetch).not.toHaveBeenCalled();
  });

  it('image error followed by Retry calls refetch (requests new presigned URL)', async () => {
    const user = userEvent.setup();
    renderViewer();
    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.error(img);
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(mockRefetch).toHaveBeenCalledOnce();
  });

  it('Retry after image error with same URL remounts the image and exits error state', async () => {
    // Verifies that remount is driven by explicit retry completion, not URL string inequality.
    let resolveRetry!: () => void;
    mockRefetch.mockReturnValue(new Promise<void>((resolve) => { resolveRetry = resolve; }));

    renderViewer();
    fireEvent.error(screen.getByAltText(/Greenhouse photo/));
    expect(screen.getByRole('alert')).toBeInTheDocument();

    // Click Retry; freshPhoto.originalUrl stays FRESH_URL_A (unchanged)
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

    await act(async () => { resolveRetry(); });

    // Error clears; img remounts into loading state; URL unchanged but img key is new
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByRole('status', { name: 'Loading image' })).toBeInTheDocument();
    const newImg = document.querySelector('img') as HTMLImageElement;
    expect(newImg.src).toContain('fresh-a');
    expect(newImg.src).not.toContain('stale');
  });

  it('Retry after image error with URL rotation shows the new URL', async () => {
    let resolveRetry!: () => void;
    mockRefetch.mockReturnValue(new Promise<void>((resolve) => { resolveRetry = resolve; }));

    renderViewer();
    fireEvent.error(screen.getByAltText(/Greenhouse photo/));
    expect(screen.getByRole('alert')).toBeInTheDocument();

    // Start retry, then simulate server rotating the presigned URL
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    mockGetPhoto.currentData = { ...PHOTO_FRESH_A, originalUrl: FRESH_URL_A2 };

    await act(async () => { resolveRetry(); });

    // Error clears; img remounts with the new URL
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByRole('status', { name: 'Loading image' })).toBeInTheDocument();
    const newImg = document.querySelector('img') as HTMLImageElement;
    expect(newImg.src).toContain('fresh-a2');
    expect(newImg.src).not.toContain('stale');
  });

  it('Data API failure shows a recoverable error state', () => {
    mockGetPhoto.currentData = undefined;
    mockGetPhoto.isError = true;
    renderViewer();
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('Failed to load photo.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
  });

  it('Retry on API error calls refetch', async () => {
    const user = userEvent.setup();
    mockGetPhoto.currentData = undefined;
    mockGetPhoto.isError = true;
    renderViewer();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(mockRefetch).toHaveBeenCalledOnce();
  });

  it('API error message does not expose the presigned URL', () => {
    mockGetPhoto.currentData = undefined;
    mockGetPhoto.isError = true;
    renderViewer();
    const alert = screen.getByRole('alert');
    expect(alert.textContent).not.toContain('http://');
    expect(alert.textContent).not.toContain('fresh');
    expect(alert.textContent).not.toContain('stale');
  });
});

describe('PhotoViewer — Retry lifecycle', () => {
  it('Retry button is not shown while refetch is in flight', async () => {
    let resolveRetry!: () => void;
    mockRefetch.mockReturnValue(new Promise<void>((resolve) => { resolveRetry = resolve; }));

    renderViewer();
    fireEvent.error(screen.getByAltText(/Greenhouse photo/));

    // Start Retry; button should disappear while isRetrying is true
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull();

    await act(async () => { resolveRetry(); });
  });

  it('failed explicit Retry keeps error state recoverable and does not expose URL', async () => {
    const user = userEvent.setup();
    mockRefetch.mockRejectedValue(new Error('Network error'));

    renderViewer();
    fireEvent.error(screen.getByAltText(/Greenhouse photo/));

    await user.click(screen.getByRole('button', { name: 'Retry' }));

    // Error state persists; Retry is available again
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
    const alert = screen.getByRole('alert');
    expect(alert.textContent).not.toContain('http://');
    expect(alert.textContent).not.toContain('fresh');
    expect(alert.textContent).not.toContain('stale');
  });

  it('stale Retry completion for photo A does not revert photo B to a loading or error state', async () => {
    let resolveA!: () => void;
    mockRefetch.mockReturnValueOnce(new Promise<void>((resolve) => { resolveA = resolve; }));

    mockImpl = (id) => ({
      ...mockGetPhoto,
      currentData: id === PHOTO_LIST_A.id ? PHOTO_FRESH_A : PHOTO_FRESH_B,
      refetch: mockRefetch,
    });

    const user = userEvent.setup();
    renderViewer({ initialIndex: 0 });

    // Trigger image error on A and start an explicit Retry
    fireEvent.error(screen.getByAltText(/Greenhouse photo/));
    await user.click(screen.getByRole('button', { name: 'Retry' }));

    // Navigate to B while A's retry is still pending
    await user.click(screen.getByRole('button', { name: 'Next photo' }));

    // B's image loads successfully
    fireEvent.load(screen.getByAltText(/Greenhouse photo/));
    expect(screen.queryByRole('status', { name: 'Loading image' })).toBeNull();
    expect(screen.queryByRole('alert')).toBeNull();

    // Resolve A's stale retry — must not affect B
    await act(async () => { resolveA(); });

    // B remains in loaded state; no loading spinner or alert injected
    expect(screen.queryByRole('status', { name: 'Loading image' })).toBeNull();
    expect(screen.queryByRole('alert')).toBeNull();
  });
});

describe('PhotoViewer — sidebar prediction states', () => {
  it('sidebar shows loading state instead of No detections during initial fetch', () => {
    mockGetPhoto.currentData = undefined;
    mockGetPhoto.isError = false;
    renderViewer();
    expect(screen.queryByText('No detections')).toBeNull();
    expect(screen.getByRole('status', { name: 'Loading detections' })).toBeInTheDocument();
  });

  it('sidebar shows loading state instead of No detections while API error Retry is pending', async () => {
    let resolveRetry!: () => void;
    mockRefetch.mockReturnValue(new Promise<void>((resolve) => { resolveRetry = resolve; }));

    mockGetPhoto.currentData = undefined;
    mockGetPhoto.isError = true;
    renderViewer();

    // Start API error Retry; isRetrying becomes true
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

    // During retry the sidebar must not claim No detections
    expect(screen.queryByText('No detections')).toBeNull();

    await act(async () => { resolveRetry(); });
  });

  it('sidebar shows neutral state instead of No detections on API error (no retry)', () => {
    mockGetPhoto.currentData = undefined;
    mockGetPhoto.isError = true;
    renderViewer();
    expect(screen.queryByText('No detections')).toBeNull();
  });

  it('sidebar shows genuine No detections for a photo with an empty predictions array', () => {
    mockGetPhoto.currentData = { ...PHOTO_FRESH_B, predictions: [] };
    renderViewer({ photos: [PHOTO_LIST_B], initialIndex: 0 });
    expect(screen.getByText('No detections')).toBeInTheDocument();
  });
});

describe('PhotoViewer — dialog and accessibility', () => {
  it('renders a dialog with role dialog', () => {
    renderViewer();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('has accessible name via aria-labelledby', () => {
    renderViewer();
    const dialog = screen.getByRole('dialog');
    expect(dialog).toHaveAttribute('aria-modal', 'true');
    expect(dialog).toHaveAttribute('aria-labelledby', 'viewer-title');
    expect(document.getElementById('viewer-title')).not.toBeNull();
  });

  it('shows the prediction class label and confidence from fresh photo data', () => {
    renderViewer();
    expect(screen.getByText('Mirid')).toBeInTheDocument();
    expect(screen.getByText('87%')).toBeInTheDocument();
  });

  it('shows "No detections" for a photo without predictions', () => {
    mockGetPhoto.currentData = PHOTO_FRESH_B;
    renderViewer({ photos: [PHOTO_LIST_B], initialIndex: 0 });
    expect(screen.getByText('No detections')).toBeInTheDocument();
  });

  it('close button calls onClose', () => {
    const { onClose } = renderViewer();
    fireEvent.click(screen.getByRole('button', { name: 'Close photo viewer' }));
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('Escape key closes the viewer', () => {
    const { onClose } = renderViewer();
    const dialog = screen.getByRole('dialog') as HTMLDialogElement;
    dialog.close();
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('restores focus to trigger element on close', () => {
    const trigger = document.createElement('button');
    document.body.appendChild(trigger);
    const focusSpy = vi.spyOn(trigger, 'focus');
    render(
      <PhotoViewer
        photos={PHOTOS}
        initialIndex={0}
        matchingClassId={null}
        triggerEl={trigger}
        onClose={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Close photo viewer' }));
    expect(focusSpy).toHaveBeenCalled();
    document.body.removeChild(trigger);
  });
});

describe('PhotoViewer — navigation', () => {
  it('Previous button is disabled on first photo', () => {
    renderViewer({ initialIndex: 0 });
    expect(screen.getByRole('button', { name: 'Previous photo' })).toBeDisabled();
  });

  it('Next button is disabled on last photo', () => {
    mockGetPhoto.currentData = PHOTO_FRESH_B;
    renderViewer({ initialIndex: 1 });
    expect(screen.getByRole('button', { name: 'Next photo' })).toBeDisabled();
  });

  it('clicking Next navigates to next photo', async () => {
    const user = userEvent.setup();
    mockImpl = (id) => ({
      ...mockGetPhoto,
      currentData: id === PHOTO_LIST_A.id ? PHOTO_FRESH_A : PHOTO_FRESH_B,
    });
    renderViewer({ initialIndex: 0 });
    await user.click(screen.getByRole('button', { name: 'Next photo' }));
    expect(screen.getByText('No detections')).toBeInTheDocument();
    expect(screen.getByText('2 / 2')).toBeInTheDocument();
  });

  it('clicking Previous navigates back', async () => {
    const user = userEvent.setup();
    mockImpl = (id) => ({
      ...mockGetPhoto,
      currentData: id === PHOTO_LIST_A.id ? PHOTO_FRESH_A : PHOTO_FRESH_B,
    });
    mockGetPhoto.currentData = PHOTO_FRESH_B;
    renderViewer({ initialIndex: 1 });
    await user.click(screen.getByRole('button', { name: 'Previous photo' }));
    expect(screen.getByText('Mirid')).toBeInTheDocument();
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
  });

  it('ArrowRight key navigates to next photo', () => {
    renderViewer({ initialIndex: 0 });
    fireEvent.keyDown(window, { key: 'ArrowRight' });
    expect(screen.getByText('2 / 2')).toBeInTheDocument();
  });

  it('ArrowLeft key navigates to previous photo', () => {
    mockImpl = (id) => ({
      ...mockGetPhoto,
      currentData: id === PHOTO_LIST_A.id ? PHOTO_FRESH_A : PHOTO_FRESH_B,
    });
    renderViewer({ initialIndex: 1 });
    fireEvent.keyDown(window, { key: 'ArrowLeft' });
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
  });

  it('ArrowLeft does nothing on first photo (bounded)', () => {
    const { onClose } = renderViewer({ initialIndex: 0 });
    fireEvent.keyDown(window, { key: 'ArrowLeft' });
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('shows position feedback (current / total)', () => {
    renderViewer({ initialIndex: 0 });
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
  });

  it('calls onClose when photo is removed from list (selection invalidation)', () => {
    const { onClose, rerender } = renderViewer({ photos: [PHOTO_LIST_A], initialIndex: 0 });
    rerender(
      <PhotoViewer
        photos={[]}
        initialIndex={0}
        matchingClassId={null}
        triggerEl={null}
        onClose={onClose}
      />,
    );
    expect(onClose).toHaveBeenCalled();
  });
});

describe('PhotoViewer — image states', () => {
  it('shows loading state while API is fetching (no data yet)', () => {
    mockGetPhoto.currentData = undefined;
    mockGetPhoto.isError = false;
    renderViewer();
    expect(screen.getByRole('status', { name: 'Loading image' })).toBeInTheDocument();
  });

  it('shows loading spinner while image is loading after data arrives', () => {
    renderViewer();
    expect(screen.getByRole('status', { name: 'Loading image' })).toBeInTheDocument();
  });

  it('shows error state when image fails to load', () => {
    renderViewer();
    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.error(img);
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('Image failed to load.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
  });

  it('image error message does not expose presigned URL', () => {
    renderViewer();
    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.error(img);
    const alert = screen.getByRole('alert');
    expect(alert.textContent).not.toContain('http://');
    expect(alert.textContent).not.toContain('fresh');
    expect(alert.textContent).not.toContain('stale');
  });

  it('renders a bbox SVG overlay for predictions after image loads', () => {
    renderViewer({ initialIndex: 0 });
    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.load(img);
    const svgs = document.querySelectorAll('svg');
    expect(svgs.length).toBeGreaterThan(0);
  });

  it('prediction inspection does not depend on hover (text is always visible)', () => {
    renderViewer({ initialIndex: 0 });
    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.load(img);
    expect(screen.getByText('Mirid')).toBeInTheDocument();
    expect(screen.getByText('87%')).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Additional fixture: photo with two same-class predictions.
// ---------------------------------------------------------------------------
const PHOTO_LIST_MULTI: typeof PHOTO_LIST_A = {
  id: 'c3d4e5f6-a7b8-9012-cdef-123456789012',
  x: 3,
  y: 4,
  h: 5,
  width: 800,
  height: 600,
  capturedAt: '2024-06-03T10:00:00Z',
  originalUrl: 'http://storage.test/stale-multi.jpg',
  predictions: [
    { classId: 'mirid', confidence: 0.87, bbox: { xMin: 0.1, yMin: 0.1, xMax: 0.5, yMax: 0.5 } },
    { classId: 'mirid', confidence: 0.62, bbox: { xMin: 0.3, yMin: 0.3, xMax: 0.9, yMax: 0.9 } },
    { classId: 'powdery_mildew', confidence: 0.45, bbox: { xMin: 0.0, yMin: 0.0, xMax: 0.3, yMax: 0.3 } },
  ],
};

const PHOTO_FRESH_MULTI: typeof PHOTO_LIST_A = {
  ...PHOTO_LIST_MULTI,
  originalUrl: 'http://storage.test/fresh-multi.jpg',
};

describe('PhotoViewer — grouped sidebar and bbox visibility', () => {
  it('same-class predictions appear as individually numbered items in the sidebar', () => {
    mockGetPhoto.currentData = PHOTO_FRESH_MULTI;
    renderViewer({ photos: [PHOTO_LIST_MULTI], initialIndex: 0 });
    // Two mirid predictions → two toggle buttons numbered 1 and 2
    const toggle1 = screen.getByRole('button', { name: /Toggle detection 1/ });
    const toggle2 = screen.getByRole('button', { name: /Toggle detection 2/ });
    expect(toggle1).toBeInTheDocument();
    expect(toggle2).toBeInTheDocument();
  });

  it('each toggle button number matches its bbox number shown in the SVG', () => {
    mockGetPhoto.currentData = PHOTO_FRESH_MULTI;
    renderViewer({ photos: [PHOTO_LIST_MULTI], initialIndex: 0 });
    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.load(img);
    // SVG should contain text nodes "1", "2", "3"
    const svgTexts = Array.from(document.querySelectorAll('text')).map((t) => t.textContent);
    expect(svgTexts).toContain('1');
    expect(svgTexts).toContain('2');
    expect(svgTexts).toContain('3');
  });

  it('toggling a prediction hides only its bbox group', async () => {
    const user = userEvent.setup();
    mockGetPhoto.currentData = PHOTO_FRESH_MULTI;
    renderViewer({ photos: [PHOTO_LIST_MULTI], initialIndex: 0 });
    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.load(img);

    // 3 predictions → 3 bbox groups in the SVG
    expect(document.querySelectorAll('svg g').length).toBe(3);

    // Hide detection 1
    await user.click(screen.getByRole('button', { name: /Toggle detection 1/ }));
    expect(document.querySelectorAll('svg g').length).toBe(2);
    expect(screen.getByRole('button', { name: /Toggle detection 1/ })).toHaveAttribute(
      'aria-pressed',
      'false',
    );

    // Restore detection 1
    await user.click(screen.getByRole('button', { name: /Toggle detection 1/ }));
    expect(document.querySelectorAll('svg g').length).toBe(3);
    expect(screen.getByRole('button', { name: /Toggle detection 1/ })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('keyboard Enter activates a toggle', async () => {
    const user = userEvent.setup();
    mockGetPhoto.currentData = PHOTO_FRESH_MULTI;
    renderViewer({ photos: [PHOTO_LIST_MULTI], initialIndex: 0 });
    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.load(img);

    expect(document.querySelectorAll('svg g').length).toBe(3);

    const toggle1 = screen.getByRole('button', { name: /Toggle detection 1/ });
    await act(async () => { toggle1.focus(); });
    await user.keyboard('{Enter}');
    expect(document.querySelectorAll('svg g').length).toBe(2);
  });

  it('navigating to another photo resets visibility state', async () => {
    const user = userEvent.setup();
    mockImpl = (id) => ({
      ...mockGetPhoto,
      currentData:
        id === PHOTO_LIST_MULTI.id ? PHOTO_FRESH_MULTI : PHOTO_FRESH_B,
    });

    const photos = [PHOTO_LIST_MULTI, PHOTO_LIST_B];
    render(
      <PhotoViewer
        photos={photos}
        initialIndex={0}
        matchingClassId={null}
        triggerEl={null}
        onClose={vi.fn()}
      />,
    );

    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.load(img);

    // Hide detection 1 on MULTI
    const toggle1 = screen.getByRole('button', { name: /Toggle detection 1/ });
    expect(toggle1).toHaveAttribute('aria-pressed', 'true');
    await user.click(toggle1);
    expect(toggle1).toHaveAttribute('aria-pressed', 'false');

    // Navigate to B (no predictions)
    await user.click(screen.getByRole('button', { name: 'Next photo' }));
    expect(screen.getByText('No detections')).toBeInTheDocument();

    // Navigate back to MULTI
    await user.click(screen.getByRole('button', { name: 'Previous photo' }));
    // Sidebar immediately shows predictions from fresh data; visibility is reset
    const resetToggle = screen.getByRole('button', { name: /Toggle detection 1/ });
    expect(resetToggle).toHaveAttribute('aria-pressed', 'true');
  });

  it('selected-class emphasis in grouped sidebar: non-matching group is dimmed', () => {
    mockGetPhoto.currentData = PHOTO_FRESH_MULTI;
    renderViewer({
      photos: [PHOTO_LIST_MULTI],
      initialIndex: 0,
      matchingClassId: 'mirid',
    });
    // Mirid group → predGroupMatch class
    expect(screen.getByText('Mirid')).toBeInTheDocument();
    // Powdery Mildew group present but at reduced opacity (not predGroupMatch)
    expect(screen.getByText('Powdery Mildew')).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Threshold filtering in the viewer
// ---------------------------------------------------------------------------
const PHOTO_LIST_THRESH: typeof PHOTO_LIST_A = {
  id: 'd4e5f6a7-b8c9-0123-def0-123456789013',
  x: 4, y: 5, h: 6,
  width: 800, height: 600,
  capturedAt: '2024-06-04T10:00:00Z',
  originalUrl: 'http://storage.test/stale-thresh.jpg',
  predictions: [
    { classId: 'mirid', confidence: 0.47, bbox: { xMin: 0.0, yMin: 0.0, xMax: 0.3, yMax: 0.3 } },
    { classId: 'mirid', confidence: 0.52, bbox: { xMin: 0.1, yMin: 0.1, xMax: 0.5, yMax: 0.5 } },
    { classId: 'mirid', confidence: 0.60, bbox: { xMin: 0.3, yMin: 0.3, xMax: 0.8, yMax: 0.8 } },
  ],
};
const PHOTO_FRESH_THRESH: typeof PHOTO_LIST_A = {
  ...PHOTO_LIST_THRESH,
  originalUrl: 'http://storage.test/fresh-thresh.jpg',
};

describe('PhotoViewer — confidence threshold filtering', () => {
  it('threshold 0.5 filters grouped list, number mapping, and toggleable bbox set', async () => {
    const user = userEvent.setup();
    mockGetPhoto.currentData = PHOTO_FRESH_THRESH;

    render(
      <PhotoViewer
        photos={[PHOTO_LIST_THRESH]}
        initialIndex={0}
        matchingClassId={null}
        minConfidence={0.5}
        triggerEl={null}
        onClose={vi.fn()}
      />,
    );

    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.load(img);

    // 0.47 excluded; 0.52 and 0.60 pass → 2 predictions numbered 1 and 2
    expect(screen.getByRole('button', { name: /Toggle detection 1/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Toggle detection 2/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Toggle detection 3/ })).toBeNull();

    // Group header shows 2× not 3×
    expect(screen.getByText('2×')).toBeInTheDocument();

    // 2 bbox groups in SVG
    expect(document.querySelectorAll('svg g').length).toBe(2);

    // Toggling detection 1 leaves 1 bbox group
    await user.click(screen.getByRole('button', { name: /Toggle detection 1/ }));
    expect(document.querySelectorAll('svg g').length).toBe(1);
  });
});

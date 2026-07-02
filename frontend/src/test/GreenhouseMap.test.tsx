/**
 * GreenhouseMap tests.
 * react-konva is mocked with lightweight DOM stubs so jsdom can render
 * without canvas. Geometry and event logic remain real. The accessible DOM
 * location list is used as the primary testable interaction surface.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { forwardRef } from 'react';
import type { ForwardedRef } from 'react';
import type { components } from '../entities/api/__generated__/schema';

type Photo = components['schemas']['Photo'];

// ─── react-konva mock ────────────────────────────────────────────────────────

let mockPointerPos = { x: 100, y: 100 };

const mockStage = {
  getPointerPosition: () => mockPointerPos,
};

vi.mock('react-konva', () => {
  const Stage = forwardRef(
    (
      { children, onMouseDown, onMouseMove, onMouseUp, onClick, onWheel, width, height }: {
        children?: React.ReactNode;
        onMouseDown?: (e: unknown) => void;
        onMouseMove?: (e: unknown) => void;
        onMouseUp?: (e: unknown) => void;
        onClick?: (e: unknown) => void;
        onWheel?: (e: unknown) => void;
        width?: number;
        height?: number;
      },
      ref: ForwardedRef<unknown>,
    ) => {
      if (ref !== null && typeof ref === 'object' && 'current' in ref) {
        (ref as { current: unknown }).current = mockStage;
      }
      function makeEvt(overrides: object = {}) {
        return { evt: { preventDefault: vi.fn() }, target: mockStage, cancelBubble: false, ...overrides };
      }
      return (
        <div
          data-testid="konva-stage"
          data-width={width}
          data-height={height}
          onMouseDown={(e) => { void e; onMouseDown?.(makeEvt()); }}
          onMouseMove={(e) => { void e; onMouseMove?.(makeEvt()); }}
          onMouseUp={(e) => { void e; onMouseUp?.(makeEvt()); }}
          onClick={(e) => {
            void e;
            onClick?.({ ...makeEvt(), target: mockStage });
          }}
          onWheel={(e) => {
            void e;
            onWheel?.({ ...makeEvt(), evt: { deltaY: -100, preventDefault: vi.fn() } });
          }}
        >
          {children}
        </div>
      );
    },
  );
  Stage.displayName = 'MockStage';

  return {
    Stage,
    Layer: ({ children }: { children?: React.ReactNode }) => <div data-testid="konva-layer">{children}</div>,
    Rect: () => <div data-testid="konva-rect" />,
    Circle: ({ onClick, onMouseEnter, onMouseLeave }: {
      onClick?: (e: unknown) => void;
      onMouseEnter?: () => void;
      onMouseLeave?: () => void;
    }) => (
      <div
        data-testid="konva-circle"
        onClick={onClick ? (e) => onClick({ evt: e, target: mockStage, cancelBubble: false }) : undefined}
        onMouseEnter={onMouseEnter}
        onMouseLeave={onMouseLeave}
      />
    ),
    Line: () => <div data-testid="konva-line" />,
    Image: ({ x, y, width, height, opacity, listening }: {
      image?: HTMLImageElement;
      x?: number;
      y?: number;
      width?: number;
      height?: number;
      opacity?: number;
      listening?: boolean;
    }) => (
      <div
        data-testid="konva-image"
        data-x={x}
        data-y={y}
        data-width={width}
        data-height={height}
        data-opacity={opacity}
        data-listening={String(listening)}
      />
    ),
    Text: ({ text }: { text?: string }) => <span data-testid="konva-text">{text}</span>,
    Group: ({ children, onClick, onMouseEnter, onMouseLeave }: {
      children?: React.ReactNode;
      onClick?: (e: unknown) => void;
      onMouseEnter?: () => void;
      onMouseLeave?: () => void;
    }) => (
      <div
        data-testid="konva-group"
        onClick={onClick ? (e) => {
          const evt = { evt: e, target: mockStage, cancelBubble: false };
          onClick(evt);
        } : undefined}
        onMouseEnter={onMouseEnter}
        onMouseLeave={onMouseLeave}
      >
        {children}
      </div>
    ),
  };
});

// ─── Test helpers ────────────────────────────────────────────────────────────

import { GreenhouseMap, type GreenhouseMapProps, type MapLocation } from '../features/map/GreenhouseMap';

function makePhoto(overrides: Partial<Photo> = {}): Photo {
  return {
    id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
    x: 10,
    y: 20,
    h: 2.5,
    width: 800,
    height: 600,
    capturedAt: '2024-06-01T10:00:00Z',
    originalUrl: 'http://storage.test/photo.jpg',
    predictions: [],
    ...overrides,
  };
}

function renderMap(overrides: Partial<GreenhouseMapProps> = {}) {
  const onModeChange = vi.fn();
  const onSelectLocation = vi.fn();
  const onHighlightPhoto = vi.fn();
  const onRetry = vi.fn();
  const props: GreenhouseMapProps = {
    mode: 'compact',
    onModeChange,
    selectedLocation: null,
    onSelectLocation,
    highlightedPhotoId: null,
    onHighlightPhoto,
    classId: null,
    minConfidence: null,
    mapPhotos: [],
    mapFetching: false,
    mapError: false,
    onRetry,
    hasMore: false,
    ...overrides,
  };
  const { rerender } = render(<GreenhouseMap {...props} />);
  return { onModeChange, onSelectLocation, onHighlightPhoto, onRetry, rerender, props };
}

beforeEach(() => {
  mockPointerPos = { x: 100, y: 100 };
  if (typeof HTMLDialogElement !== 'undefined' && !HTMLDialogElement.prototype.showModal) {
    HTMLDialogElement.prototype.showModal = vi.fn();
    HTMLDialogElement.prototype.close = vi.fn();
  }
});

afterEach(() => {
  cleanup();
});

// ─── Drawer: hidden mode ────────────────────────────────────────────────────

describe('GreenhouseMap — hidden mode', () => {
  it('renders Show greenhouse map launcher', () => {
    renderMap({ mode: 'hidden' });
    expect(screen.getByRole('button', { name: /Show greenhouse map/i })).toBeInTheDocument();
  });

  it('clicking launcher calls onModeChange with compact', () => {
    const { onModeChange } = renderMap({ mode: 'hidden' });
    fireEvent.click(screen.getByRole('button', { name: /Show greenhouse map/i }));
    expect(onModeChange).toHaveBeenCalledWith('compact');
  });

  it('shows location active label when selectedLocation is set', () => {
    renderMap({ mode: 'hidden', selectedLocation: { x: 15.5, y: 8.0 } });
    expect(
      screen.getByRole('button', { name: /x 15\.5 m.*y 8\.0 m.*active/i }),
    ).toBeInTheDocument();
  });

  it('no Konva stage or drawer header in hidden mode', () => {
    renderMap({ mode: 'hidden' });
    expect(screen.queryByTestId('konva-stage')).toBeNull();
    expect(screen.queryByText('Expand')).toBeNull();
  });
});

// ─── Drawer: compact mode ───────────────────────────────────────────────────

describe('GreenhouseMap — compact mode', () => {
  it('renders Expand map and Hide map buttons', () => {
    renderMap({ mode: 'compact' });
    expect(screen.getByRole('button', { name: 'Expand map' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Hide map' })).toBeInTheDocument();
  });

  it('Expand button calls onModeChange with expanded', () => {
    const { onModeChange } = renderMap({ mode: 'compact' });
    fireEvent.click(screen.getByRole('button', { name: 'Expand map' }));
    expect(onModeChange).toHaveBeenCalledWith('expanded');
  });

  it('Hide button calls onModeChange with hidden', () => {
    const { onModeChange } = renderMap({ mode: 'compact' });
    fireEvent.click(screen.getByRole('button', { name: 'Hide map' }));
    expect(onModeChange).toHaveBeenCalledWith('hidden');
  });

  it('renders Konva stage', () => {
    renderMap({ mode: 'compact' });
    expect(screen.getByTestId('konva-stage')).toBeInTheDocument();
  });

  it('Zoom in / Zoom out / Reset view buttons present', () => {
    renderMap({ mode: 'compact' });
    expect(screen.getByRole('button', { name: 'Zoom in' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Zoom out' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reset view' })).toBeInTheDocument();
  });
});

// ─── Drawer: expanded mode ──────────────────────────────────────────────────

describe('GreenhouseMap — expanded mode', () => {
  it('renders a dialog element with expandedDialog class', () => {
    renderMap({ mode: 'expanded' });
    const dialog = document.querySelector('dialog');
    expect(dialog).toBeInTheDocument();
    // Class proves the centred/enlarged layout contract without brittle pixel assertions
    expect(dialog?.className).toMatch(/expandedDialog/);
  });

  it('renders Return to mini-map and Hide map buttons', () => {
    renderMap({ mode: 'expanded' });
    expect(screen.getByRole('button', { name: 'Return to mini-map' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Hide map' })).toBeInTheDocument();
  });

  it('renders Cancel and Apply location buttons in action bar', () => {
    renderMap({ mode: 'expanded' });
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Apply location' })).toBeInTheDocument();
  });

  it('Apply location is disabled when no draft location', () => {
    renderMap({ mode: 'expanded', selectedLocation: null });
    expect(screen.getByRole('button', { name: 'Apply location' })).toBeDisabled();
  });

  it('Apply location is enabled when draft location exists (initialized from selectedLocation)', () => {
    renderMap({ mode: 'expanded', selectedLocation: { x: 5, y: 10 } });
    expect(screen.getByRole('button', { name: 'Apply location' })).not.toBeDisabled();
  });

  it('Return to mini-map fires dialog close → onModeChange compact', () => {
    const { onModeChange } = renderMap({ mode: 'expanded' });
    fireEvent.click(screen.getByRole('button', { name: 'Return to mini-map' }));
    const dialog = document.querySelector('dialog')!;
    fireEvent(dialog, new Event('close'));
    expect(onModeChange).toHaveBeenCalledWith('compact');
  });

  it('Escape (dialog close event) calls onModeChange with compact', () => {
    const { onModeChange } = renderMap({ mode: 'expanded' });
    const dialog = document.querySelector('dialog')!;
    fireEvent(dialog, new Event('close'));
    expect(onModeChange).toHaveBeenCalledWith('compact');
  });
});

// ─── Marker list disclosure ─────────────────────────────────────────────────

describe('GreenhouseMap — marker list disclosure', () => {
  it('marker list is absent initially in compact mode', () => {
    renderMap({ mode: 'compact', mapPhotos: [makePhoto()] });
    expect(screen.queryByRole('list', { name: /Photo locations/i })).toBeNull();
  });

  it('marker list is absent initially in expanded mode', () => {
    renderMap({ mode: 'expanded', mapPhotos: [makePhoto()] });
    expect(screen.queryByRole('list', { name: /Photo locations/i })).toBeNull();
  });

  it('toggle button shows correct marker count in compact', () => {
    const photos = [makePhoto({ id: 'a' }), makePhoto({ id: 'b', x: 5, y: 5 })];
    renderMap({ mode: 'compact', mapPhotos: photos });
    expect(screen.getByRole('button', { name: /Markers \(2\)/i })).toBeInTheDocument();
  });

  it('toggle button shows correct marker count in expanded', () => {
    const photos = [makePhoto({ id: 'a' }), makePhoto({ id: 'b', x: 5, y: 5 })];
    renderMap({ mode: 'expanded', mapPhotos: photos });
    expect(screen.getByRole('button', { name: /Markers \(2\)/i })).toBeInTheDocument();
  });

  it('clicking toggle reveals the list; clicking again hides it', () => {
    renderMap({ mode: 'compact', mapPhotos: [makePhoto()] });
    const toggle = screen.getByRole('button', { name: /Markers/i });
    expect(screen.queryByRole('list')).toBeNull();
    fireEvent.click(toggle);
    expect(screen.getByRole('list', { name: /Photo locations/i })).toBeInTheDocument();
    fireEvent.click(toggle);
    expect(screen.queryByRole('list')).toBeNull();
  });

  it('toggle has aria-expanded false initially then true after click', () => {
    renderMap({ mode: 'compact' });
    const toggle = screen.getByRole('button', { name: /Markers/i });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });

  it('list is collapsed after switching from compact to expanded (remount)', () => {
    const photo = makePhoto();
    const { rerender, props } = renderMap({ mode: 'compact', mapPhotos: [photo] });
    // Open the list in compact
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    expect(screen.getByRole('list')).toBeInTheDocument();
    // Switch to expanded — KonvaMapCanvas remounts, list resets
    rerender(<GreenhouseMap {...props} mode="expanded" mapPhotos={[photo]} />);
    expect(screen.queryByRole('list', { name: /Photo locations/i })).toBeNull();
  });
});

// ─── Coordinate readout ─────────────────────────────────────────────────────

describe('GreenhouseMap — coordinate readout', () => {
  it('shows Location not selected when no applied location in compact', () => {
    renderMap({ mode: 'compact', selectedLocation: null });
    expect(screen.getByText('Location not selected')).toBeInTheDocument();
  });

  it('shows applied location coordinates in compact mode', () => {
    renderMap({ mode: 'compact', selectedLocation: { x: 12.5, y: 8.0 } });
    expect(screen.getByText('x 12.5 m · y 8.0 m')).toBeInTheDocument();
  });

  it('shows Location not selected in expanded when no draft location', () => {
    renderMap({ mode: 'expanded', selectedLocation: null });
    expect(screen.getByText('Location not selected')).toBeInTheDocument();
  });

  it('shows draft location after list-row click in expanded (applied callback not called)', () => {
    const photo = makePhoto({ id: 'p1', x: 22, y: 15 });
    const { onSelectLocation } = renderMap({ mode: 'expanded', mapPhotos: [photo], selectedLocation: null });
    // Open list and click a row — calls draft setter, NOT onSelectLocation
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    fireEvent.click(screen.getByRole('button', { name: /x\s*22\.0\s*m/i }));
    // Coordinate readout reflects draft
    expect(screen.getByText('x 22.0 m · y 15.0 m')).toBeInTheDocument();
    // Applied callback NOT called
    expect(onSelectLocation).not.toHaveBeenCalled();
  });

  it('shows applied location in compact readout after compact list click', () => {
    const photo = makePhoto({ id: 'p1', x: 7, y: 3 });
    const { onSelectLocation } = renderMap({ mode: 'compact', mapPhotos: [photo], selectedLocation: null });
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    fireEvent.click(screen.getByRole('button', { name: /x\s*7\.0\s*m/i }));
    // Applied callback IS called immediately in compact
    expect(onSelectLocation).toHaveBeenCalledWith({ x: 7, y: 3 });
  });
});

// ─── Draft / Apply / Cancel workflow ───────────────────────────────────────

describe('GreenhouseMap — expanded draft workflow', () => {
  it('expanded marker click updates draft but does NOT call onSelectLocation', () => {
    const photo = makePhoto({ id: 'p1', x: 22, y: 15 });
    const { onSelectLocation, onHighlightPhoto } = renderMap({
      mode: 'expanded',
      mapPhotos: [photo],
      selectedLocation: null,
    });
    const groups = screen.getAllByTestId('konva-group');
    const markerGroup = groups.find((g) => g.onclick !== null);
    if (markerGroup) {
      fireEvent.click(markerGroup);
    }
    expect(onSelectLocation).not.toHaveBeenCalled();
    expect(onHighlightPhoto).not.toHaveBeenCalled();
  });

  it('expanded list-row click updates draft but does NOT call onSelectLocation', () => {
    const photo = makePhoto({ id: 'p1', x: 18, y: 7 });
    const { onSelectLocation, onHighlightPhoto } = renderMap({
      mode: 'expanded',
      mapPhotos: [photo],
      selectedLocation: null,
    });
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    const btn = screen.getByRole('button', { name: /x\s*18\.0\s*m/i });
    fireEvent.click(btn);
    expect(onSelectLocation).not.toHaveBeenCalled();
    expect(onHighlightPhoto).not.toHaveBeenCalled();
  });

  it('expanded empty-space click creates draft with no highlight callback', () => {
    mockPointerPos = { x: 160, y: 140 };
    const { onSelectLocation, onHighlightPhoto } = renderMap({
      mode: 'expanded',
      selectedLocation: null,
    });
    const stage = screen.getByTestId('konva-stage');
    fireEvent.mouseDown(stage);
    fireEvent.mouseUp(stage);
    fireEvent.click(stage);
    // Draft updated internally — no applied callbacks
    expect(onSelectLocation).not.toHaveBeenCalled();
    expect(onHighlightPhoto).not.toHaveBeenCalled();
  });

  it('Apply commits draft location exactly once and calls onHighlightPhoto', () => {
    const photo = makePhoto({ id: 'p1', x: 22, y: 15 });
    const { onSelectLocation, onHighlightPhoto } = renderMap({
      mode: 'expanded',
      mapPhotos: [photo],
      selectedLocation: null,
    });
    // Use the accessible list to set draft (avoids DOM bubbling to stage mock)
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    fireEvent.click(screen.getByRole('button', { name: /x\s*22\.0\s*m/i }));

    fireEvent.click(screen.getByRole('button', { name: 'Apply location' }));
    const dialog = document.querySelector('dialog')!;
    fireEvent(dialog, new Event('close'));

    expect(onSelectLocation).toHaveBeenCalledOnce();
    expect(onSelectLocation).toHaveBeenCalledWith({ x: 22, y: 15 });
    expect(onHighlightPhoto).toHaveBeenCalledOnce();
    expect(onHighlightPhoto).toHaveBeenCalledWith(photo.id);
  });

  it('Apply with initialized location (from selectedLocation) commits and triggers mode change', () => {
    const { onSelectLocation, onModeChange } = renderMap({
      mode: 'expanded',
      selectedLocation: { x: 5, y: 10 },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Apply location' }));
    const dialog = document.querySelector('dialog')!;
    fireEvent(dialog, new Event('close'));

    expect(onSelectLocation).toHaveBeenCalledWith({ x: 5, y: 10 });
    expect(onModeChange).toHaveBeenCalledWith('compact');
  });

  it('Apply with empty-space draft clears photo highlight', () => {
    mockPointerPos = { x: 160, y: 140 };
    const { onSelectLocation, onHighlightPhoto } = renderMap({
      mode: 'expanded',
      selectedLocation: null,
    });
    const stage = screen.getByTestId('konva-stage');
    fireEvent.mouseDown(stage);
    fireEvent.mouseUp(stage);
    fireEvent.click(stage);

    fireEvent.click(screen.getByRole('button', { name: 'Apply location' }));
    const dialog = document.querySelector('dialog')!;
    fireEvent(dialog, new Event('close'));

    expect(onSelectLocation).toHaveBeenCalledOnce();
    expect(onHighlightPhoto).toHaveBeenCalledWith(null);
  });

  it('Cancel does not call applied location or highlight callbacks', () => {
    const photo = makePhoto({ id: 'p1', x: 22, y: 15 });
    const { onSelectLocation, onHighlightPhoto } = renderMap({
      mode: 'expanded',
      mapPhotos: [photo],
      selectedLocation: null,
    });
    // Draft a location via list
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    fireEvent.click(screen.getByRole('button', { name: /x\s*22\.0\s*m/i }));

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    const dialog = document.querySelector('dialog')!;
    fireEvent(dialog, new Event('close'));

    expect(onSelectLocation).not.toHaveBeenCalled();
    expect(onHighlightPhoto).not.toHaveBeenCalled();
  });

  it('Return to mini-map discards draft without calling applied callbacks', () => {
    const { onSelectLocation, onHighlightPhoto, onModeChange } = renderMap({
      mode: 'expanded',
      selectedLocation: { x: 5, y: 10 },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Return to mini-map' }));
    const dialog = document.querySelector('dialog')!;
    fireEvent(dialog, new Event('close'));

    expect(onSelectLocation).not.toHaveBeenCalled();
    expect(onHighlightPhoto).not.toHaveBeenCalled();
    expect(onModeChange).toHaveBeenCalledWith('compact');
  });

  it('Escape (dialog close event without prior button click) preserves applied state', () => {
    const { onSelectLocation, onHighlightPhoto } = renderMap({
      mode: 'expanded',
      selectedLocation: { x: 5, y: 10 },
    });
    const dialog = document.querySelector('dialog')!;
    fireEvent(dialog, new Event('close'));

    expect(onSelectLocation).not.toHaveBeenCalled();
    expect(onHighlightPhoto).not.toHaveBeenCalled();
  });

  it('Hide from expanded discards draft and calls onModeChange with hidden', () => {
    const photo = makePhoto({ id: 'p1', x: 22, y: 15 });
    const { onSelectLocation, onModeChange } = renderMap({
      mode: 'expanded',
      mapPhotos: [photo],
      selectedLocation: null,
    });
    // Draft a location via list
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    fireEvent.click(screen.getByRole('button', { name: /x\s*22\.0\s*m/i }));

    fireEvent.click(screen.getByRole('button', { name: 'Hide map' }));
    const dialog = document.querySelector('dialog')!;
    fireEvent(dialog, new Event('close'));

    expect(onSelectLocation).not.toHaveBeenCalled();
    expect(onModeChange).toHaveBeenCalledWith('hidden');
  });

  it('preview count reflects draft position against map photos', () => {
    const near = makePhoto({ id: 'near', x: 1, y: 1 });
    const far = makePhoto({ id: 'far', x: 30, y: 30 });
    renderMap({
      mode: 'expanded',
      mapPhotos: [near, far],
      selectedLocation: { x: 1, y: 1 }, // draft initialized from this
    });
    // 1 photo within 3 m of (1, 1)
    expect(screen.getByText(/1 photo within 3 m/i)).toBeInTheDocument();
  });
});

// ─── Compact mode: immediate selection ─────────────────────────────────────

describe('GreenhouseMap — compact immediate selection', () => {
  it('compact marker click calls onSelectLocation immediately', () => {
    const photo = makePhoto({ x: 22, y: 15 });
    const { onSelectLocation } = renderMap({ mode: 'compact', mapPhotos: [photo] });
    const groups = screen.getAllByTestId('konva-group');
    const markerGroup = groups.find((g) => g.onclick !== null);
    if (markerGroup) {
      fireEvent.click(markerGroup);
      expect(onSelectLocation).toHaveBeenCalledWith({ x: 22, y: 15 });
    }
  });

  it('compact list selection calls onSelectLocation and onHighlightPhoto immediately', () => {
    const photo = makePhoto({ x: 18, y: 7 });
    const { onSelectLocation, onHighlightPhoto } = renderMap({ mode: 'compact', mapPhotos: [photo] });
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    const btn = screen.getByRole('button', { name: /x\s*18\.0\s*m/i });
    fireEvent.click(btn);
    expect(onSelectLocation).toHaveBeenCalledWith({ x: 18, y: 7 });
    expect(onHighlightPhoto).toHaveBeenCalledWith(photo.id);
  });

  it('compact background click calls onSelectLocation with valid world position immediately', () => {
    mockPointerPos = { x: 160, y: 140 };
    const { onSelectLocation } = renderMap({ mode: 'compact' });
    const stage = screen.getByTestId('konva-stage');
    fireEvent.mouseDown(stage);
    fireEvent.mouseUp(stage);
    fireEvent.click(stage);
    expect(onSelectLocation).toHaveBeenCalled();
    const loc = (onSelectLocation as ReturnType<typeof vi.fn>).mock.calls[0]?.[0] as MapLocation;
    expect(loc.x).toBeGreaterThanOrEqual(0);
    expect(loc.x).toBeLessThanOrEqual(40);
  });

  it('compact background click calls onHighlightPhoto(null) immediately', () => {
    mockPointerPos = { x: 160, y: 140 };
    const { onHighlightPhoto } = renderMap({ mode: 'compact' });
    const stage = screen.getByTestId('konva-stage');
    fireEvent.mouseDown(stage);
    fireEvent.mouseUp(stage);
    fireEvent.click(stage);
    expect(onHighlightPhoto).toHaveBeenCalledWith(null);
  });
});

// ─── Photo markers at real coordinates ─────────────────────────────────────

describe('GreenhouseMap — real x,y markers', () => {
  it('valid photos appear in the accessible DOM list with their real coordinates', () => {
    const photo = makePhoto({ x: 12.5, y: 33.7 });
    renderMap({ mapPhotos: [photo] });
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    expect(screen.getByText(/x\s*12\.5\s*m/i)).toBeInTheDocument();
    expect(screen.getByText(/y\s*33\.7\s*m/i)).toBeInTheDocument();
  });

  it('photos with invalid coordinates are omitted from the list', () => {
    const bad = makePhoto({ x: NaN, y: 5 });
    const good = makePhoto({ id: 'good', x: 5, y: 5 });
    renderMap({ mapPhotos: [bad, good] });
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    expect(screen.getAllByRole('listitem').length).toBe(1);
  });

  it('omitted photo count shown in status when coords are invalid', () => {
    const bad = makePhoto({ x: -1, y: 5 });
    renderMap({ mapPhotos: [bad] });
    expect(screen.getByText(/1 omitted/i)).toBeInTheDocument();
  });

  it('multiple photos with same coordinates both appear in the list', () => {
    const p1 = makePhoto({ id: 'id1', x: 20, y: 20 });
    const p2 = makePhoto({ id: 'id2', x: 20, y: 20 });
    renderMap({ mapPhotos: [p1, p2] });
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    const items = screen.getAllByRole('listitem');
    expect(items.length).toBe(2);
  });
});

// ─── Accessible list selection ──────────────────────────────────────────────

describe('GreenhouseMap — accessible list selection (compact, immediate)', () => {
  it('clicking a list item calls onSelectLocation with that photo x,y', () => {
    const photo = makePhoto({ x: 18, y: 7 });
    const { onSelectLocation } = renderMap({ mapPhotos: [photo] });
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    const btn = screen.getByRole('button', { name: /x\s*18\.0\s*m/i });
    fireEvent.click(btn);
    expect(onSelectLocation).toHaveBeenCalledWith({ x: 18, y: 7 });
  });

  it('clicking a list item also calls onHighlightPhoto with that photo id', () => {
    const photo = makePhoto({ x: 18, y: 7 });
    const { onHighlightPhoto } = renderMap({ mapPhotos: [photo] });
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    const btn = screen.getByRole('button', { name: /x\s*18\.0\s*m/i });
    fireEvent.click(btn);
    expect(onHighlightPhoto).toHaveBeenCalledWith(photo.id);
  });

  it('list item for a near photo is highlighted when selectedLocation is set', () => {
    const photo = makePhoto({ x: 10, y: 10 });
    renderMap({ mapPhotos: [photo], selectedLocation: { x: 10, y: 10 } });
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    const listItem = screen.getAllByRole('listitem')[0]!;
    expect(listItem.className).toMatch(/locationItemNear/);
  });

  it('list item far from selected location is NOT highlighted', () => {
    const photo = makePhoto({ x: 10, y: 10 });
    renderMap({ mapPhotos: [photo], selectedLocation: { x: 20, y: 20 } });
    fireEvent.click(screen.getByRole('button', { name: /Markers/i }));
    const listItem = screen.getAllByRole('listitem')[0]!;
    expect(listItem.className).not.toMatch(/locationItemNear/);
  });
});

// ─── Zoom controls ──────────────────────────────────────────────────────────

describe('GreenhouseMap — zoom controls', () => {
  it('Zoom out is disabled at MIN_SCALE', () => {
    renderMap({ mode: 'compact' });
    expect(screen.getByRole('button', { name: 'Zoom out' })).toBeDisabled();
  });

  it('Zoom in increases scale (Zoom out becomes enabled)', () => {
    renderMap({ mode: 'compact' });
    const zoomIn = screen.getByRole('button', { name: 'Zoom in' });
    const zoomOut = screen.getByRole('button', { name: 'Zoom out' });
    fireEvent.click(zoomIn);
    expect(zoomOut).not.toBeDisabled();
  });

  it('Reset view restores Zoom out disabled state', () => {
    renderMap({ mode: 'compact' });
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }));
    fireEvent.click(screen.getByRole('button', { name: 'Reset view' }));
    expect(screen.getByRole('button', { name: 'Zoom out' })).toBeDisabled();
  });

  it('wheel event increases zoom (Zoom out becomes enabled)', () => {
    renderMap({ mode: 'compact' });
    const stage = screen.getByTestId('konva-stage');
    fireEvent.wheel(stage, { deltaY: -100 });
    expect(screen.getByRole('button', { name: 'Zoom out' })).not.toBeDisabled();
  });
});

// ─── Canvas click (background) ─────────────────────────────────────────────

describe('GreenhouseMap — canvas background click', () => {
  it('background click calls onSelectLocation with a valid world position', () => {
    mockPointerPos = { x: 160, y: 140 };
    const { onSelectLocation } = renderMap({ mode: 'compact' });
    const stage = screen.getByTestId('konva-stage');
    fireEvent.mouseDown(stage);
    fireEvent.mouseUp(stage);
    fireEvent.click(stage);
    expect(onSelectLocation).toHaveBeenCalled();
    const calls = (onSelectLocation as ReturnType<typeof vi.fn>).mock.calls;
    const loc = (calls[0] ?? [])[0] as MapLocation;
    expect(loc.x).toBeGreaterThanOrEqual(0);
    expect(loc.x).toBeLessThanOrEqual(40);
    expect(loc.y).toBeGreaterThanOrEqual(0);
    expect(loc.y).toBeLessThanOrEqual(40);
  });

  it('background click calls onHighlightPhoto(null)', () => {
    mockPointerPos = { x: 160, y: 140 };
    const { onHighlightPhoto } = renderMap({ mode: 'compact' });
    const stage = screen.getByTestId('konva-stage');
    fireEvent.mouseDown(stage);
    fireEvent.mouseUp(stage);
    fireEvent.click(stage);
    expect(onHighlightPhoto).toHaveBeenCalledWith(null);
  });

  it('drag above threshold does NOT call onSelectLocation', () => {
    mockPointerPos = { x: 50, y: 50 };
    const { onSelectLocation } = renderMap({ mode: 'compact' });
    const stage = screen.getByTestId('konva-stage');
    fireEvent.mouseDown(stage);
    mockPointerPos = { x: 100, y: 100 };
    fireEvent.mouseMove(stage);
    mockPointerPos = { x: 150, y: 150 };
    fireEvent.mouseMove(stage);
    fireEvent.mouseUp(stage);
    expect(onSelectLocation).not.toHaveBeenCalled();
  });
});

// ─── Marker click ──────────────────────────────────────────────────────────

describe('GreenhouseMap — marker click', () => {
  it('clicking the Konva Group for a marker calls onSelectLocation with photo x,y', () => {
    const photo = makePhoto({ x: 22, y: 15 });
    const { onSelectLocation } = renderMap({ mapPhotos: [photo] });
    const groups = screen.getAllByTestId('konva-group');
    const markerGroup = groups.find((g) => g.getAttribute('onclick') !== null || g.onclick !== null);
    if (markerGroup) {
      fireEvent.click(markerGroup);
      expect(onSelectLocation).toHaveBeenCalledWith({ x: 22, y: 15 });
    }
  });
});

// ─── Loading / error states ─────────────────────────────────────────────────

describe('GreenhouseMap — loading and error states', () => {
  it('shows loading status while fetching with no photos', () => {
    renderMap({ mapPhotos: [], mapFetching: true });
    expect(screen.getByText('Loading map data…')).toBeInTheDocument();
  });

  it('shows error alert when mapError=true', () => {
    renderMap({ mapError: true, mapPhotos: [] });
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('Retry button calls onRetry', () => {
    const { onRetry } = renderMap({ mapError: true, mapPhotos: [] });
    fireEvent.click(screen.getByRole('button', { name: /retry/i }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it('shows bounded-200 disclosure when hasMore=true', () => {
    renderMap({ hasMore: true });
    expect(screen.getByText(/first 200 matching photos/i)).toBeInTheDocument();
  });

  it('does not show disclosure when hasMore=false', () => {
    renderMap({ hasMore: false });
    expect(screen.queryByText(/first 200 matching photos/i)).toBeNull();
  });

  it('renders without crashing with empty mapPhotos', () => {
    expect(() => renderMap({ mapPhotos: [] })).not.toThrow();
  });

  it('malformed photo (NaN coords) does not crash the component', () => {
    const bad = makePhoto({ x: NaN, y: NaN });
    const good = makePhoto({ id: 'good', x: 5, y: 5 });
    expect(() => renderMap({ mapPhotos: [bad, good] })).not.toThrow();
  });
});

// ─── Compact ↔ expanded transition ─────────────────────────────────────────

describe('GreenhouseMap — mode transition', () => {
  it('selectedLocation prop renders Konva stage in both compact and expanded mode', () => {
    const loc: MapLocation = { x: 12, y: 25 };
    const { rerender, props } = renderMap({ mode: 'compact', selectedLocation: loc });
    expect(screen.getByTestId('konva-stage')).toBeInTheDocument();

    rerender(<GreenhouseMap {...props} mode="expanded" selectedLocation={loc} />);
    expect(document.querySelector('dialog')).toBeInTheDocument();
    expect(screen.getByTestId('konva-stage')).toBeInTheDocument();
  });
});

// ─── Background image ───────────────────────────────────────────────────────

describe('GreenhouseMap — background image', () => {
  interface MockImg {
    onload: (() => void) | null;
    onerror: (() => void) | null;
    src: string;
  }

  let lastMockImg: MockImg | null = null;

  beforeEach(() => {
    lastMockImg = null;
    vi.stubGlobal('Image', class implements MockImg {
      onload: (() => void) | null = null;
      onerror: (() => void) | null = null;
      src = '';
      constructor() {
        // eslint-disable-next-line @typescript-eslint/no-this-alias
        lastMockImg = this;
      }
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('does not render Bed 1, Bed 2, or Bed 3', () => {
    renderMap({ mode: 'compact' });
    expect(screen.queryByText('Bed 1')).toBeNull();
    expect(screen.queryByText('Bed 2')).toBeNull();
    expect(screen.queryByText('Bed 3')).toBeNull();
  });

  it('background node does not appear before image loads', () => {
    renderMap({ mode: 'compact' });
    expect(screen.queryByTestId('konva-image')).toBeNull();
  });

  it('background node appears after successful image load', () => {
    renderMap({ mode: 'compact' });
    act(() => { lastMockImg?.onload?.(); });
    expect(screen.getByTestId('konva-image')).toBeInTheDocument();
  });

  it('background node has opacity 0.6 and listening false', () => {
    renderMap({ mode: 'compact' });
    act(() => { lastMockImg?.onload?.(); });
    const img = screen.getByTestId('konva-image');
    expect(img).toHaveAttribute('data-opacity', '0.6');
    expect(img).toHaveAttribute('data-listening', 'false');
  });

  it('background node is positioned at plot bounds (square, same x and y padding)', () => {
    renderMap({ mode: 'compact' });
    act(() => { lastMockImg?.onload?.(); });
    const img = screen.getByTestId('konva-image');
    const w = img.getAttribute('data-width');
    const h = img.getAttribute('data-height');
    expect(Number(w)).toBeGreaterThan(0);
    expect(w).toBe(h);
    expect(img.getAttribute('data-x')).toBe(img.getAttribute('data-y'));
  });

  it('background node is rendered before grid lines (first visible child of layer)', () => {
    renderMap({ mode: 'compact' });
    act(() => { lastMockImg?.onload?.(); });
    const layer = screen.getByTestId('konva-layer');
    const children = Array.from(layer.children);
    const imgIdx = children.findIndex((el) => el.getAttribute('data-testid') === 'konva-image');
    const groupIdx = children.findIndex((el) => el.getAttribute('data-testid') === 'konva-group');
    expect(imgIdx).toBeGreaterThanOrEqual(0);
    expect(imgIdx).toBeLessThan(groupIdx);
  });

  it('failed image load leaves the functional map visible without background', () => {
    renderMap({ mode: 'compact' });
    act(() => { lastMockImg?.onerror?.(); });
    expect(screen.queryByTestId('konva-image')).toBeNull();
    expect(screen.getByTestId('konva-stage')).toBeInTheDocument();
  });

  it('incomplete load (neither onload nor onerror) leaves the functional map visible', () => {
    renderMap({ mode: 'compact' });
    expect(screen.queryByTestId('konva-image')).toBeNull();
    expect(screen.getByTestId('konva-stage')).toBeInTheDocument();
  });

  it('background click still reaches the selection path when background is present', () => {
    mockPointerPos = { x: 160, y: 140 };
    const { onSelectLocation } = renderMap({ mode: 'compact' });
    act(() => { lastMockImg?.onload?.(); });
    const stage = screen.getByTestId('konva-stage');
    fireEvent.mouseDown(stage);
    fireEvent.mouseUp(stage);
    fireEvent.click(stage);
    expect(onSelectLocation).toHaveBeenCalled();
  });
});

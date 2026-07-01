import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

// Mock env config
vi.mock('../shared/config/env', () => ({
  configResult: { ok: true, config: { apiBaseUrl: 'http://api.test', apiKey: 'secret' } },
}));

import { PhotoViewer } from '../features/viewer/PhotoViewer';

const PHOTO_A = {
  id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
  x: 1,
  y: 2,
  h: 3,
  width: 800,
  height: 600,
  capturedAt: '2024-06-01T10:00:00Z',
  originalUrl: 'http://storage.test/original-a.jpg',
  predictions: [
    {
      classId: 'mirid',
      confidence: 0.87,
      bbox: { xMin: 0.1, yMin: 0.2, xMax: 0.8, yMax: 0.9 },
    },
  ],
};

const PHOTO_B = {
  id: 'b2c3d4e5-f6a7-8901-bcde-f12345678901',
  x: 2,
  y: 3,
  h: 4,
  width: 1024,
  height: 768,
  capturedAt: '2024-06-02T10:00:00Z',
  originalUrl: 'http://storage.test/original-b.jpg',
  predictions: [],
};

const PHOTOS = [PHOTO_A, PHOTO_B];

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
});

describe('PhotoViewer', () => {
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

  it('shows the prediction class label and confidence', () => {
    renderViewer();
    expect(screen.getByText('Mirid')).toBeInTheDocument();
    expect(screen.getByText('87%')).toBeInTheDocument();
  });

  it('shows "No detections" for a photo without predictions', () => {
    renderViewer({ photos: [PHOTO_B], initialIndex: 0 });
    expect(screen.getByText('No detections')).toBeInTheDocument();
  });

  it('close button calls onClose', () => {
    const { onClose } = renderViewer();
    fireEvent.click(screen.getByRole('button', { name: 'Close photo viewer' }));
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('Escape key closes the viewer', async () => {
    const { onClose } = renderViewer();
    // Simulate native dialog close event (Escape triggers it in real browser)
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

  it('Previous button is disabled on first photo', () => {
    renderViewer({ initialIndex: 0 });
    expect(screen.getByRole('button', { name: 'Previous photo' })).toBeDisabled();
  });

  it('Next button is disabled on last photo', () => {
    renderViewer({ initialIndex: 1 });
    expect(screen.getByRole('button', { name: 'Next photo' })).toBeDisabled();
  });

  it('clicking Next navigates to next photo', async () => {
    const user = userEvent.setup();
    renderViewer({ initialIndex: 0 });
    await user.click(screen.getByRole('button', { name: 'Next photo' }));
    expect(screen.getByText('No detections')).toBeInTheDocument();
    expect(screen.getByText('2 / 2')).toBeInTheDocument();
  });

  it('clicking Previous navigates back', async () => {
    const user = userEvent.setup();
    renderViewer({ initialIndex: 1 });
    await user.click(screen.getByRole('button', { name: 'Previous photo' }));
    expect(screen.getByText('Mirid')).toBeInTheDocument();
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
  });

  it('ArrowRight key navigates to next photo', async () => {
    renderViewer({ initialIndex: 0 });
    fireEvent.keyDown(window, { key: 'ArrowRight' });
    expect(screen.getByText('2 / 2')).toBeInTheDocument();
  });

  it('ArrowLeft key navigates to previous photo', async () => {
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

  it('shows loading state initially (before image loads)', () => {
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

  it('error message does not expose the original URL', () => {
    renderViewer();
    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.error(img);
    const alert = screen.getByRole('alert');
    expect(alert.textContent).not.toContain('http://storage.test');
    expect(alert.textContent).not.toContain('original-a.jpg');
  });

  it('retry resets error state', async () => {
    const user = userEvent.setup();
    renderViewer();
    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.error(img);
    expect(screen.getByRole('alert')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByRole('status', { name: 'Loading image' })).toBeInTheDocument();
  });

  it('calls onClose when photo is removed from list (selection invalidation)', () => {
    const { onClose, rerender } = renderViewer({ photos: [PHOTO_A], initialIndex: 0 });
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

  it('renders a bbox SVG overlay for predictions', () => {
    renderViewer({ initialIndex: 0 });
    // The image needs to be loaded first for the overlay to render
    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.load(img);
    const svgs = document.querySelectorAll('svg');
    // At least one SVG should be present for the overlay
    expect(svgs.length).toBeGreaterThan(0);
  });

  it('prediction inspection does not depend on hover (text is always visible)', () => {
    renderViewer({ initialIndex: 0 });
    const img = screen.getByAltText(/Greenhouse photo/);
    fireEvent.load(img);
    // Class label and confidence are in the sidebar list, not hidden behind hover
    expect(screen.getByText('Mirid')).toBeInTheDocument();
    expect(screen.getByText('87%')).toBeInTheDocument();
  });
});

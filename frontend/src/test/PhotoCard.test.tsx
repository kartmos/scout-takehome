import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import type { components } from '../entities/api/__generated__/schema';
import { PhotoCard } from '../entities/photo/PhotoCard';

vi.mock('../shared/config/env', () => ({
  configResult: { ok: true, config: { apiBaseUrl: 'http://api.test', apiKey: 'secret' } },
}));

type Photo = components['schemas']['Photo'];

const BASE_BBOX = { xMin: 0.0, yMin: 0.0, xMax: 0.5, yMax: 0.5 };
const BBOX_B = { xMin: 0.1, yMin: 0.1, xMax: 0.9, yMax: 0.9 };
const BBOX_C = { xMin: 0.2, yMin: 0.2, xMax: 0.8, yMax: 0.8 };

const PHOTO_MULTI: Photo = {
  id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
  x: 0, y: 0, h: 0,
  width: 800,
  height: 600,
  capturedAt: '2024-06-01T10:00:00Z',
  originalUrl: 'http://storage.test/photo.jpg',
  predictions: [
    { classId: 'mirid', confidence: 0.87, bbox: BASE_BBOX },
    { classId: 'mirid', confidence: 0.62, bbox: BBOX_B },
    { classId: 'powdery_mildew', confidence: 0.40, bbox: BBOX_C },
    { classId: 'powdery_mildew', confidence: 0.54, bbox: BASE_BBOX },
  ],
};

const PHOTO_SINGLE: Photo = {
  ...PHOTO_MULTI,
  predictions: [
    { classId: 'mirid', confidence: 0.58, bbox: BASE_BBOX },
  ],
};

const PHOTO_NONE: Photo = {
  ...PHOTO_MULTI,
  predictions: [],
};

function renderCard(photo: Photo, matchingClassId: string | null = null, onSelect?: () => void) {
  const handler = onSelect ?? vi.fn();
  return render(
    <PhotoCard
      photo={photo}
      matchingClassId={matchingClassId}
      onSelect={() => handler()}
    />,
  );
}

describe('PhotoCard — grouped detection summary', () => {
  it('repeated predictions of one class produce one grouped row per class', () => {
    renderCard(PHOTO_MULTI);
    // 4 predictions (2 mirid + 2 powdery_mildew) → only 2 active list rows
    const list = screen.getByRole('list', { name: 'Detections' });
    const activeItems = list.querySelectorAll('li:not([aria-hidden])');
    expect(activeItems.length).toBe(2);
    // Each class label appears exactly once
    expect(screen.getAllByText('Mirid')).toHaveLength(1);
    expect(screen.getAllByText('Powdery Mildew')).toHaveLength(1);
  });

  it('confidence range is shown for multiple predictions of one class', () => {
    renderCard(PHOTO_MULTI);
    expect(screen.getByText('62–87%')).toBeInTheDocument();
  });

  it('single prediction shows its exact confidence without a range', () => {
    renderCard(PHOTO_SINGLE);
    expect(screen.getByText('58%')).toBeInTheDocument();
    expect(screen.queryByText(/–/)).toBeNull();
  });

  it('equal min and max confidence for multiple predictions shows one value', () => {
    const photo: Photo = {
      ...PHOTO_MULTI,
      predictions: [
        { classId: 'mirid', confidence: 0.80, bbox: BASE_BBOX },
        { classId: 'mirid', confidence: 0.80, bbox: BBOX_B },
      ],
    };
    renderCard(photo);
    expect(screen.getByText('80%')).toBeInTheDocument();
    expect(screen.queryByText('80–80%')).toBeNull();
  });

  it('real groups appear alphabetically: Mirid before Powdery Mildew', () => {
    renderCard(PHOTO_MULTI);
    const items = screen.getAllByRole('listitem');
    const mIdx = items.findIndex((li) => li.textContent?.includes('Mirid'));
    const pmIdx = items.findIndex((li) => li.textContent?.includes('Powdery Mildew'));
    expect(mIdx).toBeLessThan(pmIdx);
  });

  it('renders only real detection groups, not absent-class placeholders', () => {
    renderCard(PHOTO_MULTI);
    const list = screen.getByRole('list', { name: 'Detections' });
    const items = list.querySelectorAll('li');
    // Only 2 classes present → 2 items
    expect(items.length).toBe(2);
  });

  it('absent classes produce no list items', () => {
    renderCard(PHOTO_SINGLE);
    // Only mirid has predictions — no other class rows
    const list = screen.getByRole('list', { name: 'Detections' });
    const items = list.querySelectorAll('li');
    expect(items.length).toBe(1);
    expect(items[0]?.textContent).toContain('Mirid');
  });

  it('card with no predictions shows No detections and no class rows', () => {
    renderCard(PHOTO_NONE);
    const list = screen.getByRole('list', { name: 'Detections' });
    expect(list).toHaveTextContent('No detections');
    // No real class labels
    expect(list).not.toHaveTextContent('Mirid');
    expect(list).not.toHaveTextContent('Powdery Mildew');
  });
});

describe('PhotoCard — bbox behavior is unchanged', () => {
  it('grouping the summary does not reduce the count of rendered bbox groups', () => {
    const { container } = renderCard(PHOTO_MULTI);
    // 4 predictions with valid bboxes → 4 bbox groups (each group has halo + colored rect)
    const bboxGroups = container.querySelectorAll('svg g');
    expect(bboxGroups.length).toBe(4);
  });

  it('with matchingClassId=null (All), all bbox groups have full opacity', () => {
    const { container } = renderCard(PHOTO_MULTI, null);
    const bboxGroups = container.querySelectorAll('svg g');
    bboxGroups.forEach((g) => {
      expect(Number(g.getAttribute('opacity'))).toBe(1);
    });
  });

  it('with a selected class, non-matching bbox groups are dimmed', () => {
    const { container } = renderCard(PHOTO_MULTI, 'mirid');
    const groups = Array.from(container.querySelectorAll('svg g'));
    const matching = groups.filter((g) => Number(g.getAttribute('opacity')) === 1);
    const dimmed = groups.filter((g) => Number(g.getAttribute('opacity')) < 1);
    // 2 mirid groups at full opacity, 2 powdery_mildew groups dimmed
    expect(matching.length).toBe(2);
    expect(dimmed.length).toBe(2);
  });

  it('summary rows are not interactive — do not prevent viewer from opening', () => {
    const onSelect = vi.fn();
    render(
      <PhotoCard
        photo={PHOTO_MULTI}
        matchingClassId={null}
        onSelect={() => onSelect()}
      />,
    );
    // The open button covers the media area; click it
    fireEvent.click(screen.getByRole('button', { name: /Open photo/ }));
    expect(onSelect).toHaveBeenCalledOnce();
    // Detection list items must not have click handlers
    const items = screen.getAllByRole('listitem');
    items.forEach((item) => {
      expect(item.onclick).toBeNull();
    });
  });
});

describe('PhotoCard — confidence threshold', () => {
  const PHOTO_THRESHOLD: Photo = {
    ...PHOTO_MULTI,
    predictions: [
      { classId: 'powdery_mildew', confidence: 0.47, bbox: BASE_BBOX },
      { classId: 'powdery_mildew', confidence: 0.52, bbox: BBOX_B },
      { classId: 'powdery_mildew', confidence: 0.54, bbox: BBOX_C },
    ],
  };

  it('no threshold shows all predictions (count and range)', () => {
    render(<PhotoCard photo={PHOTO_THRESHOLD} matchingClassId={null} />);
    expect(screen.getByText('3×')).toBeInTheDocument();
    expect(screen.getByText('47–54%')).toBeInTheDocument();
  });

  it('threshold 0.5 filters count, range, and bbox group count from raw values', () => {
    const { container } = render(
      <PhotoCard photo={PHOTO_THRESHOLD} matchingClassId={null} minConfidence={0.5} />,
    );
    // 0.47 < 0.5 excluded; 0.52 and 0.54 pass → 2 predictions
    expect(screen.getByText('2×')).toBeInTheDocument();
    expect(screen.getByText('52–54%')).toBeInTheDocument();
    const bboxGroups = container.querySelectorAll('svg g');
    expect(bboxGroups.length).toBe(2);
  });

  it('all predictions filtered shows No detections at this confidence', () => {
    render(
      <PhotoCard photo={PHOTO_THRESHOLD} matchingClassId={null} minConfidence={0.9} />,
    );
    const list = screen.getByRole('list', { name: 'Detections' });
    expect(list).toHaveTextContent('No detections at this confidence');
  });

  it('selected-class dimming applies only to above-threshold bbox groups', () => {
    const photo: Photo = {
      ...PHOTO_MULTI,
      predictions: [
        { classId: 'mirid', confidence: 0.87, bbox: BASE_BBOX },
        { classId: 'powdery_mildew', confidence: 0.40, bbox: BBOX_B }, // below threshold
        { classId: 'powdery_mildew', confidence: 0.54, bbox: BBOX_C }, // above threshold
      ],
    };
    const { container } = render(
      <PhotoCard photo={photo} matchingClassId="mirid" minConfidence={0.5} />,
    );
    // 2 predictions pass threshold (mirid 0.87, powdery_mildew 0.54)
    const groups = Array.from(container.querySelectorAll('svg g'));
    expect(groups.length).toBe(2);
    // mirid group at full opacity; powdery_mildew group dimmed
    const full = groups.filter((g) => Number(g.getAttribute('opacity')) === 1);
    expect(full.length).toBe(1);
  });
});

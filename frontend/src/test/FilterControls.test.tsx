import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Provider } from 'react-redux';
import { configureStore } from '@reduxjs/toolkit';
import type { Middleware } from '@reduxjs/toolkit';
import filtersReducer, { setMinConfidence } from '../features/filters/filtersSlice';
import { FilterControls } from '../features/filters/FilterControls';

function makeStore(onAction?: (action: { type: string }) => void) {
  const collector: Middleware = () => (next) => (action) => {
    onAction?.(action as { type: string });
    return next(action);
  };
  return configureStore({
    reducer: {
      filters: filtersReducer,
      scoutApi: (s = {}) => s,
    },
    middleware: (getDefault) => getDefault().concat(collector),
  });
}

function renderFilterControls(onAction?: (action: { type: string }) => void) {
  const store = makeStore(onAction);
  render(
    <Provider store={store}>
      <FilterControls />
    </Provider>,
  );
  return store;
}

describe('FilterControls confidence input — percentage UI, ratio store', () => {
  it('entering 70 and committing stores 0.7 in Redux', () => {
    const store = renderFilterControls();
    const input = screen.getByRole('spinbutton');
    fireEvent.change(input, { target: { value: '70' } });
    fireEvent.blur(input);
    expect(store.getState().filters.minConfidence).toBe(0.7);
  });

  it('entering 0 stores 0 (boundary)', () => {
    const store = renderFilterControls();
    const input = screen.getByRole('spinbutton');
    fireEvent.change(input, { target: { value: '0' } });
    fireEvent.blur(input);
    expect(store.getState().filters.minConfidence).toBe(0);
  });

  it('entering 100 stores 1 (boundary)', () => {
    const store = renderFilterControls();
    const input = screen.getByRole('spinbutton');
    fireEvent.change(input, { target: { value: '100' } });
    fireEvent.blur(input);
    expect(store.getState().filters.minConfidence).toBe(1);
  });

  it('out-of-range value is clamped: entering 150 stores 1', () => {
    const store = renderFilterControls();
    const input = screen.getByRole('spinbutton');
    fireEvent.change(input, { target: { value: '150' } });
    fireEvent.blur(input);
    expect(store.getState().filters.minConfidence).toBe(1);
  });

  it('out-of-range negative value is clamped: entering -10 stores 0', () => {
    const store = renderFilterControls();
    const input = screen.getByRole('spinbutton');
    fireEvent.change(input, { target: { value: '-10' } });
    fireEvent.blur(input);
    expect(store.getState().filters.minConfidence).toBe(0);
  });

  it('empty input clears the filter (null)', () => {
    const store = renderFilterControls();
    const input = screen.getByRole('spinbutton');
    fireEvent.change(input, { target: { value: '70' } });
    fireEvent.blur(input);
    expect(store.getState().filters.minConfidence).toBe(0.7);

    fireEvent.change(input, { target: { value: '' } });
    fireEvent.blur(input);
    expect(store.getState().filters.minConfidence).toBeNull();
  });

  it('external Redux state 0.7 is displayed as 70 in the field', () => {
    const store = makeStore();
    store.dispatch(setMinConfidence(0.7));
    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    const input = screen.getByRole('spinbutton') as HTMLInputElement;
    expect(input.value).toBe('70');
  });

  it('reset clears the visible percentage field', () => {
    const store = makeStore();
    store.dispatch(setMinConfidence(0.7));
    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    const input = screen.getByRole('spinbutton') as HTMLInputElement;
    expect(input.value).toBe('70');
    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));
    expect(input.value).toBe('');
  });

  it('blur followed by Apply dispatches setMinConfidence only once', () => {
    const dispatched: { type: string }[] = [];
    renderFilterControls((action) => {
      if (action.type === 'filters/setMinConfidence') dispatched.push(action);
    });

    const input = screen.getByRole('spinbutton');
    const applyBtn = screen.getByRole('button', { name: 'Apply' });

    fireEvent.change(input, { target: { value: '70' } });
    fireEvent.blur(input);
    fireEvent.click(applyBtn);

    expect(dispatched).toHaveLength(1);
  });

  it('active summary shows percentage format: ≥70%', () => {
    const store = makeStore();
    store.dispatch(setMinConfidence(0.7));
    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    expect(screen.getByText(/≥70%/)).toBeInTheDocument();
  });

  it('label includes % indicator', () => {
    renderFilterControls();
    expect(screen.getByText(/minimum confidence/i)).toBeInTheDocument();
    expect(screen.getByText(/minimum confidence/i).textContent).toContain('%');
  });

  it('over-range input normalizes the visible field to 100 even when Redux already holds 1', () => {
    const dispatched: { type: string }[] = [];
    const store = makeStore((action) => {
      if (action.type === 'filters/setMinConfidence') dispatched.push(action);
    });
    store.dispatch(setMinConfidence(1));
    dispatched.length = 0; // reset after seed dispatch

    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    const input = screen.getByRole('spinbutton') as HTMLInputElement;
    expect(input.value).toBe('100'); // synced from Redux

    fireEvent.change(input, { target: { value: '150' } });
    fireEvent.blur(input);

    // Visible field must clamp to 100; no duplicate dispatch since ratio is unchanged
    expect(input.value).toBe('100');
    expect(dispatched).toHaveLength(0);
  });

  it('under-range input normalizes the visible field to 0 even when Redux already holds 0', () => {
    const dispatched: { type: string }[] = [];
    const store = makeStore((action) => {
      if (action.type === 'filters/setMinConfidence') dispatched.push(action);
    });
    store.dispatch(setMinConfidence(0));
    dispatched.length = 0;

    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    const input = screen.getByRole('spinbutton') as HTMLInputElement;

    fireEvent.change(input, { target: { value: '-10' } });
    fireEvent.blur(input);

    expect(input.value).toBe('0');
    expect(dispatched).toHaveLength(0);
  });
});

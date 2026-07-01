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

function getSelect() {
  return screen.getByLabelText('Minimum confidence') as HTMLSelectElement;
}

function selectCustom() {
  fireEvent.change(getSelect(), { target: { value: 'custom' } });
}

function getCustomInput() {
  return screen.getByRole('spinbutton') as HTMLInputElement;
}

describe('FilterControls — confidence presets', () => {
  it('Any — no filter stores null', () => {
    const store = renderFilterControls();
    // First select 70% to have an active filter, then switch back to Any
    fireEvent.change(getSelect(), { target: { value: '70' } });
    expect(store.getState().filters.minConfidence).toBe(0.7);
    fireEvent.change(getSelect(), { target: { value: 'any' } });
    expect(store.getState().filters.minConfidence).toBeNull();
  });

  it('preset 70% immediately stores 0.7', () => {
    const store = renderFilterControls();
    fireEvent.change(getSelect(), { target: { value: '70' } });
    expect(store.getState().filters.minConfidence).toBe(0.7);
  });

  it('preset 100% stores 1', () => {
    const store = renderFilterControls();
    fireEvent.change(getSelect(), { target: { value: '100' } });
    expect(store.getState().filters.minConfidence).toBe(1);
  });

  it('preset 10% stores 0.1', () => {
    const store = renderFilterControls();
    fireEvent.change(getSelect(), { target: { value: '10' } });
    expect(store.getState().filters.minConfidence).toBe(0.1);
  });

  it('switching from preset to Any does not duplicate dispatch', () => {
    const dispatched: { type: string }[] = [];
    renderFilterControls((a) => {
      if (a.type === 'filters/setMinConfidence') dispatched.push(a);
    });
    fireEvent.change(getSelect(), { target: { value: '70' } });
    dispatched.length = 0;
    fireEvent.change(getSelect(), { target: { value: 'any' } });
    expect(dispatched).toHaveLength(1);
  });
});

describe('FilterControls — custom value', () => {
  it('custom 73 stores 0.73', () => {
    const store = renderFilterControls();
    selectCustom();
    const input = getCustomInput();
    fireEvent.change(input, { target: { value: '73' } });
    fireEvent.blur(input);
    expect(store.getState().filters.minConfidence).toBe(0.73);
  });

  it('custom 0 stores 0 (lower boundary)', () => {
    const store = renderFilterControls();
    selectCustom();
    fireEvent.change(getCustomInput(), { target: { value: '0' } });
    fireEvent.blur(getCustomInput());
    expect(store.getState().filters.minConfidence).toBe(0);
  });

  it('custom 100 stores 1 (upper boundary)', () => {
    const store = renderFilterControls();
    selectCustom();
    fireEvent.change(getCustomInput(), { target: { value: '100' } });
    fireEvent.blur(getCustomInput());
    expect(store.getState().filters.minConfidence).toBe(1);
  });

  it('custom over-range 150 is clamped to 100 (stores 1)', () => {
    const store = renderFilterControls();
    selectCustom();
    fireEvent.change(getCustomInput(), { target: { value: '150' } });
    fireEvent.blur(getCustomInput());
    expect(store.getState().filters.minConfidence).toBe(1);
  });

  it('custom negative -10 is clamped to 0', () => {
    const store = renderFilterControls();
    selectCustom();
    fireEvent.change(getCustomInput(), { target: { value: '-10' } });
    fireEvent.blur(getCustomInput());
    expect(store.getState().filters.minConfidence).toBe(0);
  });

  it('Enter key commits the custom value', () => {
    const store = renderFilterControls();
    selectCustom();
    const input = getCustomInput();
    fireEvent.change(input, { target: { value: '45' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(store.getState().filters.minConfidence).toBe(0.45);
  });

  it('Apply button commits the custom value', () => {
    const store = renderFilterControls();
    selectCustom();
    fireEvent.change(getCustomInput(), { target: { value: '55' } });
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
    expect(store.getState().filters.minConfidence).toBe(0.55);
  });

  it('blur followed by Apply dispatches setMinConfidence only once', () => {
    const dispatched: { type: string }[] = [];
    renderFilterControls((a) => {
      if (a.type === 'filters/setMinConfidence') dispatched.push(a);
    });
    selectCustom();
    const input = getCustomInput();
    fireEvent.change(input, { target: { value: '65' } });
    fireEvent.blur(input);
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
    expect(dispatched).toHaveLength(1);
  });

  it('over-range clamp does not dispatch when Redux already holds 1', () => {
    const dispatched: { type: string }[] = [];
    const store = makeStore((a) => {
      if (a.type === 'filters/setMinConfidence') dispatched.push(a);
    });
    store.dispatch(setMinConfidence(1));
    dispatched.length = 0;
    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    selectCustom();
    fireEvent.change(getCustomInput(), { target: { value: '150' } });
    fireEvent.blur(getCustomInput());
    expect(getCustomInput().value).toBe('100');
    expect(dispatched).toHaveLength(0);
  });
});

describe('FilterControls — Redux ↔ UI synchronisation', () => {
  it('external 0.7 selects the 70% preset in the dropdown', () => {
    const store = makeStore();
    store.dispatch(setMinConfidence(0.7));
    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    expect(getSelect().value).toBe('70');
    expect(screen.queryByRole('spinbutton')).toBeNull();
  });

  it('external 0.73 (non-preset) selects Custom and shows 73 in the input', () => {
    const store = makeStore();
    store.dispatch(setMinConfidence(0.73));
    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    expect(getSelect().value).toBe('custom');
    expect(getCustomInput().value).toBe('73');
  });

  it('reset returns the select to Any and hides the custom input', () => {
    const store = makeStore();
    store.dispatch(setMinConfidence(0.7));
    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    expect(getSelect().value).toBe('70');
    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));
    expect(getSelect().value).toBe('any');
    expect(screen.queryByRole('spinbutton')).toBeNull();
  });

  it('switching from custom to a preset clears custom input and applies the preset', () => {
    const store = renderFilterControls();
    selectCustom();
    fireEvent.change(getCustomInput(), { target: { value: '73' } });
    fireEvent.blur(getCustomInput());
    expect(store.getState().filters.minConfidence).toBe(0.73);

    fireEvent.change(getSelect(), { target: { value: '80' } });
    expect(store.getState().filters.minConfidence).toBe(0.8);
    expect(screen.queryByRole('spinbutton')).toBeNull();
  });
});

describe('FilterControls — active summary layout', () => {
  it('active summary and Clear filters are sibling elements, not nested', () => {
    const store = makeStore();
    store.dispatch(setMinConfidence(0.7));
    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    const clearBtn = screen.getByRole('button', { name: 'Clear filters' });
    const summaryText = screen.getByText(/≥70%/);
    // The bordered summary container must not contain the button
    const borderedSummary = summaryText.parentElement;
    expect(borderedSummary?.contains(clearBtn)).toBe(false);
    // The button must not contain the summary text
    expect(clearBtn.contains(summaryText)).toBe(false);
  });

  it('Clear filters is a real button element', () => {
    const store = makeStore();
    store.dispatch(setMinConfidence(0.7));
    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    const clearBtn = screen.getByRole('button', { name: 'Clear filters' });
    expect(clearBtn.tagName).toBe('BUTTON');
  });

  it('Clear filters resets both class and confidence and hides the summary', () => {
    const store = makeStore();
    store.dispatch(setMinConfidence(0.7));
    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));
    expect(store.getState().filters.minConfidence).toBeNull();
    expect(screen.queryByRole('button', { name: 'Clear filters' })).toBeNull();
  });
});

describe('FilterControls — active summary', () => {
  it('active summary shows ≥70% when preset 70% is selected', () => {
    const store = makeStore();
    store.dispatch(setMinConfidence(0.7));
    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    expect(screen.getByText(/≥70%/)).toBeInTheDocument();
  });

  it('active summary shows ≥73% for custom non-preset value', () => {
    const store = makeStore();
    store.dispatch(setMinConfidence(0.73));
    render(
      <Provider store={store}>
        <FilterControls />
      </Provider>,
    );
    expect(screen.getByText(/≥73%/)).toBeInTheDocument();
  });

  it('no active summary when no filters are set', () => {
    renderFilterControls();
    expect(screen.queryByText(/≥/)).toBeNull();
    expect(screen.queryByRole('button', { name: 'Clear filters' })).toBeNull();
  });
});

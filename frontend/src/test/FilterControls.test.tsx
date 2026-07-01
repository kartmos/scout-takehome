import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Provider } from 'react-redux';
import { configureStore } from '@reduxjs/toolkit';
import type { Middleware } from '@reduxjs/toolkit';
import filtersReducer from '../features/filters/filtersSlice';
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

describe('FilterControls confidence input', () => {
  it('blur commits confidence to the store', () => {
    const store = renderFilterControls();
    const input = screen.getByRole('spinbutton');

    fireEvent.change(input, { target: { value: '0.7' } });
    fireEvent.blur(input);

    expect(store.getState().filters.minConfidence).toBe(0.7);
  });

  it('clicking Apply after blur does not dispatch a second setMinConfidence', () => {
    const dispatched: { type: string }[] = [];
    renderFilterControls((action) => {
      if (action.type === 'filters/setMinConfidence') dispatched.push(action);
    });

    const input = screen.getByRole('spinbutton');
    const applyBtn = screen.getByRole('button', { name: 'Apply' });

    fireEvent.change(input, { target: { value: '0.7' } });
    fireEvent.blur(input);
    fireEvent.click(applyBtn);

    expect(dispatched).toHaveLength(1);
  });
});

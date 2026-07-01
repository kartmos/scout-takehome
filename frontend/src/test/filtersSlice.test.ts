import { describe, it, expect } from 'vitest';
import filtersReducer, {
  setClassId,
  setMinConfidence,
  resetFilters,
  type ClassId,
} from '../features/filters/filtersSlice';

const initial = { classId: null, minConfidence: null };

describe('filtersSlice', () => {
  it('starts with null filters', () => {
    expect(filtersReducer(undefined, { type: '@@INIT' })).toEqual(initial);
  });

  it('setClassId stores a class id', () => {
    const state = filtersReducer(initial, setClassId('mirid' as ClassId));
    expect(state.classId).toBe('mirid');
    expect(state.minConfidence).toBeNull();
  });

  it('setClassId clears when null', () => {
    const withClass = { classId: 'thrips' as ClassId, minConfidence: null };
    expect(filtersReducer(withClass, setClassId(null)).classId).toBeNull();
  });

  it('setMinConfidence stores a value', () => {
    const state = filtersReducer(initial, setMinConfidence(0.75));
    expect(state.minConfidence).toBe(0.75);
    expect(state.classId).toBeNull();
  });

  it('setMinConfidence clears when null', () => {
    const withConf = { classId: null, minConfidence: 0.5 };
    expect(filtersReducer(withConf, setMinConfidence(null)).minConfidence).toBeNull();
  });

  it('resetFilters returns initial state', () => {
    const dirty = { classId: 'mirid' as ClassId, minConfidence: 0.9 };
    expect(filtersReducer(dirty, resetFilters())).toEqual(initial);
  });

  it('both filters can coexist', () => {
    let state = filtersReducer(initial, setClassId('spider_mites' as ClassId));
    state = filtersReducer(state, setMinConfidence(0.6));
    expect(state).toEqual({ classId: 'spider_mites', minConfidence: 0.6 });
  });
});

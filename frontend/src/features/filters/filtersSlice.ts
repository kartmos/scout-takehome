import { createSlice, type PayloadAction } from '@reduxjs/toolkit';

export const CLASS_IDS = [
  'powdery_mildew',
  'mirid',
  'whitefly_aphid',
  'miner_tuta',
  'thrips',
  'spider_mites',
] as const;

export type ClassId = (typeof CLASS_IDS)[number];

export const CLASS_LABELS: Record<ClassId, string> = {
  powdery_mildew: 'Powdery Mildew',
  mirid: 'Mirid',
  whitefly_aphid: 'Whitefly / Aphid',
  miner_tuta: 'Leaf Miner / Tuta',
  thrips: 'Thrips',
  spider_mites: 'Spider Mites',
};

interface FiltersState {
  classId: ClassId | null;
  minConfidence: number | null;
}

const initialState: FiltersState = {
  classId: null,
  minConfidence: null,
};

export const filtersSlice = createSlice({
  name: 'filters',
  initialState,
  reducers: {
    setClassId(state, action: PayloadAction<ClassId | null>) {
      state.classId = action.payload;
    },
    setMinConfidence(state, action: PayloadAction<number | null>) {
      state.minConfidence = action.payload;
    },
    resetFilters() {
      return initialState;
    },
  },
});

export const { setClassId, setMinConfidence, resetFilters } = filtersSlice.actions;
export default filtersSlice.reducer;

import { configureStore } from '@reduxjs/toolkit';
import { scoutApi } from '../shared/api/scoutApi';
import filtersReducer from '../features/filters/filtersSlice';

export const store = configureStore({
  reducer: {
    [scoutApi.reducerPath]: scoutApi.reducer,
    filters: filtersReducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(scoutApi.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;

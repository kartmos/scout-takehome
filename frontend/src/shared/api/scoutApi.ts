import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';
import type { components } from '../../entities/api/__generated__/schema';
import { configResult } from '../config/env';
import { normalizeApiError, normalizeNetworkError } from '../lib/apiError';

type Photo = components['schemas']['Photo'];
type PhotoPage = components['schemas']['PhotoPage'];

interface ListPhotosParams {
  cursor?: string;
  limit?: number;
  classId?: string;
  minConfidence?: number;
  nearX?: number;
  nearY?: number;
  nearRadius?: number;
}

const baseUrl = configResult.ok ? configResult.config.apiBaseUrl : '';

export const scoutApi = createApi({
  reducerPath: 'scoutApi',
  baseQuery: fetchBaseQuery({
    baseUrl,
    prepareHeaders: (headers) => {
      if (configResult.ok) {
        headers.set('X-API-Key', configResult.config.apiKey);
      }
      return headers;
    },
  }),
  endpoints: (builder) => ({
    listPhotos: builder.query<PhotoPage, ListPhotosParams>({
      query: ({ cursor, limit, classId, minConfidence, nearX, nearY, nearRadius }) => {
        const params = new URLSearchParams();
        if (cursor !== undefined) params.set('cursor', cursor);
        if (limit !== undefined) params.set('limit', String(limit));
        if (classId !== undefined) params.set('classId', classId);
        if (minConfidence !== undefined) params.set('minConfidence', String(minConfidence));
        if (nearX !== undefined && nearY !== undefined && nearRadius !== undefined) {
          params.set('nearX', String(nearX));
          params.set('nearY', String(nearY));
          params.set('nearRadius', String(nearRadius));
        }
        const qs = params.toString();
        return { url: `/photos${qs ? `?${qs}` : ''}` };
      },
      transformErrorResponse: (response) => {
        if (typeof response.status === 'number') {
          return normalizeApiError(response.status, response.data, 'Failed to load photos');
        }
        return normalizeNetworkError(response.error);
      },
    }),
    getPhoto: builder.query<Photo, string>({
      query: (photoId) => ({ url: `/photos/${encodeURIComponent(photoId)}` }),
      transformErrorResponse: (response) => {
        if (typeof response.status === 'number') {
          return normalizeApiError(response.status, response.data, 'Failed to load photo');
        }
        return normalizeNetworkError(response.error);
      },
    }),
  }),
});

export const { useListPhotosQuery, useGetPhotoQuery } = scoutApi;

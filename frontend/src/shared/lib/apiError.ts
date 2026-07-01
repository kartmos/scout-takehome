export interface ApiError {
  status: number;
  code: string | undefined;
  requestId: string | undefined;
  message: string;
}

export function normalizeApiError(status: number, body: unknown, fallbackMessage: string): ApiError {
  if (body !== null && typeof body === 'object') {
    const err = body as Record<string, unknown>;
    return {
      status,
      code: typeof err['code'] === 'string' ? err['code'] : undefined,
      requestId: typeof err['request_id'] === 'string' ? err['request_id'] : undefined,
      message: typeof err['message'] === 'string' ? err['message'] : fallbackMessage,
    };
  }
  return { status, code: undefined, requestId: undefined, message: fallbackMessage };
}

export function normalizeNetworkError(error: unknown): ApiError {
  const message = error instanceof Error ? error.message : 'Network error';
  return { status: 0, code: 'NetworkError', requestId: undefined, message };
}

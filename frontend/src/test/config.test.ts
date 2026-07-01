import { describe, it, expect } from 'vitest';
import { normalizeApiError, normalizeNetworkError } from '../shared/lib/apiError';
import { buildThumbnailUrl } from '../shared/api/thumbnail';
import { validateBaseUrl } from '../shared/config/env';

// configResult is tested via its visible effects in other tests (mock vs real).
// Here we test the error normalization and URL-building contracts.

const TEST_ID = 'a1b2c3d4-e5f6-7890-abcd-ef1234567890';

describe('normalizeApiError', () => {
  it('extracts message, code, request_id from a typed body', () => {
    const err = normalizeApiError(404, {
      code: 'NotFound',
      request_id: 'req-abc',
      message: 'Not found',
    }, 'fallback');
    expect(err.status).toBe(404);
    expect(err.code).toBe('NotFound');
    expect(err.requestId).toBe('req-abc');
    expect(err.message).toBe('Not found');
  });

  it('falls back when body is null', () => {
    const err = normalizeApiError(500, null, 'Internal error');
    expect(err.message).toBe('Internal error');
    expect(err.code).toBeUndefined();
    expect(err.requestId).toBeUndefined();
  });

  it('requestId is surfaced (for debugging) but message does not expose the key', () => {
    const err = normalizeApiError(401, {
      code: 'AuthenticationRequired',
      request_id: 'req-xyz',
      message: 'Missing API key',
    }, 'Auth error');
    // Request ID is available for support/debugging
    expect(err.requestId).toBe('req-xyz');
    // The normalized message does not contain an API key value
    expect(err.message).not.toContain('secret');
    expect(err.message).not.toContain('Bearer');
  });
});

describe('normalizeNetworkError', () => {
  it('wraps an Error instance', () => {
    const err = normalizeNetworkError(new Error('timeout'));
    expect(err.status).toBe(0);
    expect(err.code).toBe('NetworkError');
    expect(err.message).toBe('timeout');
  });

  it('falls back for non-Error values', () => {
    const err = normalizeNetworkError('string error');
    expect(err.message).toBe('Network error');
  });
});

describe('validateBaseUrl', () => {
  describe('root-relative paths', () => {
    it('accepts /api and normalizes trailing slash', () => {
      const result = validateBaseUrl('/api');
      expect(result.ok).toBe(true);
      if (result.ok) expect(result.normalized).toBe('/api');
    });

    it('accepts /api/ and strips trailing slash', () => {
      const result = validateBaseUrl('/api/');
      expect(result.ok).toBe(true);
      if (result.ok) expect(result.normalized).toBe('/api');
    });

    it('accepts root / and normalizes to empty string', () => {
      const result = validateBaseUrl('/');
      expect(result.ok).toBe(true);
      if (result.ok) expect(result.normalized).toBe('');
    });

    it('root / generates /photos not //photos when used as base URL', () => {
      const result = validateBaseUrl('/');
      expect(result.ok).toBe(true);
      if (result.ok) {
        const path = `${result.normalized}/photos`;
        expect(path).toBe('/photos');
        expect(path.startsWith('//')).toBe(false);
      }
    });

    it('rejects protocol-relative //host', () => {
      const result = validateBaseUrl('//example.com/api');
      expect(result.ok).toBe(false);
    });

    it('rejects backslashes', () => {
      const result = validateBaseUrl('/api\\v1');
      expect(result.ok).toBe(false);
    });

    it('rejects query strings', () => {
      const result = validateBaseUrl('/api?x=1');
      expect(result.ok).toBe(false);
    });

    it('rejects hash fragments', () => {
      const result = validateBaseUrl('/api#section');
      expect(result.ok).toBe(false);
    });

    it('rejects credentials (@)', () => {
      const result = validateBaseUrl('/user@host/api');
      expect(result.ok).toBe(false);
    });

    it('rejects path traversal (..)', () => {
      const result = validateBaseUrl('/api/../etc/passwd');
      expect(result.ok).toBe(false);
    });
  });

  describe('absolute URLs (unchanged behaviour)', () => {
    it('accepts http URL', () => {
      const result = validateBaseUrl('http://localhost:8080');
      expect(result.ok).toBe(true);
      if (result.ok) expect(result.normalized).toBe('http://localhost:8080');
    });

    it('accepts https URL', () => {
      const result = validateBaseUrl('https://api.example.com');
      expect(result.ok).toBe(true);
    });

    it('rejects ftp scheme', () => {
      const result = validateBaseUrl('ftp://example.com');
      expect(result.ok).toBe(false);
    });

    it('rejects URL with credentials', () => {
      const result = validateBaseUrl('http://user:pass@host');
      expect(result.ok).toBe(false);
    });

    it('rejects URL with query string', () => {
      const result = validateBaseUrl('http://host?q=1');
      expect(result.ok).toBe(false);
    });

    it('rejects URL with fragment', () => {
      const result = validateBaseUrl('http://host#frag');
      expect(result.ok).toBe(false);
    });

    it('rejects bare hostname (no scheme, no leading /)', () => {
      const result = validateBaseUrl('localhost:8080');
      expect(result.ok).toBe(false);
    });
  });
});

describe('buildThumbnailUrl security', () => {
  it('thumbnail URL contains no API key', () => {
    // The API key is sent as X-API-Key header, never in URLs
    const url = buildThumbnailUrl({ photoId: TEST_ID, width: 400 });
    // In tests, configResult is the real module (not mocked here);
    // URL must not contain any credential-like query param
    expect(url).not.toMatch(/apiKey/i);
    expect(url).not.toMatch(/api_key/i);
    expect(url).not.toMatch(/key=/i);
    expect(url).not.toMatch(/secret/i);
    expect(url).not.toMatch(/token/i);
  });

  it('thumbnail URL uses the correct path structure', () => {
    const url = buildThumbnailUrl({ photoId: TEST_ID, width: 200, dpr: 2 });
    expect(url).toContain(`/photos/${TEST_ID}/thumbnail`);
    expect(url).toContain('width=200');
    expect(url).toContain('dpr=2');
  });

  it('rejects invalid photo IDs defensively', () => {
    expect(() => buildThumbnailUrl({ photoId: 'not-a-uuid', width: 400 })).toThrow();
  });

  it('rejects out-of-range width defensively', () => {
    expect(() => buildThumbnailUrl({ photoId: TEST_ID, width: 0 })).toThrow();
    expect(() => buildThumbnailUrl({ photoId: TEST_ID, width: 2049 })).toThrow();
  });
});

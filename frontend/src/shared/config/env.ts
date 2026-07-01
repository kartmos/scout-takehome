export interface ScoutConfig {
  apiBaseUrl: string;
  apiKey: string;
}

export type ConfigResult =
  | { ok: true; config: ScoutConfig }
  | { ok: false; error: string };

export function validateBaseUrl(raw: string): { ok: true; normalized: string } | { ok: false; reason: string } {
  // Accept root-relative paths like /api (same-origin, proxy-safe).
  if (raw.startsWith('/')) {
    if (raw.startsWith('//')) {
      return { ok: false, reason: 'protocol-relative URLs (//) are not allowed; use an absolute URL or a root-relative path like /api' };
    }
    if (raw.includes('\\')) {
      return { ok: false, reason: 'URL must not contain backslashes' };
    }
    if (raw.includes('?')) {
      return { ok: false, reason: 'URL must not contain a query string' };
    }
    if (raw.includes('#')) {
      return { ok: false, reason: 'URL must not contain a hash fragment' };
    }
    if (raw.includes('@')) {
      return { ok: false, reason: 'URL must not contain credentials' };
    }
    if (raw.includes('..')) {
      return { ok: false, reason: 'URL must not contain path traversal sequences (..)' };
    }
    return { ok: true, normalized: raw.replace(/\/+$/, '') || '/' };
  }

  // Absolute URL (http/https) validation.
  let parsed: URL;
  try {
    parsed = new URL(raw);
  } catch {
    return { ok: false, reason: 'not a valid URL' };
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return { ok: false, reason: `protocol must be http or https (got "${parsed.protocol.replace(':', '')}")` };
  }
  if (parsed.username !== '' || parsed.password !== '') {
    return { ok: false, reason: 'URL must not contain credentials (username/password)' };
  }
  if (parsed.search !== '') {
    return { ok: false, reason: 'URL must not contain a query string' };
  }
  if (parsed.hash !== '') {
    return { ok: false, reason: 'URL must not contain a hash fragment' };
  }
  return { ok: true, normalized: raw.replace(/\/+$/, '') };
}

function loadConfig(): ConfigResult {
  const rawBaseUrl: string | undefined = import.meta.env.VITE_SCOUT_API_BASE_URL;
  const rawApiKey: string | undefined = import.meta.env.VITE_SCOUT_API_KEY;

  const errors: string[] = [];

  if (rawBaseUrl === undefined || rawBaseUrl.trim() === '') {
    errors.push('VITE_SCOUT_API_BASE_URL is not set');
  } else {
    const result = validateBaseUrl(rawBaseUrl.trim());
    if (!result.ok) {
      errors.push(`VITE_SCOUT_API_BASE_URL is invalid: ${result.reason}`);
    }
  }

  if (rawApiKey === undefined || rawApiKey.trim() === '') {
    errors.push('VITE_SCOUT_API_KEY is not set');
  }

  if (errors.length > 0) {
    return {
      ok: false,
      error: errors.join('. ') + '. Copy .env.example to .env in the project root and fill in the values.',
    };
  }

  const urlResult = validateBaseUrl((rawBaseUrl as string).trim());
  if (!urlResult.ok) {
    return { ok: false, error: `VITE_SCOUT_API_BASE_URL is invalid: ${urlResult.reason}` };
  }

  return { ok: true, config: { apiBaseUrl: urlResult.normalized, apiKey: (rawApiKey as string).trim() } };
}

export const configResult: ConfigResult = loadConfig();

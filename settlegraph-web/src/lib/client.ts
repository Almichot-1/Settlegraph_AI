import type { ApiError as ApiErrorShape } from '@/generated/types';

export class ApiRequestError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
    public readonly requestId?: string,
  ) {
    super(message);
    this.name = 'ApiRequestError';
  }
}

export type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: unknown;
  idempotencyKey?: string;
  ifMatch?: number;
  signal?: AbortSignal;
};

/**
 * Normalizes HTTP status codes to standardized error codes per SettleGraph API contract
 * (401, 403, 404, 409, 412, 428, 429, 502, 503).
 */
export function normalizeErrorCode(status: number): string {
  switch (status) {
    case 401: return 'UNAUTHORIZED';
    case 403: return 'FORBIDDEN';
    case 404: return 'NOT_FOUND';
    case 409: return 'CONFLICT';
    case 412: return 'PRECONDITION_FAILED';
    case 428: return 'PRECONDITION_REQUIRED';
    case 429: return 'RATE_LIMITED';
    case 502: return 'BAD_GATEWAY';
    case 503: return 'SERVICE_UNAVAILABLE';
    default: return `HTTP_${status}`;
  }
}

/**
 * Human-readable message fallback for standard HTTP status codes
 */
export function normalizeErrorMessage(status: number): string {
  switch (status) {
    case 401: return 'Authentication required. Please sign in again.';
    case 403: return 'Access forbidden. This action or resource is restricted.';
    case 404: return 'Requested resource could not be found.';
    case 409: return 'Conflict or idempotency key payload mismatch.';
    case 412: return 'Precondition failed: resource version is stale. Refresh and retry.';
    case 428: return 'Precondition required: an If-Match version header is required for this mutation.';
    case 429: return 'Rate limit exceeded. Please back off before retrying.';
    case 502: return 'Bad gateway: upstream financial backend service is unavailable.';
    case 503: return 'Service unavailable: system is temporarily unable to handle the request.';
    default: return `HTTP request failed with status ${status}`;
  }
}

/** No bank credentials or JWT tokens are ever exposed to browser JS. */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  if (!path.startsWith('/v1/') && !path.startsWith('/health/')) throw new Error('Invalid API path');
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (options.body !== undefined) headers['Content-Type'] = 'application/json';
  if (options.idempotencyKey) headers['Idempotency-Key'] = options.idempotencyKey;
  if (options.ifMatch !== undefined) headers['If-Match'] = `"${options.ifMatch}"`;

  const response = await fetch(`/api/sg${path}`, {
    method: options.method ?? 'GET',
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
    cache: 'no-store',
    credentials: 'same-origin',
    signal: options.signal,
  });

  const payload: unknown = await response.json().catch(() => null);

  if (response.status === 401 && typeof window !== 'undefined') {
    window.dispatchEvent(new Event('settlegraph-session-expired'));
  }
  if (!response.ok) {
    const e = payload as ApiErrorShape | null;
    const code = e?.error?.code || normalizeErrorCode(response.status);
    const message = e?.error?.message || normalizeErrorMessage(response.status);
    const requestId = e?.error?.request_id || response.headers.get('x-request-id') || undefined;
    throw new ApiRequestError(response.status, code, message, requestId);
  }

  return payload as T;
}

export function qs(input: Record<string, string | number | boolean | null | undefined> = {}): string {
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(input)) {
    if (v !== null && v !== undefined && String(v) !== '') {
      params.set(k, String(v));
    }
  }
  const search = params.toString();
  return search ? `?${search}` : '';
}

/** Read full evidence/history with bounded continuation and loop detection. */
export async function allPages<T>(fetchPage: (cursor?: string) => Promise<{items:T[];next_cursor?:string|null}>): Promise<T[]> {
  const result:T[]=[]; const seen=new Set<string>(); let cursor:string|undefined;
  for (let page=0;page<1000;page++) {
    const response=await fetchPage(cursor); result.push(...response.items);
    if (!response.next_cursor) return result;
    if (seen.has(response.next_cursor)) throw new Error('The server repeated a page cursor.');
    seen.add(response.next_cursor);cursor=response.next_cursor;
  }
  throw new Error('History exceeds the supported page count. Narrow the selection.');
}

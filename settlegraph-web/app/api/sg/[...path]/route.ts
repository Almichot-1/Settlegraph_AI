import { NextRequest, NextResponse } from 'next/server';
import { cookies } from 'next/headers';
import { mockApi } from '@/server/mock-engine';

const allowedMethods = new Set(['GET', 'POST']);

function isSameOrigin(req: NextRequest): boolean {
  const origin = req.headers.get('origin');
  if (origin) {
    return origin === req.nextUrl.origin;
  }
  const referer = req.headers.get('referer');
  if (referer) {
    try {
      return new URL(referer).origin === req.nextUrl.origin;
    } catch {
      return false;
    }
  }
  const secFetchSite = req.headers.get('sec-fetch-site');
  if (secFetchSite && secFetchSite !== 'same-origin' && secFetchSite !== 'none') {
    return false;
  }
  return true;
}

function isRestrictedPath(endpoint: string): boolean {
  const normalized = endpoint.toLowerCase().replace(/\/+$/, '');
  return (
    normalized === '/v1/demo/sessions' ||
    normalized === '/v1/integrations/banksim/events' ||
    normalized.startsWith('/v1/integrations/banksim')
  );
}

async function handle(req: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  const method = req.method.toUpperCase();

  // 1. Method allowlist verification
  if (!allowedMethods.has(method)) {
    return NextResponse.json(
      { error: { code: 'METHOD_NOT_ALLOWED', message: 'Method not allowed on API gateway', request_id: 'frontend' } },
      { status: 405 }
    );
  }

  const { path } = await context.params;

  // Path traversal check
  if (!path || !path.length || path.some(seg => seg === '..' || seg === '.' || seg.includes('/') || seg.includes('\\'))) {
    return NextResponse.json(
      { error: { code: 'NOT_FOUND', message: 'Invalid API path', request_id: 'frontend' } },
      { status: 404 }
    );
  }

  const endpoint = `/${path.join('/')}`;

  // 2. Path allowlist verification: only /v1/* and /health/*
  if (!endpoint.startsWith('/v1/') && !endpoint.startsWith('/health/')) {
    return NextResponse.json(
      { error: { code: 'NOT_FOUND', message: 'Endpoint not found', request_id: 'frontend' } },
      { status: 404 }
    );
  }

  // 3. Explicit denial (403) of internal BankSim integration endpoints and demo session minting
  if (isRestrictedPath(endpoint)) {
    return NextResponse.json(
      { error: { code: 'FORBIDDEN', message: 'Access denied: BankSim integration and internal session endpoints are restricted from browser clients', request_id: 'frontend' } },
      { status: 403 }
    );
  }

  // 4. Same-origin CSRF check on mutations
  if (method !== 'GET') {
    if (!isSameOrigin(req)) {
      return NextResponse.json(
        { error: { code: 'FORBIDDEN', message: 'Cross-origin mutation denied', request_id: 'frontend' } },
        { status: 403 }
      );
    }
  }

  // 5. Session authentication check (session cookies)
  // Health checks do not require authentication
  const isHealthCheck = endpoint.startsWith('/health/');
  const jar = await cookies();
  const token = jar.get('sg_token')?.value;
  const org = jar.get('sg_org')?.value;

  if (!isHealthCheck && (!token || !org)) {
    return NextResponse.json(
      { error: { code: 'UNAUTHORIZED', message: 'Sign in first', request_id: 'frontend' } },
      { status: 401 }
    );
  }

  // 6. Mock mode handling
  if (process.env.SETTLEGRAPH_DATA_MODE !== 'backend') {
    return mockApi(req);
  }

  // 7. Backend proxy with transport header propagation
  const base = process.env.SETTLEGRAPH_API_BASE_URL ?? 'http://localhost:8080';
  const headers = new Headers({
    Accept: 'application/json',
  });

  if (token) headers.set('Authorization', `Bearer ${token}`);
  if (org) headers.set('X-Organization-Id', org);

  // Propagate transport headers
  const idempotencyKey = req.headers.get('idempotency-key');
  if (idempotencyKey) {
    headers.set('Idempotency-Key', idempotencyKey);
  }

  const ifMatch = req.headers.get('if-match');
  if (ifMatch) {
    // Preserve or enforce quoted If-Match RFC formatting
    const formattedIfMatch = ifMatch.startsWith('"') || ifMatch.startsWith('W/"') ? ifMatch : `"${ifMatch}"`;
    headers.set('If-Match', formattedIfMatch);
  }

  const contentType = req.headers.get('content-type');
  if (contentType) {
    headers.set('Content-Type', contentType);
  } else if (method !== 'GET') {
    headers.set('Content-Type', 'application/json');
  }

  const requestId = req.headers.get('x-request-id') || crypto.randomUUID();
  headers.set('X-Request-Id', requestId);

  const body = method === 'GET' ? undefined : await req.text();

  const upstream = await fetch(base + endpoint + req.nextUrl.search, {
    method,
    headers,
    body,
    cache: 'no-store',
    signal: AbortSignal.timeout(20000),
  }).catch(() => null);

  if (!upstream) {
    return NextResponse.json(
      { error: { code: 'BACKEND_UNAVAILABLE', message: 'Go API not available', request_id: requestId } },
      { status: 502 }
    );
  }

  const upstreamText = await upstream.text();
  const responseHeaders = new Headers({
    'Content-Type': upstream.headers.get('content-type') ?? 'application/json',
    'Cache-Control': 'no-store',
  });
  if (upstream.headers.has('x-request-id')) {
    responseHeaders.set('X-Request-Id', upstream.headers.get('x-request-id')!);
  }

  return new Response(upstreamText, {
    status: upstream.status,
    headers: responseHeaders,
  });
}

export const GET = handle;
export const POST = handle;

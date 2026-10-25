import { NextRequest, NextResponse } from 'next/server';
import { cookies } from 'next/headers';
import { sameOrigin, publicOrigin, allowedOperation, limitedText } from '@/server/security';
import { mockApi } from '@/server/mock-engine';

const allowedMethods = new Set(['GET', 'POST']);

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

  if (!allowedOperation(method, endpoint)) {
    return NextResponse.json({error:{code:'NOT_FOUND', message:'Unknown public operation', request_id:'frontend'}}, {status:404});
  }
  // 4. Same-origin CSRF check on mutations
  if (method !== 'GET') {
    if (!sameOrigin(req, publicOrigin(req))) {
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

  let body: string | undefined;
  try { body = method === 'GET' ? undefined : await limitedText(req); }
  catch { return NextResponse.json({error:{code:'PAYLOAD_TOO_LARGE',message:'Request body is too large or invalid',request_id:'frontend'}}, {status:413}); }
  if (process.env.SETTLEGRAPH_DATA_MODE !== 'backend') {
    return mockApi(new Request(req.url,{method,headers:req.headers,body}));
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

  for (const header of ['etag', 'retry-after']) {
    const value = upstream.headers.get(header);
    if (value) responseHeaders.set(header, value);
  }
  if (upstream.status === 401) {
    const expired = NextResponse.json({error:{code:'UNAUTHORIZED',message:'Session expired. Sign in again.',request_id:requestId}}, {status:401, headers:responseHeaders});
    expired.cookies.delete('sg_token'); expired.cookies.delete('sg_org');
    return expired;
  }
  return new Response(upstreamText, {
    status: upstream.status,
    headers: responseHeaders,
  });
}

export const GET = handle;
export const POST = handle;

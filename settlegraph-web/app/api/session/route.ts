import { NextRequest, NextResponse } from 'next/server';
import { cookies } from 'next/headers';
import fixture from '@/server/demo-session-response.json';
import type { DemoSessionResponse, UserProfile } from '@/generated/types';

const mock = () => process.env.SETTLEGRAPH_DATA_MODE !== 'backend';
const backend = () => process.env.SETTLEGRAPH_API_BASE_URL ?? 'http://localhost:8080';

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

export async function POST(req: NextRequest) {
  if (!isSameOrigin(req)) {
    return NextResponse.json(
      { error: { code: 'FORBIDDEN', message: 'Cross-origin mutation denied', request_id: 'frontend' } },
      { status: 403 }
    );
  }

  const data = await req.json().catch(() => ({}));
  const name = String(data.display_name ?? '').trim().slice(0, 80);
  if (!name) {
    return NextResponse.json(
      { error: { code: 'VALIDATION_ERROR', message: 'Display name is required', request_id: 'frontend' } },
      { status: 400 }
    );
  }

  let payload: DemoSessionResponse;
  if (mock()) {
    payload = { ...fixture, user: { ...fixture.user, display_name: name } } as DemoSessionResponse;
  } else {
    if (!process.env.DEMO_ACCESS_CODE) {
      return NextResponse.json(
        { error: { code: 'CONFIGURATION_ERROR', message: 'Backend DEMO_ACCESS_CODE is not configured on the server', request_id: 'frontend' } },
        { status: 503 }
      );
    }
    const upstream = await fetch(`${backend()}/v1/demo/sessions`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Demo-Access-Code': process.env.DEMO_ACCESS_CODE,
      },
      body: JSON.stringify({ display_name: name }),
      cache: 'no-store',
    }).catch(() => null);

    if (!upstream) {
      return NextResponse.json(
        { error: { code: 'BACKEND_UNAVAILABLE', message: 'Cannot reach Go backend', request_id: 'frontend' } },
        { status: 502 }
      );
    }
    if (!upstream.ok) {
      return NextResponse.json(
        await upstream.json().catch(() => ({ error: { code: 'LOGIN_FAILED', message: 'Demo login failed', request_id: 'frontend' } })),
        { status: upstream.status }
      );
    }
    payload = await upstream.json() as DemoSessionResponse;
  }

  // Security invariant: never expose the access_token in the client response payload
  const response = NextResponse.json({
    user: payload.user,
    organization_id: payload.default_organization_id,
  });

  const isSecure = process.env.NODE_ENV === 'production' || req.nextUrl.protocol === 'https:';
  const cookieOpts = {
    httpOnly: true,
    secure: isSecure,
    sameSite: 'lax' as const,
    path: '/',
    maxAge: 60 * 60 * 8, // 8-hour finite max age
  };

  response.cookies.set('sg_token', payload.access_token, cookieOpts);
  response.cookies.set('sg_org', payload.default_organization_id, cookieOpts);
  return response;
}

export async function GET() {
  const jar = await cookies();
  const token = jar.get('sg_token')?.value;
  const org = jar.get('sg_org')?.value;

  if (!token || !org) {
    return NextResponse.json(
      { error: { code: 'UNAUTHORIZED', message: 'Sign in first', request_id: 'frontend' } },
      { status: 401 }
    );
  }

  if (mock()) {
    return NextResponse.json({ user: fixture.user, organization_id: org });
  }

  const upstream = await fetch(`${backend()}/v1/me`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: 'no-store',
  }).catch(() => null);

  if (!upstream) {
    return NextResponse.json(
      { error: { code: 'BACKEND_UNAVAILABLE', message: 'Cannot reach Go backend', request_id: 'frontend' } },
      { status: 502 }
    );
  }
  if (!upstream.ok) {
    return NextResponse.json(
      { error: { code: 'UNAUTHORIZED', message: 'Session expired', request_id: 'frontend' } },
      { status: 401 }
    );
  }

  const user = await upstream.json() as UserProfile;
  return NextResponse.json({ user, organization_id: org });
}

export async function DELETE(req: NextRequest) {
  if (!isSameOrigin(req)) {
    return NextResponse.json(
      { error: { code: 'FORBIDDEN', message: 'Cross-origin mutation denied', request_id: 'frontend' } },
      { status: 403 }
    );
  }

  const response = NextResponse.json({ ok: true });
  const isSecure = process.env.NODE_ENV === 'production' || req.nextUrl.protocol === 'https:';
  const clearOpts = {
    httpOnly: true,
    secure: isSecure,
    sameSite: 'lax' as const,
    path: '/',
    maxAge: 0,
  };

  response.cookies.set('sg_token', '', clearOpts);
  response.cookies.set('sg_org', '', clearOpts);
  return response;
}

import { NextRequest, NextResponse } from 'next/server';
import { cookies } from 'next/headers';
import fixture from '@/server/demo-session-response.json';
import { sameOrigin, publicOrigin, limitedText } from '@/server/security';
import type { DemoSessionResponse, UserProfile } from '@/generated/types';

const mock = () => process.env.SETTLEGRAPH_DATA_MODE !== 'backend';
const backend = () => process.env.SETTLEGRAPH_API_BASE_URL ?? 'http://localhost:8080';

export async function POST(req: NextRequest) {
  if (!sameOrigin(req, publicOrigin(req))) {
    return NextResponse.json(
      { error: { code: 'FORBIDDEN', message: 'Cross-origin mutation denied', request_id: 'frontend' } },
      { status: 403 }
    );
  }

  let data: Record<string, unknown>;
  try { data = JSON.parse(await limitedText(req, 4096)); }
  catch { return NextResponse.json({error:{code:'BAD_REQUEST',message:'Invalid login request',request_id:'frontend'}},{status:400}); }
  if (!data || Array.isArray(data) || Object.keys(data).some(key => !['display_name','access_code'].includes(key))) {
    return NextResponse.json({error:{code:'BAD_REQUEST',message:'Invalid login fields',request_id:'frontend'}},{status:400});
  }
  const name = String(data.display_name ?? '').trim().slice(0, 80);
  if (!name) {
    return NextResponse.json(
      { error: { code: 'VALIDATION_ERROR', message: 'Display name is required', request_id: 'frontend' } },
      { status: 400 }
    );
  }

  let payload: DemoSessionResponse;
  if (mock()) {
    payload = { ...fixture, expires_at:new Date(Date.now()+3600000).toISOString(), user: { ...fixture.user, display_name: name } } as DemoSessionResponse;
  } else {
    const accessCode = typeof data.access_code === 'string' && data.access_code.trim()
      ? data.access_code.trim()
      : process.env.DEMO_OWNER_SHORTCUT === '1' ? process.env.DEMO_ACCESS_CODE : undefined;
    if (!accessCode) {
      return NextResponse.json(
        { error: { code: 'CONFIGURATION_ERROR', message: 'Enter the access code supplied for your demo role', request_id: 'frontend' } },
        { status: 503 }
      );
    }
    const upstream = await fetch(`${backend()}/v1/demo/sessions`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Demo-Access-Code': accessCode,
      },
      body: JSON.stringify({ display_name: name }),
      cache: 'no-store',
      signal: AbortSignal.timeout(15000),
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
        { status: upstream.status, headers: upstream.headers.has('retry-after') ? {'Retry-After':upstream.headers.get('retry-after')!} : {} }
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
    maxAge: Math.max(0, Math.min(3600, Math.floor((Date.parse(payload.expires_at) - Date.now()) / 1000))),
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
    signal: AbortSignal.timeout(15000),
    cache: 'no-store',
  }).catch(() => null);

  if (!upstream) {
    return NextResponse.json(
      { error: { code: 'BACKEND_UNAVAILABLE', message: 'Cannot reach Go backend', request_id: 'frontend' } },
      { status: 502 }
    );
  }
  if (!upstream.ok) {
    if(upstream.status===401) {
      const response=NextResponse.json({error:{code:'UNAUTHORIZED',message:'Session expired',request_id:'frontend'}},{status:401});
      response.cookies.delete('sg_token');response.cookies.delete('sg_org');
      return response;
    }
    return NextResponse.json({error:{code:'BACKEND_UNAVAILABLE',message:'Cannot validate the workspace session',request_id:'frontend'}},{status:502});
  }

  const user = await upstream.json() as UserProfile;
  return NextResponse.json({ user, organization_id: org });
}

export async function DELETE(req: NextRequest) {
  if (!sameOrigin(req, publicOrigin(req))) {
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

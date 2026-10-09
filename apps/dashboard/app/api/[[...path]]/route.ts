import { NextResponse, type NextRequest } from "next/server";
import { SESSION_COOKIE, SESSION_COOKIE_FALLBACK } from "@/lib/paths";

/**
 * Backend-for-frontend proxy: the browser only ever calls same-origin
 * /api/*, and this handler forwards to the Go API server-side.
 *
 * Why: the session lives in an HttpOnly cookie the JS bundle cannot read
 * (the API issues it, we pass Set-Cookie through), so XSS cannot exfiltrate
 * a credential the way the old localStorage token allowed. Same-origin
 * calls also remove CORS/credentials friction and let middleware.ts guard
 * routes before the app boots.
 */

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:5000";
// Mirrors users.SessionLifetime in the API; only used when sliding the
// cookie's own Max-Age on /v1/auth/me (the API slides the DB expiry).
const SESSION_MAX_AGE = 30 * 24 * 60 * 60;

// Only forward headers the API understands; everything else (cookies,
// forwarded-for from the browser, host) is transport noise we recompute.
const REQUEST_HEADERS = ["content-type", "accept", "x-workspace-id", "if-none-match", "if-modified-since"];
const RESPONSE_SKIP = new Set([
  "set-cookie",
  "content-encoding",
  "content-length",
  "transfer-encoding",
  "connection",
  "keep-alive",
]);
// Login/register responses carry a raw token for direct API clients
// (curl). The dashboard must never see it: the HttpOnly cookie is the
// credential now, so the token is stripped here.
const CREDENTIAL_PATHS = new Set(["/v1/auth/login", "/v1/auth/register"]);

type RouteContext = { params: Promise<{ path?: string[] }> };

function readSessionCookie(request: NextRequest): { name: string; value: string } | null {
  const host = request.cookies.get(SESSION_COOKIE);
  if (host?.value) return { name: SESSION_COOKIE, value: host.value };
  const plain = request.cookies.get(SESSION_COOKIE_FALLBACK);
  if (plain?.value) return { name: SESSION_COOKIE_FALLBACK, value: plain.value };
  return null;
}

function serializeSessionCookie(name: string, value: string): string {
  const attributes = [`${name}=${value}`, "Path=/", `Max-Age=${SESSION_MAX_AGE}`, "HttpOnly", "SameSite=Lax"];
  if (name.startsWith("__Host-") || process.env.NODE_ENV === "production") {
    attributes.push("Secure");
  }
  return attributes.join("; ");
}

// The edge proxy (Caddy/Vercel) sets these on inbound requests; sending
// the first hop to the API keeps its per-IP auth rate limits meaningful
// when every proxied call would otherwise arrive from the dashboard
// server's address.
function clientIPFrom(request: NextRequest): string | null {
  const real = request.headers.get("x-real-ip")?.trim();
  if (real) return real;
  const forwarded = request.headers.get("x-forwarded-for");
  if (forwarded) {
    const first = forwarded.split(",")[0]?.trim();
    if (first) return first;
  }
  return null;
}

async function proxy(request: NextRequest, { params }: RouteContext): Promise<NextResponse> {
  const segments = (await params).path ?? [];
  // Client paths are already absolute API paths (/api/v1/... → /v1/...).
  const upstreamPath = `/${segments.join("/")}`;
  const target = `${API_URL}${upstreamPath}${request.nextUrl.search}`;

  const headers = new Headers();
  for (const name of REQUEST_HEADERS) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  const clientIP = clientIPFrom(request);
  if (clientIP) headers.set("X-Real-IP", clientIP);
  const session = readSessionCookie(request);
  const authorization = request.headers.get("authorization");
  if (authorization) {
    headers.set("Authorization", authorization);
  } else if (session) {
    headers.set("Authorization", `Bearer ${session.value}`);
  }

  const method = request.method.toUpperCase();
  const init: RequestInit & { duplex?: "half" } = { method, headers, redirect: "manual" };
  if (method !== "GET" && method !== "HEAD") {
    init.body = request.body;
    init.duplex = "half";
  }

  let upstream: Response;
  try {
    upstream = await fetch(target, init);
  } catch (error) {
    console.error("api proxy: upstream request failed", upstreamPath, error);
    return NextResponse.json({ error: "unable to reach the DevPulse API" }, { status: 502 });
  }

  const responseHeaders = new Headers();
  upstream.headers.forEach((value, name) => {
    if (!RESPONSE_SKIP.has(name)) responseHeaders.append(name, value);
  });
  for (const cookie of upstream.headers.getSetCookie?.() ?? []) {
    responseHeaders.append("set-cookie", cookie);
  }

  const status = upstream.status;
  const emptyBody = method === "HEAD" || status === 204 || status === 205 || status === 304;

  if (CREDENTIAL_PATHS.has(upstreamPath) && upstream.ok && !emptyBody) {
    const text = await upstream.text();
    try {
      const payload = JSON.parse(text) as Record<string, unknown>;
      delete payload.token;
      const headers = new Headers(responseHeaders);
      headers.set("content-type", "application/json");
      return NextResponse.json(payload, { status, headers });
    } catch {
      return new NextResponse(text, { status, headers: responseHeaders });
    }
  }

  // Slide the cookie's own Max-Age whenever the session validates, so an
  // active browser keeps the cookie alive alongside the API's DB-side
  // renewal. Throttled by the API: /v1/auth/me runs once per app load.
  if (upstreamPath === "/v1/auth/me" && upstream.ok && session) {
    responseHeaders.append("set-cookie", serializeSessionCookie(session.name, session.value));
  }

  return new NextResponse(emptyBody ? null : upstream.body, { status, headers: responseHeaders });
}

export { proxy as GET, proxy as POST, proxy as PUT, proxy as PATCH, proxy as DELETE, proxy as HEAD, proxy as OPTIONS };

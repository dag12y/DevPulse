import { NextResponse, type NextRequest } from "next/server";
import { SESSION_COOKIE, SESSION_COOKIE_FALLBACK, isPublicPath } from "@/lib/paths";

// Per-request CSP nonce. Next.js automatically attaches the nonce from this
// header to the framework scripts it injects, so the policy can stay free of
// 'unsafe-inline' for scripts while still allowing the inline theme bootstrap
// in app/layout.tsx (which reads the same nonce from headers()).
//
// The session itself is an HttpOnly cookie (see app/api/[[...path]]/route.ts),
// so an XSS bug cannot read the credential; this CSP is the second layer.
const MUTATING_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"]);

function hasSessionCookie(request: NextRequest): boolean {
  return Boolean(
    request.cookies.get(SESSION_COOKIE)?.value || request.cookies.get(SESSION_COOKIE_FALLBACK)?.value,
  );
}

export function middleware(request: NextRequest) {
  const { pathname, search } = request.nextUrl;
  const isAPI = pathname === "/api" || pathname.startsWith("/api/");

  // CSRF: cookie-authenticated writes must come from our own origin.
  // SameSite=Lax already stops cross-site writes; this edge check also
  // covers older clients and non-browser tooling that ignore it.
  if (isAPI && MUTATING_METHODS.has(request.method)) {
    const site = request.headers.get("sec-fetch-site");
    if (site && site !== "same-origin" && site !== "none") {
      return NextResponse.json({ error: "cross-origin request rejected" }, { status: 403 });
    }
    const origin = request.headers.get("origin");
    if (origin && origin !== request.nextUrl.origin) {
      return NextResponse.json({ error: "cross-origin request rejected" }, { status: 403 });
    }
  }

  // Route guard for top-level page loads. Cookie presence only — real
  // validation happens server-side on the first /api call (middleware
  // cannot reach the database from the edge runtime). SPA navigations and
  // expired-but-present cookies are handled by the AppShell guard after
  // /v1/auth/me resolves. /api is excluded: those responses are JSON or
  // redirects (the OAuth start bounce), not dashboard pages, and the API
  // answers 401/403 itself when a cookie is missing or stale.
  const isDocument = request.headers.get("sec-fetch-dest") === "document";
  const envKeyMode = (process.env.NEXT_PUBLIC_API_KEY || "") !== "";
  if (isDocument && !isAPI && !isPublicPath(pathname) && !envKeyMode && !hasSessionCookie(request)) {
    const target = pathname === "/" ? "" : pathname + search;
    const login = request.nextUrl.clone();
    login.pathname = "/login";
    login.search = target ? `?next=${encodeURIComponent(target)}` : "";
    return NextResponse.redirect(login);
  }

  // Edge-safe nonce: Buffer is a Node API and not available in the
  // middleware runtime, so build base64 from Web Crypto randomness.
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  let binary = "";
  for (const b of bytes) binary += String.fromCharCode(b);
  const nonce = btoa(binary);
  const isDev = process.env.NODE_ENV !== "production";

  const csp = [
    "default-src 'self'",
    // 'strict-dynamic' lets Next.js load its own chunks without allowlisting
    // build-generated paths. 'unsafe-eval' is dev-only (React refresh).
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${isDev ? " 'unsafe-eval'" : ""}`,
    // Tailwind injects CSS at runtime in dev; chart bars use inline styles.
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data:",
    "font-src 'self'",
    // API calls go through the same-origin /api proxy; no cross-origin
    // endpoints the dashboard fetches from the browser.
    "connect-src 'self'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
    "upgrade-insecure-requests",
  ].join("; ");

  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);

  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set("Content-Security-Policy", csp);
  return response;
}

export const config = {
  // Everything except static assets and image optimization output.
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};

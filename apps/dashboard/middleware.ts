import { NextResponse, type NextRequest } from "next/server";

// Per-request CSP nonce. Next.js automatically attaches the nonce from this
// header to the framework scripts it injects, so the policy can stay free of
// 'unsafe-inline' for scripts while still allowing the inline theme bootstrap
// in app/layout.tsx (which reads the same nonce from headers()).
//
// The dashboard holds the session bearer token in localStorage, so an XSS bug
// would otherwise hand over a 30-day credential. This is the main defence.
export function middleware(request: NextRequest) {
  // Edge-safe nonce: Buffer is a Node API and not available in the
  // middleware runtime, so build base64 from Web Crypto randomness.
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  let binary = "";
  for (const b of bytes) binary += String.fromCharCode(b);
  const nonce = btoa(binary);
  const isDev = process.env.NODE_ENV !== "production";
  let apiOrigin = "http://localhost:5000";
  try {
    apiOrigin = new URL(process.env.NEXT_PUBLIC_API_URL || apiOrigin).origin;
  } catch {
    // Keep the safe default when the env value is malformed.
  }

  const csp = [
    "default-src 'self'",
    // 'strict-dynamic' lets Next.js load its own chunks without allowlisting
    // build-generated paths. 'unsafe-eval' is dev-only (React refresh).
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${isDev ? " 'unsafe-eval'" : ""}`,
    // Tailwind injects CSS at runtime in dev; chart bars use inline styles.
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data:",
    "font-src 'self'",
    // Reports are fetched directly from the API origin.
    `connect-src 'self' ${apiOrigin}`,
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
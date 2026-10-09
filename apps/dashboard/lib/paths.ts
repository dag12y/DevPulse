/**
 * Route and session-cookie helpers shared by the middleware (edge), the
 * BFF proxy (Node), and client components.
 */

/** Cookie names the API may issue: __Host- in production, plain in dev. */
export const SESSION_COOKIE = "__Host-dp_session";
export const SESSION_COOKIE_FALLBACK = "dp_session";

/** Paths reachable without a session. Prefixes match with a segment break. */
const PUBLIC_PREFIXES = ["/login", "/register", "/oauth"];

export function isPublicPath(pathname: string): boolean {
  return PUBLIC_PREFIXES.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}

/**
 * Sanitises a post-login redirect target. Only same-origin absolute paths
 * qualify: "/x", never "//evil.com" or "/\evil.com" (open redirect).
 * Anything else falls back to the dashboard root.
 */
export function safeNext(value: string | null | undefined): string {
  if (!value) return "/";
  if (!value.startsWith("/") || value.startsWith("//") || value.startsWith("/\\")) return "/";
  return value;
}

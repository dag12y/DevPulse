"use client";

/**
 * Maps API error messages (internal/users validation strings, proxy
 * failures, browser network errors) to copy a user can act on. Unknown
 * messages fall through verbatim so nothing is swallowed.
 */
export function friendlyAuthError(message: string, fallback: string): string {
  const lower = message.toLowerCase();
  if (
    lower.includes("fetch") ||
    lower.includes("network") ||
    lower.includes("load failed") ||
    lower.includes("unable to reach")
  ) {
    return "Can't reach the DevPulse API. Check your connection and that the API is running, then try again.";
  }
  if (lower.includes("invalid email or password")) {
    return "Incorrect email or password. Check for typos and try again.";
  }
  if (lower.includes("temporarily locked")) {
    // The API sets Retry-After with the remaining seconds; the message
    // is all this layer sees, so keep the guidance honest about waiting.
    return "Too many failed sign-in attempts. This account is temporarily locked — wait a few minutes, or reset your password, then try again.";
  }
  if (lower.includes("two-factor code required")) {
    return "Enter the 6-digit code from your authenticator app.";
  }
  if (lower.includes("invalid two-factor code")) {
    return "That code is incorrect or has expired. Check your authenticator app and try again.";
  }
  if (lower.includes("not verified")) {
    return "Check your inbox for your verification link, then try signing in. Didn't get it? Resend below.";
  }
  if (lower.includes("invalid or has expired")) {
    return "This link is invalid or has expired. Request a new one and try again.";
  }
  if (lower.includes("already registered")) {
    return "An account with this email already exists. Try signing in instead.";
  }
  if (lower.includes("password must be at least")) {
    return "Password must be at least 12 characters. Add a few more characters.";
  }
  if (lower.includes("password must not exceed")) {
    return "Password must not exceed 72 characters.";
  }
  if (lower.includes("email must be a valid") || lower.includes("email is required")) {
    return "Enter a valid email address (e.g. you@company.com).";
  }
  if (lower.includes("too many") || lower.includes("rate")) {
    return "Too many attempts from this device. Wait a few minutes and try again.";
  }
  if (
    lower.includes("unable to log in") ||
    lower.includes("unable to sign in") ||
    lower.includes("unable to register") ||
    lower.includes("unable to send") ||
    lower.includes("unable to verify") ||
    lower.includes("unable to reset")
  ) {
    return "Something went wrong on our side. Try again in a moment.";
  }
  return message || fallback;
}

/**
 * Maps the fixed ?error= codes the API bounces back on an OAuth
 * callback. Codes are chosen server-side; raw provider messages never
 * reach the URL, so nothing here is attacker-controlled.
 */
export function friendlyOAuthError(code: string): string {
  switch (code) {
    case "cancelled":
      return "Sign-in was cancelled at the provider. Try again, or use your password instead.";
    case "account conflict":
      return "That provider account's email already belongs to a different DevPulse account. Sign in with your password instead.";
    default:
      return "Social sign-in couldn't be completed. Try again, or use your password instead.";
  }
}

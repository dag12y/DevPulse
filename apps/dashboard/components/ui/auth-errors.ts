"use client";

export function friendlyAuthError(message: string, fallback: string): string {
  const lower = message.toLowerCase();
  if (lower.includes("fetch") || lower.includes("network") || lower.includes("load failed")) {
    return "Can't reach the DevPulse API. Check your connection and that the API is running, then try again.";
  }
  if (lower.includes("invalid") && lower.includes("credential")) {
    return "Incorrect email or password. Check for typos and try again.";
  }
  if (lower.includes("unauthorized") || lower.includes("401")) {
    return "Incorrect email or password. Check for typos and try again.";
  }
  if (lower.includes("already") && lower.includes("exist")) {
    return "An account with this email already exists. Try signing in instead.";
  }
  if (lower.includes("password") && (lower.includes("12") || lower.includes("short") || lower.includes("weak"))) {
    return "Password must be at least 12 characters. Add a few more characters and symbols.";
  }
  if (lower.includes("email") && lower.includes("valid")) {
    return "Enter a valid email address (e.g. you@company.com).";
  }
  if (lower.includes("rate") || lower.includes("429") || lower.includes("too many")) {
    return "Too many attempts. Wait a minute and try again.";
  }
  return message || fallback;
}

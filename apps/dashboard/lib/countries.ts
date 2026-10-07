/**
 * Country display helpers.
 *
 * The analytics API returns ISO 3166-1 alpha-2 codes (e.g. "US", "ET") from
 * MaxMind GeoIP enrichment, collapsing missing geography to "Unknown".
 * These helpers turn codes into flag emoji + localized names with no extra
 * dependencies or network requests.
 */

import { FALLBACK_NAMES } from "./country-names";

let displayNames: Intl.DisplayNames | null | undefined;

/** Normalize a raw API value to an uppercase ISO code, or null. */
export function normalizeCountryCode(input: string | null | undefined): string | null {
  if (!input) return null;
  const code = input.trim().toUpperCase();
  if (code === "" || code === "UNKNOWN") return null;
  if (!/^[A-Z]{2}$/.test(code)) return null;
  return code;
}

/** True when the API value means "geography unavailable". */
export function isUnknownCountry(input: string | null | undefined): boolean {
  return normalizeCountryCode(input) == null;
}

/**
 * Flag emoji for an ISO code via regional indicator symbols.
 * Returns null for "Unknown" / non-ISO values so callers can show a globe.
 */
export function countryCodeToFlag(input: string | null | undefined): string | null {
  const code = normalizeCountryCode(input);
  if (!code) return null;
  const base = 0x1f1e6; // Regional Indicator Symbol Letter A
  return String.fromCodePoint(
    ...[...code].map((char) => base + char.charCodeAt(0) - 65),
  );
}

/** Localized English country name for an ISO code; falls back to the code. */
export function getCountryName(input: string | null | undefined, locale = "en"): string {
  const code = normalizeCountryCode(input);
  if (!code) return "Unknown";
  try {
    if (displayNames === undefined) {
      displayNames =
        typeof Intl !== "undefined" && "DisplayNames" in Intl
          ? new Intl.DisplayNames([locale, "en"], { type: "region" })
          : null;
    }
    const name = displayNames?.of(code);
    if (name) return name;
  } catch {
    // Ignore and use the static map below.
  }
  return FALLBACK_NAMES[code] ?? code;
}

export interface CountryDisplay {
  /** Original API value (ISO code or "Unknown"). */
  code: string;
  /** Human-readable name ("United States", "Unknown"). */
  name: string;
  /** Flag emoji, or null for unknown geography. */
  flag: string | null;
  unknown: boolean;
}

/** Everything a country cell needs: code, localized name, and flag. */
export function getCountryDisplay(input: string | null | undefined): CountryDisplay {
  const code = normalizeCountryCode(input);
  if (!code) {
    return { code: "Unknown", name: "Unknown", flag: null, unknown: true };
  }
  return { code, name: getCountryName(code), flag: countryCodeToFlag(code), unknown: false };
}

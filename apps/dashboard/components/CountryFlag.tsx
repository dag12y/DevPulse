"use client";

import { getCountryDisplay } from "@/lib/countries";

interface CountryFlagProps {
  /** Raw API value (ISO code like "US" or "Unknown"). */
  code: string | null | undefined;
  /** Size of the flag glyph. Defaults to "md". */
  size?: "sm" | "md" | "lg";
  /** When true, render the localized name next to the flag. */
  showName?: boolean;
  className?: string;
}

const sizes = {
  sm: "text-sm",
  md: "text-lg",
  lg: "text-2xl",
} as const;

/**
 * Flag + localized country name cell.
 * Unknown geography renders a globe with an "Unknown" label.
 */
export default function CountryFlag({ code, size = "md", showName = true, className = "" }: CountryFlagProps) {
  const display = getCountryDisplay(code);
  return (
    <span className={`inline-flex items-center gap-2 ${className}`} title={display.unknown ? "Unknown" : `${display.name} (${display.code})`}>
      <span aria-hidden="true" className={`inline-block leading-none ${sizes[size]}`}>
        {display.flag ?? "🌐"}
      </span>
      {showName && (
        <span className="font-medium">
          {display.name}
          {!display.unknown && (
            <span className="ml-1.5 rounded bg-zinc-100 px-1.5 py-0.5 align-middle text-[10px] font-semibold tracking-wide text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
              {display.code}
            </span>
          )}
        </span>
      )}
    </span>
  );
}

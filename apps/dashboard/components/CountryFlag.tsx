"use client";

import { useState } from "react";
import Image from "next/image";
import { getCountryDisplay } from "@/lib/countries";

interface CountryFlagProps {
  /** Raw API value (ISO code like "US" or "Unknown"). */
  code: string | null | undefined;
  /** Size of the flag glyph. Defaults to "md". */
  size?: "sm" | "md" | "lg";
  /** When true, render the localized name before the flag. */
  showName?: boolean;
  className?: string;
}

const glyphSizes = {
  sm: "text-sm",
  md: "text-base",
  lg: "text-2xl",
} as const;

const imageSizes = {
  sm: "h-3.5 w-3.5",
  md: "h-4.5 w-4.5",
  lg: "h-6 w-6",
} as const;

/**
 * Country name + flag cell. The flag prefers the bundled SVG in
 * /public/flags and falls back to an emoji glyph when the asset is missing.
 * Unknown geography renders a globe with an "Unknown" label.
 */
export default function CountryFlag({ code, size = "md", showName = true, className = "" }: CountryFlagProps) {
  const display = getCountryDisplay(code);
  const [failedCode, setFailedCode] = useState<string | null>(null);
  const useImage = !display.unknown && failedCode !== display.code;

  return (
    <span
      className={`inline-flex items-center gap-2 ${className}`}
      title={display.unknown ? "Unknown" : `${display.name} (${display.code})`}
    >
      {showName && <span className="font-medium">{display.name}</span>}
      <span aria-hidden="true" className={`inline-flex shrink-0 items-center justify-center leading-none ${glyphSizes[size]}`}>
        {useImage ? (
          <Image
            src={`/flags/${display.code.toLowerCase()}.svg`}
            alt=""
            width={18}
            height={18}
            unoptimized
            onError={() => setFailedCode(display.code)}
            className={`${imageSizes[size]} rounded-full`}
          />
        ) : (
          display.flag ?? "🌐"
        )}
      </span>
    </span>
  );
}

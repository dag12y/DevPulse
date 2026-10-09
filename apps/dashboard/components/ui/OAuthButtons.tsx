"use client";

import { useEffect, useState } from "react";
import { getOAuthProviders, oauthStartPath } from "@/lib/api";

const buttonClass =
  "inline-flex w-full items-center justify-center gap-2.5 rounded-lg border border-zinc-300 bg-white px-4 py-2.5 text-sm font-semibold text-zinc-700 transition-colors hover:bg-zinc-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-200 dark:hover:bg-zinc-800";

function GitHubIcon() {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className="h-4 w-4" fill="currentColor">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.4 7.4 0 0 1 2-.27c.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

function GoogleIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" className="h-4 w-4">
      <path fill="#4285F4" d="M23.52 12.27c0-.85-.08-1.67-.22-2.45H12v4.64h6.46a5.52 5.52 0 0 1-2.4 3.62v3h3.88c2.26-2.09 3.58-5.17 3.58-8.81z" />
      <path fill="#34A853" d="M12 24c3.24 0 5.96-1.08 7.94-2.91l-3.88-3c-1.08.72-2.46 1.16-4.06 1.16-3.13 0-5.78-2.11-6.73-4.96H1.28v3.09A12 12 0 0 0 12 24z" />
      <path fill="#FBBC05" d="M5.27 14.29a7.2 7.2 0 0 1 0-4.58V6.62H1.28a12 12 0 0 0 0 10.76l3.99-3.09z" />
      <path fill="#EA4335" d="M12 4.77c1.76 0 3.34.61 4.58 1.79l3.44-3.44C17.95 1.19 15.24 0 12 0 7.7 0 3.99 2.48 1.28 6.62l3.99 3.09C6.22 6.88 8.87 4.77 12 4.77z" />
    </svg>
  );
}

/**
 * "Continue with GitHub/Google" links for the auth pages. The list of
 * enabled providers comes from the API (each button hides when its
 * credentials are unset), and a click is a full-page navigation into
 * the authorize redirect.
 */
export default function OAuthButtons({ next }: { next?: string }) {
  const [providers, setProviders] = useState<{ github: boolean; google: boolean } | null>(null);

  useEffect(() => {
    let active = true;
    getOAuthProviders()
      .then((list) => {
        if (active) setProviders(list);
      })
      .catch(() => {
        if (active) setProviders({ github: false, google: false });
      });
    return () => {
      active = false;
    };
  }, []);

  if (!providers || (!providers.github && !providers.google)) return null;

  return (
    <div className="mt-6">
      <div className="relative" aria-hidden="true">
        <div className="absolute inset-0 flex items-center">
          <div className="w-full border-t border-zinc-200 dark:border-zinc-800" />
        </div>
        <div className="relative flex justify-center text-xs">
          <span className="bg-white px-2 text-zinc-400 dark:bg-zinc-900 dark:text-zinc-500">or continue with</span>
        </div>
      </div>
      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        {providers.github && (
          <a href={oauthStartPath("github", next)} className={buttonClass}>
            <GitHubIcon />
            GitHub
          </a>
        )}
        {providers.google && (
          <a href={oauthStartPath("google", next)} className={buttonClass}>
            <GoogleIcon />
            Google
          </a>
        )}
      </div>
    </div>
  );
}

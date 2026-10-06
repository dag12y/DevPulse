"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import Logo from "@/components/ui/Logo";

export default function AuthShell({ title, subtitle, children, footer }: { title: string; subtitle: string; children: ReactNode; footer: ReactNode }) {
  return (
    <div className="flex min-h-dvh flex-col bg-zinc-50 lg:flex-row dark:bg-zinc-950">
      <div className="flex flex-1 items-center justify-center px-4 py-10 sm:px-8">
        <div className="w-full max-w-md">
          <Link href="/" aria-label="DevPulse home" className="inline-flex rounded-lg focus-visible:outline-2 focus-visible:outline-indigo-600">
            <Logo />
          </Link>
          <div className="mt-8 rounded-2xl border border-zinc-200 bg-white p-6 shadow-xl shadow-zinc-950/5 sm:p-8 dark:border-zinc-800 dark:bg-zinc-900 dark:shadow-none">
            <h1 className="text-2xl font-bold tracking-tight text-zinc-900 dark:text-white">{title}</h1>
            <p className="mt-1.5 text-sm leading-6 text-zinc-500 dark:text-zinc-400">{subtitle}</p>
            <div className="mt-6">{children}</div>
          </div>
          <div className="mt-5 text-center text-sm text-zinc-500 dark:text-zinc-400">{footer}</div>
          <p className="mt-6 text-center text-xs leading-5 text-zinc-400 dark:text-zinc-500">Sessions stay on this device for 30 days.</p>
        </div>
      </div>
      <div className="relative hidden flex-1 overflow-hidden bg-zinc-950 lg:block">
        <div aria-hidden="true" className="absolute inset-0 bg-[radial-gradient(ellipse_60%_50%_at_30%_10%,rgba(99,102,241,0.35),transparent),radial-gradient(ellipse_50%_60%_at_80%_90%,rgba(168,85,247,0.28),transparent)]" />
        <div className="relative flex h-full flex-col justify-center p-12 xl:p-16">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-indigo-300">DevPulse Analytics</p>
          <h2 className="mt-4 max-w-md text-4xl font-bold leading-[1.1] tracking-tight text-white">Privacy-conscious analytics your users will never notice.</h2>
          <ul className="mt-8 space-y-5">
            {[
              { title: "No cookies, no fingerprinting", body: "Aggregate views and visitors without tracking individuals." },
              { title: "One script tag", body: "Drop in analytics.js and see real-time visitors within seconds." },
              { title: "Self-host and own your data", body: "Per-project retention, export anytime." },
            ].map((item) => (
              <li key={item.title} className="flex gap-3.5">
                <span className="mt-0.5 grid h-6 w-6 shrink-0 place-items-center rounded-full bg-emerald-500/15 text-emerald-300">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="h-3.5 w-3.5"><path d="M20 6L9 17l-5-5" /></svg>
                </span>
                <span>
                  <span className="block text-sm font-semibold text-white">{item.title}</span>
                  <span className="mt-0.5 block text-sm leading-6 text-zinc-400">{item.body}</span>
                </span>
              </li>
            ))}
          </ul>
          <div className="mt-10 rounded-2xl border border-white/10 bg-white/5 p-5 backdrop-blur" aria-hidden="true">
            <div className="flex items-center justify-between">
              <p className="text-xs font-medium uppercase tracking-wider text-zinc-400">Visitors - last 7 days</p>
              <p className="rounded-full bg-emerald-500/15 px-2.5 py-1 text-xs font-semibold text-emerald-300">+24.6%</p>
            </div>
            <div className="mt-4 flex h-24 items-end gap-1.5">
              {[38, 52, 44, 66, 58, 74, 62, 84, 70, 92, 78, 100].map((h, i) => (
                <div key={i} className={`flex-1 rounded-t-md ${i === 11 ? "bg-gradient-to-t from-indigo-500 to-violet-400" : "bg-white/15"}`} style={{ height: `${h}%` }} />
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

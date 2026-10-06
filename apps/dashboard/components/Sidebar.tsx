"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState, type ReactNode } from "react";
import ProjectSelector from "@/components/ProjectSelector";
import ThemeToggle from "@/components/ThemeToggle";
import WorkspaceSelector from "@/components/WorkspaceSelector";
import Logo from "@/components/ui/Logo";
import { useAuth } from "@/lib/auth-context";

const navItems: { href: string; label: string; icon: ReactNode }[] = [
  {
    href: "/",
    label: "Overview",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="h-4 w-4 shrink-0">
        <rect x="3" y="3" width="7" height="9" rx="1.5" />
        <rect x="14" y="3" width="7" height="5" rx="1.5" />
        <rect x="14" y="12" width="7" height="9" rx="1.5" />
        <rect x="3" y="16" width="7" height="5" rx="1.5" />
      </svg>
    ),
  },
  {
    href: "/traffic",
    label: "Traffic",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true" className="h-4 w-4 shrink-0">
        <path d="M3 3v16a2 2 0 0 0 2 2h16" />
        <path d="M7 14l4-4 3 3 5-6" />
      </svg>
    ),
  },
  {
    href: "/pages",
    label: "Pages",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="h-4 w-4 shrink-0">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
        <path d="M14 2v6h6M9 13h6M9 17h6" />
      </svg>
    ),
  },
  {
    href: "/sources",
    label: "Sources",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="h-4 w-4 shrink-0">
        <circle cx="12" cy="12" r="10" />
        <path d="M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
      </svg>
    ),
  },
  {
    href: "/countries",
    label: "Countries",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="h-4 w-4 shrink-0">
        <path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0z" />
        <circle cx="12" cy="10" r="3" />
      </svg>
    ),
  },
  {
    href: "/devices",
    label: "Devices",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="h-4 w-4 shrink-0">
        <rect x="2" y="3" width="14" height="12" rx="2" />
        <path d="M6 21h6M9 15v6M16 8l5 2-5 2 2-2z" />
      </svg>
    ),
  },
  {
    href: "/realtime",
    label: "Real-time",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true" className="h-4 w-4 shrink-0">
        <circle cx="12" cy="12" r="2" />
        <path d="M7.5 7.5a6.4 6.4 0 0 0 0 9M16.5 7.5a6.4 6.4 0 0 1 0 9M4.9 4.9a10.5 10.5 0 0 0 0 14.2M19.1 4.9a10.5 10.5 0 0 1 0 14.2" />
      </svg>
    ),
  },
  {
    href: "/projects",
    label: "Projects",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="h-4 w-4 shrink-0">
        <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
      </svg>
    ),
  },
  {
    href: "/install",
    label: "Install",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="h-4 w-4 shrink-0">
        <path d="M8 9l-4 3 4 3M16 9l4 3-4 3M13 5l-2 14" />
      </svg>
    ),
  },
  {
    href: "/settings",
    label: "Settings",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="h-4 w-4 shrink-0">
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h.09a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h.09a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v.09a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
      </svg>
    ),
  },
];

function NavLinks({ pathname, onNavigate }: { pathname: string; onNavigate?: () => void }) {
  return (
    <nav aria-label="Analytics reports" className="space-y-1 flex-1">
      {navItems.map((item) => {
        const isActive = pathname === item.href;
        return (
          <Link
            key={item.href}
            href={item.href}
            aria-current={isActive ? "page" : undefined}
            onClick={onNavigate}
            className={`flex items-center gap-2.5 px-3 py-2 rounded-md text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 ${
              isActive
                ? "bg-indigo-50 text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-300"
                : "text-zinc-600 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800/50"
            }`}
          >
            {item.icon}
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}

function AccountSection() {
  const { user, logout } = useAuth();
  if (user) {
    return (
      <div className="space-y-2">
        <Link
          href="/account"
          className="block truncate px-3 py-1 text-zinc-700 dark:text-zinc-300 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
          title={user.email}
        >
          {user.email}
        </Link>
        <button
          onClick={() => logout()}
          className="block w-full rounded-md border border-zinc-300 dark:border-zinc-700 px-3 py-1.5 text-zinc-700 dark:text-zinc-300 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
        >
          Sign out
        </button>
      </div>
    );
  }
  return (
    <div className="space-y-1 px-3">
      <Link className="block rounded-md bg-indigo-600 px-3 py-2 text-center text-sm font-semibold text-white hover:bg-indigo-500 focus-visible:outline-2 focus-visible:outline-indigo-600" href="/login">Sign in</Link>
      <Link className="block px-3 py-1 text-center text-sm text-zinc-500 hover:underline focus-visible:outline-2 focus-visible:outline-indigo-600" href="/register">Create account</Link>
    </div>
  );
}

export default function Sidebar() {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);

  return (
    <>
      <div className="md:hidden w-full sticky top-0 z-30 flex items-center justify-between border-b border-zinc-200 dark:border-zinc-800 bg-white/95 dark:bg-zinc-950/95 px-4 py-3 backdrop-blur">
        <span className="text-base font-semibold text-zinc-900 dark:text-zinc-100">DevPulse</span>
        <button
          type="button"
          onClick={() => setOpen((value) => !value)}
          aria-expanded={open}
          aria-controls="mobile-nav"
          aria-label={open ? "Close navigation" : "Open navigation"}
          className="rounded-md border border-zinc-300 dark:border-zinc-700 px-3 py-1.5 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
        >
          {open ? "Close" : "Menu"}
        </button>
      </div>

      {open && (
        <div className="md:hidden fixed inset-0 z-40">
          <div aria-hidden="true" className="absolute inset-0 bg-black/40" onClick={() => setOpen(false)} />
          <div
            id="mobile-nav"
            role="dialog"
            aria-modal="true"
            aria-label="Site navigation"
            className="absolute left-0 top-0 flex h-full w-72 flex-col bg-zinc-50 dark:bg-zinc-900 p-4 shadow-xl"
          >
            <div className="mb-4">
              <WorkspaceSelector />
            </div>
            <div className="mb-4">
              <ProjectSelector />
            </div>
            <NavLinks pathname={pathname} onNavigate={() => setOpen(false)} />
            <div className="mt-6 border-t border-zinc-200 dark:border-zinc-800 pt-4 space-y-4">
              <ThemeToggle />
              <AccountSection />
            </div>
          </div>
        </div>
      )}

      <aside className="w-60 shrink-0 border-r border-zinc-200 bg-white p-4 hidden md:flex md:flex-col dark:border-zinc-800 dark:bg-zinc-900">
        <div className="mb-8 px-1">
          <Logo />
        </div>
        <div className="mb-4">
          <WorkspaceSelector />
        </div>
        <div className="mb-6">
          <ProjectSelector />
        </div>
        <NavLinks pathname={pathname} />
        <div className="mt-6 border-t border-zinc-200 dark:border-zinc-800 pt-4 space-y-4 text-sm">
          <ThemeToggle />
          <AccountSection />
        </div>
      </aside>
    </>
  );
}

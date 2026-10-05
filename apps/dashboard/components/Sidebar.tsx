"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import ProjectSelector from "@/components/ProjectSelector";
import WorkspaceSelector from "@/components/WorkspaceSelector";
import { useAuth } from "@/lib/auth-context";

const navItems = [
  { href: "/", label: "Overview" },
  { href: "/traffic", label: "Traffic" },
  { href: "/pages", label: "Pages" },
  { href: "/sources", label: "Sources" },
  { href: "/countries", label: "Countries" },
  { href: "/devices", label: "Devices" },
  { href: "/realtime", label: "Real-time" },
  { href: "/projects", label: "Projects" },
  { href: "/install", label: "Install" },
  { href: "/settings", label: "Settings" },
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
            className={`block px-3 py-2 rounded-md text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600 ${
              isActive
                ? "bg-zinc-200 dark:bg-zinc-800 text-zinc-900 dark:text-zinc-100"
                : "text-zinc-600 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800/50"
            }`}
          >
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
      <Link className="block hover:underline focus-visible:outline-2 focus-visible:outline-blue-600" href="/login">Sign in</Link>
      <Link className="block text-zinc-500 hover:underline focus-visible:outline-2 focus-visible:outline-blue-600" href="/register">Create account</Link>
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
            <div className="mt-6 border-t border-zinc-200 dark:border-zinc-800 pt-4 text-sm">
              <AccountSection />
            </div>
          </div>
        </div>
      )}

      <aside className="w-56 shrink-0 border-r border-zinc-200 dark:border-zinc-800 bg-zinc-50 dark:bg-zinc-900 p-4 hidden md:flex md:flex-col">
        <div className="mb-8">
          <h1 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">DevPulse</h1>
        </div>
        <div className="mb-4">
          <WorkspaceSelector />
        </div>
        <div className="mb-6">
          <ProjectSelector />
        </div>
        <NavLinks pathname={pathname} />
        <div className="mt-6 border-t border-zinc-200 dark:border-zinc-800 pt-4 text-sm">
          <AccountSection />
        </div>
      </aside>
    </>
  );
}

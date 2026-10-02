"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
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
];

export default function Sidebar() {
  const pathname = usePathname();
  const { user, logout } = useAuth();

  return (
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
      <nav className="space-y-1 flex-1">
        {navItems.map((item) => {
          const isActive = pathname === item.href;
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`block px-3 py-2 rounded-md text-sm font-medium transition-colors ${
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
      <div className="mt-6 border-t border-zinc-200 dark:border-zinc-800 pt-4 text-sm">
        {user ? (
          <div className="space-y-2">
            <Link
              href="/account"
              className="block truncate px-3 py-1 text-zinc-700 dark:text-zinc-300 hover:underline"
              title={user.email}
            >
              {user.email}
            </Link>
            <button
              onClick={() => logout()}
              className="block w-full rounded-md border border-zinc-300 dark:border-zinc-700 px-3 py-1.5 text-zinc-700 dark:text-zinc-300"
            >
              Sign out
            </button>
          </div>
        ) : (
          <div className="space-y-1 px-3">
            <Link className="block hover:underline" href="/login">Sign in</Link>
            <Link className="block text-zinc-500 hover:underline" href="/register">Create account</Link>
          </div>
        )}
      </div>
    </aside>
  );
}

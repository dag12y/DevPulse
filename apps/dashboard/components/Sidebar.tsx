"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const navItems = [
  { href: "/", label: "Overview" },
  { href: "/traffic", label: "Traffic" },
  { href: "/pages", label: "Pages" },
  { href: "/sources", label: "Sources" },
  { href: "/countries", label: "Countries" },
  { href: "/devices", label: "Devices" },
  { href: "/realtime", label: "Real-time" },
  { href: "/projects", label: "Projects" },
];

export default function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="w-56 shrink-0 border-r border-zinc-200 dark:border-zinc-800 bg-zinc-50 dark:bg-zinc-900 p-4 hidden md:block">
      <div className="mb-8">
        <h1 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">DevPulse</h1>
      </div>
      <nav className="space-y-1">
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
    </aside>
  );
}

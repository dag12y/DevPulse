"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import Sidebar from "@/components/Sidebar";

const AUTH_PATHS = new Set(["/login", "/register"]);

export default function AppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  if (AUTH_PATHS.has(pathname)) {
    return <main id="main-content" className="min-w-0 flex-1">{children}</main>;
  }
  return (
    <>
      <Sidebar />
      <main id="main-content" className="min-w-0 flex-1">{children}</main>
    </>
  );
}

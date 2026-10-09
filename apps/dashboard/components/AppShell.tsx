"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import type { ReactNode } from "react";
import { useAuth } from "@/lib/auth-context";
import { isPublicPath, safeNext } from "@/lib/paths";
import Sidebar from "@/components/Sidebar";

export default function AppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, loading, usingEnvKey } = useAuth();

  // Client-side route guard: covers SPA navigations (middleware only sees
  // full page loads) and sessions that died after the page was open.
  useEffect(() => {
    if (loading || usingEnvKey || user || isPublicPath(pathname)) return;
    const target = pathname + window.location.search;
    router.replace(`/login?next=${encodeURIComponent(safeNext(target))}`);
  }, [loading, usingEnvKey, user, pathname, router]);

  if (isPublicPath(pathname)) {
    return <main id="main-content" className="min-w-0 flex-1">{children}</main>;
  }
  return (
    <>
      <Sidebar />
      <main id="main-content" className="min-w-0 flex-1">{children}</main>
    </>
  );
}

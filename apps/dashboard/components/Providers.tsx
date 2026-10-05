"use client";

import type { ReactNode } from "react";
import { AuthProvider } from "@/lib/auth-context";
import { ProjectProvider } from "@/lib/project-context";
import { ThemeProvider } from "@/lib/theme-context";

export default function Providers({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider>
      <AuthProvider>
        <ProjectProvider>{children}</ProjectProvider>
      </AuthProvider>
    </ThemeProvider>
  );
}

import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { headers } from "next/headers";
import "./globals.css";
import Providers from "@/components/Providers";
import AppShell from "@/components/AppShell";

const inter = Inter({ subsets: ["latin"], display: "swap" });

export const metadata: Metadata = {
  title: "DevPulse — Privacy-conscious web analytics",
  description: "Privacy-conscious web analytics for developers. No cookies, no fingerprinting.",
};

const themeScript = `(function(){try{var t=localStorage.getItem("devpulse.theme");var d=t==="dark"||((!t||t==="system")&&window.matchMedia("(prefers-color-scheme: dark)").matches);if(d){document.documentElement.classList.add("dark")}}catch(e){}})();`;

export default async function RootLayout({ children }: LayoutProps<"/">) {
  const nonce = (await headers()).get("x-nonce") ?? undefined;

  return (
    <html lang="en" className={`h-full antialiased ${inter.className}`} suppressHydrationWarning>
      <body className="min-h-full flex flex-col md:flex-row">
        <script nonce={nonce} dangerouslySetInnerHTML={{ __html: themeScript }} />
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:m-2 focus:rounded-md focus:bg-white focus:px-3 focus:py-2 focus:text-sm focus:text-zinc-900 focus:outline-2 focus:outline-indigo-600"
        >
          Skip to main content
        </a>
        <Providers>
          <AppShell>{children}</AppShell>
        </Providers>
      </body>
    </html>
  );
}

import type { Metadata } from "next";
import "./globals.css";
import Sidebar from "@/components/Sidebar";
import Providers from "@/components/Providers";

export const metadata: Metadata = {
  title: "DevPulse",
  description: "Privacy-conscious web analytics for developers.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="min-h-full flex flex-col md:flex-row">
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:m-2 focus:rounded-md focus:bg-white focus:px-3 focus:py-2 focus:text-sm focus:text-zinc-900 focus:outline-2 focus:outline-blue-600"
        >
          Skip to main content
        </a>
        <Providers>
          <Sidebar />
          <main id="main-content" className="flex-1 min-w-0">{children}</main>
        </Providers>
      </body>
    </html>
  );
}

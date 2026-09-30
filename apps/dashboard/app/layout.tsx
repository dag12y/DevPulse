import type { Metadata } from "next";
import "./globals.css";
import Sidebar from "@/components/Sidebar";

export const metadata: Metadata = {
  title: "DevPulse",
  description: "Privacy-conscious web analytics for developers.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="min-h-full flex">
        <Sidebar />
        <main className="flex-1 min-w-0">{children}</main>
      </body>
    </html>
  );
}

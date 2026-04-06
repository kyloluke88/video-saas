import type { ReactNode } from "react";
import type { Metadata } from "next";

import "./globals.css";

const siteUrl = process.env.NEXT_PUBLIC_SITE_URL || "http://localhost:3000";

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: {
    default: "Chinese Podcast Scripts",
    template: "%s | Chinese Podcast Scripts",
  },
  description: "SEO-friendly podcast script pages, vocabulary notes, grammar summaries, and downloadable study materials.",
  alternates: {
    canonical: "/",
  },
  openGraph: {
    title: "Chinese Podcast Scripts",
    description: "SEO-friendly podcast script pages, vocabulary notes, grammar summaries, and downloadable study materials.",
    url: siteUrl,
    siteName: "Chinese Podcast Scripts",
    locale: "zh_CN",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Chinese Podcast Scripts",
    description: "SEO-friendly podcast script pages, vocabulary notes, grammar summaries, and downloadable study materials.",
  },
};

export default function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <body className="min-h-screen bg-background font-body text-foreground antialiased">
        <div className="pointer-events-none fixed inset-0 -z-10 bg-[radial-gradient(circle_at_top_left,rgba(217,119,6,0.15),transparent_28%),radial-gradient(circle_at_top_right,rgba(14,116,144,0.16),transparent_24%),linear-gradient(180deg,#fffaf5_0%,#f8f1e8_48%,#f1e5d7_100%)]" />
        {children}
      </body>
    </html>
  );
}

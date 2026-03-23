import "./globals.css";

export const metadata = {
  title: "Video SaaS Showcase",
  description: "Public-facing pages for generated conversation projects.",
};

export default function RootLayout({ children }) {
  return (
    <html lang="zh-CN">
      <body>{children}</body>
    </html>
  );
}

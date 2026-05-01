import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "FinTech Core API",
  description: "Operational reporting dashboard for FinTech Core API"
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}


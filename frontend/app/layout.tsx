import type { Metadata } from "next";
import { Plus_Jakarta_Sans } from "next/font/google";
import Navbar from "@/components/Navbar";
import "./globals.css";

const plusJakartaSans = Plus_Jakarta_Sans({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-sans",
});

export const metadata: Metadata = {
  title: "folixenda – Discover Local Events",
  description: "Find and explore music, theater, art, food, and more events near you.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={`${plusJakartaSans.variable} h-full`}>
      <body className="min-h-full flex flex-col antialiased" style={{ fontFamily: "var(--font-sans), sans-serif" }}>
        <Navbar />
        <main className="flex-1">{children}</main>
      </body>
    </html>
  );
}

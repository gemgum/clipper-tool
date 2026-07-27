import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Clipper",
  description: "Potong video panjang jadi klip pendek 9:16 bersubtitle + skor viral",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id">
      <body>{children}</body>
    </html>
  );
}

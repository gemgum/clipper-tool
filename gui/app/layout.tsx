import type { Metadata } from "next";
import "./globals.css";
import Nav from "./nav";

export const metadata: Metadata = {
  title: "Clipper",
  description: "Potong video panjang jadi klip pendek 9:16 bersubtitle + skor viral",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id">
      <body>
        <Nav />
        {children}
      </body>
    </html>
  );
}

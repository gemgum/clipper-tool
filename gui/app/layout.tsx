import type { Metadata } from "next";
import "./globals.css";
import Nav from "./nav";
import { I18nProvider } from "./i18n";

export const metadata: Metadata = {
  title: "Clipper",
  description:
    "Cut long videos into short 9:16 clips with subtitles + a viral score",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  // lang="en" adalah nilai awal; I18nProvider memperbaruinya begitu pilihan
  // bahasa yang tersimpan dibaca, supaya render server & klien tetap cocok.
  return (
    <html lang="en">
      <body>
        <I18nProvider>
          <Nav />
          {children}
        </I18nProvider>
      </body>
    </html>
  );
}

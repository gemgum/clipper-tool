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
      <head>
        {/* Tema dipasang SEBELUM halaman digambar. Kalau dibaca dari React saja,
            halaman sempat berkedip putih dulu tiap kali dibuka dalam tema gelap
            — dan kedipan itu paling terlihat justru di aplikasi desktop yang
            memuat ulang halamannya sendiri saat jendela ditinggal. */}
        <script dangerouslySetInnerHTML={{ __html:
          `try{var t=localStorage.getItem("clipper.theme");` +
          `if(!t)t=matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light";` +
          `document.documentElement.dataset.theme=t}catch(e){}` }} />
      </head>
      <body>
        <I18nProvider>
          {/* Rail kiri berdiri sendiri; bilah atas dan isi halaman berbagi
              satu kolom di kanannya. Nav merender keduanya (rail + topbar),
              jadi urutannya di sini: rail, lalu kolom kanan. */}
          <div className="app">
            <Nav />
            <div className="app-page">{children}</div>
          </div>
        </I18nProvider>
      </body>
    </html>
  );
}

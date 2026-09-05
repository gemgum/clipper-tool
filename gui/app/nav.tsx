"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Scissors, Newspaper, PenLine, Captions, Stamp, History } from "lucide-react";
import { useI18n, type MessageKey } from "./i18n";
import SettingsMenu from "./settings-menu";
import ThemeToggle from "./theme-toggle";
import AccountButton from "./account";

// Navigasi rail kiri + bilah atas.
//
// Bentuknya mengikuti aplikasi desktop, bukan situs: pekerjaan utama ada di
// rail kiri (potong video, kartu berita), sedangkan yang mengatur APLIKASINYA
// — komponen, folder, bahasa — menepi ke kanan atas. Pembedaannya bukan selera:
// yang di kiri dibuka berkali-kali sehari, yang di kanan dibuka sekali lalu
// dilupakan, dan menaruh keduanya berdampingan membuat keduanya terlihat
// sama penting.

const RAIL: { href: string; label: MessageKey; Icon: typeof Scissors }[] = [
  { href: "/", label: "tabClips", Icon: Scissors },
  { href: "/news", label: "tabNews", Icon: Newspaper },
  { href: "/writer", label: "tabWriter", Icon: PenLine },
  { href: "/captions", label: "tabCaptions", Icon: Captions },
  { href: "/watermark", label: "tabWatermark", Icon: Stamp },
  { href: "/history", label: "tabHistory", Icon: History },
];

// Ekspor statis Next menghasilkan /news/index.html, jadi alamat yang terbaca
// browser berakhir dengan garis miring ("/news/") sedangkan href di daftar
// ditulis tanpa ("/news"). Membandingkannya mentah-mentah membuat SELURUH
// halaman selain "/" kehilangan penanda posisinya — terukur, bukan dugaan.
const samePath = (a: string, b: string) =>
  (a.replace(/\/+$/, "") || "/") === (b.replace(/\/+$/, "") || "/");

export default function Nav() {
  const path = usePathname();
  const { t } = useI18n();

  return (
    <>
      <nav className="rail" aria-label="Clipper">
        <span className="rail-brand">Clipper</span>
        {RAIL.map(({ href, label, Icon }) => (
          <Link
            key={href}
            href={href}
            className={"rail-item" + (samePath(path, href) ? " active" : "")}
            aria-current={samePath(path, href) ? "page" : undefined}
          >
            <Icon className="rail-ico" aria-hidden="true" />
            <span>{t(label)}</span>
          </Link>
        ))}

        {/* Akun, tema, dan setelan MENEPI KE DASAR rail, bukan bilah atas
            sendiri. Bilah itu berisi tiga ikon dan tidak pernah lebih — satu
            baris penuh selebar jendela untuk tiga ikon adalah tinggi yang
            diambil dari isi halaman, tiap halaman, selamanya.
            Ketiganya tetap terpisah dari navigasi di atasnya oleh jarak, jadi
            "tempat kerja" dan "atur aplikasinya" masih terbaca berbeda. */}
        <div className="rail-tools">
          <AccountButton />
          <ThemeToggle />
          <SettingsMenu />
        </div>
      </nav>
    </>
  );
}

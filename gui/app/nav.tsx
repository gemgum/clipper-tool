"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Scissors, Newspaper, Clapperboard, History } from "lucide-react";
import { useI18n, type MessageKey } from "./i18n";
import SettingsMenu from "./settings-menu";

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
  { href: "/clips", label: "tabResults", Icon: Clapperboard },
  { href: "/news", label: "tabNews", Icon: Newspaper },
  { href: "/history", label: "tabHistory", Icon: History },
];

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
            className={"rail-item" + (path === href ? " active" : "")}
            aria-current={path === href ? "page" : undefined}
          >
            <Icon className="rail-ico" aria-hidden="true" />
            <span>{t(label)}</span>
          </Link>
        ))}
      </nav>

      {/* Bilah atas hanya memuat setelan aplikasi. Bahasa pindah KE DALAM
          panel itu: ia setelan, dan tempatnya bersama setelan lain — bukan
          berjajar dengan navigasi seolah sama pentingnya. */}
      <div className="topbar">
        <SettingsMenu />
      </div>
    </>
  );
}

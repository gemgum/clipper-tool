"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const TAB = [
  { href: "/", label: "Klip video" },
  { href: "/berita", label: "Kartu berita" },
];

export default function Nav() {
  const path = usePathname();
  return (
    <nav className="nav">
      <div className="nav-inner">
        <span className="brand">Clipper</span>
        {TAB.map((t) => (
          <Link
            key={t.href}
            href={t.href}
            className={"nav-tab" + (path === t.href ? " aktif" : "")}
          >
            {t.label}
          </Link>
        ))}
      </div>
    </nav>
  );
}

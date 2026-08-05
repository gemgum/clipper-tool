"use client";

import { useCallback, useEffect, useState } from "react";

export type PopupPos = { top: number; left: number; width: number; maxH: number };

// Letak & perilaku popup melayang, dipakai bersama <Popover> dan <Select>.
//
// Disatukan karena SATU aturannya mudah salah dan mahal: letaknya WAJIB
// `position: fixed`, bukan `absolute`. Tiap kolom di kerangka berzona ini punya
// `overflow-y: auto` sendiri, dan itu MEMOTONG anak yang diposisikan absolut
// begitu ia lebih lebar daripada kolomnya — dan isi popup di sini hampir selalu
// lebih lebar. Ditulis dua kali berarti dua tempat untuk salah.
//
// Karena `fixed` tidak punya induk untuk diukur, letaknya dihitung dari
// getBoundingClientRect() pemicunya lalu dijepit ke tepi jendela.
export function usePopup({
  open, setOpen, anchor, width, align = "left", selectors, maxHeight = 420,
}: {
  open: boolean;
  setOpen: (v: boolean) => void;
  anchor: React.RefObject<HTMLElement | null>;
  /** Lebar yang diminta; dijepit ke lebar jendela. */
  width: number | ((rect: DOMRect) => number);
  align?: "left" | "right";
  /** Kelas yang dianggap "di dalam popup" saat memeriksa klik di luar. */
  selectors: [string, string];
  /** Batas atas tinggi popup; isinya bergulir sendiri di dalamnya. */
  maxHeight?: number;
}) {
  const [pos, setPos] = useState<PopupPos | null>(null);

  const place = useCallback(() => {
    const r = anchor.current?.getBoundingClientRect();
    if (!r) return;
    const w = Math.min(typeof width === "function" ? width(r) : width, window.innerWidth - 32);
    const below = window.innerHeight - r.bottom - 12;
    const above = r.top - 12;
    // Membuka ke ATAS bila ruang di bawahnya lebih sempit. Ini WAJIB, bukan
    // kemewahan: tombol setelan/tema/akun berdiri di DASAR rail, jadi popup
    // yang selalu membuka ke bawah berakhir 282 px di bawah tepi layar —
    // terukur, dan artinya tidak terlihat sama sekali. Halaman tidak bisa
    // digulir untuk mengejarnya (body:has(.screen) overflow:hidden), jadi
    // popupnya benar-benar hilang.
    const down = below >= Math.min(240, above);
    const room = down ? below : above;
    const maxH = Math.max(140, Math.min(maxHeight, room));
    const wanted = align === "right" ? r.right - w : r.left;
    setPos({
      top: down ? r.bottom + 6 : Math.max(8, r.top - maxH - 6),
      left: Math.max(8, Math.min(wanted, window.innerWidth - w - 8)),
      width: w,
      maxH,
    });
  }, [anchor, width, align, maxHeight]);

  // Esc, klik di luar, ubah ukuran, dan GULIR. Yang terakhir sering terlupa:
  // popup `fixed` tidak ikut bergerak bersama kolom yang digulir, jadi
  // membiarkannya terbuka berarti ia menggantung di tempat yang salah.
  useEffect(() => {
    if (!open) return;
    const [inPop, inAnchor] = selectors;
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") setOpen(false); };
    const onDown = (e: MouseEvent) => {
      const el = e.target as HTMLElement;
      if (!el.closest(inPop) && !el.closest(inAnchor)) setOpen(false);
    };
    const onScroll = () => setOpen(false);
    window.addEventListener("keydown", onKey);
    window.addEventListener("mousedown", onDown);
    window.addEventListener("resize", place);
    window.addEventListener("scroll", onScroll, true);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", onScroll, true);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, place, setOpen]);

  // Dibuka dari luar (mis. tombol Cari) tidak melewati pemicunya, jadi letaknya
  // dihitung di sini juga — kalau tidak, popupnya muncul di koordinat lama.
  useEffect(() => { if (open) place(); }, [open, place]);

  return { pos, place };
}

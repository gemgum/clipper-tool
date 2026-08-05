"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// Popup yang MENEMPEL pada tombolnya. Satu komponen untuk semua — jelajah
// berita, palet warna, dan apa pun berikutnya.
//
// Dibuat jadi komponen bersama bukan karena rapi, melainkan karena satu
// detailnya mudah salah dan mahal: letaknya WAJIB `position: fixed`, bukan
// `absolute`. Tiap kolom di kerangka berzona ini punya `overflow-y: auto`
// sendiri, dan itu MEMOTONG anak yang diposisikan absolut begitu ia lebih lebar
// daripada kolomnya. Popup di sini hampir selalu lebih lebar (kisi tiga kolom,
// deretan contekan warna), jadi kesalahan itu pasti terjadi — bukan mungkin.
//
// Karena `fixed` tidak punya induk untuk diukur, letaknya dihitung dari
// getBoundingClientRect() tombolnya, lalu dijepit ke lebar jendela supaya tidak
// pernah keluar layar di jendela terkecil (900 px).
export default function Popover({
  label, width = 320, align = "left", buttonClass = "ghost", disabled, onOpen, children,
}: {
  label: React.ReactNode;
  width?: number;
  align?: "left" | "right";
  buttonClass?: string;
  disabled?: boolean;
  onOpen?: () => void;
  children: (close: () => void) => React.ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number; width: number } | null>(null);
  const btn = useRef<HTMLButtonElement>(null);

  const place = useCallback(() => {
    const r = btn.current?.getBoundingClientRect();
    if (!r) return;
    const w = Math.min(width, window.innerWidth - 32);
    const wanted = align === "right" ? r.right - w : r.left;
    setPos({
      top: r.bottom + 6,
      left: Math.max(16, Math.min(wanted, window.innerWidth - w - 16)),
      width: w,
    });
  }, [width, align]);

  const close = useCallback(() => setOpen(false), []);

  const toggle = () => {
    if (open) { setOpen(false); return; }
    place();
    setOpen(true);
    onOpen?.();
  };

  // Esc dan klik di luar — dua jalan keluar yang dicari orang tanpa diajari.
  // Gulir ikut menutup: popup `fixed` tidak ikut bergerak bersama kolom yang
  // digulir, jadi membiarkannya terbuka berarti ia menggantung di tempat yang
  // salah.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") setOpen(false); };
    const onDown = (e: MouseEvent) => {
      const el = e.target as HTMLElement;
      if (!el.closest(".popover") && !el.closest(".popover-anchor")) setOpen(false);
    };
    window.addEventListener("keydown", onKey);
    window.addEventListener("mousedown", onDown);
    window.addEventListener("resize", place);
    window.addEventListener("scroll", close, true);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", close, true);
    };
  }, [open, place, close]);

  return (
    <div className="popover-anchor">
      <button ref={btn} type="button" disabled={disabled}
        className={buttonClass + (open ? " active" : "")}
        aria-expanded={open} onClick={toggle}>
        {label}
      </button>
      {open && pos && (
        <div className="popover" style={{ top: pos.top, left: pos.left, width: pos.width }}>
          {children(close)}
        </div>
      )}
    </div>
  );
}

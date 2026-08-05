"use client";

import { useRef } from "react";

// DragRow: deretan mendatar yang bisa DISERET, bukan hanya digulir.
//
// Bilah gulir mendatar mudah terlewat dan sulit ditarik dengan trackpad;
// menarik isinya langsung adalah gerakan yang orang coba lebih dulu. Roda
// tetikus vertikal ikut dibelokkan jadi gerak mendatar, sebab di deretan
// mendatar itulah arti "gulir" yang diharapkan.
export default function DragRow({ children }: { children: React.ReactNode }) {
  const box = useRef<HTMLDivElement | null>(null);
  const drag = useRef<{ x: number; left: number } | null>(null);

  const down = (e: React.PointerEvent) => {
    // Hanya seretan pada latar barisnya. Tanpa ini, menyeret di atas tombol
    // atau <video> ikut tertangkap dan kliknya tidak pernah sampai.
    if ((e.target as HTMLElement).closest("button, a, video, input")) return;
    if (box.current) drag.current = { x: e.clientX, left: box.current.scrollLeft };
  };
  const move = (e: React.PointerEvent) => {
    const d = drag.current;
    if (!d || !box.current) return;
    box.current.scrollLeft = d.left - (e.clientX - d.x);
  };
  const up = () => { drag.current = null; };

  return (
    <div ref={box} className="strip"
      onPointerDown={down} onPointerMove={move} onPointerUp={up} onPointerLeave={up}
      onWheel={(e) => {
        if (!box.current || Math.abs(e.deltaY) <= Math.abs(e.deltaX)) return;
        box.current.scrollLeft += e.deltaY;
      }}>
      {children}
    </div>
  );
}

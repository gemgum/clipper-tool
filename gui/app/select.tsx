"use client";

import { ChevronDown, Check } from "lucide-react";
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";

export type Option = {
  value: string;
  label: string;
  /** Keterangan kecil di kanan — ukuran model, "perlu unduh", dsb. */
  note?: string;
  disabled?: boolean;
};

// Daftar pilihan milik sendiri, pengganti <select> bawaan.
//
// Alasannya bukan selera: daftar yang dibuka <select> digambar SISTEM OPERASI,
// bukan halaman. Ia tidak mewarisi satu pun token warna kita — di tema gelap ia
// tetap putih menyilaukan, sorotannya biru bawaan Windows, dan padding-nya
// rapat. Tidak ada CSS yang bisa menyentuhnya; satu-satunya jalan adalah
// menggambar daftarnya sendiri.
//
// Popupnya `position: fixed` dengan letak dihitung dari tombolnya — alasan yang
// sama dengan popover.tsx: tiap kolom di kerangka ini punya `overflow-y: auto`
// sendiri, dan anak yang `absolute` akan terpotong.
//
// Papan ketik dilayani penuh, sebab <select> bawaan melayaninya dan
// menggantinya dengan yang tidak berarti membuat aplikasi ini lebih sulit
// dipakai, bukan lebih bagus: panah atas/bawah, Home/End, Enter, Esc.
export default function Select({
  value, onChange, options, disabled, title, ariaLabel, id,
}: {
  value: string;
  onChange: (v: string) => void;
  options: Option[];
  disabled?: boolean;
  title?: string;
  ariaLabel?: string;
  id?: string;
}) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number; width: number; maxH: number } | null>(null);
  const [cursor, setCursor] = useState(0);
  const btn = useRef<HTMLButtonElement>(null);
  const list = useRef<HTMLDivElement>(null);

  // Kalau nilainya belum cocok dengan satu pun pilihan — misalnya model Ollama
  // bawaan "qwen2.5" sementara yang terpasang "llama3.1:latest" — tampilkan
  // pilihan PERTAMA, persis seperti <select> bawaan. Tanpa ini kotaknya tampil
  // KOSONG sampai efek auto-pilih selesai berjalan, dan itu terbaca seperti bug.
  const current = options.find((o) => o.value === value) ?? options[0];

  const place = useCallback(() => {
    const r = btn.current?.getBoundingClientRect();
    if (!r) return;
    // Buka ke ATAS bila ruang di bawahnya lebih sempit — daftar yang menembus
    // tepi bawah jendela adalah daftar yang separuhnya tidak bisa dilihat.
    const below = window.innerHeight - r.bottom - 12;
    const above = r.top - 12;
    const down = below >= Math.min(240, above);
    const maxH = Math.max(120, Math.min(280, down ? below : above));
    setPos({
      top: down ? r.bottom + 4 : Math.max(8, r.top - maxH - 4),
      left: Math.max(8, Math.min(r.left, window.innerWidth - r.width - 8)),
      width: Math.max(r.width, Math.min(280, window.innerWidth - 32)),
      maxH,
    });
  }, []);

  const openList = () => {
    if (disabled) return;
    place();
    setCursor(Math.max(0, options.findIndex((o) => o.value === value)));
    setOpen(true);
  };

  useEffect(() => {
    if (!open) return;
    const away = (e: MouseEvent) => {
      const el = e.target as HTMLElement;
      if (!el.closest(".sel-list") && !el.closest(".sel-anchor")) setOpen(false);
    };
    const scroll = () => setOpen(false);
    window.addEventListener("mousedown", away);
    window.addEventListener("resize", place);
    window.addEventListener("scroll", scroll, true);
    return () => {
      window.removeEventListener("mousedown", away);
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", scroll, true);
    };
  }, [open, place]);

  // Baris terpilih digulir ke dalam pandangan begitu daftarnya terbuka —
  // tanpa ini, daftar panjang (model Ollama, durasi klip) selalu terbuka di
  // bagian atas dan pilihan yang sedang berlaku tidak terlihat.
  useLayoutEffect(() => {
    if (!open) return;
    list.current?.querySelector<HTMLElement>('[data-on="1"]')
      ?.scrollIntoView({ block: "nearest" });
  }, [open, cursor]);

  const pick = (o: Option) => { if (!o.disabled) { onChange(o.value); setOpen(false); btn.current?.focus(); } };

  const onKey = (e: React.KeyboardEvent) => {
    if (!open) {
      if (["Enter", " ", "ArrowDown", "ArrowUp"].includes(e.key)) { e.preventDefault(); openList(); }
      return;
    }
    const step = (d: number) => {
      e.preventDefault();
      setCursor((c) => {
        let n = c;
        for (let i = 0; i < options.length; i++) {
          n = (n + d + options.length) % options.length;
          if (!options[n].disabled) break;
        }
        return n;
      });
    };
    if (e.key === "ArrowDown") step(1);
    else if (e.key === "ArrowUp") step(-1);
    else if (e.key === "Home") { e.preventDefault(); setCursor(0); }
    else if (e.key === "End") { e.preventDefault(); setCursor(options.length - 1); }
    else if (e.key === "Enter" || e.key === " ") { e.preventDefault(); if (options[cursor]) pick(options[cursor]); }
    else if (e.key === "Escape") { e.preventDefault(); setOpen(false); }
    else if (e.key === "Tab") setOpen(false);
  };

  return (
    <div className="sel-anchor">
      <button ref={btn} type="button" id={id} disabled={disabled} title={title}
        aria-label={ariaLabel} aria-haspopup="listbox" aria-expanded={open}
        className={"sel" + (open ? " open" : "")}
        onClick={() => (open ? setOpen(false) : openList())} onKeyDown={onKey}>
        <span className="sel-value">{current?.label ?? ""}</span>
        <ChevronDown className="ico sel-chev" aria-hidden="true" />
      </button>

      {open && pos && (
        <div className="sel-list" role="listbox" ref={list}
          style={{ top: pos.top, left: pos.left, width: pos.width, maxHeight: pos.maxH }}>
          {options.map((o, i) => (
            <button
              key={o.value}
              type="button"
              role="option"
              aria-selected={o.value === value}
              data-on={o.value === value ? "1" : undefined}
              className={"sel-opt" + (i === cursor ? " cursor" : "") + (o.disabled ? " off" : "")}
              onMouseEnter={() => setCursor(i)}
              onClick={() => pick(o)}
            >
              <Check className="ico sel-tick" aria-hidden="true" />
              {/* Nama di baris pertama, keterangan di baris KEDUA. Sebaris
                  berdampingan, keterangan panjang seperti "8.0B · 4.9 GB · siap"
                  mendesak nama modelnya sampai hilang sama sekali — terlihat
                  pada potret pertama. */}
              <span className="sel-opt-text">
                <span className="sel-opt-label">{o.label}</span>
                {o.note && <span className="sel-note">{o.note}</span>}
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

"use client";

import { useState } from "react";
import { ChevronDown } from "lucide-react";

// Kelompok setelan yang bisa diciutkan.
//
// Dipakai di kolom artikel halaman berita, dan alasannya bukan kerapian:
// isi kolom itu MUNCUL DAN HILANG mengikuti keadaan — kolom tautan hilang saat
// pindah ke jelajah, daftar paragraf muncul sesudah dianalisis, seluruh blok
// Isi baru ada setelah artikelnya termuat. Tiap kali itu terjadi, kelompok di
// bawahnya melompat, dan yang sedang dibaca pengguna pindah tempat.
//
// Menciutkan yang sudah selesai membuat kolomnya diam: sesudah artikel dipilih,
// bagian Artikel tidak perlu terbuka lagi.
//
// Judulnya <button>, bukan <div> dengan onClick: ia memang tombol, dan hanya
// dengan begitu ia bisa dijangkau Tab dan dibaca pembaca layar.
export default function Section({
  title, defaultOpen = true, children,
}: {
  title: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div className="group">
      <button type="button" className="group-title group-toggle"
        aria-expanded={open} onClick={() => setOpen((v) => !v)}>
        <span>{title}</span>
        <ChevronDown className={"ico chev" + (open ? " open" : "")} aria-hidden="true" />
      </button>
      {open && children}
    </div>
  );
}

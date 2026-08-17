"use client";

import { useI18n } from "./i18n";

// Tombol Mulai/Batal + kemajuan job. Berdiri paling bawah di kolom setelan:
// urutan tombolnya mengikuti urutan keputusannya, jadi ia tidak bisa ditekan
// sebelum sumber dan mesin AI di atasnya diisi.
//
// Bilah kemajuan pindah KE SINI dari `.screen-head` (6 Agustus 2026). Di kepala
// halaman ia hanya dirender saat ada job, jadi memulai job mendorong seluruh
// halaman turun dan menyelesaikannya menariknya kembali. Di sini tempatnya
// SELALU ADA — kosong saat diam — dan tidak ada yang bergerak sama sekali.
//
// Panel ini TIDAK punya baris teks keadaan. Dulu ada, dan ia salah tiga kali
// sekaligus: saat diam ia menulis "Not running" — keterangan yang mengulang
// bilah kosong dan tombol bertuliskan Start; saat berjalan ia menulis kalimat
// tahap yang HURUF DEMI HURUF sama dengan yang sudah masuk kotak log; dan
// kalimat itu (alamat Ollama + keterangan WSL + nomor bagian) jauh lebih
// panjang daripada lebar panelnya, sehingga ia menembus keluar kotaknya.
// Kemajuan sudah terbaca dari bilahnya, rinciannya dari kotak log.
export default function RunPanel({
  busy, testing, disabled, cancellable, onStart, onCancel, progress,
}: {
  busy: boolean;
  /** Uji LLM sedang berjalan — tombolnya mati DAN mengatakan alasannya. */
  testing: boolean;
  disabled: boolean;
  cancellable: boolean;
  onStart: () => void;
  onCancel: () => void;
  progress: number;
}) {
  const { t } = useI18n();

  const pct = Math.round(progress * 100);
  return (
    <div className="panel start-panel">
      <div className="run-progress">
        <div className="progress-outer"><div className="progress-inner" style={{ width: `${pct}%` }} /></div>
      </div>
      {/* Tombol mati saja tidak cukup: tombol yang kelabu tanpa sebab terbaca
          sebagai aplikasi macet. Selama uji LLM ia menyebutkan apa yang
          ditunggu. */}
      <button onClick={onStart} disabled={disabled}>
        {testing ? t("llmTesting") : busy ? t("processing") : t("start")}
      </button>
      {/* Selalu dirender, dimatikan saat tidak ada yang bisa dibatalkan: tombol
          yang muncul-hilang menggeser panel di bawah kursor tepat ketika job
          mulai berjalan. */}
      <button className="ghost" onClick={onCancel} disabled={!cancellable}>{t("cancel")}</button>
    </div>
  );
}

"use client";

import { useEffect, useRef } from "react";
import { useI18n } from "./i18n";

// Kotak log proses. SELALU ada, juga saat masih kosong (6 Agustus 2026): dulu
// ia hanya muncul setelah baris pertama masuk, jadi sebelum job dijalankan
// halaman ini tidak punya tempat yang jelas untuk memantau jalannya pekerjaan —
// dan kotak yang tiba-tiba menyembul juga menggeser seluruh kolom.
//
// Inilah satu-satunya kotak di kolom ini yang memang BOLEH bergulir sendiri,
// dan itu bukan pelanggaran melainkan cara aplikasi desktop bekerja: yang haram
// adalah harus menggulir HALAMAN untuk menemukan tombol (notes/29).
//
// Gulir-ke-bawah-otomatis tinggal di sini, bukan di halaman: hanya komponen ini
// yang tahu kapan barisnya bertambah, dan hanya ia yang punya elemennya.
//
// Barisnya tetap string, bukan komponen — itu sebabnya lambangnya memakai
// ✓ ↓ ↑ ↻ yang ada di font mana pun, bukan emoji (notes/29).
export default function LogPanel({ logs }: { logs: string[] }) {
  const { t } = useI18n();
  const boxRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => { boxRef.current?.scrollTo(0, boxRef.current.scrollHeight); }, [logs]);

  return (
    <div className="panel log-panel">
      <div className="group-title">{t("log")}</div>
      <div className="logbox" ref={boxRef}>
        {logs.length
          ? logs.map((l, i) => <div key={i}>{l}</div>)
          : <div className="meta">{t("logEmpty")}</div>}
      </div>
    </div>
  );
}

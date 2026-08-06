"use client";

import { CircleCheck, OctagonX } from "lucide-react";
import { useI18n } from "./i18n";

// Tombol Mulai/Batal + kemajuan job. Berdiri paling bawah di kolom setelan:
// urutan tombolnya mengikuti urutan keputusannya, jadi ia tidak bisa ditekan
// sebelum sumber dan mesin AI di atasnya diisi.
//
// Bilah kemajuan pindah KE SINI dari `.screen-head` (6 Agustus 2026). Di kepala
// halaman ia hanya dirender saat ada job, jadi memulai job mendorong seluruh
// halaman turun dan menyelesaikannya menariknya kembali. Di sini tempatnya
// SELALU ADA — kosong saat diam — dan tidak ada yang bergerak sama sekali.
export default function RunPanel({
  busy, disabled, cancellable, onStart, onCancel, status, stage, message, progress,
}: {
  busy: boolean;
  disabled: boolean;
  cancellable: boolean;
  onStart: () => void;
  onCancel: () => void;
  status: string;
  stage: string;
  message: string;
  progress: number;
}) {
  const { t } = useI18n();

  const pct = Math.round(progress * 100);
  return (
    <div className="panel start-panel">
      <div className="run-progress">
        <div className="progress-outer"><div className="progress-inner" style={{ width: `${pct}%` }} /></div>
        <div className="stage">
          {status === "done" ? <><CircleCheck className="ico" aria-hidden="true" /> {t("statusDone")} (100%)</>
            : status === "error" ? <><OctagonX className="ico" aria-hidden="true" /> {t("statusStopped")}</>
            : !status ? t("statusIdle")
            : `${stage || t("processing")}${message ? ` — ${message}` : ""} (${pct}%)`}
        </div>
      </div>
      <button onClick={onStart} disabled={disabled}>{busy ? t("processing") : t("start")}</button>
      {/* Selalu dirender, dimatikan saat tidak ada yang bisa dibatalkan: tombol
          yang muncul-hilang menggeser panel di bawah kursor tepat ketika job
          mulai berjalan. */}
      <button className="ghost" onClick={onCancel} disabled={!cancellable}>{t("cancel")}</button>
    </div>
  );
}

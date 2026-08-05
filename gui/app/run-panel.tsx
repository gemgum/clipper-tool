"use client";

import { useI18n } from "./i18n";

// Tombol Mulai/Batal. Berdiri paling bawah di kolom setelan: urutan tombolnya
// mengikuti urutan keputusannya, jadi ia tidak bisa ditekan sebelum sumber dan
// mesin AI di atasnya diisi.
//
// Bilah kemajuan TIDAK di sini melainkan di .screen-head — di kerangka berzona
// kepala tidak ikut bergulir, jadi kemajuan job tetap terlihat berapa pun
// panjang kolomnya (notes/29).
export default function RunPanel({
  busy, disabled, cancellable, onStart, onCancel,
}: {
  busy: boolean;
  disabled: boolean;
  cancellable: boolean;
  onStart: () => void;
  onCancel: () => void;
}) {
  const { t } = useI18n();

  return (
    <div className="panel start-panel">
      <button onClick={onStart} disabled={disabled}>{busy ? t("processing") : t("start")}</button>
      {cancellable && <button className="ghost" onClick={onCancel}>{t("cancel")}</button>}
    </div>
  );
}

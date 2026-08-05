"use client";

import { useState } from "react";
import { useI18n } from "./i18n";

// Dari mana videonya dan ke mana hasilnya — satu baris di kepala panel
// pratinjau, sebab keduanya menentukan APA yang muncul di sana.
//
// Kotak seret-lepas setinggi ~110 px DIBUANG (6 Agustus 2026). Ia mengulang apa
// yang sudah bisa dilakukan dua kolom di bawahnya, dan tingginya persis yang
// membuat halaman ini harus digulir di jendela terkecil. Menyeret berkas tetap
// jalan: SELURUH baris ini yang jadi sasaran lepasnya.
//
// Berkas yang dilepas TIDAK diunggah lebih dulu: onFile menanyakannya ke engine
// (/api/locate) yang jalan di mesin yang sama, dan unggahan hanya cadangan bila
// engine tak menemukannya (notes/24).
export default function SourceRow({
  path, setPath, outputDir, setOutputDir, onPick, onFile, uploading, uploadPct,
}: {
  path: string;
  setPath: (v: string) => void;
  outputDir: string;
  setOutputDir: (v: string) => void;
  onPick: (which: "video" | "out") => void;
  onFile: (f: File) => void;
  uploading: boolean;
  uploadPct: number;
}) {
  const { t } = useI18n();
  const [dragOver, setDragOver] = useState(false);

  return (
    <div className={`source-row ${dragOver ? "over" : ""}`}
      onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
      onDragLeave={() => setDragOver(false)}
      onDrop={(e) => { e.preventDefault(); setDragOver(false); const f = e.dataTransfer.files?.[0]; if (f) onFile(f); }}>
      <div className="field">
        <label>{t("videoPath")}</label>
        <div className="path-row">
          <input value={path} onChange={(e) => setPath(e.target.value)} placeholder="/home/user/video.mp4" />
          <button className="ghost" onClick={() => onPick("video")}>{t("pickerGo")}…</button>
        </div>
      </div>
      <div className="field">
        <label>{t("outputDir")}</label>
        <div className="path-row">
          <input value={outputDir} onChange={(e) => setOutputDir(e.target.value)} placeholder={t("outputDirPlaceholder")} />
          <button className="ghost" onClick={() => onPick("out")}>{t("pickerGo")}…</button>
        </div>
      </div>
      {/* Bilah unggah hanya ada saat benar-benar mengunggah — dan itu jalur
          cadangan yang jarang terpakai, sebab berkas lokal dibaca di tempat. */}
      {uploading && (
        <div className="upload-bar">
          <div className="progress-outer"><div className="progress-inner" style={{ width: `${uploadPct * 100}%` }} /></div>
          <span className="meta">{t("uploadingPct", { pct: Math.round(uploadPct * 100) })}</span>
        </div>
      )}
    </div>
  );
}

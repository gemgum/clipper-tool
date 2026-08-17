"use client";

import { FolderOpen } from "lucide-react";
import { useState } from "react";
import { eng } from "./engine";
import { useI18n } from "./i18n";

// Satu kartu klip. Dipakai halaman Clips (hasil job terakhir) DAN halaman
// History (job lama) — bentuk yang sama di dua tempat, jadi satu berkas.

export type Clip = {
  id: string; job_id: string; start: number; end: number; duration: number;
  score: number; title?: string; hashtags?: string[];
  reasons: { hook: number; emotion: number; clarity: number; shareability: number; standalone: number };
  video_path?: string; video_path_raw?: string; subtitle_srt?: string; transcript_txt?: string;
};

export const scoreClass = (n: number) => (n >= 75 ? "high" : n >= 50 ? "mid" : "low");
export const formatTime = (s: number) => {
  const m = Math.floor(s / 60);
  const r = Math.round(s % 60);
  return `${m}:${String(r).padStart(2, "0")}`;
};

export default function ClipCard({ c }: { c: Clip }) {
  const { t } = useI18n();
  const [failed, setFailed] = useState("");
  const file = (variant?: string) =>
    eng(`/api/jobs/${c.job_id}/clips/${c.id}/file${variant ? `?variant=${variant}` : ""}`);

  // Berkasnya SUDAH ada di disk — di folder keluaran yang dipilih pengguna
  // sendiri. Menekan tombol ini membuka foldernya dengan berkas itu tersorot;
  // tidak ada yang diunduh, tidak ada salinan kedua, dan keempat berkas (video
  // bersubtitle, video bersih, .srt, .txt) terlihat sekaligus sebab memang
  // sudah bersebelahan. Alasan lengkapnya di engine/internal/api/reveal.go.
  const openFolder = async () => {
    setFailed("");
    try {
      const res = await fetch(eng(`/api/jobs/${c.job_id}/clips/${c.id}/reveal`), {
        method: "POST", headers: { "content-type": "application/json" }, body: "{}",
      });
      if (!res.ok) setFailed((await res.json()).error || t("revealFailed"));
    } catch (e: any) { setFailed(e.message); }
  };

  return (
    <div className="clip">
      <video src={file()} controls preload="metadata" />
      <div className="body">
        <span className={`score ${scoreClass(c.score)}`}>{c.score}</span><span className="meta"> /100</span>
        <div className="title">{c.title || t("noTitle")}</div>
        <div className="meta">{formatTime(c.start)}–{formatTime(c.end)} · {Math.round(c.duration)}s</div>
        <div className="reasons">
          {t("reasonHook")} {c.reasons.hook} · {t("reasonEmotion")} {c.reasons.emotion} · {t("reasonClarity")} {c.reasons.clarity} · {t("reasonShare")} {c.reasons.shareability} · {t("reasonStandalone")} {c.reasons.standalone}
        </div>
        {c.hashtags?.map((h) => <span className="tag" key={h}>{h}</span>)}
        <div style={{ marginTop: 8, display: "flex", gap: 8, flexWrap: "wrap" }}>
          <button className="dl" onClick={openFolder} title={t("revealTip")}>
            <FolderOpen className="ico" aria-hidden="true" /> {t("reveal")}
          </button>
          {failed && <span className="err">{failed}</span>}
        </div>
      </div>
    </div>
  );
}

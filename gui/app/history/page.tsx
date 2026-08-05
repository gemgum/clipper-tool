"use client";

import { useEffect, useState } from "react";
import { eng, engineURL } from "../engine";
import { useI18n } from "../i18n";
import ClipCard, { type Clip } from "../clip-card";

// Riwayat keluaran: semua job yang pernah dijalankan, terbaru di atas.
//
// Job dibuka satu per satu, bukan semuanya sekaligus: satu job bisa berisi
// sepuluh video, dan memuat semuanya berarti sepuluh kali sepuluh <video> yang
// diminta browser sebelum pengguna melihat satu pun.

type Job = {
  id: string; status: string; created_at: string; source?: string; clips?: Clip[];
};

export default function HistoryPage() {
  const { t, lang } = useI18n();
  const [jobs, setJobs] = useState<Job[] | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    fetch(eng(`/api/jobs`))
      .then((r) => r.json())
      .then((d: Job[]) => setJobs(Array.isArray(d) ? d.slice().sort((a, b) => (a.created_at < b.created_at ? 1 : -1)) : []))
      .catch(() => { setError(t("engineUnreachable", { url: engineURL() })); setJobs([]); });
  }, [t]);

  const when = (iso: string) => {
    try { return new Date(iso).toLocaleString(lang === "id" ? "id-ID" : "en-GB"); } catch { return iso; }
  };

  return (
    <div className="screen">
      <div className="screen-head">
        <h1>{t("tabHistory")}</h1>
        <p className="sub">{t("historySub")}</p>
        {error && <div className="err">{error}</div>}
      </div>
      <div className="screen-body one">
        <div className="screen-main">
          {jobs === null ? (
            <div className="meta">{t("loading")}</div>
          ) : jobs.length === 0 ? (
            <div className="panel"><div className="meta">{t("historyEmpty")}</div></div>
          ) : (
            jobs.map((j) => {
              const clips = (j.clips || []).slice().sort((a, b) => b.score - a.score);
              const isOpen = open === j.id;
              return (
                <div className="panel" key={j.id}>
                  <div className="req-row">
                    <div className={`req-dot ${j.status === "done" ? "on" : j.status === "error" ? "off" : "idle"}`} />
                    <div className="req-main">
                      <div className="req-name">{when(j.created_at)}</div>
                      <div className="req-path" title={j.source}>{j.source || "—"}</div>
                      <div className="meta">{t("clipCount", { n: clips.length })} · {j.status}</div>
                    </div>
                    <div className="req-actions">
                      <button className="ghost" disabled={clips.length === 0}
                        onClick={() => setOpen(isOpen ? null : j.id)}>
                        {isOpen ? t("historyHide") : t("historyShow")}
                      </button>
                    </div>
                  </div>
                  {isOpen && <div className="clips" style={{ marginTop: 12 }}>{clips.map((c) => <ClipCard c={c} key={c.id} />)}</div>}
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}

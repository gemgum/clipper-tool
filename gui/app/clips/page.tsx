"use client";

import { useEffect, useState } from "react";
import { eng, engineURL } from "../engine";
import { useI18n } from "../i18n";
import ClipCard, { type Clip } from "../clip-card";
import Alerts from "../alerts";

// Hasil job TERAKHIR, halaman sendiri.
//
// Dulu daftar ini menempel di bawah halaman klip, jadi ia baru terlihat setelah
// menggulir melewati seluruh setelan — padahal ia yang dicari begitu pekerjaan
// selesai. Riwayat job lama ada di halaman History.

type Job = { id: string; status: string; created_at: string; source?: string; clips?: Clip[] };

export default function ClipsPage() {
  const { t } = useI18n();
  const [job, setJob] = useState<Job | null>(null);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    fetch(eng(`/api/jobs`))
      .then((r) => r.json())
      .then((jobs: Job[]) => {
        if (!Array.isArray(jobs)) return;
        const latest = jobs
          .slice()
          .sort((a, b) => (a.created_at < b.created_at ? 1 : -1))[0];
        setJob(latest || null);
      })
      .catch(() => setError(t("engineUnreachable", { url: engineURL() })))
      .finally(() => setBusy(false));
  }, [t]);

  const clips = (job?.clips || []).slice().sort((a, b) => b.score - a.score);

  return (
    <div className="screen">
      <Alerts items={[error && { kind: "error" as const, text: error }]} />
      <div className="screen-body one">
        <div className="screen-main">
          {busy ? (
            <div className="meta">{t("loading")}</div>
          ) : clips.length === 0 ? (
            <div className="panel"><div className="meta">{t("resultsEmpty")}</div></div>
          ) : (
            <div className="panel">
              <h3 style={{ marginTop: 0 }}>{t("clipCount", { n: clips.length })}</h3>
              <div className="clips">{clips.map((c) => <ClipCard c={c} key={c.id} />)}</div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

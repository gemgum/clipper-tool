"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { eng } from "./engine";
import { useI18n } from "./i18n";
import { useAISettings } from "./ai";
import { useEngines } from "./engine-picker";

// Status di DASAR rail (keputusan pemilik 9 Oktober 2026, pengganti strip
// identitas 38 px DESIGN-Clipper-Lanjutan §2): pekerjaan yang sedang jalan,
// kalau tidak ada, keadaan mesin AI global. Titik selalu berpasangan dengan
// kata (§9 StatusDot) — titik saja tidak dibaca pembaca layar maupun orang
// buta warna. Mengkliknya membuka tempat yang bisa menindaklanjuti.
export default function RailStatus() {
  const { t } = useI18n();
  const { data } = useAISettings();
  const { engines } = useEngines();
  const [job, setJob] = useState<{ progress: number } | null>(null);

  useEffect(() => {
    let alive = true;
    const poll = () => fetch(eng("/api/jobs")).then((r) => r.json()).then((jobs) => {
      if (!alive || !Array.isArray(jobs)) return;
      const run = jobs.find((j) => j.status === "running" || j.status === "queued");
      setJob(run ? { progress: run.progress || 0 } : null);
    }).catch(() => {});
    poll();
    const id = setInterval(poll, 5000);
    return () => { alive = false; clearInterval(id); };
  }, []);

  if (job) {
    return (
      <Link href="/history" className="rail-status" aria-live="polite">
        <span className="rail-dot run" aria-hidden="true" />
        <span>{t("railJobRunning", { pct: Math.round(job.progress * 100) })}</span>
      </Link>
    );
  }
  const g = data?.global;
  if (!g) return null;
  const info = engines.find((e) => e.id === g.engine);
  const ready = g.engine === "heuristic" || !!info?.ready;
  return (
    <Link href="/requirements" className="rail-status" title={info?.name || g.engine}>
      <span className={"rail-dot " + (ready ? "ok" : "bad")} aria-hidden="true" />
      <span>{ready ? t("railAIReady") : t("railAINotReady")}</span>
      {g.model && <code>{g.model}</code>}
    </Link>
  );
}

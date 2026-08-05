"use client";

import { useCallback, useEffect, useState } from "react";
import { CheckSquare, Square, Trash2, Download } from "lucide-react";
import { eng, engineURL } from "../engine";
import { useI18n } from "../i18n";
import ClipCard, { type Clip } from "../clip-card";
import DragRow from "../drag-row";

// Riwayat keluaran: tiap job satu baris, klipnya berjajar MENYAMPING.
//
// Menyamping, bukan menumpuk ke bawah: satu job bisa berisi sepuluh klip, dan
// sepuluh kartu bertumpuk berarti job kedua sudah di luar layar sebelum job
// pertama selesai dibaca. Deretan mendatar menjaga tiap job tetap satu baris —
// dan barisnya yang digulir, bukan halamannya.

type Job = {
  id: string; status: string; created_at: string; source?: string; clips?: Clip[];
};
type Card = {
  id: string; made: string; bytes: number; file: string; zip: string; caption?: string;
};

export default function HistoryPage() {
  const { t, lang } = useI18n();
  const [jobs, setJobs] = useState<Job[] | null>(null);
  const [error, setError] = useState("");
  // Kunci pilihan: "<jobId>/<clipId>". Satu himpunan untuk seluruh halaman,
  // jadi "hapus terpilih" bisa memotong beberapa job sekaligus.
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);
  // Dua jenis keluaran, satu halaman. Dipisah dengan tombol berdampingan, bukan
  // dua halaman rail: keduanya jawaban atas pertanyaan yang sama — "apa yang
  // sudah pernah saya buat" — dan memisahkannya berarti mencari di dua tempat.
  const [tab, setTab] = useState<"clips" | "cards">("clips");
  const [cards, setCards] = useState<Card[] | null>(null);

  const load = useCallback(() => {
    fetch(eng(`/api/jobs`))
      .then((r) => r.json())
      .then((d: Job[]) => setJobs(Array.isArray(d)
        ? d.slice().sort((a, b) => (a.created_at < b.created_at ? 1 : -1)) : []))
      .catch(() => { setError(t("engineUnreachable", { url: engineURL() })); setJobs([]); });
  }, [t]);

  const loadCards = useCallback(() => {
    fetch(eng(`/api/cards`))
      .then((r) => r.json())
      .then((d: Card[]) => setCards(Array.isArray(d) ? d : []))
      .catch(() => setCards([]));
  }, []);

  useEffect(load, [load]);
  useEffect(loadCards, [loadCards]);

  const when = (iso: string) => {
    try { return new Date(iso).toLocaleString(lang === "id" ? "id-ID" : "en-GB"); } catch { return iso; }
  };

  const key = (jobId: string, clipId: string) => `${jobId}/${clipId}`;
  const toggle = (k: string) => setPicked((p) => {
    const n = new Set(p);
    if (n.has(k)) n.delete(k); else n.add(k);
    return n;
  });

  // Pilih-semua bekerja PER JOB dan bersifat toggle: kalau job itu sudah
  // terpilih seluruhnya, menekannya melepas semuanya. Satu tombol, dua arah —
  // tombol "lepas semua" terpisah cuma menambah yang harus dicari.
  const toggleJob = (j: Job) => {
    const ks = (j.clips || []).map((c) => key(j.id, c.id));
    const allOn = ks.length > 0 && ks.every((k) => picked.has(k));
    setPicked((p) => {
      const n = new Set(p);
      ks.forEach((k) => (allOn ? n.delete(k) : n.add(k)));
      return n;
    });
  };

  // Penghapusan dijalankan BERURUTAN, bukan serentak: tiap permintaan membuang
  // berkas di disk, dan sepuluh penghapusan sekaligus tidak lebih cepat — hanya
  // lebih sulit dilaporkan kalau salah satunya gagal.
  const removePicked = async () => {
    if (picked.size === 0 || !confirm(t("historyConfirm", { n: picked.size }))) return;
    setBusy(true);
    setError("");
    for (const k of picked) {
      // Kartu dikenali dari awalan "card/"; klip memakai "<jobId>/<clipId>".
      const url = k.startsWith("card/")
        ? `/api/cards/${k.slice(5)}`
        : `/api/jobs/${k.split("/")[0]}/clips/${k.split("/")[1]}`;
      try {
        const res = await fetch(eng(url), { method: "DELETE" });
        if (!res.ok) setError((await res.json()).error || t("historyDeleteFailed"));
      } catch (e: any) { setError(e.message); }
    }
    setPicked(new Set());
    setBusy(false);
    load();
    loadCards();
  };

  const size = (b: number) => (b > 1e6 ? `${(b / 1e6).toFixed(1)} MB` : `${Math.round(b / 1e3)} kB`);

  return (
    <div className="screen">
      {error && <div className="screen-head"><div className="err">{error}</div></div>}

      <div className="screen-body one">
        <div className="screen-main">
          {/* Bilah aksi hanya muncul saat ada yang dipilih — tombol hapus yang
              selalu terlihat tapi mati sepanjang waktu cuma kebisingan. */}
          {picked.size > 0 && (
            <div className="panel pick-bar">
              <span>{t("historyPicked", { n: picked.size })}</span>
              <button className="ghost tiny" onClick={() => setPicked(new Set())}>{t("historyClearPick")}</button>
              <button className="danger" disabled={busy} onClick={removePicked}>
                <Trash2 className="ico" aria-hidden="true" /> {busy ? t("historyDeleting") : t("historyDelete")}
              </button>
            </div>
          )}

          <div className="panel seg-panel">
            <div className="seg">
              <button className={tab === "clips" ? "active" : ""} onClick={() => setTab("clips")}>
                {t("tabResults")}{jobs ? ` (${jobs.length})` : ""}
              </button>
              <button className={tab === "cards" ? "active" : ""} onClick={() => setTab("cards")}>
                {t("tabNews")}{cards ? ` (${cards.length})` : ""}
              </button>
            </div>
          </div>

          {tab === "cards" ? (
            cards === null ? (
              <div className="meta">{t("loading")}</div>
            ) : cards.length === 0 ? (
              <div className="panel"><div className="meta">{t("cardHistoryEmpty")}</div></div>
            ) : (
              <div className="panel">
                <div className="run-head">
                  <div className="run-when">
                    <div className="run-title">{t("cardHistory")}</div>
                  </div>
                  <span className="meta">{cards.length}</span>
                  <button className="ghost tiny" onClick={() => {
                    const ks = cards.map((c) => `card/${c.id}`);
                    const allOn = ks.every((k) => picked.has(k));
                    setPicked((p) => {
                      const n = new Set(p);
                      ks.forEach((k) => (allOn ? n.delete(k) : n.add(k)));
                      return n;
                    });
                  }}>
                    {cards.every((c) => picked.has(`card/${c.id}`))
                      ? <><CheckSquare className="ico" aria-hidden="true" /> {t("historyNone")}</>
                      : <><Square className="ico" aria-hidden="true" /> {t("historyAll")}</>}
                  </button>
                </div>
                <DragRow>
                  {cards.map((c) => {
                    const k = `card/${c.id}`;
                    const on = picked.has(k);
                    return (
                      <div className={"strip-item card-strip" + (on ? " on" : "")} key={c.id}>
                        <button className="strip-pick" aria-pressed={on}
                          aria-label={t(on ? "historyDeselect" : "historySelect")}
                          onClick={() => toggle(k)}>
                          {on ? <CheckSquare className="ico" aria-hidden="true" /> : <Square className="ico" aria-hidden="true" />}
                        </button>
                        <div className="clip">
                          {/* eslint-disable-next-line @next/next/no-img-element */}
                          <img src={eng(c.file)} alt="" loading="lazy" />
                          <div className="body">
                            <div className="meta">{when(c.made)} · {size(c.bytes)}</div>
                            {c.caption && <div className="card-cap" title={c.caption}>{c.caption}</div>}
                            <p><a className="dl" href={eng(c.zip)}>
                              <Download className="ico" aria-hidden="true" /> {t("downloadZip")}
                            </a></p>
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </DragRow>
              </div>
            )
          ) : jobs === null ? (
            <div className="meta">{t("loading")}</div>
          ) : jobs.length === 0 ? (
            <div className="panel"><div className="meta">{t("historyEmpty")}</div></div>
          ) : (
            jobs.map((j) => {
              const clips = (j.clips || []).slice().sort((a, b) => b.score - a.score);
              const ks = clips.map((c) => key(j.id, c.id));
              const allOn = ks.length > 0 && ks.every((k) => picked.has(k));
              return (
                <div className="panel" key={j.id}>
                  <div className="run-head">
                    <div className={`req-dot ${j.status === "done" ? "on" : j.status === "error" ? "off" : "idle"}`} />
                    <div className="run-when">
                      {/* Tanggal & waktu jadi JUDUL barisnya: itu yang dicari
                          orang saat menelusuri riwayat, bukan nama berkasnya. */}
                      <div className="run-title">{when(j.created_at)}</div>
                      <div className="req-path" title={j.source}>{j.source || "—"}</div>
                    </div>
                    <span className="meta">{t("clipCount", { n: clips.length })} · {j.status}</span>
                    {clips.length > 0 && (
                      <button className="ghost tiny" onClick={() => toggleJob(j)}>
                        {allOn ? <CheckSquare className="ico" aria-hidden="true" /> : <Square className="ico" aria-hidden="true" />}
                        {allOn ? t("historyNone") : t("historyAll")}
                      </button>
                    )}
                  </div>

                  {clips.length > 0 && (
                    <DragRow>
                      {clips.map((c) => {
                        const k = key(j.id, c.id);
                        const on = picked.has(k);
                        return (
                          <div className={"strip-item" + (on ? " on" : "")} key={c.id}>
                            <button className="strip-pick" aria-pressed={on}
                              aria-label={t(on ? "historyDeselect" : "historySelect")}
                              onClick={() => toggle(k)}>
                              {on ? <CheckSquare className="ico" aria-hidden="true" /> : <Square className="ico" aria-hidden="true" />}
                            </button>
                            <ClipCard c={c} />
                          </div>
                        );
                      })}
                    </DragRow>
                  )}
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}

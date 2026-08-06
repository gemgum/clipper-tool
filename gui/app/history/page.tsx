"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CheckSquare, Square, Trash2, Download } from "lucide-react";
import { eng, engineURL } from "../engine";
import Alerts from "../alerts";
import { useI18n } from "../i18n";
import ClipCard, { type Clip } from "../clip-card";
import DragRow from "../drag-row";

// Riwayat keluaran.
//
// KLIP berjajar menyamping per job: satu job bisa berisi sepuluh klip, dan
// sepuluh kartu bertumpuk berarti job kedua sudah di luar layar sebelum job
// pertama selesai dibaca. KARTU BERITA mengalir ke bawah dalam kisi — jumlahnya
// tidak dikelompokkan apa pun, jadi satu baris mendatar cuma menyisakan panel
// kosong selebar layar (terlihat di potret 6 Agustus 2026).
//
// Memilih: klik = pilih ini saja, Ctrl/Cmd+klik = tambah/lepas, Shift+klik =
// rentang. Sama seperti pemilih berkas mana pun — itu yang tangan orang sudah
// tahu tanpa diberi tahu.

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
  // Kunci pilihan: "<jobId>/<clipId>" untuk klip, "card/<id>" untuk kartu.
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);
  const [tab, setTab] = useState<"clips" | "cards">("clips");
  const [cards, setCards] = useState<Card[] | null>(null);
  // Titik jangkar untuk Shift+klik.
  const lastPick = useRef<string>("");

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

  // Urutan tampil semua yang bisa dipilih di tab yang sedang terbuka —
  // dibutuhkan Shift+klik, yang artinya "semua yang ADA DI ANTARA keduanya".
  const order = useMemo(() => (
    tab === "cards"
      ? (cards || []).map((c) => `card/${c.id}`)
      : (jobs || []).flatMap((j) => (j.clips || []).slice()
          .sort((a, b) => b.score - a.score).map((c) => key(j.id, c.id)))
  ), [tab, cards, jobs]);

  // pick menerjemahkan klik + tombol penyertanya jadi pilihan baru.
  const pick = useCallback((k: string, e: { ctrlKey: boolean; metaKey: boolean; shiftKey: boolean }) => {
    setPicked((prev) => {
      if (e.shiftKey && lastPick.current) {
        const a = order.indexOf(lastPick.current), b = order.indexOf(k);
        if (a >= 0 && b >= 0) {
          const next = new Set(prev);
          order.slice(Math.min(a, b), Math.max(a, b) + 1).forEach((x) => next.add(x));
          return next;
        }
      }
      if (e.ctrlKey || e.metaKey) {
        const next = new Set(prev);
        if (next.has(k)) next.delete(k); else next.add(k);
        return next;
      }
      // Klik polos pada satu-satunya yang terpilih = lepas; selain itu pilih
      // hanya yang ini. Tanpa aturan pertama tidak ada cara melepas pilihan
      // terakhir tanpa memakai tombol penyertanya.
      if (prev.size === 1 && prev.has(k)) return new Set();
      return new Set([k]);
    });
    lastPick.current = k;
  }, [order]);

  const toggle = (k: string) => setPicked((p) => {
    const n = new Set(p);
    if (n.has(k)) n.delete(k); else n.add(k);
    lastPick.current = k;
    return n;
  });

  // Pilih-semua bekerja PER KELOMPOK dan bersifat toggle.
  const toggleAll = (keys: string[]) => {
    const allOn = keys.length > 0 && keys.every((k) => picked.has(k));
    setPicked((p) => {
      const n = new Set(p);
      keys.forEach((k) => (allOn ? n.delete(k) : n.add(k)));
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

  // Unduh SATU zip berisi semua yang dicentang (engine: /api/download).
  // Bukan sederet tautan: browser bertanya "izinkan mengunduh banyak berkas?"
  // pada yang kedua, dan WebView2 kadang diam-diam menolak sisanya.
  const downloadURL = useMemo(() => {
    const q = new URLSearchParams();
    picked.forEach((k) => (k.startsWith("card/") ? q.append("card", k.slice(5)) : q.append("clip", k)));
    return eng(`/api/download?${q.toString()}`);
  }, [picked]);

  const size = (b: number) => (b > 1e6 ? `${(b / 1e6).toFixed(1)} MB` : `${Math.round(b / 1e3)} kB`);

  const pickProps = (k: string) => ({
    onClick: (e: React.MouseEvent) => {
      // Tombol & tautan di dalam kartu tetap milik mereka sendiri.
      if ((e.target as HTMLElement).closest("button, a, video, input")) return;
      pick(k, e);
    },
  });

  return (
    <div className="screen">
      <Alerts items={[error && { kind: "error" as const, text: error }]} />

      <div className="screen-body one">
        <div className="screen-main">
          {/* Satu bilah: pemilih jenis keluaran DI KIRI, aksi atas pilihan di
              kanan. Aksinya SELALU dirender (dimatikan saat tidak ada yang
              dipilih) — bilah yang muncul-hilang mendorong seluruh daftar turun
              tepat saat pengguna sedang mengklik isinya. */}
          <div className="panel seg-panel">
            <div className="seg">
              <button className={tab === "clips" ? "active" : ""} onClick={() => setTab("clips")}>
                {t("tabResults")}{jobs ? ` (${jobs.length})` : ""}
              </button>
              <button className={tab === "cards" ? "active" : ""} onClick={() => setTab("cards")}>
                {t("tabNews")}{cards ? ` (${cards.length})` : ""}
              </button>
            </div>
            <span className="meta pick-count">{t("historyPicked", { n: picked.size })}</span>
            <button className="ghost tiny" disabled={picked.size === 0}
              onClick={() => setPicked(new Set())}>{t("historyClearPick")}</button>
            <a className={"btn-dl" + (picked.size === 0 ? " off" : "")}
              href={picked.size === 0 ? undefined : downloadURL}
              aria-disabled={picked.size === 0}>
              <Download className="ico" aria-hidden="true" /> {t("historyDownload")}
            </a>
            <button className="danger" disabled={busy || picked.size === 0} onClick={removePicked}>
              <Trash2 className="ico" aria-hidden="true" /> {busy ? t("historyDeleting") : t("historyDelete")}
            </button>
          </div>

          {tab === "cards" ? (
            cards === null ? (
              <div className="panel"><div className="meta">{t("loading")}</div></div>
            ) : cards.length === 0 ? (
              <div className="panel"><div className="meta">{t("cardHistoryEmpty")}</div></div>
            ) : (
              <div className="panel">
                <div className="run-head">
                  <div className="run-when"><div className="run-title">{t("cardHistory")}</div></div>
                  <span className="meta">{cards.length}</span>
                  <button className="ghost tiny" onClick={() => toggleAll(order)}>
                    {order.every((k) => picked.has(k))
                      ? <><CheckSquare className="ico" aria-hidden="true" /> {t("historyNone")}</>
                      : <><Square className="ico" aria-hidden="true" /> {t("historyAll")}</>}
                  </button>
                </div>
                {/* Kisi yang mengalir ke bawah — kartu berikutnya turun satu
                    baris, bukan hilang di kanan layar. */}
                <div className="card-grid">
                  {cards.map((c) => {
                    const k = `card/${c.id}`;
                    const on = picked.has(k);
                    return (
                      <div className={"card-tile" + (on ? " on" : "")} key={c.id} {...pickProps(k)}>
                        <button className="strip-pick" aria-pressed={on}
                          aria-label={t(on ? "historyDeselect" : "historySelect")}
                          onClick={() => toggle(k)}>
                          {on ? <CheckSquare className="ico" aria-hidden="true" /> : <Square className="ico" aria-hidden="true" />}
                        </button>
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img src={eng(c.file)} alt="" loading="lazy" />
                        <div className="card-tile-foot">
                          <span className="meta">{when(c.made)} · {size(c.bytes)}</span>
                          <a className="dl" href={eng(c.zip)} title={t("downloadZip")} aria-label={t("downloadZip")}>
                            <Download className="ico" aria-hidden="true" />
                          </a>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            )
          ) : jobs === null ? (
            <div className="panel"><div className="meta">{t("loading")}</div></div>
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
                      <button className="ghost tiny" onClick={() => toggleAll(ks)}>
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
                          <div className={"strip-item" + (on ? " on" : "")} key={c.id} {...pickProps(k)}>
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

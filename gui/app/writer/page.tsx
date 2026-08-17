"use client";

// Tab ketiga: penulis artikel (notes/38).
//
// Bentuknya menyalin halaman "/" apa adanya — dua kolom, panel bernama, tanpa
// bilah atas: kiri yang DILIHAT (artikel hasil + log), kanan yang DIISI lalu
// dijalankan (keranjang sumber, mesin AI, tombol Mulai).
//
// Tab sendiri, bukan menumpang /news, karena aturannya berlawanan: di sana LLM
// tidak boleh menulis satu kata pun, di sini ia memang menulis. Mencampurnya
// cepat atau lambat membuat teks karangan keluar sebagai kutipan verbatim.

import { useCallback, useEffect, useRef, useState } from "react";
import { X, Copy, RotateCw, Check, Link2 } from "lucide-react";
import { eng } from "../engine";
import { useI18n } from "../i18n";
import { useCopyLink } from "../copy-link";
import Alerts from "../alerts";
import EnginePicker, { useEngines } from "../engine-picker";
import LogPanel from "../log-panel";
import RunPanel from "../run-panel";

// Batas artikel sumber. Angkanya BUKAN tetap: tahap 2 mengirim seluruh fakta
// dari semua sumber dalam satu panggilan, jadi model berjendela kecil menabrak
// dinding konteks di lima sumber. Engine yang tahu jendelanya (writer.
// MaxSourcesFor), jadi angkanya ditanyakan — dan ditanyakan SEBELUM tombol
// ditekan, bukan dilaporkan sebagai galat sesudah lima menit menunggu.
// Yang di sini hanya nilai awal sampai jawabannya datang.
const MAX_SOURCES = 5;

type Article = { title: string; url: string; source: string; image?: string; date?: string; domain?: string };
type Violation = { kind: string; text: string; detail: string };
type Draft = { title: string; lead: string; body: string[]; words: number; tags?: string[]; violations?: Violation[] };
// SourceRef = sumber yang BENAR-BENAR dipakai job itu, dari engine. Bukan
// keranjang di layar: keranjang masih bisa diubah setelah job jalan, dan kaki
// artikel harus menyebut yang sama persis dengan yang ditulis ke artikel.md.
type SourceRef = { title: string; url: string; media: string };
type PostJob = {
  id: string;
  status: string;
  stage: string;
  progress: number;
  log?: string[];
  error?: string;
  result?: { post: { dir: string; image?: string }; draft: Draft; sources?: SourceRef[] };
};

export default function WriterPage() {
  const { t, lang } = useI18n();

  // --- keranjang sumber ---
  const [basket, setBasket] = useState<Article[]>([]);
  const [paste, setPaste] = useState("");
  const [typed, setTyped] = useState("");
  const [items, setItems] = useState<Article[]>([]);
  const [listBusy, setListBusy] = useState(false);

  // --- mesin ---
  //
  // Kunci API TIDAK ada di sini: ia disetel sekali seumur pemasangan di halaman
  // setelan, sedangkan mesin & model dipilih tiap kali bekerja (notes/39).
  const { engines } = useEngines();
  const [engine, setEngine] = useState("ollama");
  const [model, setModel] = useState("");
  // Mesin tahap MENULIS, terpisah bila diminta. Kedua tahap punya sifat yang
  // berbeda tajam: membaca fakta itu lima panggilan menyalin-ulang yang murah,
  // menulis cuma satu panggilan tapi di situlah mutu model terasa (notes/39).
  const [splitEngines, setSplitEngines] = useState(false);
  const [writeEngine, setWriteEngine] = useState("ollama");
  const [writeModel, setWriteModel] = useState("");

  // --- job ---
  const [jobId, setJobId] = useState("");
  const [job, setJob] = useState<PostJob | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState("");
  const esRef = useRef<EventSource | null>(null);

  const busy = job?.status === "running" || (!!jobId && !job);

  const loadList = useCallback(async (q: string) => {
    setListBusy(true);
    setError("");
    try {
      const url = q
        ? eng(`/api/news/list?max=30&q=${encodeURIComponent(q)}`)
        : eng("/api/news/list?max=30&feed=all");
      const r = await fetch(url);
      if (!r.ok) throw new Error(await r.text());
      setItems((await r.json()) ?? []);
    } catch (e) {
      setError(String(e));
    } finally {
      setListBusy(false);
    }
  }, []);

  useEffect(() => { loadList(""); }, [loadList]);


  // Satu langganan SSE untuk seluruh halaman, dibuka sekali. Kabar job yang
  // sedang berjalan datang dari sini, bukan dari polling: satu job memanggil
  // LLM berkali-kali dan berjalan menit-menitan.
  useEffect(() => {
    const es = new EventSource(eng("/api/posts/events"));
    esRef.current = es;
    es.addEventListener("post", (ev) => {
      const j: PostJob = JSON.parse((ev as MessageEvent).data);
      setJobId((cur) => {
        if (cur && j.id !== cur) return cur;
        setJob(j);
        if (j.log) setLogs(j.log);
        if (j.error) setError(j.error);
        return cur || j.id;
      });
    });
    return () => { es.close(); esRef.current = null; };
  }, []);

  const inBasket = (url: string) => basket.some((a) => a.url === url);

  // Mengklik berita = memasukkannya ke keranjang, dan mengkliknya lagi
  // mengeluarkannya. Tombol "+ Add" tersendiri cuma menyempitkan sasaran klik
  // jadi seukuran tulisan padahal SELURUH baris tidak punya arti lain di tab
  // ini. Tombol kecil di baris itu dipakai untuk yang memang tidak bisa
  // ditebak: menyalin tautannya.
  const toggle = (a: Article) => {
    setBasket((cur) => {
      if (cur.some((x) => x.url === a.url)) return cur.filter((x) => x.url !== a.url);
      return cur.length >= maxSources ? cur : [...cur, a];
    });
  };

  // Batas sumber untuk mesin yang MENULIS — tahap itu yang memuat semua fakta
  // sekaligus. Tanpa mesin tulis terpisah, mesin bacanya yang dipakai.
  const [maxSources, setMaxSources] = useState(MAX_SOURCES);
  useEffect(() => {
    const id = splitEngines ? writeEngine : engine;
    const m = splitEngines ? writeModel : model;
    if (!id) return;
    let alive = true;
    fetch(eng(`/api/posts/limits?engine=${encodeURIComponent(id)}&model=${encodeURIComponent(m)}`))
      .then((r) => r.json())
      .then((d) => { if (alive && d?.max_sources > 0) setMaxSources(d.max_sources); })
      .catch(() => { /* batas bawaan tetap berlaku */ });
    return () => { alive = false; };
  }, [engine, model, splitEngines, writeEngine, writeModel]);

  const { copyLink, copied: copiedLink, busy: copyBusy } = useCopyLink({
    onResolved: (from, to) =>
      setItems((list) => list.map((a) => (a.url === from ? { ...a, url: to } : a))),
    onError: setError,
  });

  // addPasted menerima beberapa alamat sekaligus, satu per baris. Alamat yang
  // ditempel belum punya judul — engine yang membacanya nanti; di sini cukup
  // domainnya supaya barisnya tidak kosong.
  const addPasted = () => {
    const urls = paste.split(/[\s,]+/).map((s) => s.trim()).filter((s) => /^https?:\/\//.test(s));
    setBasket((cur) => {
      const out = [...cur];
      for (const u of urls) {
        if (out.length >= maxSources) break;
        if (out.some((x) => x.url === u)) continue;
        let host = u;
        try { host = new URL(u).hostname.replace(/^www\./, ""); } catch { /* biarkan apa adanya */ }
        out.push({ title: u, url: u, source: host });
      }
      return out;
    });
    setPaste("");
  };

  const start = async () => {
    setError("");
    setLogs([]);
    setJob(null);
    setJobId("");
    try {
      const r = await fetch(eng("/api/posts"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          urls: basket.map((a) => a.url),
          engine,
          model,
          write_engine: splitEngines ? writeEngine : "",
          write_model: splitEngines ? writeModel : "",
          lang,
        }),
      });
      const data = await r.json();
      if (!r.ok) throw new Error(data?.error || data?.message || "failed to start");
      setJobId(data.id);
    } catch (e) {
      setError(String(e));
    }
  };

  // Membatalkan job. Satu job memanggil LLM berkali-kali dan berjalan
  // menit-menitan; tanpa ini satu-satunya cara menghentikan pilihan yang salah
  // adalah menutup aplikasinya.
  const cancel = async () => {
    if (!jobId) return;
    try {
      const r = await fetch(eng(`/api/posts/${jobId}/cancel`), {
        method: "POST", headers: { "Content-Type": "application/json" }, body: "{}",
      });
      if (!r.ok) throw new Error(await r.text());
    } catch (e) {
      setError(String(e));
    }
  };

  const draft = job?.result?.draft;
  const violations = draft?.violations ?? [];
  const used = job?.result?.sources ?? [];

  const copy = async (what: "title" | "body") => {
    if (!draft) return;
    // Yang disalin = artikel LENGKAP: tagar dan kaki sumber ikut, sebab inilah
    // yang ditempel ke media pemilik proyek. Atribusi yang harus diingat sendiri
    // adalah atribusi yang cepat atau lambat lupa ditempel.
    const foot = used.map((s) => `${s.title} — ${s.url}`);
    const text = what === "title"
      ? draft.title
      : [draft.title, "", draft.lead, "", ...draft.body,
         ...(draft.tags?.length ? ["", draft.tags.join(" ")] : []),
         ...(foot.length ? ["", t("writerSources") + ":", ...foot] : [])].join("\n\n");
    try {
      await navigator.clipboard.writeText(text);
      setCopied(what);
      setTimeout(() => setCopied(""), 1500);
    } catch {
      setError(t("errCopy"));
    }
  };

  return (
    <div className="screen">
      <Alerts items={[error && { kind: "error" as const, text: error }]} />

      <div className="screen-body two">
        {/* KIRI: yang dilihat. */}
        <div className="screen-main">
          <div className="panel post-panel">
            <div className="group-title with-action">
              <span>{t("writerArticleTitle")}</span>
              {draft && (
                <span className="post-actions">
                  <button className="ghost tiny" onClick={() => copy("title")}>
                    <Copy className="ico" aria-hidden="true" /> {copied === "title" ? t("copied") : t("writerCopyTitle")}
                  </button>
                  <button className="ghost tiny" onClick={() => copy("body")}>
                    <Copy className="ico" aria-hidden="true" /> {copied === "body" ? t("copied") : t("writerCopyBody")}
                  </button>
                </span>
              )}
            </div>

            {!draft ? (
              <p className="stage">{busy ? t("writerRunning") : t("writerEmpty", { max: maxSources })}</p>
            ) : (
              <div className="post-view">
                {violations.length > 0 && (
                  <div className="warnbox">
                    <strong>{t("writerUnverified", { n: violations.length })}</strong>
                    <div className="meta">{t("writerUnverifiedHint")}</div>
                    <ul>
                      {violations.map((v, i) => (
                        <li key={i}><code>{v.kind}</code> {v.detail}</li>
                      ))}
                    </ul>
                  </div>
                )}
                {job?.result?.post.image && (
                  <img className="post-image" alt="" src={eng(`/api/posts/${job.id}/file?name=image`)} />
                )}
                <h2>{draft.title}</h2>
                <p className="post-lead">{draft.lead}</p>
                {draft.body.map((p, i) => <p key={i}>{p}</p>)}
                {!!draft.tags?.length && <p className="post-tags">{draft.tags.join(" ")}</p>}
                {used.length > 0 && (
                  <div className="post-sources">
                    <div className="meta">{t("writerSources")}</div>
                    <ul>
                      {used.map((s) => (
                        <li key={s.url}>
                          <a href={s.url} target="_blank" rel="noreferrer">{s.title || s.url}</a>
                          {s.media && <span className="meta"> — {s.media}</span>}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
                <div className="meta post-foot">
                  {t("writerWords", { n: draft.words })} · {t("writerFolder", { dir: job!.result!.post.dir })}
                </div>
              </div>
            )}
          </div>

          <LogPanel logs={logs} />
        </div>

        {/* KANAN: yang diisi & dijalankan. */}
        <div className="screen-col">
          {/* Keranjang TIDAK berpanel sendiri.
              Panel terpisah menghabiskan 69 px hanya untuk judul, padding, dan
              jaraknya — dan kolom ini juga memuat daftar berita, mesin AI, serta
              tombol Mulai. Terukur: keranjang berisi 5 sumber mendorong tombol
              Mulai 188 px keluar jendela. Lagipula tempatnya memang di sini:
              isinya persis apa yang baru dicentang dari daftar di bawahnya. */}
          <div className="panel feed-panel">
            <div className="group-title with-action">
              <span>{t("writerBasket", { n: basket.length, max: maxSources })}</span>
              <button className="ghost tiny icon-only" disabled={listBusy}
                title={t("reloadFeeds")} aria-label={t("reloadFeeds")}
                onClick={() => loadList(typed.trim())}>
                <RotateCw className="ico" aria-hidden="true" />
              </button>
            </div>

            {basket.length > 0 && (
              <div className="basket">
                {basket.map((a) => (
                  <div key={a.url} className="basket-item">
                    <span className="basket-text">
                      <span className="news-title">{a.title}</span>
                      <span className="meta">{a.source}</span>
                    </span>
                    <button className="ghost tiny icon-only" title={t("writerRemove")} aria-label={t("writerRemove")}
                      onClick={() => setBasket((cur) => cur.filter((x) => x.url !== a.url))}>
                      <X className="ico" aria-hidden="true" />
                    </button>
                  </div>
                ))}
              </div>
            )}

            {/* Tempel tautan: untuk artikel yang TIDAK ada di feed. Beberapa
                sekaligus, satu per baris. */}
            <div className="path-row">
              <input
                value={paste}
                onChange={(e) => setPaste(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && addPasted()}
                placeholder={t("writerPastePlaceholder")}
              />
              <button onClick={addPasted} disabled={!paste.trim() || basket.length >= maxSources}>
                {t("writerAddLinks")}
              </button>
            </div>

            <div className="search">
              <input
                value={typed}
                onChange={(e) => setTyped(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && loadList(typed.trim())}
                placeholder={t("searchPlaceholder")}
                aria-label={t("search")}
              />
              <button onClick={() => loadList(typed.trim())} disabled={listBusy}>{t("search")}</button>
            </div>

            <div className="news-list">
              {items.map((a) => (
                <div key={a.url}
                  className={"news-item" + (inBasket(a.url) ? " active" : "")}
                  role="button" tabIndex={0}
                  title={inBasket(a.url) ? t("writerRemove") : t("writerAddLinks")}
                  onClick={() => toggle(a)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") { e.preventDefault(); toggle(a); }
                  }}>
                  {a.image && <img src={a.image} alt="" loading="lazy" />}
                  <div className="news-text">
                    <div className="news-title">{a.title}</div>
                    <div className="news-foot">
                      {/* Centang hijau = sudah di keranjang. Penanda, bukan
                          tombol: yang menambah dan mengeluarkan adalah barisnya
                          sendiri. */}
                      {inBasket(a.url) && (
                        <Check className="ico added-mark" aria-label={t("writerAdded")} />
                      )}
                      <span className="meta">{a.source} · {a.date || a.domain}</span>
                      <button
                        className={"copy-btn" + (copiedLink === a.url ? " ok" : "")}
                        title={t("copyLinkTitle")} aria-label={t("copyLinkTitle")}
                        disabled={copyBusy === a.url}
                        onClick={(e) => { e.stopPropagation(); copyLink(a.url); }}>
                        {copyBusy === a.url ? t("copyOpening")
                          : copiedLink === a.url ? t("copied")
                          : <><Link2 className="ico" aria-hidden="true" /> {t("copyLink")}</>}
                      </button>
                    </div>
                  </div>
                </div>
              ))}
              {listBusy && <div className="meta feed-more">{t("loadingNews")}</div>}
            </div>
          </div>

          <div className="panel">
            <div className="group-title">{t("writerEngineTitle")}</div>
            <EnginePicker
              title={splitEngines ? t("writerStageRead") : undefined}
              engines={engines} engine={engine} setEngine={setEngine}
              model={model} setModel={setModel} busy={busy}
            />
            <label className="chk">
              <input type="checkbox" checked={splitEngines} disabled={busy}
                onChange={(e) => setSplitEngines(e.target.checked)} />
              {t("writerSplitEngines")}
            </label>
            {splitEngines && (
              <EnginePicker
                title={t("writerStageWrite")}
                engines={engines} engine={writeEngine} setEngine={setWriteEngine}
                model={writeModel} setModel={setWriteModel} busy={busy}
              />
            )}
          </div>

          <RunPanel
            busy={busy} testing={false}
            disabled={busy || basket.length === 0}
            cancellable={busy && !!jobId}
            onStart={start} onCancel={cancel}
            progress={job?.progress ?? 0}
          />
        </div>
      </div>
    </div>
  );
}

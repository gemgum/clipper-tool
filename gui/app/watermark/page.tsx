"use client";

// Tab keenam: watermark untuk video yang SUDAH jadi.
//
// Halaman klip memotong lalu membakar identitas; halaman ini hanya membakar.
// Yang dipakai orang saat klipnya sudah dipotong di tempat lain — atau saat
// kontesnya menuntut satu berkas panjang berlogo.
//
// Bentuknya menyalin halaman "/" apa adanya (CLAUDE.md → Tampilan): kiri yang
// DILIHAT (bingkai pratinjau + setelan yang mengubah rupanya, lalu log), kanan
// yang DIISI lalu dijalankan (daftar video, mutu, folder tujuan, tombol Mulai).
//
// Ikon: lucide-react (ISC). Tanpa satu emoji pun — alasannya di gui/app/page.tsx.

import { useCallback, useEffect, useRef, useState } from "react";
import { Film, Folder, X } from "lucide-react";

import { eng } from "../engine";
import { useI18n } from "../i18n";
import Alerts from "../alerts";
import WatermarkPanel from "../watermark-panel";
import { DEFAULT_WATERMARK, watermarkToAPI, watermarkOn, wrapHeadline } from "../watermark-model";
import type { Watermark } from "../watermark-model";
import { CENTER_X, CENTER_Y, PLAY_H, PLAY_W, useLayerDrag } from "../drag";
import Guides, { GridPicker } from "../guides";
import LogPanel from "../log-panel";
import Picker from "../picker";
import RunPanel from "../run-panel";
import Select from "../select";
import { useKeep, useRestore } from "../persist";

type FileResult = { video: string; name: string; output?: string; seconds?: number; error?: string };
type WatermarkJob = {
  id: string;
  status: string;
  stage: string;
  progress: number;
  log?: string[];
  error?: string;
  result?: { files: FileResult[] };
};

const baseName = (p: string) => p.split(/[\\/]/).pop() || p;
const hex = (c: string) => (c === "yellow" ? "#ffdd00" : "#ffffff");

export default function WatermarkPage() {
  const { t } = useI18n();

  const [videos, setVideos] = useState<string[]>([]);
  const [paste, setPaste] = useState("");
  const [picking, setPicking] = useState<"" | "file" | "folder" | "out" | "banner">("");
  const [outDir, setOutDir] = useState("");
  const [quality, setQuality] = useState("hd");
  const [dragOver, setDragOver] = useState(false);

  const [watermark, setWatermarkState] = useState<Watermark>(DEFAULT_WATERMARK);
  const [wmOpen, setWmOpen] = useState(true);
  const setWatermark = useCallback((patch: Partial<Watermark>) => {
    setWatermarkState((b) => ({ ...b, ...patch }));
  }, []);
  // Menggeser banner ikut membawa headline-nya — aturan yang sama dengan
  // halaman klip, dan alasannya juga sama (lihat page.tsx).
  const moveWatermark = useCallback((x: number, y: number) => {
    setWatermarkState((b) => ({ ...b, x, y, hlX: b.hlX + (x - b.x), hlY: b.hlY + (y - b.y) }));
  }, []);
  const moveHeadline = useCallback((x: number, y: number) => {
    setWatermarkState((b) => ({ ...b, hlX: x, hlY: y }));
  }, []);

  const [jobId, setJobId] = useState("");
  const [job, setJob] = useState<WatermarkJob | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [error, setError] = useState("");

  const busy = job?.status === "running" || (!!jobId && !job);

  // Kisi & garis tengah: penanda yang sama dengan halaman klip. Tanpa keduanya
  // tidak ada satu pun tanda bahwa gambar dan teks di atas bingkai BISA
  // dipegang — dan itu persis yang dilaporkan waktu halaman ini tidak punya.
  const [grid, setGrid] = useState(20);
  const [alwaysGuides, setAlwaysGuides] = useState(false);

  const boxRef = useRef<HTMLDivElement | null>(null);
  const { dragAt, dragProps } = useLayerDrag(boxRef, grid);
  const wmDrag = dragProps(watermark.x, watermark.y, moveWatermark, CENTER_Y, PLAY_H);
  const headlineDrag = dragProps(watermark.hlX, watermark.hlY, moveHeadline, CENTER_Y, PLAY_H);

  // Bingkai pratinjau diambil dari video PERTAMA di daftar: watermark-nya sama
  // untuk semuanya, jadi satu contoh sudah cukup untuk menaruhnya.
  const first = videos[0] || "";
  const [frame, setFrame] = useState("");
  useEffect(() => {
    if (!first) { setFrame(""); return; }
    setFrame(eng(`/api/frame?path=${encodeURIComponent(first)}&t=1&reframe=center&background=black&zoom=100`));
  }, [first]);

  // Setelan watermark LENGKET, dan itu inti fiturnya: identitas akun dipilih
  // sekali, bukan disusun ulang tiap kali hendak memposting.
  useKeep("watermark", { videos, outDir, quality, watermark });
  useRestore<Record<string, unknown>>("watermark", (v) => {
    if (Array.isArray(v.videos)) setVideos(v.videos as string[]);
    if (typeof v.outDir === "string") setOutDir(v.outDir);
    if (typeof v.quality === "string") setQuality(v.quality);
    if (v.watermark && typeof v.watermark === "object") {
      // hlSource dipaksa "text": halaman ini tidak punya klip, jadi tidak ada
      // judul LLM — dan setelan tersimpan dari versi lain tidak boleh
      // menyelundupkannya masuk lalu ditolak engine setelah tombol ditekan.
      setWatermarkState({ ...DEFAULT_WATERMARK, ...(v.watermark as Partial<Watermark>), hlSource: "text" });
    }
  });

  // Satu langganan SSE untuk seluruh halaman. Membakar video panjang itu
  // hitungan menit per berkas, jadi kabarnya datang dari sini — bukan polling.
  useEffect(() => {
    const es = new EventSource(eng("/api/watermark/events"));
    es.addEventListener("watermark", (ev) => {
      const j: WatermarkJob = JSON.parse((ev as MessageEvent).data);
      setJobId((cur) => {
        if (cur && j.id !== cur) return cur;
        setJob(j);
        if (j.log) setLogs(j.log);
        if (j.error) setError(j.error);
        return cur || j.id;
      });
    });
    return () => es.close();
  }, []);

  const add = useCallback((paths: string[]) => {
    setVideos((cur) => {
      const out = [...cur];
      for (const p of paths) {
        // Kutip pembungkus dibuang di sini juga: "Copy as path" di Explorer
        // selalu memasangnya, dan daftar di layar harus menampilkan nama
        // berkasnya — bukan `"C:\…mp4"` lengkap dengan kutip.
        const path = p.trim().replace(/^["']|["']$/g, "").trim();
        if (path && !out.includes(path)) out.push(path);
      }
      return out;
    });
  }, []);

  const addFolder = useCallback(async (dir: string) => {
    try {
      const r = await fetch(eng(`/api/browse?dir=${encodeURIComponent(dir)}`));
      const data = await r.json();
      if (!r.ok) throw new Error(data.error || "browse failed");
      const found = (data.entries || []).filter((e: { video?: boolean }) => e.video)
        .map((e: { path: string }) => e.path);
      if (!found.length) { setError(t("capFolderEmpty")); return; }
      add(found);
    } catch (e) {
      setError(String(e));
    }
  }, [add, t]);

  // Berkas yang dilepas TIDAK diunggah: engine jalan di mesin yang sama, jadi
  // ia ditanya di mana berkasnya (notes/24).
  const dropFiles = useCallback(async (files: FileList) => {
    for (const f of Array.from(files)) {
      try {
        const r = await fetch(eng("/api/locate"), {
          method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ name: f.name, size: f.size }),
        });
        const data = await r.json();
        if (r.ok && data.path) { add([data.path]); continue; }
      } catch { /* engine mati: dilaporkan di bawah */ }
      setError(`${f.name}: ${t("capFailed")}`);
    }
  }, [add, t]);

  const start = async () => {
    setError(""); setLogs([]); setJob(null); setJobId("");
    try {
      const r = await fetch(eng("/api/watermark"), {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          videos, quality, out_dir: outDir,
          // Font headline mengikuti bawaan halaman klip; satu pemilih font untuk
          // seluruh aplikasi, jadi pratinjau di sini memakai metrik yang sama.
          font: "Montserrat",
          watermark: watermarkToAPI(watermark, "Montserrat"),
        }),
      });
      const data = await r.json();
      if (!r.ok) throw new Error(data?.error || data?.message || "failed to start");
      setJobId(data.id);
    } catch (e) {
      setError(String(e));
    }
  };

  const cancel = async () => {
    if (!jobId) return;
    try {
      const r = await fetch(eng(`/api/watermark/${jobId}/cancel`), {
        method: "POST", headers: { "Content-Type": "application/json" }, body: "{}",
      });
      if (!r.ok) throw new Error(await r.text());
    } catch (e) {
      setError(String(e));
    }
  };

  const files = job?.result?.files ?? [];

  return (
    <div className="screen">
      <Alerts items={[error && { kind: "error" as const, text: error }]} />

      <div className="screen-body two">
        {/* KIRI: yang dilihat, dan setelan yang mengubah rupanya. */}
        <div className="screen-main">
          <div className="panel">
            <div className="sub-layout">
              <div className="sub-preview">
                <div className="preview9x16" ref={boxRef}>
                  {frame ? (
                    /* eslint-disable-next-line @next/next/no-img-element */
                    <img src={frame} alt="preview" draggable={false} />
                  ) : (
                    <div className="preview-empty">
                      <div className="pe-icon" aria-hidden="true" />
                      <div className="pe-title">{t("wmEmptyFrame")}</div>
                    </div>
                  )}
                  <Guides grid={grid} visible={dragAt !== null || alwaysGuides}
                    atX={(dragAt?.x ?? watermark.x) === CENTER_X}
                    atY={(dragAt?.y ?? watermark.y) === CENTER_Y}
                    dragAt={dragAt} />
                  {watermark.image && (
                    /* eslint-disable-next-line @next/next/no-img-element */
                    <img className="wmoverlay" alt="watermark" draggable={false}
                      src={eng(`/api/image?path=${encodeURIComponent(watermark.image)}`)}
                      style={{
                        left: `${(watermark.x / PLAY_W) * 100}%`, top: `${(watermark.y / PLAY_H) * 100}%`,
                        // KOTAK-nya yang diberi ukuran; object-fit: contain
                        // (globals.css) yang memuat gambarnya ke dalam.
                        width: `${watermark.width}%`, height: `${watermark.height}%`,
                      }}
                      {...wmDrag} />
                  )}
                  {watermark.hlText.trim() && (
                    <div className="suboverlay headlineoverlay"
                      style={{
                        left: `${(watermark.hlX / PLAY_W) * 100}%`, top: `${(watermark.hlY / PLAY_H) * 100}%`,
                        fontFamily: `"Montserrat", sans-serif`,
                        fontSize: `calc(${watermark.hlSize / PLAY_H} * var(--pvh))`,
                        color: hex(watermark.hlColor),
                        textShadow: watermark.hlOutline > 0
                          ? "-2px -2px 0 #000,2px -2px 0 #000,-2px 2px 0 #000,2px 2px 0 #000,0 0 4px #000"
                          : "none",
                      }}
                      {...headlineDrag}>
                      {wrapHeadline(watermark.hlText, watermark.hlSize)
                        .map((line, i) => <div key={i}>{line}</div>)}
                    </div>
                  )}
                </div>
              </div>

              <div className="sub-settings">
                <WatermarkPanel watermark={watermark} setWatermark={setWatermark} allowLLM={false}
                  open={wmOpen} setOpen={setWmOpen}
                  onPickImage={() => setPicking("banner")}
                  gridControl={
                    <GridPicker grid={grid} setGrid={setGrid}
                      always={alwaysGuides} setAlways={setAlwaysGuides} />
                  } />

                {/* Hasil per berkas. Daftar, jadi ia BOLEH bergulir — dan
                    tingginya dipaku supaya kolom ini tidak tumbuh tiap satu
                    video selesai. */}
                {!!files.length && (
                  <div className="group">
                    <div className="group-title">{t("wmResults")}</div>
                    <div className="basket watermark-results">
                      {files.map((f) => (
                        <div key={f.video} className="basket-item">
                          <span className="basket-text">
                            <span className="news-title" title={f.output || f.video}>{f.name}</span>
                            <span className="meta">
                              {f.error ? f.error : t("wmSaved", { name: baseName(f.output || "") })}
                            </span>
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>

          <LogPanel logs={logs} />
        </div>

        {/* KANAN: yang diisi & dijalankan. */}
        <div className="screen-col">
          <div className="panel feed-panel">
            <div className="group-title">{t("capVideos", { n: videos.length })}</div>

            <div className={"cap-drop" + (dragOver ? " over" : "")}
              onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
              onDragLeave={() => setDragOver(false)}
              onDrop={(e) => {
                e.preventDefault(); setDragOver(false);
                if (e.dataTransfer.files?.length) dropFiles(e.dataTransfer.files);
              }}>
              {videos.length > 0 && (
                <div className="basket cap-list">
                  {videos.map((v) => (
                    <div key={v} className="basket-item">
                      <Film className="ico" aria-hidden="true" />
                      <span className="basket-text">
                        <span className="news-title" title={v}>{baseName(v)}</span>
                      </span>
                      <button className="ghost tiny icon-only" title={t("capRemove")} aria-label={t("capRemove")}
                        disabled={busy}
                        onClick={() => setVideos((cur) => cur.filter((x) => x !== v))}>
                        <X className="ico" aria-hidden="true" />
                      </button>
                    </div>
                  ))}
                </div>
              )}

              <div className="path-row">
                <button className="ghost" onClick={() => setPicking("file")}>
                  <Film className="ico" aria-hidden="true" /> {t("capPickVideo")}
                </button>
                <button className="ghost" onClick={() => setPicking("folder")}>
                  <Folder className="ico" aria-hidden="true" /> {t("capPickFolder")}
                </button>
              </div>

              <div className="path-row">
                <input value={paste} onChange={(e) => setPaste(e.target.value)}
                  onKeyDown={(e) => { if (e.key === "Enter") { add(paste.split("\n")); setPaste(""); } }}
                  placeholder={t("capPastePlaceholder")} />
                <button disabled={!paste.trim()} onClick={() => { add(paste.split("\n")); setPaste(""); }}>
                  {t("capAdd")}
                </button>
              </div>

              <div className="field">
                <label>{t("outputDir")}</label>
                <div className="path-row">
                  <input value={outDir} onChange={(e) => setOutDir(e.target.value)}
                    placeholder={t("wmOutPlaceholder")} disabled={busy} />
                  <button className="ghost" onClick={() => setPicking("out")}>{t("pickerGo")}…</button>
                </div>
              </div>

              {/* Satu kalimat, dan ia memang perlu ada: video yang bukan 9:16
                  ditolak per berkas, dan mengetahuinya SEBELUM menunggu satu
                  encode penuh jauh lebih murah daripada sesudahnya. */}
              <p className="meta">{t("wmNote")}</p>
            </div>
          </div>

          <div className="panel">
            <div className="group-title">{t("wmOutputTitle")}</div>
            <div className="grid3">
              <div className="field">
                <label title={t("wmQualityTip")}>{t("quality")}</label>
                <Select value={quality} onChange={setQuality} disabled={busy} options={[
                  { value: "draft", label: t("qualityDraft") },
                  { value: "hd", label: t("qualityHd") },
                  { value: "max", label: t("qualityMax") },
                ]} />
              </div>
            </div>
          </div>

          <RunPanel
            busy={busy} testing={false}
            disabled={busy || videos.length === 0 || !watermarkOn(watermark)}
            cancellable={busy && !!jobId}
            onStart={start} onCancel={cancel}
            progress={job?.progress ?? 0}
          />
        </div>
      </div>

      {picking && (
        <Picker
          mode={picking === "folder" || picking === "out" ? "folder" : "file"}
          onPick={(p) => {
            if (picking === "folder") addFolder(p);
            else if (picking === "out") setOutDir(p);
            else if (picking === "banner") setWatermark({ image: p });
            else add([p]);
            setPicking("");
          }}
          onClose={() => setPicking("")}
        />
      )}
    </div>
  );
}

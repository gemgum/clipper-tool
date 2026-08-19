"use client";

// Tab keempat: pembuat caption.
//
// Bentuknya menyalin halaman "/" apa adanya — dua kolom, panel bernama, tanpa
// bilah atas: kiri yang DILIHAT (caption hasil + log), kanan yang DIISI lalu
// dijalankan (daftar video, setelan, mesin AI, tombol Mulai).
//
// Satu video atau tiga puluh adalah alur yang SAMA: bulk cuma daftar yang lebih
// panjang. Tidak ada mode kedua, tidak ada tombol kedua.

import { useCallback, useEffect, useRef, useState } from "react";
import { X, Copy, Folder, Film } from "lucide-react";
import { eng } from "../engine";
import { useI18n } from "../i18n";
import Alerts from "../alerts";
import EnginePicker, { useEngines } from "../engine-picker";
import Select from "../select";
import LogPanel from "../log-panel";
import Picker from "../picker";
import RunPanel from "../run-panel";
import Stepper from "../stepper";

type WhisperModel = { name: string; downloaded: boolean; size: string };
type Variant = { hook: string; body: string; tags?: string[]; violations?: string[] };
type FileResult = {
  video: string;
  name: string;
  txt?: string;
  variants?: Variant[];
  video_seconds?: number;
  used_seconds?: number;
  error?: string;
};
type CaptionJob = {
  id: string;
  status: string;
  stage: string;
  progress: number;
  log?: string[];
  error?: string;
  result?: { files: FileResult[]; engine: string };
};

// Nama berkas dari sebuah path, tanpa peduli pemisah mana yang dipakai OS-nya.
const baseName = (p: string) => p.split(/[\\/]/).pop() || p;

// Urutan mutu model whisper, terbaik dulu. Nama yang tidak ada di daftar ini
// (model baru) dianggap paling belakang — ia tidak boleh diam-diam terpilih.
const WHISPER_RANK = ["large-v3-turbo", "large-v3", "large", "medium", "small", "base", "tiny"];

const best = (models: WhisperModel[]) =>
  models
    .filter((m) => m.downloaded)
    .sort((a, b) =>
      (WHISPER_RANK.indexOf(a.name) + 1 || 99) - (WHISPER_RANK.indexOf(b.name) + 1 || 99))[0]?.name;

const fmtDur = (sec: number) => {
  const s = Math.round(sec);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
};

export default function CaptionsPage() {
  const { t } = useI18n();

  // --- daftar video ---
  const [videos, setVideos] = useState<string[]>([]);
  const [paste, setPaste] = useState("");
  const [picking, setPicking] = useState<"" | "file" | "folder" | "out">("");
  // Folder tujuan. KOSONG = di sebelah tiap videonya, dan itu bawaannya: di
  // situlah orang mencarinya saat hendak memposting.
  const [outDir, setOutDir] = useState("");
  const [dragOver, setDragOver] = useState(false);

  // --- setelan ---
  //
  // Model whisper ADA DI SINI, bukan dianggap urusan pemasangan: caption hanya
  // sebaik kata-kata yang dipakai menulisnya, dan itu yang paling menentukan
  // hasilnya. Tanpa pilihan ini halaman selalu memakai model bawaan, dan
  // percakapan sehari-hari bahasa Indonesia adalah yang paling dirugikan.
  const [models, setModels] = useState<WhisperModel[]>([]);
  const [whisper, setWhisper] = useState("");
  const [minutes, setMinutes] = useState(5);
  const [variants, setVariants] = useState(3);
  const [terms, setTerms] = useState("");

  // --- mesin ---
  const { engines } = useEngines();
  const [engine, setEngine] = useState("ollama");
  const [model, setModel] = useState("");

  // --- job ---
  const [jobId, setJobId] = useState("");
  const [job, setJob] = useState<CaptionJob | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState("");

  const busy = job?.status === "running" || (!!jobId && !job);

  // Daftar model ditarik ulang tiap jendela kembali fokus: model diunduh di
  // halaman Requirements, dan tanpa ini halaman terus mengira ia belum ada.
  //
  // Yang dipilih otomatis adalah model TERBAIK yang sudah ada, bukan yang
  // pertama di daftar. Bedanya nyata: "yang pertama" jatuh ke base — model uji
  // cepat — dan caption yang ditulis dari transkrip base tidak bisa diselamatkan
  // tahap mana pun sesudahnya. Klip pendek, jadi model besar pun terjangkau.
  useEffect(() => {
    const load = () =>
      fetch(eng("/api/models")).then((r) => r.json()).then((m: WhisperModel[]) => {
        setModels(m);
        setWhisper((cur) => {
          if (m.some((x) => x.name === cur && x.downloaded)) return cur;
          return best(m) ?? cur;
        });
      }).catch(() => {});
    load();
    window.addEventListener("focus", load);
    return () => window.removeEventListener("focus", load);
  }, []);

  // Satu langganan SSE untuk seluruh halaman, dibuka sekali. Job berjalan
  // menit-menitan per video, jadi kabarnya datang dari sini — bukan polling.
  useEffect(() => {
    const es = new EventSource(eng("/api/captions/events"));
    es.addEventListener("caption", (ev) => {
      const j: CaptionJob = JSON.parse((ev as MessageEvent).data);
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
        // Kutip pembungkus dibuang di sini juga, bukan cuma di engine: "Copy as
        // path" di Explorer selalu memasangnya, dan daftar di layar harus
        // menampilkan nama berkasnya — bukan `"C:\…mp4"` lengkap dengan kutip.
        const path = p.trim().replace(/^["']|["']$/g, "").trim();
        if (path && !out.includes(path)) out.push(path);
      }
      return out;
    });
  }, []);

  // Seluruh isi folder sekaligus — jalan bulk yang sebenarnya. Engine sudah
  // menandai mana yang video di /api/browse, jadi penyaringannya tidak perlu
  // ditebak dari akhiran nama berkas di sini.
  const addFolder = useCallback(async (dir: string) => {
    try {
      const r = await fetch(eng(`/api/browse?dir=${encodeURIComponent(dir)}`));
      const data = await r.json();
      if (!r.ok) throw new Error(data.error || "browse failed");
      const found = (data.entries || []).filter((e: any) => e.video).map((e: any) => e.path);
      if (!found.length) { setError(t("capFolderEmpty")); return; }
      add(found);
    } catch (e) {
      setError(String(e));
    }
  }, [add, t]);

  // Berkas yang dilepas TIDAK diunggah: engine jalan di mesin yang sama, jadi
  // ia ditanya di mana berkasnya (notes/24). Beberapa sekaligus boleh.
  const dropFiles = useCallback(async (files: FileList) => {
    for (const f of Array.from(files)) {
      try {
        const r = await fetch(eng("/api/locate"), {
          method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ name: f.name, size: f.size }),
        });
        const data = await r.json();
        if (r.ok && data.path) { add([data.path]); continue; }
      } catch { /* engine mati: laporkan di bawah */ }
      setError(`${f.name}: ${t("capFailed")}`);
    }
  }, [add, t]);

  const start = async () => {
    setError("");
    setLogs([]);
    setJob(null);
    setJobId("");
    try {
      const r = await fetch(eng("/api/captions"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          videos, engine, model, minutes, variants, terms,
          out_dir: outDir, whisper_model: whisper,
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
      const r = await fetch(eng(`/api/captions/${jobId}/cancel`), {
        method: "POST", headers: { "Content-Type": "application/json" }, body: "{}",
      });
      if (!r.ok) throw new Error(await r.text());
    } catch (e) {
      setError(String(e));
    }
  };

  // Yang disalin = caption LENGKAP dengan tagarnya: itulah yang ditempel ke
  // aplikasi sebelah, dan tagar yang harus disusun ulang sendiri adalah tagar
  // yang cepat atau lambat lupa ikut.
  const copy = async (key: string, v: Variant) => {
    const text = [v.hook, v.body, v.tags?.length ? v.tags.map((x) => "#" + x).join(" ") : ""]
      .filter(Boolean).join("\n\n");
    try {
      await navigator.clipboard.writeText(text);
      setCopied(key);
      setTimeout(() => setCopied(""), 1500);
    } catch {
      setError(t("errCopy"));
    }
  };

  const files = job?.result?.files ?? [];

  return (
    <div className="screen">
      <Alerts items={[error && { kind: "error" as const, text: error }]} />

      <div className="screen-body two">
        {/* KIRI: yang dilihat. */}
        <div className="screen-main">
          <div className="panel post-panel">
            <div className="group-title">{t("capTitle")}</div>

            {!files.length ? (
              <p className="stage">{busy ? t("capRunning") : t("capEmpty")}</p>
            ) : (
              <div className="post-view">
                {files.map((f) => (
                  <div key={f.video} className="cap-file">
                    <div className="cap-file-head">
                      <strong>{f.name}</strong>
                      <span className="meta" title={f.txt || f.video}>
                        {f.error
                          ? `${t("capFailed")}: ${f.error}`
                          : [
                              f.used_seconds && f.video_seconds
                                ? t("capRead", { used: fmtDur(f.used_seconds), total: fmtDur(f.video_seconds) })
                                : "",
                              f.txt ? t("capSaved", { name: baseName(f.txt) }) : "",
                            ].filter(Boolean).join(" · ")}
                      </span>
                    </div>
                    {(f.variants ?? []).map((v, i) => (
                      <div key={i} className="cap-variant">
                        <button className="ghost tiny cap-copy" onClick={() => copy(f.video + i, v)}>
                          <Copy className="ico" aria-hidden="true" />
                          {copied === f.video + i ? t("copied") : t("capCopy")}
                        </button>
                        <p className="cap-hook">{v.hook}</p>
                        <p>{v.body}</p>
                        {!!v.tags?.length && <p className="post-tags">{v.tags.map((x) => "#" + x).join(" ")}</p>}
                        {/* Pagar isi. Tidak menghalangi apa pun — ia menandai
                            baris mana yang perlu dicocokkan dengan videonya. */}
                        {(v.violations ?? []).map((w, k) => (
                          <p key={k} className="cap-warn">{t("capCheck")}: {w}</p>
                        ))}
                      </div>
                    ))}
                    {!f.error && !(f.variants ?? []).length && <p className="meta">{t("capNoSpeech")}</p>}
                  </div>
                ))}
              </div>
            )}
          </div>

          <LogPanel logs={logs} />
        </div>

        {/* KANAN: yang diisi & dijalankan. */}
        <div className="screen-col">
          <div className="panel feed-panel">
            <div className="group-title">{t("capVideos", { n: videos.length })}</div>

            {/* Seluruh panel jadi sasaran lepas — kotak seret-lepas tersendiri
                cuma mengulang apa yang sudah bisa dilakukan tombol di bawahnya,
                dan tingginya diambil dari isi halaman (notes/29). */}
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

              {/* Folder tujuan duduk DI SINI, di panel yang merentang — bukan
                  di panel setelan yang tingginya pas. Satu baris tambahan di
                  sana menaikkan kolom 53 px, dan kolomnya sudah mentok. */}
              {/* Keduanya BERLABEL. Sempat tidak, demi menghemat tinggi, dan
                  akibatnya dua kotak isian berdiri tanpa satu pun keterangan
                  tentang gunanya — pertanyaan pertama yang muncul begitu
                  halamannya dipakai. Panel ini yang merentang, jadi labelnya
                  tidak mengambil tinggi dari panel mana pun. */}
              <div className="field">
                <label>{t("outputDir")}</label>
                <div className="path-row">
                  <input value={outDir} onChange={(e) => setOutDir(e.target.value)}
                    placeholder={t("capOutPlaceholder")} disabled={busy} />
                  <button className="ghost" onClick={() => setPicking("out")}>{t("pickerGo")}…</button>
                </div>
              </div>

              <div className="field">
                <label title={t("capTermsTip")}>{t("terms")}</label>
                <input value={terms} onChange={(e) => setTerms(e.target.value)}
                  placeholder={t("termsPlaceholder")} disabled={busy} title={t("capTermsTip")} />
              </div>
            </div>
          </div>

          <div className="panel">
            <div className="group-title">{t("capSettingsTitle")}</div>
            {/* Ketiganya SEBARIS. Daftar istilah sempat dipindah ke baris
                sendiri supaya keterangannya tidak terpotong, dan itu menambah
                53 px pada kolom yang tepat pas — terukur, bukan dugaan.
                Keterangan lengkapnya tetap terbaca lewat tooltip judulnya. */}
            <div className="grid3">
              <div className="field">
                <label title={t("capMinutesTip")}>{t("capMinutes")}</label>
                <Stepper value={minutes} onChange={setMinutes} min={1} max={60} suffix={t("capMinutesUnit")} />
              </div>
              <div className="field">
                <label title={t("capVariantsTip")}>{t("capVariants")}</label>
                <Stepper value={variants} onChange={setVariants} min={1} max={5} />
              </div>
              <div className="field">
                <label title={t("capWhisperTip")}>{t("whisperModel")}</label>
                <Select value={whisper} onChange={setWhisper} options={models.map((m) => ({
                  value: m.name, label: m.name,
                  note: m.downloaded ? m.size : t("modelNotDownloaded"),
                }))} />
              </div>
            </div>
          </div>

          <div className="panel">
            <div className="group-title">{t("writerEngineTitle")}</div>
            <EnginePicker
              engines={engines} engine={engine} setEngine={setEngine}
              model={model} setModel={setModel} busy={busy}
            />
          </div>

          <RunPanel
            busy={busy} testing={false}
            disabled={busy || videos.length === 0}
            cancellable={busy && !!jobId}
            onStart={start} onCancel={cancel}
            progress={job?.progress ?? 0}
          />
        </div>
      </div>

      {picking && (
        <Picker
          mode={picking === "file" ? "file" : "folder"}
          onPick={(p) => {
            if (picking === "folder") addFolder(p);
            else if (picking === "out") setOutDir(p);
            else add([p]);
            setPicking("");
          }}
          onClose={() => setPicking("")}
        />
      )}
    </div>
  );
}

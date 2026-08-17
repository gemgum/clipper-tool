"use client";

// Halaman klip. Isinya tinggal KERANGKA dan KEADAAN: dua kolom, tujuh panel
// bernama, dan state yang dipakai lebih dari satu panel (job, preset tersimpan,
// dan bahan permintaan /api/jobs).
//
// Dipecah begini bukan karena rapi lebih enak dilihat. Berkas ini pernah 1.300
// baris dengan belasan tingkat bersarang, dan memindahkan satu blok di dalamnya
// GAGAL TIGA KALI berturut-turut: panel mendarat tersarang di dalam panel lain,
// TypeScript tetap lolos (ia memeriksa tag seimbang, bukan tag berada di induk
// yang benar), dan salahnya baru terlihat di browser. Dengan panel jadi satu tag
// masing-masing, salah tempat langsung kelihatan di berkas sepanjang 20 baris
// (notes/29).
//
// Ikon: lucide-react (ISC). Emoji dibuang bukan karena selera — gunting dan
// folder tampil sebagai KOTAK KOSONG di jendela Tauri Linux yang tidak punya
// font emoji. Karena itu berkas ini juga tidak memuat satu emoji pun, termasuk
// di komentarnya: greplah, dan hasil kosong berarti aturannya masih dipegang.
//
// Lambang yang TIDAK diganti: ⚠ ✓ ✕ → ↗ ↓ ↑ ✗. Semuanya simbol teks biasa yang
// ada di font mana pun.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useI18n } from "./i18n";
import Alerts from "./alerts";
import { eng, engineURL } from "./engine";
import Picker from "./picker";
import { useKeep, useRestore } from "./persist";

import SourceRow from "./source-row";
import PreviewPanel, { CENTER_X, CENTER_Y, PLATFORMS, PLAY_H, PLAY_W, linesFor } from "./preview-panel";
import type { Font, FontCheck } from "./preview-panel";
import FramePanel, { zoomBounds } from "./frame-panel";
import SetupPanel from "./setup-panel";
import type { WhisperModel } from "./setup-panel";
import RunPanel from "./run-panel";
import LogPanel from "./log-panel";

export default function Home() {
  const { lang, t } = useI18n();

  const [path, setPath] = useState("");
  const [outputDir, setOutputDir] = useState("");
  const [mode, setMode] = useState("offline");
  const [model, setModel] = useState("base");
  const [resolution, setResolution] = useState("1080p");
  const [quality, setQuality] = useState("hd");
  const [reframe, setReframe] = useState("center");
  const [background, setBackground] = useState("blur");
  const [zoom, setZoom] = useState(zoomBounds("center").natural);
  const [fps, setFps] = useState(0);

  // Mesin AI (scoring). Yang di sini hanya yang ikut ke /api/jobs dan ke preset
  // tersimpan — kunci API dan status Ollama dipegang <AiEnginePanel> sendiri.
  const [claudeModel, setClaudeModel] = useState("claude-haiku-4-5");
  const [offlineEngine, setOfflineEngine] = useState("ollama"); // ollama | heuristic
  const [ollamaModel, setOllamaModel] = useState("llama3.1");
  const [transcriptFix, setTranscriptFix] = useState(true);
  const [terms, setTerms] = useState("");
  const [durationPreset, setDurationPreset] = useState("auto");
  const [maxClips, setMaxClips] = useState(10);
  const [models, setModels] = useState<WhisperModel[]>([]);

  // Subtitle
  const [fonts, setFonts] = useState<Font[]>([]);
  const [subFont, setSubFont] = useState("Montserrat");
  const [subSize, setSubSize] = useState(72);
  const [subColor, setSubColor] = useState("white");
  const [subX, setSubX] = useState(540);
  const [subY, setSubY] = useState(960);
  const [subOutline, setSubOutline] = useState(4);
  const [subBox, setSubBox] = useState(false);
  const [subMode, setSubMode] = useState("normal");       // normal | karaoke | word
  const [subHighlight, setSubHighlight] = useState("yellow");
  const [subSpeed, setSubSpeed] = useState("normal");     // slow | normal | dense
  const [platform, setPlatform] = useState("tiktok");     // "off" = tanpa pembatas
  const [saveMode, setSaveMode] = useState("burn");       // burn | clean | both

  // Font manual (di luar daftar bawaan) + hasil pengecekannya di engine.
  // Tinggal di sini, bukan di <PreviewPanel>: start() menolak job yang fontnya
  // belum lolos cek, jadi halaman ini harus tahu hasilnya.
  const fontFromPreset = useRef<string>("");
  const [fontManual, setFontManual] = useState(false);
  const [fontCheck, setFontCheck] = useState<FontCheck | null>(null);
  const [fontChecking, setFontChecking] = useState(false);

  // Upload
  const [uploading, setUploading] = useState(false);
  const [uploadPct, setUploadPct] = useState(0);
  // Pemilih berkas milik sendiri: "video" untuk sumber, "out" untuk folder
  // keluaran, null = tertutup.
  const [picker, setPicker] = useState<null | "video" | "out">(null);

  // Job
  const [jobId, setJobId] = useState<string | null>(null);
  const [status, setStatus] = useState("");
  const [stage, setStage] = useState("");
  const [progress, setProgress] = useState(0);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  // Uji LLM dipegang DI SINI, bukan di dalam SetupPanel: yang perlu tahu
  // uji sedang jalan adalah tombol Mulai di panel sebelah. Model yang sedang
  // dipaksa memuat diri tidak bisa melayani job sekaligus — job yang dimulai di
  // tengah uji cuma menunggu antrean tanpa memberitahu apa yang ditunggunya.
  const [testing, setTesting] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);

  const locale = lang === "id" ? "id-ID" : "en-GB";
  const addLog = useCallback((text: string) => {
    setLogs((prev) => [...prev, `[${new Date().toLocaleTimeString(locale)}] ${text}`]);
  }, [locale]);

  // Daftar model ditarik ulang setiap jendela kembali fokus.
  //
  // Model diunduh di HALAMAN LAIN (Requirements), dan tanpa penarikan ulang
  // halaman ini terus mengira modelnya belum ada — peringatannya bertahan dan
  // tombol Mulai tetap mati sampai aplikasi ditutup. Itu yang membuat "harus
  // direstart dulu".
  useEffect(() => {
    const loadModels = () =>
      fetch(eng(`/api/models`)).then((r) => r.json()).then((m: WhisperModel[]) => {
        setModels(m);
        setModel((cur) => {
          // Pilihan pengguna dipertahankan selama modelnya benar-benar ada;
          // kalau tidak, ambil yang sudah terunduh.
          if (m.some((x) => x.name === cur && x.downloaded)) return cur;
          return m.find((x) => x.downloaded)?.name ?? cur;
        });
      }).catch(() => {});
    window.addEventListener("focus", loadModels);
    return () => window.removeEventListener("focus", loadModels);
  }, []);

  useEffect(() => {
    fetch(eng(`/api/models`)).then((r) => r.json()).then((m: WhisperModel[]) => {
      setModels(m);
      const downloaded = m.find((x) => x.downloaded);
      if (downloaded) setModel(downloaded.name);
    }).catch(() => addLog(`⚠ ${t("engineUnreachable", { url: engineURL() })}`));
    fetch(eng(`/api/fonts`)).then((r) => r.json()).then((f: Font[]) => {
      setFonts(f);
      // Jangan menimpa font dari preset — daftar font datang belakangan, dulu
      // pilihan tersimpan selalu tergantikan font pertama tiap halaman dimuat.
      if (f[0] && !fontFromPreset.current) setSubFont(f[0].name);
      // Font tersimpan yang bukan bawaan berarti font manual.
      else if (fontFromPreset.current && !f.some((x) => x.name === fontFromPreset.current)) setFontManual(true);
    }).catch(() => {});
  }, [addLog, t]);

  // Font manual divalidasi di engine (format + benar-benar terpasang), ditunda
  // 400 ms supaya tidak memanggil fc-match tiap huruf yang diketik.
  useEffect(() => {
    if (!fontManual) { setFontCheck(null); return; }
    const name = subFont.trim();
    if (!name) { setFontCheck(null); return; }
    setFontChecking(true);
    const timer = setTimeout(() => {
      fetch(eng(`/api/font-check?name=${encodeURIComponent(name)}`))
        .then((r) => r.json()).then(setFontCheck)
        .catch(() => setFontCheck({ valid: false, name, family: "", source: "", error: "engine unreachable", scale: 0 }))
        .finally(() => setFontChecking(false));
    }, 400);
    return () => { clearTimeout(timer); setFontChecking(false); };
  }, [fontManual, subFont]);

  // Muat preset tersimpan (localStorage) sekali di awal.
  useEffect(() => {
    try {
      const s = JSON.parse(localStorage.getItem("clipper.preset") || "{}");
      if (s.resolution) setResolution(s.resolution);
      if (s.quality) setQuality(s.quality);
      // Preset lama menyimpan reframe:"fit" — pilihan yang kini sudah tidak ada
      // di dropdown. Dibiarkan, select-nya tampil kosong DAN engine
      // menerjemahkannya jadi zoom 0, menimpa zoom tersimpan tanpa pemberitahuan.
      // Jadi dimigrasikan di sini, sama seperti yang dilakukan engine.
      if (s.reframe) setReframe(s.reframe);
      // Arti zoom sempat berubah dua kali di tengah pengembangan. Preset yang
      // menyimpan penandanya dibuang saja ke default — menebak-nebak arti angka
      // lama lebih berisiko daripada mulai dari setelan bawaan.
      if (typeof s.zoom === "number" && !s.zoomAxis) setZoom(s.zoom);
      if (s.background) setBackground(s.background);
      if (typeof s.fps === "number") setFps(s.fps);
      if (s.claudeModel) setClaudeModel(s.claudeModel);
      if (s.offlineEngine) setOfflineEngine(s.offlineEngine);
      if (s.ollamaModel) setOllamaModel(s.ollamaModel);
      if (typeof s.transcriptFix === "boolean") setTranscriptFix(s.transcriptFix);
      if (typeof s.terms === "string") setTerms(s.terms);
      if (s.durationPreset) setDurationPreset(s.durationPreset);
      if (s.maxClips) setMaxClips(s.maxClips);
      if (s.subFont) { setSubFont(s.subFont); fontFromPreset.current = s.subFont; }
      if (s.subSize) setSubSize(s.subSize);
      if (s.subColor) setSubColor(s.subColor);
      if (typeof s.subOutline === "number") setSubOutline(s.subOutline);
      if (typeof s.subBox === "boolean") setSubBox(s.subBox);
      if (s.subMode) setSubMode(s.subMode);
      else if (typeof s.subKaraoke === "boolean" && s.subKaraoke) setSubMode("karaoke"); // preset lama
      if (s.subHighlight) setSubHighlight(s.subHighlight);
      if (s.subSpeed) setSubSpeed(s.subSpeed);
      if (s.platform) setPlatform(s.platform);
      if (s.saveMode) setSaveMode(s.saveMode);
    } catch {}
  }, []);

  // Simpan preset setiap kali setelan berubah.
  useEffect(() => {
    const preset = { resolution, quality, reframe, background, zoom, fps, claudeModel, offlineEngine, ollamaModel, transcriptFix, terms, durationPreset, maxClips, subFont, subSize, subColor, subOutline, subBox, subMode, subHighlight, subSpeed, platform, saveMode };
    try { localStorage.setItem("clipper.preset", JSON.stringify(preset)); } catch {}
  }, [resolution, quality, reframe, background, zoom, fps, claudeModel, offlineEngine, ollamaModel, transcriptFix, terms, durationPreset, maxClips, subFont, subSize, subColor, subOutline, subBox, subMode, subHighlight, subSpeed, platform, saveMode]);

  // Pulihkan log job TERAKHIR, lalu sambung ulang bila ia masih berjalan.
  //
  // Dua-duanya di sini, dan urutannya penting. Kotak log dulu cuma state React:
  // berpindah tab melepas komponen halaman ini dan seluruh isinya hilang, tanpa
  // satu pun tempat lain yang menyimpannya. Sekarang engine menulisnya ke
  // <DataDir>/jobs/<id>.log dan halaman tinggal membacanya kembali — jadi yang
  // dipulihkan bukan cuma job yang masih hidup, melainkan juga yang sudah
  // selesai atau gagal. Justru yang gagal yang paling ingin dibaca ulang.
  //
  // Log diisi lewat setLogs, bukan addLog: barisnya sudah bercap waktu dari
  // engine, dan menambahkan cap kedua di depannya cuma membuatnya tak terbaca.
  useEffect(() => {
    fetch(eng(`/api/jobs`)).then((r) => r.json()).then(async (jobs: any[]) => {
      if (!Array.isArray(jobs) || !jobs.length) return;
      // Job yang masih hidup didahulukan atas yang sekadar paling baru: pada
      // antrean berisi dua, yang terbaru justru yang belum mulai dan lognya
      // masih kosong, sedangkan yang sedang berjalan itulah yang ingin dilihat.
      const byNewest = [...jobs].sort((a, b) => (a.created_at < b.created_at ? 1 : -1));
      const newest = byNewest.find((j) => j.status === "running") || byNewest[0];
      try {
        const res = await fetch(eng(`/api/jobs/${newest.id}/log`));
        const data = await res.json();
        if (Array.isArray(data.lines) && data.lines.length) setLogs(data.lines);
      } catch {
        // Log yang tak terbaca tidak boleh menghalangi penyambungan ulang.
      }
      if (newest.status === "running" || newest.status === "queued") {
        setStatus(newest.status);
        setStage(newest.stage || "");
        setProgress(newest.progress || 0);
        setBusy(true);
        setJobId(newest.id); // memicu langganan SSE → progress lanjut terlihat
        addLog(t("logReconnect", { id: newest.id }));
      }
    }).catch(() => {});
  }, [addLog, t]);

  const selectedModel = models.find((m) => m.name === model);
  const modelMissing = selectedModel && !selectedModel.downloaded;

  // Isian tab klip ikut disimpan, dengan alasan yang sama seperti tab kartu:
  // halaman bisa termuat ulang tanpa diminta, dan menyusun ulang setelan render
  // dari awal jauh lebih menyakitkan daripada mengetik satu path.
  //
  // Yang TIDAK disimpan: daftar klip & status job — itu milik engine, dan
  // ditarik ulang saat halaman dibuka.
  useKeep("clips", {
    path, outputDir, mode, model, resolution, quality, reframe, background, zoom, fps,
    durationPreset, maxClips, saveMode, transcriptFix, terms,
    offlineEngine, claudeModel, ollamaModel,
    subFont, subSize, subX, subY, subColor, subOutline, subBox, subMode, subHighlight, subSpeed,
  });
  useRestore<Record<string, unknown>>("clips", (v) => {
    const set = <T,>(fn: (x: T) => void, val: unknown) => {
      if (val !== undefined && val !== null) fn(val as T);
    };
    set(setPath, v.path);
    set(setOutputDir, v.outputDir);
    set(setMode, v.mode);
    set(setModel, v.model);
    set(setResolution, v.resolution);
    set(setQuality, v.quality);
    set(setReframe, v.reframe);
    set(setBackground, v.background);
    set(setZoom, v.zoom);
    set(setFps, v.fps);
    set(setDurationPreset, v.durationPreset);
    set(setMaxClips, v.maxClips);
    set(setSaveMode, v.saveMode);
    set(setTranscriptFix, v.transcriptFix);
    set(setTerms, v.terms);
    set(setOfflineEngine, v.offlineEngine);
    set(setClaudeModel, v.claudeModel);
    set(setOllamaModel, v.ollamaModel);
    set(setSubFont, v.subFont);
    set(setSubSize, v.subSize);
    set(setSubX, v.subX);
    set(setSubY, v.subY);
    set(setSubColor, v.subColor);
    set(setSubOutline, v.subOutline);
    set(setSubBox, v.subBox);
    set(setSubMode, v.subMode);
    set(setSubHighlight, v.subHighlight);
    set(setSubSpeed, v.subSpeed);
  });

  const uploadFile = useCallback((file: File) => {
    setUploading(true); setUploadPct(0);
    addLog(t("logUploadStart", { name: file.name, size: (file.size / 1e6).toFixed(0) }));
    const form = new FormData(); form.append("file", file);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", eng(`/api/upload`));
    xhr.upload.onprogress = (e) => { if (e.lengthComputable) setUploadPct(e.loaded / e.total); };
    xhr.onload = () => {
      setUploading(false);
      try {
        const data = JSON.parse(xhr.responseText);
        if (data.path) { setPath(data.path); addLog(t("logUploadDone", { name: data.name })); }
        else addLog(t("logUploadFailed", { error: data.error }));
      } catch { addLog(t("logUploadInvalid")); }
    };
    xhr.onerror = () => { setUploading(false); addLog(t("logUploadNetwork")); };
    xhr.send(form);
  }, [addLog, t]);

  // useFile menerima berkas dari seret & lepas atau dari pemilih browser.
  //
  // Browser tidak memberi tahu di mana berkas itu berada — hanya nama dan
  // ukurannya. Padahal berkasnya ada di mesin yang sama dengan engine, jadi
  // engine ditanya dulu: kalau ia menemukannya, path itu langsung dipakai dan
  // tidak ada satu byte pun yang disalin. Unggahan hanya cadangan.
  const useFile = useCallback(async (file: File) => {
    addLog(t("logLocating", { name: file.name }));
    try {
      const res = await fetch(eng(`/api/locate`), {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ name: file.name, size: file.size }),
      });
      const data = await res.json();
      if (res.ok && data.path) {
        setPath(data.path);
        addLog(t("logLocated", { path: data.path }));
        return;
      }
    } catch {
      // Engine mati atau versinya lama: unggahan di bawah tetap jalan.
    }
    addLog(t("logLocateMiss", { name: file.name }));
    uploadFile(file);
  }, [addLog, t, uploadFile]);

  const start = useCallback(async () => {
    // Font manual yang belum lolos pengecekan ditolak di sini — kalau diteruskan,
    // libass diam-diam mengganti fontnya dan hasil render tidak sesuai preview.
    if (fontManual && !fontCheck?.valid) {
      addLog(t("logFontInvalid", { font: subFont }) + (fontCheck?.error ? ` (${fontCheck.error})` : ""));
      return;
    }
    setError(""); setProgress(0); setStage(""); setMessage("");
    setBusy(true); setStatus("queued"); setJobId(null);
    addLog(t("logJobStart", { resolution, quality, duration: durationPreset, font: subFont }));
    try {
      const res = await fetch(eng(`/api/jobs`), {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({
          source: { type: "path", value: path },
          options: {
            mode, whisper_model: model, resolution, quality, reframe, background,
            zoom: Number(zoom), fps: Number(fps),
            provider: mode === "hybrid" ? "claude" : offlineEngine,
            llm_model: claudeModel, ollama_model: ollamaModel,
            transcript_fix: transcriptFix ? "on" : "off",
            terms: terms.split(/[,;\n]/).map((v) => v.trim()).filter(Boolean),
            duration_preset: durationPreset, max_clips: Number(maxClips), output_dir: outputDir,
            subtitle_output: saveMode,
            subtitle: {
              font: subFont, size: Number(subSize), x: subX, y: subY, color: subColor, bold: true,
              outline: Number(subOutline), box: subBox,
              mode: subMode, highlight_color: subHighlight, speed: subSpeed,
            },
          },
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || t("errCreateJob"));
      setJobId(data.id); addLog(t("logJobCreated", { id: data.id }));
    } catch (e: any) { setError(e.message); setBusy(false); setStatus("error"); addLog(`⚠ ${e.message}`); }
  }, [path, mode, model, resolution, quality, reframe, background, zoom, fps, offlineEngine, claudeModel, ollamaModel, transcriptFix, terms, durationPreset, maxClips, outputDir, subFont, subSize, subX, subY, subColor, subOutline, subBox, subMode, subHighlight, subSpeed, saveMode, addLog, fontManual, fontCheck, t]);

  const cancel = useCallback(async () => {
    if (!jobId) return;
    addLog(t("logCancelling"));
    // Tanpa badan, tapi jenisnya tetap disebut: engine menolak POST yang bukan
    // JSON, dan itulah yang memaksa halaman asing melewati preflight lebih dulu.
    await fetch(eng(`/api/jobs/${jobId}/cancel`), {
      method: "POST",
      headers: { "content-type": "application/json" },
    }).catch(() => {});
  }, [jobId, addLog, t]);

  useEffect(() => {
    if (!jobId) return;
    const events = new EventSource(eng(`/api/jobs/${jobId}/events`));
    events.addEventListener("progress", (e: MessageEvent) => {
      const d = JSON.parse(e.data);
      if (d.status) setStatus(d.status);
      if (d.stage) setStage(d.stage);
      if (typeof d.progress === "number") setProgress(d.progress);
      if (d.message) { setMessage(d.message); addLog(`${d.stage || ""}: ${d.message}`); }
      // Ringkasan waktu per tahap, hanya ada di peristiwa terakhir. Dicatat
      // tanpa awalan tahap: isinya tabel monospace yang perataannya harus utuh
      // (.logbox sudah pre-wrap + monospace).
      if (d.summary) addLog(d.summary);
    });
    events.addEventListener("clip", (e: MessageEvent) => {
      const clip = JSON.parse(e.data);
      addLog(t("logClip", { id: clip.id, score: clip.score }));
    });
    events.addEventListener("done", () => {
      setStatus("done"); setProgress(1); setBusy(false); addLog(t("logFinished")); events.close();
    });
    events.addEventListener("error", (e: MessageEvent) => {
      let msg = "error";
      try { msg = JSON.parse((e as any).data).message || msg; } catch {}
      setError(msg); setStatus("error"); setBusy(false); addLog(`⚠ ${msg}`); events.close();
    });
    return () => events.close();
  }, [jobId, addLog, t]);

  // Font yang di-@font-face untuk preview: bawaan + font manual yang lolos cek.
  const previewFonts = useMemo(() => {
    const names = fonts.map((f) => f.name);
    if (fontManual && fontCheck?.valid && !names.includes(fontCheck.family)) names.push(fontCheck.family);
    return names;
  }, [fonts, fontManual, fontCheck]);

  // Angka zoom artinya berbeda per mode, jadi membawanya menyeberang saat mode
  // berganti hanya menghasilkan nilai yang tak berarti. Disetel ke titik awal
  // mode barunya.
  const changeReframe = useCallback((next: string) => {
    setReframe(next);
    setZoom(zoomBounds(next).natural);
  }, []);

  // Geometri blok subtitle. Tinggal di halaman, bukan di <PreviewPanel>, sebab
  // <FramePanel> memakainya juga: peringatan "menabrak zona" dan tombol "taruh
  // di area aman" keduanya butuh tinggi blok yang sama.
  //
  // Jarak antarbaris libass PERSIS sebesar ukuran fontnya, berapa pun fontnya —
  // diukur: dua baris pada ukuran 72 berjarak 72 px. Faktor 1,17 yang dulu ada
  // di sini diukur saat font bawaan tidak pernah benar-benar dipakai libass
  // (notes/29), jadi ia mengukur font cadangan sistem, bukan Montserrat.
  const blockH = Math.round(subSize * linesFor(subMode, subSpeed));
  // Jangkar ada di tepi atas blok, jadi supaya blok terlihat di tengah bidang
  // titiknya harus setengah tinggi blok di atas garis tengah.
  const centerAnchorY = Math.round(CENTER_Y - blockH / 2);

  // Zona yang tertutup UI aplikasi (undefined = pembatas dimatikan).
  const zone = platform !== "off" ? PLATFORMS[platform]?.zone : undefined;
  // Yang boleh menabrak zona adalah blok penuhnya. Kalau hanya titik jangkar
  // yang diperiksa, halaman 2-3 baris bisa menjulur ke zona bawah sementara
  // peringatannya diam saja.
  const inUnsafe = !!zone && (
    subY / PLAY_H < zone.top ||
    (subY + blockH) / PLAY_H > 1 - zone.bottom ||
    subX / PLAY_W > 1 - zone.right
  );
  // Taruh subtitle tepat di atas zona bawah — disisakan setinggi blok, karena
  // baris tambahan tumbuh ke bawah dari titik ini. X tetap di tengah bidang:
  // subtitle ditulis rata tengah terhadap titik ini, jadi menengahkannya ke
  // lebar sisa (di luar kolom tombol kanan) justru membuatnya miring ke kiri.
  const placeSafe = () => {
    setSubX(CENTER_X);
    setSubY(zone ? Math.round(PLAY_H * (1 - zone.bottom) - blockH - 40) : centerAnchorY);
  };

  return (
    <div className="screen">
      {/* Muat font asli agar preview akurat — termasuk font manual yang lolos cek.
          DUA aturan per font, tegak dan tebal: kalau hanya satu yang dipasang,
          browser menebalkan sendiri face tegaknya, dan penebalan buatan itu tidak
          sama dengan face tebal sungguhan yang dipakai libass saat merender.
          Font satu-bobot (Anton, Bebas Neue) dilayani berkas yang sama untuk
          kedua aturan — engine yang menentukan, GUI tidak perlu tahu. */}
      <style dangerouslySetInnerHTML={{ __html: previewFonts.flatMap((n) => {
        const src = (w: string) => eng(`/api/font-file?name=${encodeURIComponent(n)}&weight=${w}`);
        return [
          `@font-face{font-family:"${n}";font-weight:400;src:url("${src("400")}");font-display:swap;}`,
          `@font-face{font-family:"${n}";font-weight:700;src:url("${src("700")}");font-display:swap;}`,
        ];
      }).join("") }} />

      {/* Galat & peringatan MELAYANG (alerts.tsx), tidak lagi disisipkan sebagai
          kepala halaman: kepala yang muncul-hilang menggeser seluruh isi di
          bawahnya, dan itu terjadi persis saat pengguna sedang menekan sesuatu.
          Kemajuan job pindah ke panel Start, yang tempatnya selalu ada. */}
      <Alerts items={[
        error && { kind: "error" as const, text: error },
        modelMissing && { kind: "warn" as const, key: `model-${model}`,
          // Tautan ke halaman yang bisa MEMASANGNYA, bukan perintah terminal:
          // `./setup.sh` adalah skrip pengembang yang bahkan tidak ada di
          // aplikasi Windows, dan menyuruh orang menjalankannya sama dengan
          // menyuruhnya berhenti.
          text: <>{t("modelMissingWarn", { model })} {t("modelMissingWarnTail")}{" "}
            <a href="/requirements">{t("openRequirements")}</a></> },
      ]} />

      {/* Pemilih berkas: modal, jadi ia berdiri di luar grid kolom. */}
      {picker && (
        <Picker
          mode={picker === "out" ? "folder" : "file"}
          start={picker === "out" ? outputDir : path}
          onPick={(p) => { picker === "out" ? setOutputDir(p) : setPath(p); setPicker(null); }}
          onClose={() => setPicker(null)}
        />
      )}

      {/* DUA kolom, dan hanya dua. Kiri yang dilihat & dihasilkan, kanan yang
          disetel lalu dijalankan. Kalau suatu saat ada panel yang mendarat di
          induk yang salah lagi, di sinilah kelihatannya. */}
      <div className="screen-body two">
        <div className="screen-main">
          {/* Satu panel, bukan dua: sumber duduk di kepala pratinjau sebab ia
              yang menentukan gambar di bawahnya. Judul panel ikut dibuang —
              bingkai 9:16 sudah mengatakan sendiri apa isinya. */}
          <div className="panel">
            <SourceRow
              path={path} setPath={setPath}
              outputDir={outputDir} setOutputDir={setOutputDir}
              onPick={setPicker}
              onFile={useFile} uploading={uploading} uploadPct={uploadPct}
            />
            <PreviewPanel
              path={path} reframe={reframe} background={background} zoom={zoom} zone={zone}
              fonts={fonts} subFont={subFont} setSubFont={setSubFont}
              fontManual={fontManual} setFontManual={setFontManual}
              fontCheck={fontCheck} fontChecking={fontChecking}
              subSize={subSize} setSubSize={setSubSize}
              subColor={subColor} setSubColor={setSubColor}
              subOutline={subOutline} setSubOutline={setSubOutline}
              subBox={subBox} setSubBox={setSubBox}
              subMode={subMode} setSubMode={setSubMode}
              subHighlight={subHighlight} setSubHighlight={setSubHighlight}
              subSpeed={subSpeed} setSubSpeed={setSubSpeed}
              subX={subX} setSubX={setSubX} subY={subY} setSubY={setSubY}
              blockH={blockH} centerAnchorY={centerAnchorY}
              platform={platform} setPlatform={setPlatform}
              inUnsafe={inUnsafe} onPlaceSafe={placeSafe} addLog={addLog}
            >
              {/* Kendali bingkai duduk di ujung kolom setelan pratinjau: ketiganya
                  langsung mengubah gambar di sebelah kirinya. */}
              <FramePanel
                reframe={reframe} onReframe={changeReframe}
                background={background} setBackground={setBackground}
                zoom={zoom} setZoom={setZoom}
              />
            </PreviewPanel>
          </div>

          <LogPanel logs={logs} />
        </div>

        <div className="screen-col">
          <SetupPanel
            mode={mode} setMode={setMode} model={model} setModel={setModel} models={models}
            resolution={resolution} setResolution={setResolution}
            quality={quality} setQuality={setQuality} fps={fps} setFps={setFps}
            durationPreset={durationPreset} setDurationPreset={setDurationPreset}
            maxClips={maxClips} setMaxClips={setMaxClips}
            saveMode={saveMode} setSaveMode={setSaveMode}
            claudeModel={claudeModel} setClaudeModel={setClaudeModel}
            offlineEngine={offlineEngine} setOfflineEngine={setOfflineEngine}
            ollamaModel={ollamaModel} setOllamaModel={setOllamaModel}
            transcriptFix={transcriptFix} setTranscriptFix={setTranscriptFix}
            terms={terms} setTerms={setTerms} addLog={addLog}
            testing={testing} setTesting={setTesting}
          />

          <RunPanel
            busy={busy} testing={testing} disabled={busy || testing || !path || !!modelMissing}
            cancellable={busy && !!jobId} onStart={start} onCancel={cancel}
            status={status} stage={stage} message={message} progress={progress}
          />
        </div>
      </div>
    </div>
  );
}

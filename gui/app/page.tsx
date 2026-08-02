"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useI18n } from "./i18n";

const ENGINE = process.env.NEXT_PUBLIC_ENGINE_URL || "http://127.0.0.1:8787";
const PLAY_W = 1080, PLAY_H = 1920; // ruang koordinat subtitle

type Reasons = { hook: number; emotion: number; clarity: number; shareability: number; standalone: number };
type Clip = {
  id: string; job_id: string; start: number; end: number; duration: number;
  score: number; reasons: Reasons; title?: string; hashtags?: string[]; transcript: string; status: string;
  video_path?: string; video_path_raw?: string; subtitle_srt?: string;
  transcript_txt?: string;
};
type WhisperModel = { name: string; size: string; downloaded: boolean };
type Font = { name: string };
// Hasil /api/font-check: valid = font benar-benar ada (bawaan atau sistem).
type FontCheck = { valid: boolean; name: string; family: string; source: string; error: string };

// Titik tengah bidang 9:16 + toleransi magnet (dalam koordinat 1080×1920).
// 20 ≈ 5 piksel layar pada preview 270 px, cukup terasa tanpa bikin susah
// menaruh subtitle sedikit di luar tengah.
const CENTER_X = 540, CENTER_Y = 960, MAGNET = 20;
// Baris contoh di preview — panjangnya sengaja mendekati batas nyata satu baris
// (~22 karakter pada ukuran font bawaan).
const SAMPLE_LINES = ["sampleLine1", "sampleLine2", "sampleLine3"] as const;

// Zoom dibaca RELATIF terhadap titik awal modenya, jadi batas bawah & nilai
// awalnya berbeda per mode. Harus sama dengan engine/config.
//   fit    : mulai 0 (seluruh video masuk) → hanya bisa NAIK
//   center : mulai 100 (memenuhi bingkai)   → hanya bisa TURUN
// Keduanya berhenti di 100: di situ gambar sudah memenuhi bingkai.
const ZOOM_MAX = 100, ZOOM_STEP = 5;
const ZOOM_BOUNDS: Record<string, { min: number; natural: number }> = {
  fit: { min: 0, natural: 0 },
  center: { min: 5, natural: 100 },
};
const zoomBounds = (mode: string) => ZOOM_BOUNDS[mode] ?? ZOOM_BOUNDS.center;

// Satu model Ollama terpasang, sudah dinilai engine (siap/tidak + alasannya).
type OllamaModel = {
  name: string; base: string; params: string; quant: string;
  bytes: number; context: number; ready: boolean; note: string;
};
type OllamaStatus = { running: boolean; models?: string[]; installed?: OllamaModel[] };

// Saran model bila belum ada satu pun yang terpasang.
const OLLAMA_SUGGESTED = ["qwen2.5", "llama3.1", "gemma2"];

// Nama Ollama tanpa tag: "qwen2.5:latest" → "qwen2.5". Perbandingan nama SELALU
// lewat sini — dulu dropdown membandingkan persis sehingga "qwen2.5" yang sudah
// terpasang sebagai "qwen2.5:latest" tetap dicap "perlu unduh".
const baseName = (m: string) => (m.includes(":") ? m.slice(0, m.indexOf(":")) : m);
const sameModel = (a: string, b: string) => a === b || baseName(a) === baseName(b);

const sizeGB = (b: number) => (b > 0 ? `${(b / 1e9).toFixed(1)} GB` : "");

// Area yang tertutup UI tiap aplikasi (fraksi tinggi/lebar frame 9:16).
// Angka awal hasil pengukuran kasar — sesuaikan bila UI aplikasi berubah.
type Zone = { top: number; bottom: number; right: number };
const PLATFORMS: Record<string, { label: string; zone: Zone }> = {
  tiktok: { label: "TikTok", zone: { top: 0.08, bottom: 0.20, right: 0.16 } },
  reels: { label: "Instagram Reels", zone: { top: 0.07, bottom: 0.17, right: 0.15 } },
  shorts: { label: "YouTube Shorts", zone: { top: 0.06, bottom: 0.13, right: 0.14 } },
  generic: { label: "", zone: { top: 0.08, bottom: 0.20, right: 0.16 } },
};

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

  // Mesin AI (scoring)
  const [apiKey, setApiKey] = useState("");
  const [hasKey, setHasKey] = useState(false);
  const [claudeModel, setClaudeModel] = useState("claude-haiku-4-5");
  const [offlineEngine, setOfflineEngine] = useState("ollama"); // ollama | heuristic
  const [ollamaModel, setOllamaModel] = useState("qwen2.5");
  const [ollamaStatus, setOllamaStatus] = useState<OllamaStatus | null>(null);
  const [pulling, setPulling] = useState(false);
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
  const fontFromPreset = useRef<string>("");
  const [fontManual, setFontManual] = useState(false);
  const [fontCheck, setFontCheck] = useState<FontCheck | null>(null);
  const [fontChecking, setFontChecking] = useState(false);

  // Preview
  const [previewOn, setPreviewOn] = useState(false);
  const [isDragging, setIsDragging] = useState(false); // sedang menggeser subtitle
  const [alwaysGuides, setAlwaysGuides] = useState(false); // paksa grid tetap tampil
  const [previewBusy, setPreviewBusy] = useState(false);
  const [duration, setDuration] = useState(0);
  const [previewTime, setPreviewTime] = useState(5);
  // Dinaikkan tiap muat ulang manual supaya URL frame berubah — kalau URL-nya
  // sama persis, browser memakai gambar lama meski videonya sudah ditimpa.
  const [previewNonce, setPreviewNonce] = useState(0);
  const boxRef = useRef<HTMLDivElement | null>(null);
  const draggingRef = useRef(false);

  // Upload
  const [uploading, setUploading] = useState(false);
  const [uploadPct, setUploadPct] = useState(0);
  const [dragOver, setDragOver] = useState(false);

  // Job
  const [jobId, setJobId] = useState<string | null>(null);
  const [status, setStatus] = useState("");
  const [stage, setStage] = useState("");
  const [progress, setProgress] = useState(0);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [clips, setClips] = useState<Clip[]>([]);
  const [busy, setBusy] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);
  const logRef = useRef<HTMLDivElement | null>(null);

  const locale = lang === "id" ? "id-ID" : "en-GB";
  const addLog = useCallback((text: string) => {
    setLogs((prev) => [...prev, `[${new Date().toLocaleTimeString(locale)}] ${text}`]);
  }, [locale]);

  useEffect(() => {
    fetch(`${ENGINE}/api/models`).then((r) => r.json()).then((m: WhisperModel[]) => {
      setModels(m);
      const downloaded = m.find((x) => x.downloaded);
      if (downloaded) setModel(downloaded.name);
    }).catch(() => addLog(`⚠ ${t("engineUnreachable")}`));
    fetch(`${ENGINE}/api/fonts`).then((r) => r.json()).then((f: Font[]) => {
      setFonts(f);
      // Jangan menimpa font dari preset — daftar font datang belakangan, dulu
      // pilihan tersimpan selalu tergantikan font pertama tiap halaman dimuat.
      if (f[0] && !fontFromPreset.current) setSubFont(f[0].name);
      // Font tersimpan yang bukan bawaan berarti font manual.
      else if (fontFromPreset.current && !f.some((x) => x.name === fontFromPreset.current)) setFontManual(true);
    }).catch(() => {});
  }, [addLog, t]);

  useEffect(() => { logRef.current?.scrollTo(0, logRef.current.scrollHeight); }, [logs]);

  // Font manual divalidasi di engine (format + benar-benar terpasang), ditunda
  // 400 ms supaya tidak memanggil fc-match tiap huruf yang diketik.
  useEffect(() => {
    if (!fontManual) { setFontCheck(null); return; }
    const name = subFont.trim();
    if (!name) { setFontCheck(null); return; }
    setFontChecking(true);
    const timer = setTimeout(() => {
      fetch(`${ENGINE}/api/font-check?name=${encodeURIComponent(name)}`)
        .then((r) => r.json()).then(setFontCheck)
        .catch(() => setFontCheck({ valid: false, name, family: "", source: "", error: "engine unreachable" }))
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

  // Sambung ulang ke job yang sedang berjalan (mis. setelah tab di-reload/tab baru).
  useEffect(() => {
    fetch(`${ENGINE}/api/jobs`).then((r) => r.json()).then((jobs: any[]) => {
      if (!Array.isArray(jobs)) return;
      const active = jobs
        .filter((j) => j.status === "running" || j.status === "queued")
        .sort((a, b) => (a.created_at < b.created_at ? 1 : -1))[0];
      if (active) {
        setClips(active.clips || []);
        setStatus(active.status);
        setStage(active.stage || "");
        setProgress(active.progress || 0);
        setBusy(true);
        setJobId(active.id); // memicu langganan SSE → progress lanjut terlihat
        addLog(t("logReconnect", { id: active.id }));
      }
    }).catch(() => {});
  }, [addLog, t]);

  // Status API key.
  useEffect(() => {
    fetch(`${ENGINE}/api/settings`).then((r) => r.json()).then((d) => setHasKey(!!d.has_key)).catch(() => {});
  }, []);


  // silent = pengecekan berkala; jangan kosongkan status agar UI tak berkedip.
  const checkOllama = useCallback((silent = false) => {
    if (!silent) setOllamaStatus(null);
    fetch(`${ENGINE}/api/ollama/status`).then((r) => r.json()).then(setOllamaStatus)
      .catch(() => setOllamaStatus({ running: false }));
  }, []);

  const ollamaActive = mode === "offline" && offlineEngine === "ollama";

  useEffect(() => {
    if (ollamaActive) checkOllama();
  }, [ollamaActive, checkOllama]);

  // Selama panel Ollama terbuka, status disegarkan sendiri: berkala tiap 15 detik
  // dan tiap jendela kembali fokus — jadi model yang baru di-pull dari terminal
  // langsung terbaca tanpa perlu menekan "cek ulang".
  useEffect(() => {
    if (!ollamaActive) return;
    const timer = setInterval(() => checkOllama(true), 15000);
    const onFocus = () => checkOllama(true);
    window.addEventListener("focus", onFocus);
    return () => { clearInterval(timer); window.removeEventListener("focus", onFocus); };
  }, [ollamaActive, checkOllama]);

  const ollamaInstalled = useMemo<OllamaModel[]>(() => {
    const installed = ollamaStatus?.installed;
    if (installed?.length) return installed;
    // Engine versi lama hanya mengirim daftar nama.
    return (ollamaStatus?.models || []).map((n) => ({
      name: n, base: baseName(n), params: "", quant: "", bytes: 0, context: 0, ready: true, note: "",
    }));
  }, [ollamaStatus]);

  const selectedOllama = useMemo(
    () => ollamaInstalled.find((m) => sameModel(m.name, ollamaModel)),
    [ollamaInstalled, ollamaModel],
  );

  // Auto-pilih: kalau pilihan sekarang belum terpasang, ambil model terpasang
  // yang dinilai siap lebih dulu; kalau tak ada yang siap, ambil yang pertama.
  useEffect(() => {
    if (!ollamaStatus?.running || !ollamaInstalled.length) return;
    if (ollamaInstalled.some((m) => sameModel(m.name, ollamaModel))) return;
    setOllamaModel((ollamaInstalled.find((m) => m.ready) || ollamaInstalled[0]).name);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ollamaInstalled, ollamaStatus?.running]);

  const saveKey = useCallback(async () => {
    try {
      const res = await fetch(`${ENGINE}/api/settings`, {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ anthropic_api_key: apiKey }),
      });
      const data = await res.json();
      setHasKey(!!data.has_key);
      addLog(data.has_key ? t("logKeySaved") : t("logKeyEmpty"));
      if (data.has_key) setApiKey("");
    } catch { addLog(t("logKeyFailed")); }
  }, [apiKey, addLog, t]);

  const pullModel = useCallback(async () => {
    setPulling(true);
    addLog(t("logPullStart", { model: ollamaModel }));
    try {
      const res = await fetch(`${ENGINE}/api/ollama/pull`, {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ model: ollamaModel }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "failed");
      addLog(t("logPullDone", { model: ollamaModel }));
      checkOllama();
    } catch (e: any) { addLog(t("logPullFailed", { error: e.message })); }
    finally { setPulling(false); }
  }, [ollamaModel, addLog, checkOllama, t]);

  const selectedModel = models.find((m) => m.name === model);
  const modelMissing = selectedModel && !selectedModel.downloaded;

  // reframe ikut dikirim: preview memakai mode yang sama dengan render, jadi
  // koordinat subtitle diatur di atas geometri yang benar (di mode "muat utuh"
  // videonya cuma mengisi pita tengah, bukan seluruh kanvas).
  const frameUrl = useMemo(
    () => `${ENGINE}/api/frame?path=${encodeURIComponent(path)}&t=${previewTime.toFixed(2)}`
      + `&reframe=${encodeURIComponent(reframe)}&background=${encodeURIComponent(background)}`
      + `&zoom=${zoom}`
      + `&n=${previewNonce}`,
    [path, previewTime, reframe, background, zoom, previewNonce]
  );

  // silent = dipicu otomatis oleh video baru; kegagalannya tidak perlu
  // diteriakkan ke log karena path bisa saja masih setengah diketik.
  const loadPreview = useCallback(async (silent = false) => {
    if (!path) return;
    setPreviewBusy(true);
    try {
      const res = await fetch(`${ENGINE}/api/probe?path=${encodeURIComponent(path)}`);
      const text = await res.text();
      let data: any;
      try { data = JSON.parse(text); }
      catch { throw new Error(t("errOldEngine")); }
      if (!res.ok) throw new Error(data.error || t("errReadVideo"));
      setDuration(data.duration || 0);
      setPreviewTime(Math.min(5, (data.duration || 10) / 2));
      setPreviewNonce((n) => n + 1);
      setPreviewOn(true);
    } catch (e: any) {
      if (!silent) addLog(t("logPreviewFailed", { error: e.message }));
    } finally { setPreviewBusy(false); }
  }, [path, addLog, t]);

  // Kembali ke keadaan awal: frame dilepas, durasi & waktu direset. Dipakai
  // tombol "reset" dan otomatis saat videonya berganti.
  const resetPreview = useCallback(() => {
    setPreviewOn(false);
    setPreviewBusy(false);
    setDuration(0);
    setPreviewTime(5);
  }, []);

  // Video baru masuk (unggah, seret-lepas, atau path diketik) → preview lama
  // dibuang lalu dimuat ulang sendiri. Ditunda 500 ms supaya path yang sedang
  // diketik tidak memicu satu permintaan per huruf.
  useEffect(() => {
    resetPreview();
    if (!path) return;
    const timer = setTimeout(() => loadPreview(true), 500);
    return () => clearTimeout(timer);
  }, [path, resetPreview, loadPreview]);

  // Geser subtitle di atas frame.
  //
  // Selisih titik pegang disimpan saat tombol ditekan, lalu ikut dijumlahkan
  // tiap gerakan. Dulu posisi langsung disamakan dengan kursor, jadi teks
  // melompat ke bawah kursor begitu disentuh — itu yang terasa "kurang stabil".
  const grab = useRef({ dx: 0, dy: 0 });

  const previewPoint = useCallback((e: React.PointerEvent) => {
    const rect = boxRef.current!.getBoundingClientRect();
    return {
      x: ((e.clientX - rect.left) / rect.width) * PLAY_W,
      y: ((e.clientY - rect.top) / rect.height) * PLAY_H,
    };
  }, []);

  // Berapa baris yang mungkin muncul sekaligus — harus sama dengan Pacing() di
  // engine (lambat/normal 2, padat 3; mode word selalu 1 kata per layar).
  const maxLines = subMode === "word" ? 1 : subSpeed === "dense" ? 3 : 2;
  // Tinggi blok di ruang 1080x1920. Faktor 1,17 diukur dari render libass yang
  // sebenarnya (jarak antarbaris pada ukuran font 72 = 84 px).
  const blockH = Math.round(subSize * 1.17 * maxLines);
  // Jangkar ada di tepi atas blok, jadi supaya blok terlihat di tengah bidang
  // titiknya harus setengah tinggi blok di atas garis tengah.
  const centerAnchorY = Math.round(CENTER_Y - blockH / 2);

  const startDrag = useCallback((e: React.PointerEvent) => {
    if (!boxRef.current) return;
    e.preventDefault();
    const p = previewPoint(e);
    grab.current = { dx: subX - p.x, dy: subY - p.y };
    draggingRef.current = true;
    setIsDragging(true);
    e.currentTarget.setPointerCapture?.(e.pointerId);
  }, [subX, subY, previewPoint]);

  const onPointerMove = useCallback((e: React.PointerEvent) => {
    if (!draggingRef.current || !boxRef.current) return;
    const p = previewPoint(e);
    let nx = p.x + grab.current.dx;
    let ny = p.y + grab.current.dy;
    // Magnet ke garis tengah — inilah rasa "berhenti" saat melewati tengah.
    if (Math.abs(nx - CENTER_X) < MAGNET) nx = CENTER_X;
    if (Math.abs(ny - centerAnchorY) < MAGNET) ny = centerAnchorY;
    setSubX(Math.round(Math.max(0, Math.min(PLAY_W, nx))));
    // Batas bawah dikurangi tinggi blok: yang harus tetap di dalam bingkai
    // adalah seluruh blok, bukan cuma titik jangkarnya.
    setSubY(Math.round(Math.max(0, Math.min(PLAY_H - blockH, ny))));
  }, [previewPoint, blockH, centerAnchorY]);

  const endDrag = useCallback(() => {
    draggingRef.current = false;
    setIsDragging(false);
  }, []);

  const atCenterX = subX === CENTER_X;
  const atCenterY = subY === centerAnchorY;
  const guidesVisible = isDragging || alwaysGuides;

  const uploadFile = useCallback((file: File) => {
    setUploading(true); setUploadPct(0);
    addLog(t("logUploadStart", { name: file.name, size: (file.size / 1e6).toFixed(0) }));
    const form = new FormData(); form.append("file", file);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `${ENGINE}/api/upload`);
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

  const start = useCallback(async () => {
    // Font manual yang belum lolos pengecekan ditolak di sini — kalau diteruskan,
    // libass diam-diam mengganti fontnya dan hasil render tidak sesuai preview.
    if (fontManual && !fontCheck?.valid) {
      addLog(t("logFontInvalid", { font: subFont }) + (fontCheck?.error ? ` (${fontCheck.error})` : ""));
      return;
    }
    setError(""); setClips([]); setProgress(0); setStage(""); setMessage("");
    setBusy(true); setStatus("queued"); setJobId(null);
    addLog(t("logJobStart", { resolution, quality, duration: durationPreset, font: subFont }));
    try {
      const res = await fetch(`${ENGINE}/api/jobs`, {
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
    await fetch(`${ENGINE}/api/jobs/${jobId}/cancel`, { method: "POST" }).catch(() => {});
  }, [jobId, addLog, t]);

  useEffect(() => {
    if (!jobId) return;
    const events = new EventSource(`${ENGINE}/api/jobs/${jobId}/events`);
    events.addEventListener("progress", (e: MessageEvent) => {
      const d = JSON.parse(e.data);
      if (d.status) setStatus(d.status);
      if (d.stage) setStage(d.stage);
      if (typeof d.progress === "number") setProgress(d.progress);
      if (d.message) { setMessage(d.message); addLog(`${d.stage || ""}: ${d.message}`); }
    });
    events.addEventListener("clip", (e: MessageEvent) => {
      const clip: Clip = JSON.parse(e.data);
      setClips((prev) => (prev.find((x) => x.id === clip.id) ? prev : [...prev, clip]));
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

  const scoreClass = (s: number) => (s >= 70 ? "high" : s >= 50 ? "mid" : "low");
  const formatTime = (s: number) => `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, "0")}`;
  const hex = (c: string) => (c === "yellow" ? "#ffdd00" : c === "green" ? "#4ade80" : c === "cyan" ? "#38bdf8" : "#ffffff");
  const colorHex = hex(subColor);
  const highlightHex = hex(subHighlight);

  // Font yang di-@font-face untuk preview: bawaan + font manual yang lolos cek.
  const previewFonts = useMemo(() => {
    const names = fonts.map((f) => f.name);
    if (fontManual && fontCheck?.valid && !names.includes(fontCheck.family)) names.push(fontCheck.family);
    return names;
  }, [fonts, fontManual, fontCheck]);

  // Mulai zoom 100 gambar menutupi bingkai di kedua mode, jadi latarnya tidak
  // akan terlihat.
  const noEmptySpace = zoom >= 100;
  const bounds = zoomBounds(reframe);

  // Angka zoom artinya berbeda per mode, jadi membawanya menyeberang saat mode
  // berganti hanya menghasilkan nilai yang tak berarti. Disetel ke titik awal
  // mode barunya.
  const changeReframe = useCallback((mode: string) => {
    setReframe(mode);
    setZoom(zoomBounds(mode).natural);
  }, []);

  const platformLabel = (key: string) =>
    key === "generic" ? t("platformGeneric") : PLATFORMS[key]?.label || key;

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
    <div className="wrap">
      {/* Muat font asli agar preview akurat — termasuk font manual yang lolos cek */}
      <style dangerouslySetInnerHTML={{ __html: previewFonts.map((n) =>
        `@font-face{font-family:"${n}";src:url("${ENGINE}/api/font-file?name=${encodeURIComponent(n)}");font-display:swap;}`
      ).join("") }} />
      <h1>✂️ Clipper</h1>
      <p className="sub">{t("brandTagline")}</p>

      {/* 1. Sumber */}
      <div className="panel">
        <div className={`dropzone ${dragOver ? "over" : ""}`}
          onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
          onDragLeave={() => setDragOver(false)}
          onDrop={(e) => { e.preventDefault(); setDragOver(false); const f = e.dataTransfer.files?.[0]; if (f) uploadFile(f); }}>
          {uploading ? (
            <>
              <div>{t("uploadingPct", { pct: Math.round(uploadPct * 100) })}</div>
              <div className="progress-outer" style={{ marginTop: 8 }}><div className="progress-inner" style={{ width: `${uploadPct * 100}%` }} /></div>
            </>
          ) : (
            <>
              <div><strong>{t("dropTitle")}</strong></div>
              <div className="meta">{t("dropOr")} <label className="linklike">{t("dropPick")}
                <input type="file" accept="video/*" style={{ display: "none" }} onChange={(e) => e.target.files?.[0] && uploadFile(e.target.files[0])} />
              </label> {t("dropOrPaste")}</div>
            </>
          )}
        </div>
        <div className="field">
          <label>{t("videoPath")}</label>
          <input value={path} onChange={(e) => setPath(e.target.value)} placeholder="/home/user/video.mp4" />
        </div>
        <div className="field">
          <label>{t("outputDir")}</label>
          <input value={outputDir} onChange={(e) => setOutputDir(e.target.value)} placeholder={t("outputDirPlaceholder")} />
        </div>
      </div>

      {/* 2. Setelan render — dikelompokkan menurut pertanyaan yang dijawabnya:
          dengan apa diproses, seperti apa hasilnya, bagaimana masuk bingkai,
          dan berapa banyak klip. */}
      <div className="panel">
        <div className="meta" style={{ marginBottom: 14 }}>{t("renderSettings")}</div>

        <div className="group">
          <div className="group-title">{t("groupEngine")}</div>
          <div className="row">
            <div className="field"><label>{t("mode")}</label>
              <select value={mode} onChange={(e) => setMode(e.target.value)}>
                <option value="offline">{t("modeOffline")}</option>
                <option value="hybrid">{t("modeHybrid")}</option>
              </select></div>
            <div className="field"><label>{t("whisperModel")}</label>
              <select value={model} onChange={(e) => setModel(e.target.value)}>
                {models.map((m) => <option key={m.name} value={m.name}>{m.name} {m.size} {m.downloaded ? "✓" : `✗ ${t("modelNotDownloaded")}`}</option>)}
              </select></div>
          </div>
        </div>

        <div className="group">
          <div className="group-title">{t("groupQuality")}</div>
          <div className="row">
            <div className="field"><label>{t("resolution")}</label>
              <select value={resolution} onChange={(e) => setResolution(e.target.value)}>
                <option value="720p">720p (HD)</option>
                <option value="1080p">1080p (Full HD)</option>
                <option value="1440p">1440p (2K)</option>
              </select></div>
            <div className="field"><label>{t("quality")}</label>
              <select value={quality} onChange={(e) => setQuality(e.target.value)}>
                <option value="draft">{t("qualityDraft")}</option>
                <option value="hd">{t("qualityHd")}</option>
                <option value="max">{t("qualityMax")}</option>
              </select></div>
            <div className="field"><label title={t("fpsTip")}>{t("fps")} ⓘ</label>
              <select value={fps} onChange={(e) => setFps(Number(e.target.value))}>
                <option value={0}>{t("fpsSource")}</option>
                <option value={24}>24</option>
                <option value={30}>30</option>
                <option value={60}>60</option>
              </select></div>
          </div>
        </div>


        <div className="group">
          <div className="group-title">{t("groupClips")}</div>
          <div className="row">
            <div className="field"><label title={t("clipDurationTip")}>{t("clipDuration")} ⓘ</label>
              <select value={durationPreset} onChange={(e) => setDurationPreset(e.target.value)}>
                <option value="auto">{t("durationAuto")}</option>
                <option value="30">{t("durationAbout", { n: "30s" })}</option>
                <option value="60">{t("durationAbout", { n: "60s" })}</option>
                <option value="90">{t("durationAbout", { n: "90s" })}</option>
                <option value="120">{t("durationAbout", { n: "2 min" })}</option>
                <option value="180">{t("durationAbout", { n: "3 min" })}</option>
              </select></div>
            <div className="field"><label title={t("maxClipsTip")}>{t("maxClips")} ⓘ</label>
              <input type="number" min={1} max={50} value={maxClips} onChange={(e) => setMaxClips(Number(e.target.value))} /></div>
            <div className="field"><label title={t("saveClipsTip")}>{t("saveClips")} ⓘ</label>
              <select value={saveMode} onChange={(e) => setSaveMode(e.target.value)}>
                <option value="burn">{t("saveBurn")}</option>
                <option value="clean">{t("saveClean")}</option>
                <option value="both">{t("saveBoth")}</option>
              </select></div>
          </div>
        </div>
      </div>

      {/* 2b. Mesin AI (scoring) — berubah menurut mode */}
      <div className="panel">
        <div className="meta" style={{ marginBottom: 10 }}>{t("aiEngine", { mode })}</div>
        {mode === "hybrid" ? (
          <>
            <div className="field">
              <label>{t("apiKeyClaude")} {hasKey && <span className="ok">{t("keyStored")}</span>}</label>
              <div style={{ display: "flex", gap: 8 }}>
                <input type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)}
                  placeholder={hasKey ? t("keyPlaceholderStored") : "sk-ant-..."} />
                <button onClick={saveKey} disabled={!apiKey}>{t("save")}</button>
              </div>
            </div>
            <div className="field">
              <label>{t("claudeModel")}</label>
              <select value={claudeModel} onChange={(e) => setClaudeModel(e.target.value)}>
                <option value="claude-haiku-4-5">{t("claudeHaiku")}</option>
                <option value="claude-sonnet-5">{t("claudeSonnet")}</option>
                <option value="claude-opus-4-8">{t("claudeOpus")}</option>
              </select>
            </div>
            {!hasKey && <div className="warnbox">{t("noKeyWarning")}</div>}
          </>
        ) : (
          <>
            <div className="field">
              <label>{t("offlineEngine")}</label>
              <select value={offlineEngine} onChange={(e) => setOfflineEngine(e.target.value)}>
                <option value="ollama">{t("offlineOllama")}</option>
                <option value="heuristic">{t("offlineHeuristic")}</option>
              </select>
              <div className="meta">{t("engineNoFallback")}</div>
            </div>
            {offlineEngine === "ollama" && (
              <>
                <div className="field">
                  <label>{t("localModel")}</label>
                  <select value={ollamaModel} onChange={(e) => setOllamaModel(e.target.value)}>
                    {ollamaInstalled.map((m) => {
                      const specs = [m.params, m.quant, sizeGB(m.bytes)].filter(Boolean).join(" · ");
                      return (
                        <option key={m.name} value={m.name}>
                          {m.name}{specs ? ` — ${specs}` : ""} {m.ready ? t("modelReady") : t("modelNotCapable")}
                        </option>
                      );
                    })}
                    {/* Saran hanya muncul bila belum terpasang — dicek per nama dasar
                        supaya "qwen2.5" tidak tampil ganda dengan "qwen2.5:latest". */}
                    {OLLAMA_SUGGESTED.filter((s) => !ollamaInstalled.some((m) => sameModel(m.name, s))).map((s) => (
                      <option key={s} value={s}>{s} ({t("modelNeedsDownload")})</option>
                    ))}
                  </select>
                </div>
                <div className="meta">
                  {ollamaStatus === null ? t("checkingOllama")
                    : !ollamaStatus.running ? (
                      <span className="warn">{t("ollamaNotDetected")} <code>ollama serve</code>. <button className="ghost tiny" onClick={() => checkOllama()}>{t("recheck")}</button></span>
                    ) : !selectedOllama ? (
                      <span className="warn">{t("ollamaModelMissing", { model: ollamaModel })} <button className="ghost tiny" onClick={pullModel} disabled={pulling}>{pulling ? t("downloading") : t("downloadModel")}</button></span>
                    ) : selectedOllama.ready ? (
                      <span className="ok">{t("ollamaReady", { model: selectedOllama.name })}{selectedOllama.params ? ` (${selectedOllama.params})` : ""}</span>
                    ) : (
                      <span className="warn">{t("ollamaModelWeak", { model: selectedOllama.name, note: selectedOllama.note })} <button className="ghost tiny" onClick={pullModel} disabled={pulling}>{pulling ? t("downloading") : t("downloadSelected")}</button></span>
                    )}
                  {ollamaStatus?.running && !ollamaInstalled.length && t("noModelsInstalled")}
                </div>
              </>
            )}
          </>
        )}

        {/* Koreksi transkrip berlaku di kedua mode, jadi ditaruh di luar
            percabangan hybrid/offline. Ia memakai LLM juga saat mesin skornya
            heuristik — peringatannya ditampilkan tepat di sini supaya tidak
            mengagetkan saat job berhenti. */}
        <div className="field" style={{ marginTop: 4 }}>
          <label className="chk" title={t("transcriptFixTip")}>
            <input type="checkbox" checked={transcriptFix}
              onChange={(e) => setTranscriptFix(e.target.checked)} /> {t("transcriptFix")} ⓘ
          </label>
          {transcriptFix && (
            <>
              <div className="meta">{t("transcriptFixNote")}</div>
              {mode !== "hybrid" && offlineEngine === "heuristic" && (
                <div className="warn" style={{ marginTop: 6 }}>⚠ {t("transcriptFixNeedsLLM")}</div>
              )}
              {/* Daftar istilah menempel di bawah koreksi transkrip karena hanya
                  tahap itu yang memakainya — tanpa centang di atas, isian ini
                  tidak berpengaruh apa pun. */}
              <div style={{ marginTop: 10 }}>
                <label htmlFor="terms">{t("terms")}</label>
                <input id="terms" type="text" value={terms}
                  placeholder={t("termsPlaceholder")}
                  onChange={(e) => setTerms(e.target.value)} />
                <div className="meta" style={{ marginTop: 6 }}>{t("termsNote")}</div>
              </div>
            </>
          )}
        </div>
      </div>

      {/* 3. Setelan subtitle + preview geser */}
      <div className="panel">
        <div className="meta" style={{ marginBottom: 14 }}>{t("clipAppearance")}</div>

        <div className="sub-layout">
        {/* Kiri: bingkai preview. Bingkainya selalu ada — walau frame video
            belum dimuat — supaya posisi subtitle tetap bisa diatur lebih dulu. */}
        <div className="sub-preview">
          <div className="preview9x16" ref={boxRef}>
            {previewOn ? (
              /* eslint-disable-next-line @next/next/no-img-element */
              <img src={frameUrl} alt="preview" draggable={false} />
            ) : (
              <div className="preview-empty">
                <div className="pe-icon" aria-hidden="true" />
                <div className="pe-title">{t("emptyFrame")}</div>
              </div>
            )}
            {zone && (
              <>
                <div className="safezone" style={{ top: 0, left: 0, right: 0, height: `${zone.top * 100}%` }}>
                  <span>{t("zoneTop")}</span>
                </div>
                <div className="safezone" style={{ bottom: 0, left: 0, right: 0, height: `${zone.bottom * 100}%` }}>
                  <span>{t("zoneBottom")}</span>
                </div>
                <div className="safezone" style={{
                  top: `${zone.top * 100}%`, bottom: `${zone.bottom * 100}%`,
                  right: 0, width: `${zone.right * 100}%`,
                }}>
                  <span className="vert">{t("zoneRight")}</span>
                </div>
              </>
            )}
            {/* Garis tengah: muncul saat digeser (atau dikunci lewat centang).
                Kelas "on" = subtitle sedang menempel di garis itu. */}
            {guidesVisible && (
              <>
                <div className={`guide v${atCenterX ? " on" : ""}`} />
                <div className={`guide h${atCenterY ? " on" : ""}`} />
                {atCenterX && atCenterY && <div className="guide xy" />}
              </>
            )}
            <div className="suboverlay"
              style={{
                left: `${(subX / PLAY_W) * 100}%`, top: `${(subY / PLAY_H) * 100}%`,
                fontFamily: `"${subFont}", sans-serif`,
                fontSize: `calc(${subSize / PLAY_H} * var(--pvh))`,
                color: colorHex,
                background: subBox ? "rgba(0,0,0,0.6)" : "transparent",
                borderRadius: subBox ? 8 : 0,
                padding: subBox ? "4px 14px" : "2px 8px",
                textShadow: subBox ? "none" : (subOutline > 0
                  ? "-2px -2px 0 #000,2px -2px 0 #000,-2px 2px 0 #000,2px 2px 0 #000,0 0 4px #000"
                  : "none"),
              }}
              onPointerDown={startDrag}
              onPointerMove={onPointerMove}
              onPointerUp={endDrag}
              onPointerCancel={endDrag}>
              {/* Contoh ditampilkan sebanyak baris terbanyak yang mungkin
                  muncul, bukan satu baris pendek. Dulu preview selalu 1 baris
                  sehingga pengguna menaruhnya pas, lalu terkejut waktu hasilnya
                  2 baris. */}
              {subMode === "word" ? (
                <div style={{ color: highlightHex }}>{t("sampleWord")}</div>
              ) : (
                Array.from({ length: maxLines }, (_, i) => {
                  const line = t(SAMPLE_LINES[i]);
                  const isLast = i === maxLines - 1;
                  if (subMode !== "karaoke" || !isLast) return <div key={i}>{line}</div>;
                  // Karaoke: kata terakhir yang sedang disorot.
                  const words = line.split(" ");
                  const tail = words.pop();
                  return (
                    <div key={i}>
                      {words.join(" ")} <span style={{ color: highlightHex }}>{tail}</span>
                    </div>
                  );
                })
              )}
            </div>
          </div>

          {/* Kendali preview ditaruh DI BAWAH gambar: tombolnya mengubah gambar
              itu, jadi urutan bacanya lebih masuk akal daripada di atas. */}
          <div className="preview-actions">
            {!previewOn ? (
              <>
                <button className="ghost" disabled={!path || previewBusy} onClick={() => loadPreview()}>
                  {previewBusy ? t("loadingPreview") : t("loadPreview")}
                </button>
                <p className="meta" style={{ margin: 0 }}>
                  {path ? t("previewHintVideo") : t("previewHintNoVideo")}
                </p>
              </>
            ) : (
              <>
                <button className="ghost tiny" disabled={previewBusy} onClick={() => loadPreview()}>
                  {previewBusy ? t("loading") : t("reloadPreview")}
                </button>
                <button className="ghost tiny" onClick={resetPreview}>{t("resetPreview")}</button>
                <label className="chk"><input type="checkbox" checked={alwaysGuides}
                  onChange={(e) => setAlwaysGuides(e.target.checked)} /> {t("guidesAlways")}</label>
              </>
            )}
          </div>

          {previewOn && (
            <>
              <div className="field">
                <label>{t("previewTime", { t: previewTime.toFixed(1) })}</label>
                <input type="range" min={0} max={Math.max(1, Math.floor(duration))} step={1}
                  value={previewTime} onChange={(e) => setPreviewTime(Number(e.target.value))} />
              </div>
              <div className="meta">
                {t("previewNote", { zoom })}
              </div>
            </>
          )}
        </div>

        {/* Kanan: setelan, dikelompokkan menurut apa yang diubahnya —
            rupa huruf, ketebalan, lalu perilaku, lalu batas platform. */}
        <div className="sub-settings">
          <div className="row">
            <div className="field"><label>{t("font")}</label>
              <select
                value={fontManual ? "__manual__" : subFont}
                onChange={(e) => {
                  if (e.target.value === "__manual__") { setFontManual(true); return; }
                  setFontManual(false); setSubFont(e.target.value);
                }}>
                {fonts.map((f) => <option key={f.name} value={f.name}>{f.name}</option>)}
                <option value="__manual__">{t("fontOther")}</option>
              </select>
              {fontManual && (
                <>
                  <input style={{ marginTop: 6 }} value={subFont} spellCheck={false}
                    placeholder={t("fontPlaceholder")} onChange={(e) => setSubFont(e.target.value)} />
                  <div className="meta">
                    {fontChecking ? t("fontChecking")
                      : !fontCheck ? t("fontHint")
                      : fontCheck.valid ? <span className="ok">{t("fontFound", { family: fontCheck.family, source: fontCheck.source })}</span>
                      : <span className="warn">⚠ {fontCheck.error}</span>}
                  </div>
                </>
              )}</div>
            <div className="field"><label>{t("color")}</label>
              <select value={subColor} onChange={(e) => setSubColor(e.target.value)}>
                <option value="white">{t("colorWhite")}</option>
                <option value="yellow">{t("colorYellow")}</option>
              </select></div>
            <div className="field"><label>{t("effect")}</label>
              <label className="chk"><input type="checkbox" checked={subBox} onChange={(e) => setSubBox(e.target.checked)} /> {t("boxBackground")}</label>
            </div>
          </div>

          {/* Dua penggeser ditumpuk memenuhi lebar: penggeser butuh jarak
              tempuh panjang agar nilainya bisa diatur dengan halus. */}
          <div className="field"><label>{t("size", { n: subSize })}</label>
            <input type="range" min={40} max={140} value={subSize} onChange={(e) => setSubSize(Number(e.target.value))} /></div>
          <div className="field"><label>{t("outline", { n: subOutline })}</label>
            <input type="range" min={0} max={12} value={subOutline} onChange={(e) => setSubOutline(Number(e.target.value))} /></div>

          <div className="row">
            <div className="field"><label title={t("subStyleTip")}>{t("subStyle")} ⓘ</label>
              <select value={subMode} onChange={(e) => setSubMode(e.target.value)}>
                <option value="normal">{t("subNormal")}</option>
                <option value="karaoke">{t("subKaraoke")}</option>
                <option value="word">{t("subWord")}</option>
              </select></div>
            {subMode !== "normal" && (
              <div className="field"><label>{t("highlightColor")}</label>
                <select value={subHighlight} onChange={(e) => setSubHighlight(e.target.value)}>
                  <option value="yellow">{t("colorYellow")}</option>
                  <option value="white">{t("colorWhite")}</option>
                  <option value="green">{t("colorGreen")}</option>
                  <option value="cyan">{t("colorCyan")}</option>
                </select></div>
            )}
            <div className="field"><label title={t("subSpeedTip")}>{t("subSpeed")} ⓘ</label>
              <select value={subSpeed} onChange={(e) => setSubSpeed(e.target.value)}>
                <option value="slow">{t("speedSlow")}</option>
                <option value="normal">{t("speedNormal")}</option>
                <option value="dense">{t("speedDense")}</option>
              </select></div>
            <div className="field"><label>{t("position")}</label>
              <div className="position-value">x={subX} y={subY}
                <button className="ghost tiny" onClick={() => { setSubX(CENTER_X); setSubY(CENTER_Y); }}>{t("resetCentre")}</button>
              </div></div>
          </div>

          {/* Pembatas platform dan "taruh di area aman" digabung: keduanya soal
              batas yang sama, dan tombolnya tak berarti apa-apa tanpa platform
              yang sedang dipilih di sebelahnya. */}
          <div className="field"><label title={t("platformGuideTip")}>{t("platformGuide")} ⓘ</label>
            <div className="bounds">
              <select value={platform} onChange={(e) => setPlatform(e.target.value)}>
                {Object.keys(PLATFORMS).map((k) => <option key={k} value={k}>{platformLabel(k)}</option>)}
                <option value="off">{t("platformNone")}</option>
              </select>
              <button className="ghost tiny" disabled={!zone} onClick={placeSafe}>{t("placeSafe")}</button>
            </div>
            {inUnsafe && <div className="warn" style={{ marginTop: 6 }}>{t("unsafeWarning", { platform: platformLabel(platform) })}</div>}
          </div>

          {/* Cara video dipasang ke bingkai ditaruh DI SINI, bukan di panel
              setelan render: ketiganya langsung mengubah gambar di preview
              sebelah kiri, jadi hasilnya terlihat saat itu juga. */}
          <div className="group" style={{ marginTop: 18, marginBottom: 0 }}>
            <div className="group-title">{t("groupVideoInFrame")}</div>
            <div className="row">
              {/* Tiga cara memasangkan video ke bingkai 9:16 — pilihan yang
                  berdiri sendiri, bukan titik pada satu sumbu. Mode "Whole
                  Picture" itulah alasan pilihan latar di sebelahnya ada. */}
              <div className="field"><label title={t("fitModeTip")}>{t("fitMode")} ⓘ</label>
                <select value={reframe} onChange={(e) => changeReframe(e.target.value)}>
                  <option value="center">{t("fitCenter")}</option>
                  <option value="fit">{t("fitWhole")}</option>
                  <option value="face_follow" disabled>{t("fitFace")}</option>
                </select></div>
              <div className="field"><label title={t("backgroundTip")}>{t("background")} ⓘ</label>
                <select value={background} onChange={(e) => setBackground(e.target.value)}
                  disabled={noEmptySpace}>
                  <option value="blur">{t("backgroundBlur")}</option>
                  <option value="black">{t("backgroundBlack")}</option>
                </select></div>
            </div>

            {/* Zoom berdiri sendiri dari pilihan di atas: ia mengatur seberapa
                besar video duduk di dalam bingkai, kelipatan 5%. */}
            <div className="field">
              <label title={t("zoomTip")}>{t("zoomLabel", { n: zoom })} ⓘ</label>
              <input type="range" min={bounds.min} max={ZOOM_MAX} step={ZOOM_STEP}
                value={zoom} onChange={(e) => setZoom(Number(e.target.value))} />
              <div className="zoom-ends">
                <span>{reframe === "fit" ? t("zoomEndWhole") : t("zoomEndSmall")}</span>
                <span>{t("zoomEndFull")}</span>
              </div>
            </div>
            {/* Satu keterangan saja pada satu waktu — dua sekaligus bikin panel
                ramai dan yang satu selalu menjelaskan keadaan yang sudah lewat. */}
            {zoom >= 100 ? (
              <div className="meta">{t("zoomFullNote")}</div>
            ) : reframe === "fit" && zoom === 0 ? (
              <div className="meta">{t("fitWholeNote")}</div>
            ) : reframe === "fit" ? (
              <div className="meta">{t("fitWholeCropNote")}</div>
            ) : null}
          </div>
        </div>
        </div>
      </div>

      {modelMissing && (
        <div className="warnbox">{t("modelMissingWarn", { model })} <code> ./setup.sh {model}</code> {t("modelMissingWarnTail")}</div>
      )}

      <div className="panel">
        <div style={{ display: "flex", gap: 10 }}>
          <button onClick={start} disabled={busy || !path || !!modelMissing}>{busy ? t("processing") : t("start")}</button>
          {busy && jobId && <button className="ghost" onClick={cancel}>{t("cancel")}</button>}
        </div>
      </div>

      {((status && status !== "queued") || busy) && (
        <div className="panel">
          <div className="progress-outer"><div className="progress-inner" style={{ width: `${Math.round(progress * 100)}%` }} /></div>
          <div className="stage">{status === "done" ? t("statusDone") : status === "error" ? t("statusStopped") : `${stage} — ${message}`} ({Math.round(progress * 100)}%)</div>
          {error && <div className="err">⚠ {error}</div>}
        </div>
      )}

      {logs.length > 0 && (
        <div className="panel">
          <div className="meta" style={{ marginBottom: 6 }}>{t("log")}</div>
          <div className="logbox" ref={logRef}>{logs.map((l, i) => <div key={i}>{l}</div>)}</div>
        </div>
      )}

      {clips.length > 0 && (
        <div className="panel">
          <h3 style={{ marginTop: 0 }}>{t("clipCount", { n: clips.length })}</h3>
          <div className="clips">
            {clips.slice().sort((a, b) => b.score - a.score).map((c) => (
              <div className="clip" key={c.id}>
                <video src={`${ENGINE}/api/jobs/${c.job_id}/clips/${c.id}/file`} controls preload="metadata" />
                <div className="body">
                  <span className={`score ${scoreClass(c.score)}`}>{c.score}</span><span className="meta"> /100</span>
                  <div className="title">{c.title || t("noTitle")}</div>
                  <div className="meta">{formatTime(c.start)}–{formatTime(c.end)} · {Math.round(c.duration)}s</div>
                  <div className="reasons">
                    {t("reasonHook")} {c.reasons.hook} · {t("reasonEmotion")} {c.reasons.emotion} · {t("reasonClarity")} {c.reasons.clarity} · {t("reasonShare")} {c.reasons.shareability} · {t("reasonStandalone")} {c.reasons.standalone}
                  </div>
                  {c.hashtags?.map((h) => <span className="tag" key={h}>{h}</span>)}
                  <div style={{ marginTop: 8, display: "flex", gap: 8, flexWrap: "wrap" }}>
                    <a className="dl" href={`${ENGINE}/api/jobs/${c.job_id}/clips/${c.id}/file`} download>
                      ⬇ {c.video_path_raw && c.video_path_raw !== c.video_path ? t("downloadWithSubs") : t("downloadPlain")}
                    </a>
                    {c.video_path_raw && c.video_path_raw !== c.video_path && (
                      <a className="dl" href={`${ENGINE}/api/jobs/${c.job_id}/clips/${c.id}/file?variant=clean`} download>⬇ {t("downloadClean")}</a>
                    )}
                    {c.transcript_txt && (
                      <a className="dl" href={`${ENGINE}/api/jobs/${c.job_id}/clips/${c.id}/file?variant=txt`}
                         download title={t("downloadTxtTip")}>⬇ .txt</a>
                    )}
                    {c.subtitle_srt && (
                      <a className="dl" href={`${ENGINE}/api/jobs/${c.job_id}/clips/${c.id}/file?variant=srt`} download>⬇ .srt</a>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

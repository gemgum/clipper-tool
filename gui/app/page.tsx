"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

const ENGINE = process.env.NEXT_PUBLIC_ENGINE_URL || "http://127.0.0.1:8787";
const PLAY_W = 1080, PLAY_H = 1920; // ruang koordinat subtitle

type Reasons = { hook: number; emotion: number; clarity: number; shareability: number; standalone: number };
type Clip = {
  id: string; job_id: string; start: number; end: number; duration: number;
  score: number; reasons: Reasons; title?: string; hashtags?: string[]; transcript: string; status: string;
  video_path?: string; video_path_raw?: string; subtitle_srt?: string;
};
type Model = { name: string; size: string; downloaded: boolean };
type Font = { name: string };
// Hasil /api/font-check: valid = font benar-benar ada (bawaan atau sistem).
type FontCheck = { valid: boolean; name: string; family: string; source: string; error: string };

// Titik tengah bidang 9:16 + toleransi magnet (dalam koordinat 1080×1920).
// 20 ≈ 5 piksel layar pada preview 270 px, cukup terasa tanpa bikin susah
// menaruh subtitle sedikit di luar tengah.
const TENGAH_X = 540, TENGAH_Y = 960, MAGNET = 20;

// Satu model Ollama terpasang, sudah dinilai engine (siap/tidak + alasannya).
type OllamaModel = {
  name: string; base: string; params: string; quant: string;
  bytes: number; context: number; ready: boolean; note: string;
};
type OllamaStatus = { running: boolean; models?: string[]; installed?: OllamaModel[] };

// Saran model bila belum ada satu pun yang terpasang.
const OLLAMA_SARAN = ["qwen2.5", "llama3.1", "gemma2"];

// Nama Ollama tanpa tag: "qwen2.5:latest" → "qwen2.5". Perbandingan nama SELALU
// lewat sini — dulu dropdown membandingkan persis sehingga "qwen2.5" yang sudah
// terpasang sebagai "qwen2.5:latest" tetap dicap "perlu unduh".
const baseName = (m: string) => (m.includes(":") ? m.slice(0, m.indexOf(":")) : m);
const samaModel = (a: string, b: string) => a === b || baseName(a) === baseName(b);

const ukuranGB = (b: number) => (b > 0 ? `${(b / 1e9).toFixed(1)} GB` : "");

// Area yang tertutup UI tiap aplikasi (fraksi tinggi/lebar frame 9:16).
// Angka awal hasil pengukuran kasar — sesuaikan bila UI aplikasi berubah.
type Zone = { top: number; bottom: number; right: number };
const PLATFORMS: Record<string, { label: string; zone: Zone }> = {
  tiktok: { label: "TikTok", zone: { top: 0.08, bottom: 0.20, right: 0.16 } },
  reels: { label: "Instagram Reels", zone: { top: 0.07, bottom: 0.17, right: 0.15 } },
  shorts: { label: "YouTube Shorts", zone: { top: 0.06, bottom: 0.13, right: 0.14 } },
  umum: { label: "Umum (paling aman)", zone: { top: 0.08, bottom: 0.20, right: 0.16 } },
};

export default function Home() {
  const [path, setPath] = useState("");
  const [outputDir, setOutputDir] = useState("");
  const [mode, setMode] = useState("offline");
  const [model, setModel] = useState("base");
  const [resolution, setResolution] = useState("1080p");
  const [quality, setQuality] = useState("hd");
  const [reframe, setReframe] = useState("center");
  const [fps, setFps] = useState(0);

  // Mesin AI (scoring)
  const [apiKey, setApiKey] = useState("");
  const [hasKey, setHasKey] = useState(false);
  const [claudeModel, setClaudeModel] = useState("claude-haiku-4-5");
  const [offlineEngine, setOfflineEngine] = useState("ollama"); // ollama | heuristic
  const [ollamaModel, setOllamaModel] = useState("qwen2.5");
  const [ollamaStatus, setOllamaStatus] = useState<OllamaStatus | null>(null);
  const [pulling, setPulling] = useState(false);
  const [durationPreset, setDurationPreset] = useState("auto");
  const [maxClips, setMaxClips] = useState(10);
  const [models, setModels] = useState<Model[]>([]);

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
  const [subSpeed, setSubSpeed] = useState("normal");     // lambat | normal | padat
  const [platform, setPlatform] = useState("tiktok");     // "off" = tanpa pembatas
  const [saveMode, setSaveMode] = useState("burn");       // burn | clean | both

  // Font manual (di luar daftar bawaan) + hasil pengecekannya di engine.
  const fontDariPreset = useRef<string>("");
  const [fontManualOn, setFontManualOn] = useState(false);
  const [fontCheck, setFontCheck] = useState<FontCheck | null>(null);
  const [fontChecking, setFontChecking] = useState(false);

  // Preview
  const [previewOn, setPreviewOn] = useState(false);
  const [seretAktif, setSeretAktif] = useState(false); // sedang menggeser subtitle
  const [gridSelalu, setGridSelalu] = useState(false); // paksa grid tetap tampil
  const [previewBusy, setPreviewBusy] = useState(false);
  const [duration, setDuration] = useState(0);
  const [previewTime, setPreviewTime] = useState(5);
  // Dinaikkan tiap muat ulang manual supaya URL frame berubah — kalau URL-nya
  // sama persis, browser memakai gambar lama meski videonya sudah ditimpa.
  const [previewNonce, setPreviewNonce] = useState(0);
  const boxRef = useRef<HTMLDivElement | null>(null);
  const dragging = useRef(false);

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

  const addLog = useCallback((t: string) => {
    setLogs((p) => [...p, `[${new Date().toLocaleTimeString("id-ID")}] ${t}`]);
  }, []);

  useEffect(() => {
    fetch(`${ENGINE}/api/models`).then((r) => r.json()).then((m: Model[]) => {
      setModels(m);
      const dl = m.find((x) => x.downloaded);
      if (dl) setModel(dl.name);
    }).catch(() => addLog("⚠ Engine tidak terjangkau"));
    fetch(`${ENGINE}/api/fonts`).then((r) => r.json()).then((f: Font[]) => {
      setFonts(f);
      // Jangan menimpa font dari preset — daftar font datang belakangan, dulu
      // pilihan tersimpan selalu tergantikan font pertama tiap halaman dimuat.
      if (f[0] && !fontDariPreset.current) setSubFont(f[0].name);
      // Font tersimpan yang bukan bawaan berarti font manual.
      else if (fontDariPreset.current && !f.some((x) => x.name === fontDariPreset.current)) setFontManualOn(true);
    }).catch(() => {});
  }, [addLog]);

  useEffect(() => { logRef.current?.scrollTo(0, logRef.current.scrollHeight); }, [logs]);

  // Font manual divalidasi di engine (format + benar-benar terpasang), ditunda
  // 400 ms supaya tidak memanggil fc-match tiap huruf yang diketik.
  useEffect(() => {
    if (!fontManualOn) { setFontCheck(null); return; }
    const nama = subFont.trim();
    if (!nama) { setFontCheck(null); return; }
    setFontChecking(true);
    const t = setTimeout(() => {
      fetch(`${ENGINE}/api/font-check?name=${encodeURIComponent(nama)}`)
        .then((r) => r.json()).then(setFontCheck)
        .catch(() => setFontCheck({ valid: false, name: nama, family: "", source: "", error: "engine tidak terjangkau" }))
        .finally(() => setFontChecking(false));
    }, 400);
    return () => { clearTimeout(t); setFontChecking(false); };
  }, [fontManualOn, subFont]);

  // Muat preset tersimpan (localStorage) sekali di awal.
  useEffect(() => {
    try {
      const s = JSON.parse(localStorage.getItem("clipper.preset") || "{}");
      if (s.resolution) setResolution(s.resolution);
      if (s.quality) setQuality(s.quality);
      if (s.reframe) setReframe(s.reframe);
      if (typeof s.fps === "number") setFps(s.fps);
      if (s.claudeModel) setClaudeModel(s.claudeModel);
      if (s.offlineEngine) setOfflineEngine(s.offlineEngine);
      if (s.ollamaModel) setOllamaModel(s.ollamaModel);
      if (s.durationPreset) setDurationPreset(s.durationPreset);
      if (s.maxClips) setMaxClips(s.maxClips);
      if (s.subFont) { setSubFont(s.subFont); fontDariPreset.current = s.subFont; }
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
    const s = { resolution, quality, reframe, fps, claudeModel, offlineEngine, ollamaModel, durationPreset, maxClips, subFont, subSize, subColor, subOutline, subBox, subMode, subHighlight, subSpeed, platform, saveMode };
    try { localStorage.setItem("clipper.preset", JSON.stringify(s)); } catch {}
  }, [resolution, quality, reframe, fps, claudeModel, offlineEngine, ollamaModel, durationPreset, maxClips, subFont, subSize, subColor, subOutline, subBox, subMode, subHighlight, subSpeed, platform, saveMode]);

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
        addLog(`🔄 Menyambung ke job berjalan: ${active.id}`);
      }
    }).catch(() => {});
  }, [addLog]);

  // Status API key.
  useEffect(() => {
    fetch(`${ENGINE}/api/settings`).then((r) => r.json()).then((d) => setHasKey(!!d.has_key)).catch(() => {});
  }, []);

  // diam = pengecekan berkala; jangan kosongkan status agar UI tak berkedip.
  const checkOllama = useCallback((diam = false) => {
    if (!diam) setOllamaStatus(null);
    fetch(`${ENGINE}/api/ollama/status`).then((r) => r.json()).then(setOllamaStatus)
      .catch(() => setOllamaStatus({ running: false }));
  }, []);

  const ollamaAktif = mode === "offline" && offlineEngine === "ollama";

  useEffect(() => {
    if (ollamaAktif) checkOllama();
  }, [ollamaAktif, checkOllama]);

  // Selama panel Ollama terbuka, status disegarkan sendiri: berkala tiap 15 detik
  // dan tiap jendela kembali fokus — jadi model yang baru di-pull dari terminal
  // langsung terbaca tanpa perlu menekan "cek ulang".
  useEffect(() => {
    if (!ollamaAktif) return;
    const t = setInterval(() => checkOllama(true), 15000);
    const onFocus = () => checkOllama(true);
    window.addEventListener("focus", onFocus);
    return () => { clearInterval(t); window.removeEventListener("focus", onFocus); };
  }, [ollamaAktif, checkOllama]);

  const ollamaInstalled = useMemo<OllamaModel[]>(() => {
    const inst = ollamaStatus?.installed;
    if (inst?.length) return inst;
    // Engine versi lama hanya mengirim daftar nama.
    return (ollamaStatus?.models || []).map((n) => ({
      name: n, base: baseName(n), params: "", quant: "", bytes: 0, context: 0, ready: true, note: "",
    }));
  }, [ollamaStatus]);

  const ollamaTerpilih = useMemo(
    () => ollamaInstalled.find((m) => samaModel(m.name, ollamaModel)),
    [ollamaInstalled, ollamaModel],
  );

  // Auto-pilih: kalau pilihan sekarang belum terpasang, ambil model terpasang
  // yang dinilai siap lebih dulu; kalau tak ada yang siap, ambil yang pertama.
  useEffect(() => {
    if (!ollamaStatus?.running || !ollamaInstalled.length) return;
    if (ollamaInstalled.some((m) => samaModel(m.name, ollamaModel))) return;
    setOllamaModel((ollamaInstalled.find((m) => m.ready) || ollamaInstalled[0]).name);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ollamaInstalled, ollamaStatus?.running]);

  const saveKey = useCallback(async () => {
    try {
      const r = await fetch(`${ENGINE}/api/settings`, {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ anthropic_api_key: apiKey }),
      });
      const d = await r.json();
      setHasKey(!!d.has_key);
      addLog(d.has_key ? "✅ API key tersimpan" : "⚠ API key kosong");
      if (d.has_key) setApiKey("");
    } catch { addLog("⚠ Gagal menyimpan API key"); }
  }, [apiKey, addLog]);

  const pullModel = useCallback(async () => {
    setPulling(true);
    addLog(`⬇ Mengunduh model ${ollamaModel} via Ollama (bisa beberapa menit)…`);
    try {
      const r = await fetch(`${ENGINE}/api/ollama/pull`, {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ model: ollamaModel }),
      });
      const d = await r.json();
      if (!r.ok) throw new Error(d.error || "gagal");
      addLog(`✅ Model ${ollamaModel} siap`);
      checkOllama();
    } catch (e: any) { addLog(`⚠ Unduh model gagal: ${e.message}`); }
    finally { setPulling(false); }
  }, [ollamaModel, addLog, checkOllama]);

  const selectedModel = models.find((m) => m.name === model);
  const modelMissing = selectedModel && !selectedModel.downloaded;

  // reframe ikut dikirim: preview memakai mode yang sama dengan render, jadi
  // koordinat subtitle diatur di atas geometri yang benar (di mode "muat utuh"
  // videonya cuma mengisi pita tengah, bukan seluruh kanvas).
  const frameUrl = useMemo(
    () => `${ENGINE}/api/frame?path=${encodeURIComponent(path)}&t=${previewTime.toFixed(2)}`
      + `&reframe=${encodeURIComponent(reframe)}&n=${previewNonce}`,
    [path, previewTime, reframe, previewNonce]
  );

  // diam = dipicu otomatis oleh video baru; kegagalannya tidak perlu diteriakkan
  // ke log karena path bisa saja masih setengah diketik.
  const loadPreview = useCallback(async (diam = false) => {
    if (!path) return;
    setPreviewBusy(true);
    try {
      const r = await fetch(`${ENGINE}/api/probe?path=${encodeURIComponent(path)}`);
      const txt = await r.text();
      let d: any;
      try { d = JSON.parse(txt); }
      catch { throw new Error("respons bukan JSON — engine versi lama? Hentikan lalu jalankan ulang ./bin/clipper serve"); }
      if (!r.ok) throw new Error(d.error || "gagal membaca video");
      setDuration(d.duration || 0);
      setPreviewTime(Math.min(5, (d.duration || 10) / 2));
      setPreviewNonce((n) => n + 1);
      setPreviewOn(true);
    } catch (e: any) {
      if (!diam) addLog(`⚠ Preview gagal: ${e.message}`);
    } finally { setPreviewBusy(false); }
  }, [path, addLog]);

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
    const t = setTimeout(() => loadPreview(true), 500);
    return () => clearTimeout(t);
  }, [path, resetPreview, loadPreview]);

  // Geser subtitle di atas frame.
  //
  // Selisih titik pegang disimpan saat tombol ditekan, lalu ikut dijumlahkan
  // tiap gerakan. Dulu posisi langsung disamakan dengan kursor, jadi teks
  // melompat ke bawah kursor begitu disentuh — itu yang terasa "kurang stabil".
  const pegang = useRef({ dx: 0, dy: 0 });

  const titikPreview = useCallback((e: React.PointerEvent) => {
    const rect = boxRef.current!.getBoundingClientRect();
    return {
      x: ((e.clientX - rect.left) / rect.width) * PLAY_W,
      y: ((e.clientY - rect.top) / rect.height) * PLAY_H,
    };
  }, []);

  const mulaiSeret = useCallback((e: React.PointerEvent) => {
    if (!boxRef.current) return;
    e.preventDefault();
    const p = titikPreview(e);
    pegang.current = { dx: subX - p.x, dy: subY - p.y };
    dragging.current = true;
    setSeretAktif(true);
    e.currentTarget.setPointerCapture?.(e.pointerId);
  }, [subX, subY, titikPreview]);

  const onPointerMove = useCallback((e: React.PointerEvent) => {
    if (!dragging.current || !boxRef.current) return;
    const p = titikPreview(e);
    let nx = p.x + pegang.current.dx;
    let ny = p.y + pegang.current.dy;
    // Magnet ke garis tengah — inilah rasa "berhenti" saat melewati tengah.
    if (Math.abs(nx - TENGAH_X) < MAGNET) nx = TENGAH_X;
    if (Math.abs(ny - TENGAH_Y) < MAGNET) ny = TENGAH_Y;
    setSubX(Math.round(Math.max(0, Math.min(PLAY_W, nx))));
    setSubY(Math.round(Math.max(0, Math.min(PLAY_H, ny))));
  }, [titikPreview]);

  const selesaiSeret = useCallback(() => {
    dragging.current = false;
    setSeretAktif(false);
  }, []);

  const diTengahX = subX === TENGAH_X;
  const diTengahY = subY === TENGAH_Y;
  const gridTampil = seretAktif || gridSelalu;

  const uploadFile = useCallback((file: File) => {
    setUploading(true); setUploadPct(0);
    addLog(`⬆ Unggah ${file.name} (${(file.size / 1e6).toFixed(0)} MB)…`);
    const fd = new FormData(); fd.append("file", file);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `${ENGINE}/api/upload`);
    xhr.upload.onprogress = (e) => { if (e.lengthComputable) setUploadPct(e.loaded / e.total); };
    xhr.onload = () => {
      setUploading(false);
      try { const d = JSON.parse(xhr.responseText);
        if (d.path) { setPath(d.path); addLog(`✅ Unggah selesai: ${d.name}`); }
        else addLog(`⚠ Unggah gagal: ${d.error}`);
      } catch { addLog("⚠ Respons unggah tidak valid"); }
    };
    xhr.onerror = () => { setUploading(false); addLog("⚠ Unggah gagal (jaringan)"); };
    xhr.send(fd);
  }, [addLog]);

  const start = useCallback(async () => {
    // Font manual yang belum lolos pengecekan ditolak di sini — kalau diteruskan,
    // libass diam-diam mengganti fontnya dan hasil render tidak sesuai preview.
    if (fontManualOn && !fontCheck?.valid) {
      addLog(`⚠ Font "${subFont}" belum valid — perbaiki dulu namanya${fontCheck?.error ? ` (${fontCheck.error})` : ""}`);
      return;
    }
    setError(""); setClips([]); setProgress(0); setStage(""); setMessage("");
    setBusy(true); setStatus("queued"); setJobId(null);
    addLog(`▶ Job — ${resolution}/${quality}, durasi=${durationPreset}, font=${subFont}`);
    try {
      const res = await fetch(`${ENGINE}/api/jobs`, {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({
          source: { type: "path", value: path },
          options: {
            mode, whisper_model: model, resolution, quality, reframe, fps: Number(fps),
            provider: mode === "hybrid" ? "claude" : offlineEngine,
            llm_model: claudeModel, ollama_model: ollamaModel,
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
      if (!res.ok) throw new Error(data.error || "gagal membuat job");
      setJobId(data.id); addLog(`📋 Job ${data.id}`);
    } catch (e: any) { setError(e.message); setBusy(false); setStatus("error"); addLog(`⚠ ${e.message}`); }
  }, [path, mode, model, resolution, quality, reframe, fps, offlineEngine, claudeModel, ollamaModel, durationPreset, maxClips, outputDir, subFont, subSize, subX, subY, subColor, subOutline, subBox, subMode, subHighlight, subSpeed, saveMode, addLog, fontManualOn, fontCheck]);

  const cancel = useCallback(async () => {
    if (!jobId) return;
    addLog("⏹ Membatalkan…");
    await fetch(`${ENGINE}/api/jobs/${jobId}/cancel`, { method: "POST" }).catch(() => {});
  }, [jobId, addLog]);

  useEffect(() => {
    if (!jobId) return;
    const es = new EventSource(`${ENGINE}/api/jobs/${jobId}/events`);
    es.addEventListener("progress", (e: MessageEvent) => {
      const d = JSON.parse(e.data);
      if (d.status) setStatus(d.status);
      if (d.stage) setStage(d.stage);
      if (typeof d.progress === "number") setProgress(d.progress);
      if (d.message) { setMessage(d.message); addLog(`${d.stage || ""}: ${d.message}`); }
    });
    es.addEventListener("clip", (e: MessageEvent) => {
      const c: Clip = JSON.parse(e.data);
      setClips((p) => (p.find((x) => x.id === c.id) ? p : [...p, c]));
      addLog(`🎬 ${c.id} (skor ${c.score})`);
    });
    es.addEventListener("done", () => { setStatus("done"); setProgress(1); setBusy(false); addLog("✅ Selesai"); es.close(); });
    es.addEventListener("error", (e: MessageEvent) => {
      let m = "error"; try { m = JSON.parse((e as any).data).message || m; } catch {}
      setError(m); setStatus("error"); setBusy(false); addLog(`⚠ ${m}`); es.close();
    });
    return () => es.close();
  }, [jobId, addLog]);

  const scoreClass = (s: number) => (s >= 70 ? "high" : s >= 50 ? "mid" : "low");
  const fmt = (t: number) => `${Math.floor(t / 60)}:${String(Math.floor(t % 60)).padStart(2, "0")}`;
  const hex = (c: string) => (c === "yellow" ? "#ffdd00" : c === "green" ? "#4ade80" : c === "cyan" ? "#38bdf8" : "#ffffff");
  const colorHex = hex(subColor);
  const highlightHex = hex(subHighlight);

  // Font yang di-@font-face untuk preview: bawaan + font manual yang lolos cek.
  const fontPreview = useMemo(() => {
    const n = fonts.map((f) => f.name);
    if (fontManualOn && fontCheck?.valid && !n.includes(fontCheck.family)) n.push(fontCheck.family);
    return n;
  }, [fonts, fontManualOn, fontCheck]);

  // Zona yang tertutup UI aplikasi (undefined = pembatas dimatikan).
  const zone = platform !== "off" ? PLATFORMS[platform]?.zone : undefined;
  const inUnsafe = !!zone && (
    subY / PLAY_H < zone.top || subY / PLAY_H > 1 - zone.bottom || subX / PLAY_W > 1 - zone.right
  );
  // Taruh subtitle tepat di atas zona bawah. X tetap di tengah bidang: subtitle
  // ditulis rata tengah terhadap titik ini, jadi menengahkannya ke lebar sisa
  // (di luar kolom tombol kanan) justru membuatnya miring ke kiri.
  const placeSafe = () => {
    setSubX(TENGAH_X);
    setSubY(zone ? Math.round(PLAY_H * (1 - zone.bottom) - 140) : TENGAH_Y);
  };

  return (
    <div className="wrap">
      {/* Muat font asli agar preview akurat — termasuk font manual yang lolos cek */}
      <style dangerouslySetInnerHTML={{ __html: fontPreview.map((n) =>
        `@font-face{font-family:"${n}";src:url("${ENGINE}/api/font-file?name=${encodeURIComponent(n)}");font-display:swap;}`
      ).join("") }} />
      <h1>✂️ Clipper</h1>
      <p className="sub">Video panjang → klip 9:16 HD bersubtitle + skor viral (Indonesia)</p>

      {/* 1. Sumber */}
      <div className="panel">
        <div className={`dropzone ${dragOver ? "over" : ""}`}
          onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
          onDragLeave={() => setDragOver(false)}
          onDrop={(e) => { e.preventDefault(); setDragOver(false); const f = e.dataTransfer.files?.[0]; if (f) uploadFile(f); }}>
          {uploading ? (
            <>
              <div>Mengunggah… {Math.round(uploadPct * 100)}%</div>
              <div className="progress-outer" style={{ marginTop: 8 }}><div className="progress-inner" style={{ width: `${uploadPct * 100}%` }} /></div>
            </>
          ) : (
            <>
              <div><strong>Seret & lepas video ke sini</strong></div>
              <div className="meta">atau <label className="linklike">pilih file
                <input type="file" accept="video/*" style={{ display: "none" }} onChange={(e) => e.target.files?.[0] && uploadFile(e.target.files[0])} />
              </label> · atau tempel path manual</div>
            </>
          )}
        </div>
        <div className="field">
          <label>Path video</label>
          <input value={path} onChange={(e) => setPath(e.target.value)} placeholder="/home/user/video.mp4" />
        </div>
        <div className="field">
          <label>Folder output (opsional — kosong = data/&lt;job&gt;)</label>
          <input value={outputDir} onChange={(e) => setOutputDir(e.target.value)} placeholder="/home/user/hasil-klip" />
        </div>
      </div>

      {/* 2. Setelan render */}
      <div className="panel">
        <div className="meta" style={{ marginBottom: 10 }}>Setelan render</div>
        <div className="row">
          <div className="field"><label>Mode</label>
            <select value={mode} onChange={(e) => setMode(e.target.value)}>
              <option value="offline">offline (gratis)</option>
              <option value="hybrid">hybrid (Claude API)</option>
            </select></div>
          <div className="field"><label>Model Whisper</label>
            <select value={model} onChange={(e) => setModel(e.target.value)}>
              {models.map((m) => <option key={m.name} value={m.name}>{m.name} {m.size} {m.downloaded ? "✓" : "✗ belum"}</option>)}
            </select></div>
          <div className="field"><label>Resolusi</label>
            <select value={resolution} onChange={(e) => setResolution(e.target.value)}>
              <option value="720p">720p (HD)</option>
              <option value="1080p">1080p (Full HD)</option>
              <option value="1440p">1440p (2K)</option>
            </select></div>
          <div className="field"><label>Kualitas</label>
            <select value={quality} onChange={(e) => setQuality(e.target.value)}>
              <option value="draft">draft (cepat)</option>
              <option value="hd">HD (seimbang)</option>
              <option value="max">max (terbaik, lambat)</option>
            </select></div>
        </div>
        <div className="row">
          <div className="field"><label title="Panjang tiap klip">Durasi klip ⓘ</label>
            <select value={durationPreset} onChange={(e) => setDurationPreset(e.target.value)}>
              <option value="auto">auto (45 dtk – 2 mnt)</option>
              <option value="30">± 30 detik</option>
              <option value="60">± 60 detik</option>
              <option value="90">± 90 detik</option>
              <option value="120">± 2 menit</option>
              <option value="180">± 3 menit</option>
            </select></div>
          <div className="field"><label title="Batas atas jumlah klip dari 1 video">Jumlah klip maksimum ⓘ</label>
            <input type="number" min={1} max={50} value={maxClips} onChange={(e) => setMaxClips(Number(e.target.value))} /></div>
          <div className="field"><label title="Klip polos berguna bila subtitle mau diatur ulang di editor lain">Simpan klip ⓘ</label>
            <select value={saveMode} onChange={(e) => setSaveMode(e.target.value)}>
              <option value="burn">Dengan subtitle (dibakar)</option>
              <option value="clean">Klip saja — tanpa subtitle</option>
              <option value="both">Keduanya (2 berkas per klip)</option>
            </select></div>
        </div>
        <div className="row">
          <div className="field"><label title="Cara memuat video landscape ke bingkai tegak 9:16">Cara pas ke 9:16 ⓘ</label>
            <select value={reframe} onChange={(e) => setReframe(e.target.value)}>
              <option value="center">Isi penuh (zoom/crop)</option>
              <option value="fit">Muat utuh (latar blur) — paling tajam</option>
              <option value="face_follow" disabled>Ikut wajah — belum tersedia</option>
            </select></div>
          <div className="field"><label title="Kehalusan gerak, bukan ketajaman">FPS ⓘ</label>
            <select value={fps} onChange={(e) => setFps(Number(e.target.value))}>
              <option value={0}>Ikut sumber</option>
              <option value={24}>24</option>
              <option value={30}>30</option>
              <option value={60}>60</option>
            </select></div>
        </div>
      </div>

      {/* 2b. Mesin AI (scoring) — berubah menurut mode */}
      <div className="panel">
        <div className="meta" style={{ marginBottom: 10 }}>Mesin AI — memilih & menilai momen (mode: {mode})</div>
        {mode === "hybrid" ? (
          <>
            <div className="field">
              <label>API Key Claude {hasKey && <span className="ok">✓ tersimpan</span>}</label>
              <div style={{ display: "flex", gap: 8 }}>
                <input type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)}
                  placeholder={hasKey ? "•••• (isi untuk mengganti)" : "sk-ant-..."} />
                <button onClick={saveKey} disabled={!apiKey}>Simpan</button>
              </div>
            </div>
            <div className="field">
              <label>Model Claude (makin kuat = makin bagus & mahal)</label>
              <select value={claudeModel} onChange={(e) => setClaudeModel(e.target.value)}>
                <option value="claude-haiku-4-5">Haiku 4.5 — murah & cepat</option>
                <option value="claude-sonnet-5">Sonnet 5 — seimbang</option>
                <option value="claude-opus-4-8">Opus 4.8 — terbaik</option>
              </select>
            </div>
            {!hasKey && <div className="warnbox">Belum ada API key — mode hybrid <b>akan gagal</b>. Masukkan key, atau pindah ke mode offline.</div>}
          </>
        ) : (
          <>
            <div className="field">
              <label>Mesin skor offline</label>
              <select value={offlineEngine} onChange={(e) => setOfflineEngine(e.target.value)}>
                <option value="ollama">AI lokal (Ollama) — lebih pintar</option>
                <option value="heuristic">Heuristik — tanpa AI, cepat</option>
              </select>
              <div className="meta">Mesin pilihan Anda dipakai apa adanya — bila gagal, job berhenti dengan pesan sebabnya (tidak diam-diam diganti mesin lain).</div>
            </div>
            {offlineEngine === "ollama" && (
              <>
                <div className="field">
                  <label>Model lokal (dijalankan via Ollama)</label>
                  <select value={ollamaModel} onChange={(e) => setOllamaModel(e.target.value)}>
                    {ollamaInstalled.map((m) => {
                      const spek = [m.params, m.quant, ukuranGB(m.bytes)].filter(Boolean).join(" · ");
                      return (
                        <option key={m.name} value={m.name}>
                          {m.name}{spek ? ` — ${spek}` : ""} {m.ready ? "✓ siap" : "⚠ kurang memadai"}
                        </option>
                      );
                    })}
                    {/* Saran hanya muncul bila belum terpasang — dicek per nama dasar
                        supaya "qwen2.5" tidak tampil ganda dengan "qwen2.5:latest". */}
                    {OLLAMA_SARAN.filter((s) => !ollamaInstalled.some((m) => samaModel(m.name, s))).map((s) => (
                      <option key={s} value={s}>{s} (perlu unduh)</option>
                    ))}
                  </select>
                </div>
                <div className="meta">
                  {ollamaStatus === null ? "Mengecek Ollama…"
                    : !ollamaStatus.running ? (
                      <span className="warn">⚠ Ollama tak terdeteksi. Install dari ollama.com lalu jalankan <code>ollama serve</code>. <button className="ghost tiny" onClick={() => checkOllama()}>cek ulang</button></span>
                    ) : !ollamaTerpilih ? (
                      <span className="warn">Ollama jalan, tapi {ollamaModel} belum ada. <button className="ghost tiny" onClick={pullModel} disabled={pulling}>{pulling ? "mengunduh…" : "⬇ unduh model"}</button></span>
                    ) : ollamaTerpilih.ready ? (
                      <span className="ok">✓ Ollama jalan, model {ollamaTerpilih.name} siap{ollamaTerpilih.params ? ` (${ollamaTerpilih.params})` : ""}</span>
                    ) : (
                      <span className="warn">⚠ {ollamaTerpilih.name} terpasang tapi {ollamaTerpilih.note}. Job akan berhenti bila model gagal memilih momen — pakai model lain atau <button className="ghost tiny" onClick={pullModel} disabled={pulling}>{pulling ? "mengunduh…" : "⬇ unduh model terpilih"}</button></span>
                    )}
                  {ollamaStatus?.running && !ollamaInstalled.length && " — belum ada model terpasang sama sekali."}
                </div>
              </>
            )}
          </>
        )}
      </div>

      {/* 3. Setelan subtitle + preview geser */}
      <div className="panel">
        <div className="meta" style={{ marginBottom: 10 }}>Setelan subtitle — geser teks di preview untuk atur posisi</div>
        <div className="row">
          <div className="field"><label>Font</label>
            <select
              value={fontManualOn ? "__manual__" : subFont}
              onChange={(e) => {
                if (e.target.value === "__manual__") { setFontManualOn(true); return; }
                setFontManualOn(false); setSubFont(e.target.value);
              }}>
              {fonts.map((f) => <option key={f.name} value={f.name}>{f.name}</option>)}
              <option value="__manual__">✏️ Font lain — ketik manual…</option>
            </select>
            {fontManualOn && (
              <>
                <input style={{ marginTop: 6 }} value={subFont} spellCheck={false}
                  placeholder="mis. Poppins" onChange={(e) => setSubFont(e.target.value)} />
                <div className="meta">
                  {fontChecking ? "Memeriksa font…"
                    : !fontCheck ? "Ketik nama family font — huruf/angka, spasi, titik, ' & atau -"
                    : fontCheck.valid ? <span className="ok">✓ {fontCheck.family} ditemukan ({fontCheck.source})</span>
                    : <span className="warn">⚠ {fontCheck.error}</span>}
                </div>
              </>
            )}</div>
          <div className="field"><label>Ukuran ({subSize})</label>
            <input type="range" min={40} max={140} value={subSize} onChange={(e) => setSubSize(Number(e.target.value))} /></div>
          <div className="field"><label>Warna</label>
            <select value={subColor} onChange={(e) => setSubColor(e.target.value)}>
              <option value="white">Putih</option>
              <option value="yellow">Kuning</option>
            </select></div>
          <div className="field"><label>Posisi</label>
            <div className="meta">x={subX} y={subY}
              <button className="ghost tiny" onClick={() => { setSubX(540); setSubY(960); }}>reset tengah</button>
            </div></div>
        </div>
        <div className="row">
          <div className="field"><label title="Cara kata ditampilkan di layar">Gaya subtitle ⓘ</label>
            <select value={subMode} onChange={(e) => setSubMode(e.target.value)}>
              <option value="normal">Normal — kalimat utuh</option>
              <option value="karaoke">Highlight per-kata — kata aktif disorot</option>
              <option value="word">Satu kata per layar — gaya viral</option>
            </select></div>
          {subMode !== "normal" && (
            <div className="field"><label>Warna sorot</label>
              <select value={subHighlight} onChange={(e) => setSubHighlight(e.target.value)}>
                <option value="yellow">Kuning</option>
                <option value="white">Putih</option>
                <option value="green">Hijau</option>
                <option value="cyan">Biru muda</option>
              </select></div>
          )}
          <div className="field"><label title="Makin lambat = makin sedikit teks sekaligus & tampil lebih lama">Kecepatan subtitle ⓘ</label>
            <select value={subSpeed} onChange={(e) => setSubSpeed(e.target.value)}>
              <option value="lambat">Lambat — paling mudah dibaca</option>
              <option value="normal">Normal</option>
              <option value="padat">Padat — teks lebih banyak</option>
            </select></div>
        </div>
        <div className="row">
          <div className="field"><label>Garis tepi ({subOutline})</label>
            <input type="range" min={0} max={12} value={subOutline} onChange={(e) => setSubOutline(Number(e.target.value))} /></div>
          <div className="field"><label>Efek</label>
            <label className="chk"><input type="checkbox" checked={subBox} onChange={(e) => setSubBox(e.target.checked)} /> Latar kotak</label>
          </div>
          <div className="field"><label title="Area yang tertutup tombol & caption aplikasi">Pembatas sosmed ⓘ</label>
            <select value={platform} onChange={(e) => setPlatform(e.target.value)}>
              {Object.entries(PLATFORMS).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
              <option value="off">Tanpa pembatas</option>
            </select></div>
          <div className="field"><label>Posisi aman</label>
            <button className="ghost tiny" onClick={placeSafe}>⤓ Taruh di area aman</button>
            {inUnsafe && <div className="warn" style={{ marginTop: 6 }}>⚠ Subtitle masuk area yang tertutup UI {PLATFORMS[platform]?.label}</div>}
          </div>
        </div>

        {!previewOn ? (
          <button className="ghost" disabled={!path || previewBusy} onClick={() => loadPreview()}>
            {previewBusy ? "Memuat preview…" : "👁 Muat preview frame"}
          </button>
        ) : (
          <>
            <div className="row" style={{ gap: 8, marginBottom: 10 }}>
              <button className="ghost tiny" disabled={previewBusy} onClick={() => loadPreview()}>
                {previewBusy ? "memuat…" : "🔄 muat ulang preview"}
              </button>
              <button className="ghost tiny" onClick={resetPreview}>✕ reset preview</button>
              <label className="chk"><input type="checkbox" checked={gridSelalu}
                onChange={(e) => setGridSelalu(e.target.checked)} /> tampilkan garis tengah terus</label>
            </div>
            <div className="preview-wrap">
              <div className="preview9x16" ref={boxRef}>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={frameUrl} alt="preview" draggable={false} />
                {zone && (
                  <>
                    <div className="safezone" style={{ top: 0, left: 0, right: 0, height: `${zone.top * 100}%` }}>
                      <span>ketutup UI atas</span>
                    </div>
                    <div className="safezone" style={{ bottom: 0, left: 0, right: 0, height: `${zone.bottom * 100}%` }}>
                      <span>caption &amp; nama akun</span>
                    </div>
                    <div className="safezone" style={{
                      top: `${zone.top * 100}%`, bottom: `${zone.bottom * 100}%`,
                      right: 0, width: `${zone.right * 100}%`,
                    }}>
                      <span className="vert">tombol aksi</span>
                    </div>
                  </>
                )}
                {/* Garis tengah: muncul saat digeser (atau dikunci lewat centang).
                    Kelas "on" = subtitle sedang menempel di garis itu. */}
                {gridTampil && (
                  <>
                    <div className={`guide v${diTengahX ? " on" : ""}`} />
                    <div className={`guide h${diTengahY ? " on" : ""}`} />
                    {diTengahX && diTengahY && <div className="guide xy" />}
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
                  onPointerDown={mulaiSeret}
                  onPointerMove={onPointerMove}
                  onPointerUp={selesaiSeret}
                  onPointerCancel={selesaiSeret}>
                  {subMode === "word" ? (
                    <span style={{ color: highlightHex }}>Contoh</span>
                  ) : subMode === "karaoke" ? (
                    <>Contoh <span style={{ color: highlightHex }}>subtitle</span></>
                  ) : "Contoh subtitle"}
                </div>
              </div>
            </div>
            <div className="field" style={{ maxWidth: 360 }}>
              <label>Waktu frame preview: {previewTime.toFixed(1)}s</label>
              <input type="range" min={0} max={Math.max(1, Math.floor(duration))} step={1}
                value={previewTime} onChange={(e) => setPreviewTime(Number(e.target.value))} />
            </div>
            <div className="meta">
              Preview memakai mode reframe yang sedang dipilih
              ({reframe === "fit" ? "muat utuh + latar blur" : "isi penuh, crop tengah"}),
              jadi posisi subtitle yang kamu atur di sini sama dengan hasil render.
              Font di preview masih kira-kira — font asli dipakai saat render.
            </div>
          </>
        )}
      </div>

      {modelMissing && (
        <div className="warnbox">Model <b>{model}</b> belum diunduh. Jalankan <code> ./setup.sh {model}</code> lalu muat ulang.</div>
      )}

      <div className="panel">
        <div style={{ display: "flex", gap: 10 }}>
          <button onClick={start} disabled={busy || !path || !!modelMissing}>{busy ? "Memproses…" : "Mulai proses"}</button>
          {busy && jobId && <button className="ghost" onClick={cancel}>⏹ Batalkan</button>}
        </div>
      </div>

      {((status && status !== "queued") || busy) && (
        <div className="panel">
          <div className="progress-outer"><div className="progress-inner" style={{ width: `${Math.round(progress * 100)}%` }} /></div>
          <div className="stage">{status === "done" ? "✅ Selesai" : status === "error" ? "⛔ Berhenti" : `${stage} — ${message}`} ({Math.round(progress * 100)}%)</div>
          {error && <div className="err">⚠ {error}</div>}
        </div>
      )}

      {logs.length > 0 && (
        <div className="panel">
          <div className="meta" style={{ marginBottom: 6 }}>Log</div>
          <div className="logbox" ref={logRef}>{logs.map((l, i) => <div key={i}>{l}</div>)}</div>
        </div>
      )}

      {clips.length > 0 && (
        <div className="panel">
          <h3 style={{ marginTop: 0 }}>{clips.length} klip</h3>
          <div className="clips">
            {clips.slice().sort((a, b) => b.score - a.score).map((c) => (
              <div className="clip" key={c.id}>
                <video src={`${ENGINE}/api/jobs/${c.job_id}/clips/${c.id}/file`} controls preload="metadata" />
                <div className="body">
                  <span className={`score ${scoreClass(c.score)}`}>{c.score}</span><span className="meta"> /100</span>
                  <div className="title">{c.title || "(tanpa judul)"}</div>
                  <div className="meta">{fmt(c.start)}–{fmt(c.end)} · {Math.round(c.duration)}s</div>
                  <div className="reasons">hook {c.reasons.hook} · emo {c.reasons.emotion} · jelas {c.reasons.clarity} · share {c.reasons.shareability} · mandiri {c.reasons.standalone}</div>
                  {c.hashtags?.map((h) => <span className="tag" key={h}>{h}</span>)}
                  <div style={{ marginTop: 8, display: "flex", gap: 8, flexWrap: "wrap" }}>
                    <a className="dl" href={`${ENGINE}/api/jobs/${c.job_id}/clips/${c.id}/file`} download>
                      ⬇ {c.video_path_raw && c.video_path_raw !== c.video_path ? "bersubtitle" : "unduh"}
                    </a>
                    {c.video_path_raw && c.video_path_raw !== c.video_path && (
                      <a className="dl" href={`${ENGINE}/api/jobs/${c.job_id}/clips/${c.id}/file?varian=polos`} download>⬇ polos</a>
                    )}
                    {c.subtitle_srt && (
                      <a className="dl" href={`${ENGINE}/api/jobs/${c.job_id}/clips/${c.id}/file?varian=srt`} download>⬇ .srt</a>
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

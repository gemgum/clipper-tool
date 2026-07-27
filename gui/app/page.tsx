"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

const ENGINE = process.env.NEXT_PUBLIC_ENGINE_URL || "http://127.0.0.1:8787";
const PLAY_W = 1080, PLAY_H = 1920; // ruang koordinat subtitle

type Reasons = { hook: number; emotion: number; clarity: number; shareability: number; standalone: number };
type Clip = {
  id: string; job_id: string; start: number; end: number; duration: number;
  score: number; reasons: Reasons; title?: string; hashtags?: string[]; transcript: string; status: string;
};
type Model = { name: string; size: string; downloaded: boolean };
type Font = { name: string };

export default function Home() {
  const [path, setPath] = useState("");
  const [outputDir, setOutputDir] = useState("");
  const [mode, setMode] = useState("offline");
  const [model, setModel] = useState("base");
  const [resolution, setResolution] = useState("1080p");
  const [quality, setQuality] = useState("hd");
  const [reframe, setReframe] = useState("center");
  const [fps, setFps] = useState(0);
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
  const [subKaraoke, setSubKaraoke] = useState(false);

  // Preview
  const [previewOn, setPreviewOn] = useState(false);
  const [duration, setDuration] = useState(0);
  const [previewTime, setPreviewTime] = useState(5);
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
      setFonts(f); if (f[0]) setSubFont(f[0].name);
    }).catch(() => {});
  }, [addLog]);

  useEffect(() => { logRef.current?.scrollTo(0, logRef.current.scrollHeight); }, [logs]);

  // Muat preset tersimpan (localStorage) sekali di awal.
  useEffect(() => {
    try {
      const s = JSON.parse(localStorage.getItem("clipper.preset") || "{}");
      if (s.resolution) setResolution(s.resolution);
      if (s.quality) setQuality(s.quality);
      if (s.reframe) setReframe(s.reframe);
      if (typeof s.fps === "number") setFps(s.fps);
      if (s.durationPreset) setDurationPreset(s.durationPreset);
      if (s.maxClips) setMaxClips(s.maxClips);
      if (s.subFont) setSubFont(s.subFont);
      if (s.subSize) setSubSize(s.subSize);
      if (s.subColor) setSubColor(s.subColor);
      if (typeof s.subOutline === "number") setSubOutline(s.subOutline);
      if (typeof s.subBox === "boolean") setSubBox(s.subBox);
      if (typeof s.subKaraoke === "boolean") setSubKaraoke(s.subKaraoke);
    } catch {}
  }, []);

  // Simpan preset setiap kali setelan berubah.
  useEffect(() => {
    const s = { resolution, quality, reframe, fps, durationPreset, maxClips, subFont, subSize, subColor, subOutline, subBox, subKaraoke };
    try { localStorage.setItem("clipper.preset", JSON.stringify(s)); } catch {}
  }, [resolution, quality, reframe, fps, durationPreset, maxClips, subFont, subSize, subColor, subOutline, subBox, subKaraoke]);

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

  const selectedModel = models.find((m) => m.name === model);
  const modelMissing = selectedModel && !selectedModel.downloaded;

  const frameUrl = useMemo(
    () => `${ENGINE}/api/frame?path=${encodeURIComponent(path)}&t=${previewTime.toFixed(2)}`,
    [path, previewTime]
  );

  const loadPreview = useCallback(async () => {
    if (!path) return;
    try {
      const r = await fetch(`${ENGINE}/api/probe?path=${encodeURIComponent(path)}`);
      const txt = await r.text();
      let d: any;
      try { d = JSON.parse(txt); }
      catch { throw new Error("respons bukan JSON — engine versi lama? Hentikan lalu jalankan ulang ./bin/clipper serve"); }
      if (!r.ok) throw new Error(d.error || "gagal membaca video");
      setDuration(d.duration || 0);
      setPreviewTime(Math.min(5, (d.duration || 10) / 2));
      setPreviewOn(true);
    } catch (e: any) { addLog(`⚠ Preview gagal: ${e.message}`); }
  }, [path, addLog]);

  // Drag subtitle di atas frame.
  const onPointerMove = useCallback((e: React.PointerEvent) => {
    if (!dragging.current || !boxRef.current) return;
    const rect = boxRef.current.getBoundingClientRect();
    let nx = ((e.clientX - rect.left) / rect.width) * PLAY_W;
    let ny = ((e.clientY - rect.top) / rect.height) * PLAY_H;
    nx = Math.max(0, Math.min(PLAY_W, nx));
    ny = Math.max(0, Math.min(PLAY_H, ny));
    setSubX(Math.round(nx)); setSubY(Math.round(ny));
  }, []);

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
            duration_preset: durationPreset, max_clips: Number(maxClips), output_dir: outputDir,
            subtitle: {
              font: subFont, size: Number(subSize), x: subX, y: subY, color: subColor, bold: true,
              outline: Number(subOutline), box: subBox, karaoke: subKaraoke,
            },
          },
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "gagal membuat job");
      setJobId(data.id); addLog(`📋 Job ${data.id}`);
    } catch (e: any) { setError(e.message); setBusy(false); setStatus("error"); addLog(`⚠ ${e.message}`); }
  }, [path, mode, model, resolution, quality, reframe, fps, durationPreset, maxClips, outputDir, subFont, subSize, subX, subY, subColor, subOutline, subBox, subKaraoke, addLog]);

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
  const colorHex = subColor === "yellow" ? "#ffdd00" : "#ffffff";

  return (
    <div className="wrap">
      {/* Muat font asli agar preview akurat */}
      <style dangerouslySetInnerHTML={{ __html: fonts.map((f) =>
        `@font-face{font-family:"${f.name}";src:url("${ENGINE}/api/font-file?name=${encodeURIComponent(f.name)}") format("truetype");font-display:swap;}`
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
              <option value="auto">auto (30 dtk – 3 mnt)</option>
              <option value="30">± 30 detik</option>
              <option value="60">± 60 detik</option>
              <option value="90">± 90 detik</option>
              <option value="120">± 2 menit</option>
              <option value="180">± 3 menit</option>
            </select></div>
          <div className="field"><label title="Batas atas jumlah klip dari 1 video">Jumlah klip maksimum ⓘ</label>
            <input type="number" min={1} max={50} value={maxClips} onChange={(e) => setMaxClips(Number(e.target.value))} /></div>
        </div>
        <div className="row">
          <div className="field"><label title="Cara memuat video landscape ke bingkai tegak 9:16">Cara pas ke 9:16 ⓘ</label>
            <select value={reframe} onChange={(e) => setReframe(e.target.value)}>
              <option value="center">Isi penuh (zoom/crop)</option>
              <option value="fit">Muat utuh (latar blur) — paling tajam</option>
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

      {/* 3. Setelan subtitle + preview geser */}
      <div className="panel">
        <div className="meta" style={{ marginBottom: 10 }}>Setelan subtitle — geser teks di preview untuk atur posisi</div>
        <div className="row">
          <div className="field"><label>Font</label>
            <select value={subFont} onChange={(e) => setSubFont(e.target.value)}>
              {fonts.map((f) => <option key={f.name} value={f.name}>{f.name}</option>)}
            </select></div>
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
          <div className="field"><label>Garis tepi ({subOutline})</label>
            <input type="range" min={0} max={12} value={subOutline} onChange={(e) => setSubOutline(Number(e.target.value))} /></div>
          <div className="field"><label>Efek</label>
            <label className="chk"><input type="checkbox" checked={subBox} onChange={(e) => setSubBox(e.target.checked)} /> Latar kotak</label>
            <label className="chk"><input type="checkbox" checked={subKaraoke} onChange={(e) => setSubKaraoke(e.target.checked)} /> Karaoke per-kata</label>
          </div>
        </div>

        {!previewOn ? (
          <button className="ghost" disabled={!path} onClick={loadPreview}>👁 Muat preview frame</button>
        ) : (
          <>
            <div className="preview-wrap">
              <div className="preview9x16" ref={boxRef}
                onPointerMove={onPointerMove}
                onPointerUp={() => (dragging.current = false)}
                onPointerLeave={() => (dragging.current = false)}>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={frameUrl} alt="preview" draggable={false} />
                <div className="suboverlay"
                  style={{
                    left: `${(subX / PLAY_W) * 100}%`, top: `${(subY / PLAY_H) * 100}%`,
                    fontFamily: `"${subFont}", sans-serif`,
                    fontSize: `calc(${subSize / PLAY_H} * var(--pvh))`,
                    color: subKaraoke && subColor === "white" ? "#ffdd00" : colorHex,
                    background: subBox ? "rgba(0,0,0,0.6)" : "transparent",
                    borderRadius: subBox ? 8 : 0,
                    padding: subBox ? "4px 14px" : "2px 8px",
                    textShadow: subBox ? "none" : (subOutline > 0
                      ? "-2px -2px 0 #000,2px -2px 0 #000,-2px 2px 0 #000,2px 2px 0 #000,0 0 4px #000"
                      : "none"),
                  }}
                  onPointerDown={(e) => { dragging.current = true; (e.target as HTMLElement).setPointerCapture?.(e.pointerId); }}>
                  Contoh subtitle
                </div>
              </div>
            </div>
            <div className="field" style={{ maxWidth: 360 }}>
              <label>Waktu frame preview: {previewTime.toFixed(1)}s</label>
              <input type="range" min={0} max={Math.max(1, Math.floor(duration))} step={1}
                value={previewTime} onChange={(e) => setPreviewTime(Number(e.target.value))} />
            </div>
            <div className="meta">Catatan: font di preview kira-kira (font asli dipakai saat render).</div>
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
                  <div style={{ marginTop: 8 }}><a className="dl" href={`${ENGINE}/api/jobs/${c.job_id}/clips/${c.id}/file`} download>⬇ unduh</a></div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

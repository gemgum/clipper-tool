"use client";

// Ikon: lucide-react (ISC) — alasannya di gui/app/page.tsx.
import { Download } from "lucide-react";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useI18n } from "./i18n";
import { eng } from "./engine";

export type WhisperModel = { name: string; size: string; downloaded: boolean };

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

// Semua yang disetel sebelum menekan Mulai: mesin, mutu, jumlah klip, dan mesin
// skor. Dulu dua panel terpisah (Render settings + AI engine), digabung 6
// Agustus 2026 — keduanya menjawab pertanyaan yang sama ("job ini dijalankan
// bagaimana"), dan dua bingkai + dua judul memakan tinggi yang justru membuat
// kolom ini harus digulir.
//
// Yang dipegang sendiri komponen ini: kunci API, status Ollama, dan unduhan
// model — tidak ada yang lain di halaman ini memakainya. Yang dititipkan
// halaman tetap di atas, sebab job dan preset tersimpan membutuhkannya.
export default function SetupPanel({
  mode, setMode, model, setModel, models,
  resolution, setResolution, quality, setQuality, fps, setFps,
  durationPreset, setDurationPreset, maxClips, setMaxClips, saveMode, setSaveMode,
  claudeModel, setClaudeModel, offlineEngine, setOfflineEngine,
  ollamaModel, setOllamaModel, transcriptFix, setTranscriptFix, terms, setTerms, addLog,
}: {
  mode: string; setMode: (v: string) => void;
  model: string; setModel: (v: string) => void;
  models: WhisperModel[];
  resolution: string; setResolution: (v: string) => void;
  quality: string; setQuality: (v: string) => void;
  fps: number; setFps: (v: number) => void;
  durationPreset: string; setDurationPreset: (v: string) => void;
  maxClips: number; setMaxClips: (v: number) => void;
  saveMode: string; setSaveMode: (v: string) => void;
  claudeModel: string; setClaudeModel: (v: string) => void;
  offlineEngine: string; setOfflineEngine: (v: string) => void;
  ollamaModel: string; setOllamaModel: (v: string) => void;
  transcriptFix: boolean; setTranscriptFix: (v: boolean) => void;
  terms: string; setTerms: (v: string) => void;
  addLog: (text: string) => void;
}) {
  const { t } = useI18n();

  const [apiKey, setApiKey] = useState("");
  const [hasKey, setHasKey] = useState(false);
  const [ollamaStatus, setOllamaStatus] = useState<OllamaStatus | null>(null);
  const [pulling, setPulling] = useState(false);

  // Status API key.
  useEffect(() => {
    fetch(eng(`/api/settings`)).then((r) => r.json()).then((d) => setHasKey(!!d.has_key)).catch(() => {});
  }, []);

  // silent = pengecekan berkala; jangan kosongkan status agar UI tak berkedip.
  const checkOllama = useCallback((silent = false) => {
    if (!silent) setOllamaStatus(null);
    fetch(eng(`/api/ollama/status`)).then((r) => r.json()).then(setOllamaStatus)
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
      const res = await fetch(eng(`/api/settings`), {
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
      const res = await fetch(eng(`/api/ollama/pull`), {
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

  return (
    <div className="panel">
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

        {/* Mesin skor menempel di kelompok Engine, bukan berdiri sebagai panel
            sendiri: ia menjawab pertanyaan yang sama — dengan apa job ini
            dikerjakan. */}
        {mode === "hybrid" ? (
          <div className="row" style={{ marginTop: 12 }}>
            <div className="field">
              <label>{t("apiKeyClaude")} {hasKey && <span className="ok">{t("keyStored")}</span>}</label>
              <div className="path-row">
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
              {!hasKey && <div className="warn">⚠ {t("noKeyWarning")}</div>}
            </div>
          </div>
        ) : (
          <div className="row" style={{ marginTop: 12 }}>
            <div className="field">
              <label>{t("offlineEngine")}</label>
              <select value={offlineEngine} onChange={(e) => setOfflineEngine(e.target.value)}>
                <option value="ollama">{t("offlineOllama")}</option>
                <option value="heuristic">{t("offlineHeuristic")}</option>
              </select>
            </div>
            {offlineEngine === "ollama" && (
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
                {/* Hanya KEADAAN yang ditulis di sini — "siap" tidak perlu
                    kalimat, cuma lampu hijau. Yang butuh tindakan tetap lengkap
                    dengan tombolnya. */}
                {ollamaStatus !== null && !ollamaStatus.running ? (
                  <div className="warn">⚠ {t("ollamaNotDetected")} <code>ollama serve</code> <button className="ghost tiny" onClick={() => checkOllama()}>{t("recheck")}</button></div>
                ) : ollamaStatus?.running && !selectedOllama ? (
                  <div className="warn">⚠ <button className="ghost tiny" onClick={pullModel} disabled={pulling}>{pulling ? t("downloading") : <><Download className="ico" aria-hidden="true" /> {t("downloadModel")}</>}</button></div>
                ) : selectedOllama && !selectedOllama.ready ? (
                  <div className="warn">⚠ {selectedOllama.note} <button className="ghost tiny" onClick={pullModel} disabled={pulling}>{pulling ? t("downloading") : <><Download className="ico" aria-hidden="true" /> {t("downloadSelected")}</>}</button></div>
                ) : null}
              </div>
            )}
          </div>
        )}

        {/* Koreksi transkrip berlaku di kedua mode, jadi ditaruh di luar
            percabangan hybrid/offline. Ia memakai LLM juga saat mesin skornya
            heuristik — peringatannya di sini supaya tidak mengagetkan saat job
            berhenti. */}
        <div className="field" style={{ marginTop: 12 }}>
          <label className="chk" title={t("transcriptFixTip")}>
            <input type="checkbox" checked={transcriptFix}
              onChange={(e) => setTranscriptFix(e.target.checked)} /> {t("transcriptFix")} ⓘ
          </label>
          {transcriptFix && (
            <>
              {mode !== "hybrid" && offlineEngine === "heuristic" && (
                <div className="warn">⚠ {t("transcriptFixNeedsLLM")}</div>
              )}
              {/* Daftar istilah menempel di bawah koreksi transkrip karena hanya
                  tahap itu yang memakainya — tanpa centang di atas, isian ini
                  tidak berpengaruh apa pun. */}
              <label htmlFor="terms">{t("terms")}</label>
              <input id="terms" type="text" value={terms}
                placeholder={t("termsPlaceholder")}
                onChange={(e) => setTerms(e.target.value)} />
            </>
          )}
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
  );
}

"use client";

// Ikon: lucide-react (ISC) — alasannya di gui/app/page.tsx.
import { Download, Info, Plug } from "lucide-react";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useI18n } from "./i18n";
import { eng } from "./engine";
import Select from "./select";
import Warn from "./warn";

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
  // Hasil sapaan ke model: null = belum dicoba.
  const [ping, setPing] = useState<{ ok: boolean; reply?: string; error?: string; ms: number } | null>(null);
  const [pinging, setPinging] = useState(false);

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

  // "Terpasang" tidak sama dengan "bisa dipakai": model bisa terdaftar di
  // `ollama list` tapi gagal dimuat karena RAM kurang, atau perlu belasan menit
  // untuk masuk memori. Tombol ini membuktikannya dengan menyapa modelnya —
  // sekaligus memuatnya, jadi job berikutnya tidak lagi menunggu diam-diam.
  const pingModel = useCallback(async () => {
    setPinging(true);
    setPing(null);
    addLog(t("logPingStart", { model: ollamaModel }));
    try {
      const res = await fetch(eng(`/api/ollama/ping`), {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ model: ollamaModel }),
      });
      const data = await res.json();
      setPing(data);
      addLog(data.ok
        ? t("logPingOk", { model: ollamaModel, ms: data.ms, reply: data.reply })
        : `⚠ ${data.error}`);
    } catch (e: any) {
      setPing({ ok: false, error: e.message, ms: 0 });
      addLog(`⚠ ${e.message}`);
    } finally { setPinging(false); }
  }, [ollamaModel, addLog, t]);

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
        <div className="grid3">
          <div className="field"><label>{t("mode")}</label>
            <Select value={mode} onChange={setMode} options={[
              { value: "offline", label: t("modeOffline") },
              { value: "hybrid", label: t("modeHybrid") },
            ]} /></div>
          <div className="field"><label>{t("whisperModel")}</label>
            <Select value={model} onChange={setModel} options={models.map((m) => ({
              value: m.name, label: m.name,
              note: m.downloaded ? m.size : t("modelNotDownloaded"),
            }))} /></div>
        </div>

        {/* Mesin skor menempel di kelompok Engine, bukan berdiri sebagai panel
            sendiri: ia menjawab pertanyaan yang sama — dengan apa job ini
            dikerjakan. */}
        {mode === "hybrid" ? (
          <div className="row" style={{ marginTop: 12 }}>
            <div className="field">
              <label>{t("apiKeyClaude")} {hasKey && <span className="ok">{t("keyStored")}</span>}
                {!hasKey && <Warn>{t("noKeyWarning")}</Warn>}</label>
              <div className="path-row">
                <input type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)}
                  placeholder={hasKey ? t("keyPlaceholderStored") : "sk-ant-..."} />
                <button onClick={saveKey} disabled={!apiKey}>{t("save")}</button>
              </div>
            </div>
            <div className="field">
              <label>{t("claudeModel")}</label>
              <Select value={claudeModel} onChange={setClaudeModel} options={[
                { value: "claude-haiku-4-5", label: t("claudeHaiku") },
                { value: "claude-sonnet-5", label: t("claudeSonnet") },
                { value: "claude-opus-4-8", label: t("claudeOpus") },
              ]} />
            </div>
          </div>
        ) : (
          /* Kisi tiga kolom, sama seperti baris di atasnya: mesin, model, dan
             tombol ujinya berdiri di garis yang sama. Dulu .row membagi lebar
             sendiri dan tombol Test menggantung di bawah kotak model, jadi tidak
             ada satu pun tepi yang sejajar. */
          <div className="grid3" style={{ marginTop: 12 }}>
            <div className="field">
              <label>{t("offlineEngine")}</label>
              <Select value={offlineEngine} onChange={setOfflineEngine} options={[
                { value: "ollama", label: t("offlineOllama") },
                { value: "heuristic", label: t("offlineHeuristic") },
              ]} />
            </div>
            {offlineEngine === "ollama" && (
              <>
              <div className="field">
                <label>{t("localModel")}
                  {/* Keadaan yang butuh tindakan jadi LAMBANG di label, lengkap
                      dengan tombolnya di dalam popup. Sebagai baris teks, ia
                      menggeser seluruh kolom tiap kali Ollama dinyalakan atau
                      model diganti. */}
                  {ollamaStatus !== null && !ollamaStatus.running ? (
                    <Warn>{t("ollamaNotDetected")} <code>ollama serve</code>
                      <button className="ghost tiny" onClick={() => checkOllama()}>{t("recheck")}</button>
                    </Warn>
                  ) : ollamaStatus?.running && !selectedOllama ? (
                    <Warn>{t("modelNeedsDownload")}
                      <button className="ghost tiny" onClick={pullModel} disabled={pulling}>
                        {pulling ? t("downloading") : <><Download className="ico" aria-hidden="true" /> {t("downloadModel")}</>}
                      </button>
                    </Warn>
                  ) : selectedOllama && !selectedOllama.ready ? (
                    <Warn>{selectedOllama.note}
                      <button className="ghost tiny" onClick={pullModel} disabled={pulling}>
                        {pulling ? t("downloading") : <><Download className="ico" aria-hidden="true" /> {t("downloadSelected")}</>}
                      </button>
                    </Warn>
                  ) : ping && !ping.ok ? (
                    <Warn>{ping.error}</Warn>
                  ) : null}
                </label>
                <Select value={ollamaModel} onChange={setOllamaModel} options={[
                  ...ollamaInstalled.map((m) => ({
                    value: m.name,
                    label: m.name,
                    // Spesifikasi jadi keterangan kecil, bukan sambungan nama:
                    // "llama3.1:latest — 8.0B · Q4_K_M · 4.9 GB ✓ ready" satu baris
                    // penuh itu yang membuat daftarnya sesak dan tidak terbaca.
                    note: [m.params, sizeGB(m.bytes), m.ready ? t("modelReady") : t("modelNotCapable")]
                      .filter(Boolean).join(" · "),
                  })),
                  // Saran hanya muncul bila belum terpasang — dicek per nama dasar
                  // supaya "qwen2.5" tidak tampil ganda dengan "qwen2.5:latest".
                  ...OLLAMA_SUGGESTED.filter((x) => !ollamaInstalled.some((m) => sameModel(m.name, x)))
                    .map((x) => ({ value: x, label: x, note: t("modelNeedsDownload") })),
                ]} />
              </div>
              {/* Tombol uji berdiri di kolom KETIGA, sejajar dengan kedua kotak
                  di kirinya. Hasilnya tidak menambah baris: berhasil → tombol
                  hijau dengan waktunya di tooltip, gagal → lambang peringatan
                  di label model. */}
              <div className="field field-check">
                <button className={"ghost" + (ping?.ok ? " ok-btn" : "")} onClick={pingModel}
                  title={ping?.ok ? t("pingOk", { ms: ping.ms }) : t("pingTestTip")}
                  disabled={pinging || !ollamaStatus?.running}>
                  {pinging ? t("pingBusy") : <><Plug className="ico" aria-hidden="true" /> {t("pingTest")}</>}
                </button>
              </div>
              </>
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
              onChange={(e) => setTranscriptFix(e.target.checked)} /> {t("transcriptFix")} <Info className="ico hint" aria-hidden="true" />
            {transcriptFix && mode !== "hybrid" && offlineEngine === "heuristic" && (
              <Warn>{t("transcriptFixNeedsLLM")}</Warn>
            )}
          </label>
          {transcriptFix && (
            <>
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
        <div className="grid3">
          <div className="field"><label>{t("resolution")}</label>
            <Select value={resolution} onChange={setResolution} options={[
              { value: "720p", label: "720p" }, { value: "1080p", label: "1080p" },
              { value: "1440p", label: "1440p" },
            ]} /></div>
          <div className="field"><label>{t("quality")}</label>
            <Select value={quality} onChange={setQuality} options={[
              { value: "draft", label: t("qualityDraft") }, { value: "hd", label: t("qualityHd") },
              { value: "max", label: t("qualityMax") },
            ]} /></div>
          <div className="field"><label title={t("fpsTip")}>{t("fps")} <Info className="ico hint" aria-hidden="true" /></label>
            <Select value={String(fps)} onChange={(v) => setFps(Number(v))} options={[
              { value: "0", label: t("fpsSource") }, { value: "24", label: "24" },
              { value: "30", label: "30" }, { value: "60", label: "60" },
            ]} /></div>
        </div>
      </div>

      <div className="group">
        <div className="group-title">{t("groupClips")}</div>
        <div className="grid3">
          <div className="field"><label title={t("clipDurationTip")}>{t("clipDuration")} <Info className="ico hint" aria-hidden="true" /></label>
            <Select value={durationPreset} onChange={setDurationPreset} options={[
              { value: "auto", label: t("durationAuto") },
              ...["30s", "60s", "90s", "2 min", "3 min"].map((n, i) => ({
                value: ["30", "60", "90", "120", "180"][i], label: t("durationAbout", { n }),
              })),
            ]} /></div>
          <div className="field"><label title={t("maxClipsTip")}>{t("maxClips")} <Info className="ico hint" aria-hidden="true" /></label>
            <input type="number" min={1} max={50} value={maxClips} onChange={(e) => setMaxClips(Number(e.target.value))} /></div>
          <div className="field"><label title={t("saveClipsTip")}>{t("saveClips")} <Info className="ico hint" aria-hidden="true" /></label>
            <Select value={saveMode} onChange={setSaveMode} options={[
              { value: "burn", label: t("saveBurn") }, { value: "clean", label: t("saveClean") },
              { value: "both", label: t("saveBoth") },
            ]} /></div>
        </div>
      </div>
    </div>
  );
}

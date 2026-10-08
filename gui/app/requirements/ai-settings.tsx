"use client";

import { useEffect, useState } from "react";
import { useI18n } from "../i18n";
import { eng } from "../engine";
import { saveAI, useAISettings } from "../ai";
import type { AITool } from "../ai";
import { useEngines } from "../engine-picker";
import type { EngineInfo } from "../engine-picker";

// Tab "Mesin AI" (DESIGN-Clipper-Lanjutan §7): mesin yang dipakai SEMUA alat,
// uji koneksi, dan pengecualian per alat. Kunci & alamat tiap mesin tetap
// diisi di daftar "Mesin lain" di bawahnya (EngineSettings).
export default function AISettingsPanel() {
  const { t } = useI18n();
  const { data, setData } = useAISettings();
  const { engines } = useEngines();
  const [editing, setEditing] = useState(false);
  const [test, setTest] = useState<{ ok: boolean; text: string } | null>(null);
  const [testing, setTesting] = useState(false);

  const g = data?.global;
  const info = engines.find((e) => e.id === g?.engine);
  const ready = !!info?.ready;
  const host = (() => { try { return info?.base_url ? new URL(info.base_url).host : ""; } catch { return info?.base_url || ""; } })();

  const runTest = async () => {
    if (!g) return;
    setTesting(true); setTest(null);
    try {
      const res = await fetch(eng("/api/engines/test"), {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ id: g.engine, model: g.model }),
      });
      const d = await res.json();
      setTest(d.ok ? { ok: true, text: t("setTestOk") } : { ok: false, text: t("setTestFailed", { error: d.error || `HTTP ${res.status}` }) });
    } catch (e: any) {
      setTest({ ok: false, text: t("setTestFailed", { error: e.message }) });
    } finally { setTesting(false); }
  };

  const tools: { id: AITool; name: string }[] = [
    { id: "clips", name: t("tabClips") }, { id: "news", name: t("tabNews") },
    { id: "writer", name: t("tabWriter") }, { id: "captions", name: t("tabCaptions") },
  ];

  return (
    <>
      <section className="card">
        <h2>{t("setCurrentEngine")}</h2>
        <p className="step-hint flush">{t("setCurrentEngineHint")}</p>
        {g && !editing && (
          <div className="ai-current">
            <span className={"set-dot " + (ready ? "ok" : "bad")} aria-hidden="true" />
            <div className="grow">
              <p className="set-name">{info?.name || g.engine} <span className="meta">· {ready ? t("setStatusReady") : (info && !info.has_key ? t("setNoKey") : t("setNotRunning"))}</span></p>
              <p className="ai-sub">{[g.model, host].filter(Boolean).join(" · ")}</p>
            </div>
            <button type="button" className="ai-btn" onClick={runTest} disabled={testing}>{testing ? t("llmTesting") : t("setTest")}</button>
            <button type="button" className="ghost" onClick={() => setEditing(true)}>{t("setChange")}</button>
          </div>
        )}
        {g && editing && (
          <Chooser engines={engines} initial={g} allowHeuristic={false}
            onCancel={() => setEditing(false)}
            onSave={async (c) => { setData(await saveAI("", c.engine, c.model)); setEditing(false); setTest(null); }} />
        )}
        {test && <p className={test.ok ? "set-ok" : "set-err"} role="status">{test.text}</p>}
      </section>

      <section className="card">
        <h2>{t("setExceptions")}</h2>
        <p className="step-hint flush">{t("setExceptionsHint")}</p>
        {data && tools.map((tool) => (
          <ExceptionRow key={tool.id} name={tool.name} engines={engines} current={data.overrides[tool.id]}
            allowHeuristic={tool.id === "clips"}
            onSave={async (c) => setData(await saveAI(tool.id, c?.engine || "", c?.model || ""))} />
        ))}
      </section>
    </>
  );
}

// Pemilih mesin + model. Model boleh kosong = model bawaan mesin itu.
function Chooser({ engines, initial, allowHeuristic, onSave, onCancel }: {
  engines: EngineInfo[]; initial: { engine: string; model: string }; allowHeuristic: boolean;
  onSave: (c: { engine: string; model: string }) => Promise<void>; onCancel?: () => void;
}) {
  const { t } = useI18n();
  const [engine, setEngine] = useState(initial.engine);
  const [model, setModel] = useState(initial.model);
  const [models, setModels] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  useEffect(() => {
    if (!engine || engine === "heuristic") { setModels([]); return; }
    fetch(eng(`/api/engines/${engine}/models`)).then((r) => r.json()).then((d) => setModels(d.models || [])).catch(() => setModels([]));
  }, [engine]);
  const ready = engines.filter((e) => e.ready || e.id === engine);
  const listID = `models-${engine}`;
  return (
    <div className="ai-chooser">
      <div className="field">
        <label>{t("setEngineLabel")}</label>
        <select value={engine} onChange={(e) => { setEngine(e.target.value); setModel(""); }}>
          {ready.map((e) => <option key={e.id} value={e.id}>{e.name}</option>)}
          {allowHeuristic && <option value="heuristic">{t("offlineHeuristic")}</option>}
        </select>
      </div>
      {engine !== "heuristic" && (
        <div className="field">
          <label>{t("setModelLabel")}</label>
          <input list={listID} value={model} placeholder={engines.find((e) => e.id === engine)?.model || ""} onChange={(e) => setModel(e.target.value)} />
          <datalist id={listID}>{models.map((m) => <option key={m} value={m} />)}</datalist>
        </div>
      )}
      <div className="ai-chooser-actions">
        <button type="button" className="primary" disabled={saving || !engine} onClick={async () => {
          setSaving(true); setError("");
          try { await onSave({ engine, model: engine === "heuristic" ? "" : model }); } catch (e: any) { setError(e.message); }
          finally { setSaving(false); }
        }}>{t("save")}</button>
        {onCancel && <button type="button" className="ghost" onClick={onCancel}>{t("cancel")}</button>}
      </div>
      {error && <p className="set-err">{error}</p>}
    </div>
  );
}

function ExceptionRow({ name, engines, current, allowHeuristic, onSave }: {
  name: string; engines: EngineInfo[]; current: { engine: string; model: string } | null; allowHeuristic: boolean;
  onSave: (c: { engine: string; model: string } | null) => Promise<void>;
}) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  const label = current
    ? `${current.engine === "heuristic" ? t("offlineHeuristic") : (engines.find((e) => e.id === current.engine)?.name || current.engine)}${current.model ? " · " + current.model : ""}`
    : t("setFollowMain");
  return (
    <div className="set-row">
      <div className="set-main">
        <p className="set-name">{name}</p>
        <p className="meta">{label}</p>
        {open && (
          <Chooser engines={engines} initial={current || { engine: engines.find((e) => e.ready)?.id || "", model: "" }}
            allowHeuristic={allowHeuristic} onCancel={() => setOpen(false)}
            onSave={async (c) => { await onSave(c); setOpen(false); }} />
        )}
      </div>
      {!open && (
        <div className="set-actions">
          <button type="button" className="ghost" onClick={() => setOpen(true)}>{current ? t("setChange") : t("setUseOther")}</button>
          {current && <button type="button" className="ghost" onClick={() => onSave(null)}>{t("setFollowMainBtn")}</button>}
        </div>
      )}
    </div>
  );
}

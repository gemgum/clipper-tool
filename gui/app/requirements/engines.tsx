"use client";

// Bagian "Mesin AI" di halaman setelan (notes/39).
//
// Di SINI kunci API diisi, dan hanya di sini. Tab-tab hanya MEMILIH mesin yang
// sudah siap — pembedaannya bukan kerapian: kunci disetel sekali seumur
// pemasangan, sedangkan mesin & model dipilih tiap kali bekerja.
//
// Bagian ini sengaja TIDAK digabung dengan "Separate applications" di bawahnya.
// "Belum ada kuncinya" dan "belum terpasang programnya" kelihatan seperti
// masalah yang sama, padahal penyelesaiannya sama sekali berbeda.

import { useState } from "react";
import { Plug } from "lucide-react";
import { eng } from "../engine";
import { useI18n } from "../i18n";
import { useEngines, type EngineInfo } from "../engine-picker";

type Result = { ok: boolean; schema: boolean; strict: boolean; error?: string; models?: string[] };

export default function EngineSettings() {
  const { engines, reload } = useEngines();
  const { t } = useI18n();

  return (
    <div className="panel">
      <div className="meta" style={{ marginBottom: 4 }}>{t("enginesTitle")}</div>
      <div className="meta" style={{ marginBottom: 12 }}>{t("enginesHint")}</div>
      {engines.map((e) => <Row key={e.id} e={e} onSaved={reload} />)}
    </div>
  );
}

function Row({ e, onSaved }: { e: EngineInfo; onSaved: () => void }) {
  const { t } = useI18n();
  const [key, setKey] = useState("");
  const [base, setBase] = useState(e.base_url);
  const [model, setModel] = useState(e.model);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [result, setResult] = useState<Result | null>(null);

  const local = e.kind === "local";

  const save = async () => {
    setSaving(true);
    try {
      const body: Record<string, string> = { id: e.id, base_url: base, model };
      // Kunci hanya dikirim bila diketik. Field yang tidak dikirim tidak
      // disentuh engine — tanpa itu, menyimpan model saja akan menghapus kunci
      // yang sudah ada.
      if (key) body.api_key = key;
      const r = await fetch(eng("/api/engines"), {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!r.ok) throw new Error(await r.text());
      setKey("");
      setResult(null);
      onSaved();
    } catch (err) {
      setResult({ ok: false, schema: false, strict: false, error: String(err) });
    } finally {
      setSaving(false);
    }
  };

  const test = async () => {
    setTesting(true);
    setResult(null);
    try {
      const r = await fetch(eng("/api/engines/test"), {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ id: e.id, model }),
      });
      setResult(await r.json());
    } catch (err) {
      setResult({ ok: false, schema: false, strict: false, error: String(err) });
    } finally {
      setTesting(false);
    }
  };

  const status = local
    ? (e.ready ? t("engineReady") : t("engineOffline"))
    : (e.has_key ? t("engineReady") : t("engineNoKey"));

  return (
    <div className="engine-row">
      <div className="engine-row-head">
        <span className={"req-dot " + (e.ready ? "on" : "idle")} />
        <span className="engine-row-name">{e.name}</span>
        <span className="meta">{status}</span>
        {/* Tautan ke tempat kuncinya diambil. Tanpa ini, "isi kuncinya" adalah
            perintah tanpa alamat. */}
        {e.keys_url && !e.has_key && (
          <a className="meta" href={e.keys_url} target="_blank" rel="noreferrer">{t("engineGetKey")}</a>
        )}
      </div>

      <div className="engine-row-fields">
        {!local && (
          <div className="field">
            <label htmlFor={`k-${e.id}`}>{t("engineKeyLabel")}</label>
            <input id={`k-${e.id}`} type="password" value={key}
              placeholder={e.has_key ? t("keyPlaceholderStored") : t("engineKeyPlaceholder")}
              onChange={(ev) => setKey(ev.target.value)} />
          </div>
        )}
        <div className="field">
          <label htmlFor={`b-${e.id}`}>{t("engineBaseLabel")}</label>
          <input id={`b-${e.id}`} value={base} placeholder={local ? t("llmServerAuto") : ""}
            onChange={(ev) => setBase(ev.target.value)} />
        </div>
        <div className="field">
          <label htmlFor={`m-${e.id}`}>{t("engineModelLabel")}</label>
          <input id={`m-${e.id}`} value={model} onChange={(ev) => setModel(ev.target.value)} />
        </div>
        <div className="field engine-actions">
          <label className="engine-result engine-row-result">
            {result?.ok && result.strict && <span className="ok">{t("engineOK")}</span>}
            {result?.ok && !result.strict && (
              <span className="meta" title={t("engineNoSchemaTip")}>{t("engineNoSchema")}</span>
            )}
            {result && !result.ok && <span className="bad" title={result.error}>{t("engineFailed")}</span>}
          </label>
          <div className="path-row">
            <button onClick={save} disabled={saving}>{t("save")}</button>
            <button className="ghost" onClick={test} disabled={testing || !e.ready}>
              <Plug className="ico" aria-hidden="true" /> {testing ? t("engineTesting") : t("engineTest")}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

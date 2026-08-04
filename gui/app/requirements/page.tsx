"use client";

// Halaman Requirements: daftar komponen, statusnya, dan tombol pasang.
//
// Ini yang menggantikan setup.sh bagi pengguna aplikasi. Engine yang tahu apa
// yang ada dan apa yang kurang — halaman ini hanya menampilkan jawabannya dan
// meneruskan tombolnya, supaya "apa yang belum terpasang" tidak pernah jadi
// tebakan.

import { useCallback, useEffect, useRef, useState } from "react";
import { useI18n } from "../i18n";
import { eng } from "../engine";

type Component = {
  id: string;
  name: string;
  kind: "tool" | "model" | "app";
  required: boolean;
  installed: boolean;
  path: string;
  detail: string;
  size: string;
  installable: boolean;
  hint: string;
  url: string;
};

type Requirements = {
  components: Component[];
  missing: string[];
  data_dir: string;
  models_dir: string;
  tools_dir: string;
  dev: boolean;
};

// Kemajuan satu pemasangan yang sedang berjalan.
type Running = { id: string; value: number; message: string };

export default function RequirementsPage() {
  const { t } = useI18n();
  const [req, setReq] = useState<Requirements | null>(null);
  const [error, setError] = useState("");
  const [running, setRunning] = useState<Running | null>(null);
  const [busy, setBusy] = useState(true);
  // Pesan hasil per komponen (selesai / gagal), hilang saat dicoba lagi.
  const [notes, setNotes] = useState<Record<string, string>>({});
  const abort = useRef<AbortController | null>(null);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      const res = await fetch(eng(`/api/requirements`));
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "failed");
      setReq(data);
      setError("");
    } catch {
      setError(t("engineUnreachable"));
    } finally {
      setBusy(false);
    }
  }, [t]);

  useEffect(() => {
    load();
    return () => abort.current?.abort();
  }, [load]);

  // install membaca aliran SSE dari respons POST.
  //
  // EventSource tidak dipakai karena ia hanya bisa GET, sedangkan memasang
  // komponen jelas mengubah keadaan mesin. Format datanya tetap SSE — sama
  // dengan progres job.
  const install = useCallback(
    async (c: Component) => {
      setNotes((n) => ({ ...n, [c.id]: "" }));
      setRunning({ id: c.id, value: 0, message: t("reqStarting") });
      const ctrl = new AbortController();
      abort.current = ctrl;
      try {
        const res = await fetch(eng(`/api/requirements/install`), {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ id: c.id }),
          signal: ctrl.signal,
        });
        if (!res.ok || !res.body) {
          const data = await res.json().catch(() => ({}));
          throw new Error(data.error || `HTTP ${res.status}`);
        }
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        let failed = "";
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          // Satu pesan SSE berakhir di baris kosong.
          const chunks = buffer.split("\n\n");
          buffer = chunks.pop() || "";
          for (const chunk of chunks) {
            let event = "message";
            let data = "";
            for (const line of chunk.split("\n")) {
              if (line.startsWith("event:")) event = line.slice(6).trim();
              if (line.startsWith("data:")) data += line.slice(5).trim();
            }
            if (!data) continue;
            const payload = JSON.parse(data);
            if (event === "progress") {
              setRunning({ id: c.id, value: payload.value, message: payload.message });
            } else if (event === "error") {
              failed = payload.message;
            } else if (event === "done" && payload.components) {
              setReq((r) => (r ? { ...r, components: payload.components } : r));
            }
          }
        }
        if (failed) throw new Error(failed);
        setNotes((n) => ({ ...n, [c.id]: t("reqInstalled") }));
        load(); // tarik status lengkap (termasuk "kurang apa lagi")
      } catch (e: any) {
        if (e.name !== "AbortError") {
          setNotes((n) => ({ ...n, [c.id]: `⚠ ${e.message}` }));
        }
      } finally {
        setRunning(null);
        abort.current = null;
      }
    },
    [load, t]
  );

  const remove = useCallback(
    async (c: Component) => {
      try {
        const res = await fetch(eng(`/api/requirements/remove`), {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ id: c.id }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        setNotes((n) => ({ ...n, [c.id]: t("reqRemoved") }));
        load();
      } catch (e: any) {
        setNotes((n) => ({ ...n, [c.id]: `⚠ ${e.message}` }));
      }
    },
    [load, t]
  );

  // Engine mengirim daftar kosong, bukan null — tapi versi lama tidak, dan satu
  // baris pertahanan di sini lebih murah daripada halaman yang gagal dirender.
  const missing = req?.missing ?? [];

  const groups: { title: string; kind: Component["kind"] }[] = [
    { title: t("reqGroupTools"), kind: "tool" },
    { title: t("reqGroupModels"), kind: "model" },
    { title: t("reqGroupApps"), kind: "app" },
  ];

  return (
    <main className="wrap">
      <h1>{t("reqTitle")}</h1>
      <p className="sub">{t("reqSubtitle")}</p>

      {error && <div className="panel err">{error}</div>}

      {missing.length > 0 && (
        <div className="warnbox">{t("reqMissing", { list: missing.join(", ") })}</div>
      )}
      {req && missing.length === 0 && !busy && (
        <div className="okbox">{t("reqAllReady")}</div>
      )}

      {groups.map((g) => {
        const items = req?.components.filter((c) => c.kind === g.kind) || [];
        if (items.length === 0) return null;
        return (
          <div className="panel" key={g.kind}>
            <div className="meta" style={{ marginBottom: 12 }}>{g.title}</div>
            {items.map((c) => {
              const live = running?.id === c.id;
              return (
                <div className="req-row" key={c.id}>
                  <div className={`req-dot ${c.installed ? "on" : c.required ? "off" : "idle"}`} />
                  <div className="req-main">
                    <div className="req-name">
                      {c.name}
                      {c.required && !c.installed && <span className="req-tag">{t("reqRequired")}</span>}
                      {c.size && !c.installed && <span className="meta"> · {c.size}</span>}
                    </div>
                    <div className="meta">{c.detail}</div>
                    {c.installed && c.path && <div className="req-path">{c.path}</div>}
                    {!c.installed && !c.installable && c.hint && <div className="meta">{c.hint}</div>}
                    {notes[c.id] && <div className="meta">{notes[c.id]}</div>}
                    {live && (
                      <div style={{ marginTop: 8 }}>
                        <div className="progress-outer">
                          <div
                            className="progress-inner"
                            style={{ width: `${Math.max(0, running!.value) * 100}%` }}
                          />
                        </div>
                        <div className="meta" style={{ marginTop: 4 }}>{running!.message}</div>
                      </div>
                    )}
                  </div>
                  <div className="req-actions">
                    {!c.installed && c.installable && (
                      <button disabled={!!running} onClick={() => install(c)}>
                        {live ? t("reqInstalling") : t("reqInstall")}
                      </button>
                    )}
                    {!c.installed && !c.installable && c.url && (
                      <a className="dl" href={c.url} target="_blank" rel="noreferrer">
                        {t("reqOpenDownload")} ↗
                      </a>
                    )}
                    {c.installed && c.kind === "model" && (
                      <button className="ghost" disabled={!!running} onClick={() => remove(c)}>
                        {t("reqRemove")}
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        );
      })}

      {req && (
        <div className="panel">
          <div className="meta" style={{ marginBottom: 10 }}>{t("reqWhereTitle")}</div>
          <div className="req-path">{t("reqWhereModels")}: {req.models_dir}</div>
          <div className="req-path">{t("reqWhereTools")}: {req.tools_dir}</div>
          <div className="req-path">{t("reqWhereData")}: {req.data_dir}</div>
          {req.dev && <div className="meta" style={{ marginTop: 8 }}>{t("reqDevNote")}</div>}
        </div>
      )}

      <button className="ghost" disabled={busy || !!running} onClick={load}>
        {busy ? t("loading") : t("reqRefresh")}
      </button>
    </main>
  );
}

"use client";

// Halaman Requirements: daftar komponen, statusnya, dan tombol pasang.
//
// Ini yang menggantikan setup.sh bagi pengguna aplikasi. Engine yang tahu apa
// yang ada dan apa yang kurang — halaman ini hanya menampilkan jawabannya dan
// meneruskan tombolnya, supaya "apa yang belum terpasang" tidak pernah jadi
// tebakan.

import { useCallback, useEffect, useRef, useState } from "react";
import { useI18n } from "../i18n";
import { eng, engineURL } from "../engine";
import Picker from "../picker";
import Alerts from "../alerts";
import Warn from "../warn";

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
  pointable: boolean;
};

type Folders = {
  clips_dir: string;
  cards_dir: string;
  clips_dir_used: string;
  cards_dir_used: string;
};

type Requirements = {
  components: Component[];
  missing: string[];
  data_dir: string;
  models_dir: string;
  tools_dir: string;
  dev: boolean;
};

// Kemajuan pemasangan, apa adanya dari engine.
//
// Halaman ini TIDAK lagi menjalankan unduhannya. Ia hanya menonton: engine yang
// mengunduh, di latar, dan tetap jalan walau jendela ditinggal atau ditutup.
type Install = {
  id: string;
  running: boolean;
  value: number;
  message: string;
  bytes: number;
  total: number;
  error?: string;
  done: boolean;
};

export default function RequirementsPage() {
  const { t } = useI18n();
  const [req, setReq] = useState<Requirements | null>(null);
  const [error, setError] = useState("");
  const [installs, setInstalls] = useState<Record<string, Install>>({});
  const [picking, setPicking] = useState<Component | null>(null);
  const [folders, setFolders] = useState<Folders | null>(null);
  // Folder mana yang sedang dipilih: "clips" | "cards" | null.
  const [pickingFolder, setPickingFolder] = useState<null | "clips" | "cards">(null);
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
      const fs = await fetch(eng(`/api/settings`)).then((r) => r.json());
      setFolders(fs);
    } catch {
      setError(t("engineUnreachable", { url: engineURL() }));
    } finally {
      setBusy(false);
    }
  }, [t]);

  useEffect(() => {
    load();
    return () => abort.current?.abort();
  }, [load]);

  // install hanya MEMULAI. Kemajuannya datang lewat langganan di bawah, jadi
  // menutup atau meninggalkan halaman ini tidak menghentikan apa pun.
  const install = useCallback(
    async (c: Component) => {
      setNotes((n) => ({ ...n, [c.id]: "" }));
      try {
        const res = await fetch(eng(`/api/requirements/install`), {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ id: c.id }),
        });
        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          throw new Error(data.error || `HTTP ${res.status}`);
        }
      } catch (e: any) {
        setNotes((n) => ({ ...n, [c.id]: `⚠ ${e.message}` }));
      }
    },
    []
  );

  // Langganan kabar pemasangan. Pesan pertama dari engine berisi keadaan
  // terkini, jadi halaman yang baru dibuka langsung menampilkan unduhan yang
  // sedang berjalan — termasuk yang dimulai sebelum halaman ini dibuka.
  useEffect(() => {
    const es = new EventSource(eng(`/api/requirements/events`));
    es.addEventListener("install", (ev) => {
      const st: Install = JSON.parse((ev as MessageEvent).data);
      setInstalls((prev) => ({ ...prev, [st.id]: st }));
      if (st.done) load();
    });
    return () => es.close();
  }, [load]);

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

  // Menunjuk program yang sudah ada di komputer, bagi yang tidak mau (atau
  // tidak bisa) mengunduh ulang — dan bagi Chrome/Edge yang memang tidak
  // pernah kita unduh sendiri.
  const setPath = useCallback(
    async (c: Component, path: string) => {
      setPicking(null);
      try {
        const res = await fetch(eng(`/api/requirements/path`), {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ id: c.id, path }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        setNotes((n) => ({ ...n, [c.id]: t("reqPathSaved") }));
        load();
      } catch (e: any) {
        setNotes((n) => ({ ...n, [c.id]: `⚠ ${e.message}` }));
      }
    },
    [load, t]
  );

  // Menyimpan tempat klip / kartu. Folder kosong = kembali ke folder data.
  const saveFolder = useCallback(
    async (which: "clips" | "cards", dir: string) => {
      setPickingFolder(null);
      try {
        const res = await fetch(eng(`/api/settings/folders`), {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ [which]: dir }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        setFolders(data);
      } catch (e: any) {
        setError(e.message);
      }
    },
    []
  );

  // Yang kurang DAN bisa dipasang engine — dipakai tombol "pasang semua" di
  // notifikasi. Yang tidak bisa dipasang (Ollama, Chrome) tidak ikut: tombol
  // yang menjanjikan sesuatu yang tidak bisa dikerjakan lebih buruk daripada
  // tidak ada tombol.
  const installable = (req?.components || []).filter((c) => c.required && !c.installed && c.installable);
  const anyInstalling = Object.values(installs).some((st) => st.running);

  const groups: { title: string; kind: Component["kind"] }[] = [
    { title: t("reqGroupTools"), kind: "tool" },
    { title: t("reqGroupModels"), kind: "model" },
    { title: t("reqGroupApps"), kind: "app" },
  ];

  return (
    <main className="screen">
      {pickingFolder && (
        <Picker
          mode="folder"
          start={pickingFolder === "clips" ? folders?.clips_dir_used : folders?.cards_dir_used}
          onPick={(p) => saveFolder(pickingFolder, p)}
          onClose={() => setPickingFolder(null)}
        />
      )}

      {picking && (
        <Picker
          mode="file"
          title={t("pickerProgramTitle", { name: picking.name })}
          hint={t("pickerProgramHint")}
          start={picking.path}
          onPick={(p) => setPath(picking, p)}
          onClose={() => setPicking(null)}
        />
      )}

      {/* Galat & "ada yang kurang" melayang di atas halaman (alerts.tsx), jadi
          munculnya tidak pernah menggeser daftar komponen yang sedang dibaca.
          "Semuanya siap" tidak lagi ditampilkan sebagai kotak: titik hijau di
          tiap baris sudah mengatakannya, dan kabar baik tidak perlu menyita
          tempat. */}
      <Alerts items={[
        error && { kind: "error" as const, text: error },
        missing.length > 0 && { kind: "warn" as const, key: `missing-${missing.join(",")}`,
          // Notifikasi yang cuma menyebut apa yang kurang menyuruh orang mencari
          // barisnya sendiri di daftar sepanjang sebelas komponen. Tombolnya
          // ada DI SINI: satu klik memasang semua yang kurang dan bisa dipasang.
          text: <>{t("reqMissing", { list: missing.join(", ") })}{" "}
            {installable.length > 0 && (
              <button className="ghost tiny" disabled={anyInstalling}
                onClick={() => installable.forEach(install)}>
                {anyInstalling ? t("reqInstalling") : t("reqInstallMissing", { n: installable.length })}
              </button>
            )}</> },
      ]} />

      <div className="screen-body">
      {/* Kiri: yang ditindaklanjuti. Bergulir di dalam kotaknya sendiri. */}
      <div className="screen-main">
      {groups.map((g) => {
        const items = req?.components.filter((c) => c.kind === g.kind) || [];
        if (items.length === 0) return null;
        return (
          <div className="panel" key={g.kind}>
            <div className="meta" style={{ marginBottom: 12 }}>{g.title}</div>
            {items.map((c) => {
              const st = installs[c.id];
              const live = st?.running ?? false;
              return (
                <div className="req-row" key={c.id}>
                  <div className={`req-dot ${c.installed ? "on" : c.required ? "off" : "idle"}`} />
                  <div className="req-main">
                    <div className="req-name">
                      {c.name}
                      {st?.error && <Warn>{st.error}</Warn>}
                      {c.required && !c.installed && <span className="req-tag">{t("reqRequired")}</span>}
                      {c.size && !c.installed && <span className="meta"> · {c.size}</span>}
                    </div>
                    {/* SATU baris keterangan, apa pun keadaannya: saat memasang
                        ia berganti jadi kabar kemajuan, bukan menambah baris di
                        bawahnya. Baris yang bertambah mendorong seluruh daftar
                        komponen turun tepat saat tombol Install ditekan. */}
                    <div className="meta req-line">{live ? st.message : c.detail}</div>
                    {c.installed && c.path && <div className="req-path">{c.path}</div>}
                    {!c.installed && !c.installable && c.hint && <div className="meta req-line">{c.hint}</div>}
                    {notes[c.id] && <div className="meta req-line">{notes[c.id]}</div>}
                    {/* Bilah kemajuan menempel di DASAR barisnya, di luar aliran:
                        munculnya tidak menambah satu piksel pun. */}
                    {live && (
                      <div className="req-progress">
                        <div className="progress-inner" style={{ width: `${Math.max(0, st.value) * 100}%` }} />
                      </div>
                    )}
                  </div>
                  <div className="req-actions">
                    {!c.installed && c.installable && (
                      <button disabled={live} onClick={() => install(c)}>
                        {live ? t("reqInstalling") : t("reqInstall")}
                      </button>
                    )}
                    {/* Menunjuk sendiri hanya untuk yang MEMANG berkas — engine
                        yang menentukannya (Component.Pointable), bukan halaman
                        ini. Ollama terpasang sebagai layanan jaringan, dan
                        tombol "pakai berkas lain" di barisnya dulu membuka
                        pemilih video: dialog yang tidak berarti apa pun. */}
                    {c.pointable && (
                      <button className="ghost" disabled={live} onClick={() => setPicking(c)}>
                        {c.installed ? t("reqPathChange") : t("reqPathPick")}
                      </button>
                    )}
                    {!c.installed && !c.installable && c.url && (
                      <a className="dl" href={c.url} target="_blank" rel="noreferrer">
                        {t("reqOpenDownload")} ↗
                      </a>
                    )}
                    {c.installed && c.kind === "model" && (
                      <button className="ghost" disabled={live} onClick={() => remove(c)}>
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

      </div>

      {/* Kanan: keterangan tempat. Jarang disentuh, jadi ia menepi — bukan
          mendorong daftar komponen keluar layar seperti sebelumnya. */}
      <div className="screen-side">
      {req && (
        <div className="panel">
          <div className="meta" style={{ marginBottom: 12 }}>{t("reqWhereTitle")}</div>

          {/* Milik pengguna — bisa dipindah. Ditaruh di atas karena inilah yang
              orang cari saat membuka bagian ini. */}
          {(
            [
              ["clips", t("reqFolderClips"), folders?.clips_dir_used, folders?.clips_dir],
              ["cards", t("reqFolderCards"), folders?.cards_dir_used, folders?.cards_dir],
            ] as const
          ).map(([key, label, used, custom]) => (
            <div className="req-row" key={key}>
              <div className="req-dot idle" />
              <div className="req-main">
                <div className="req-name">{label}</div>
                <div className="req-path" title={used || ""}>{used || "—"}</div>
                {!custom && <div className="meta">{t("reqFolderDefault")}</div>}
              </div>
              <div className="req-actions">
                <button className="ghost" onClick={() => setPickingFolder(key)}>
                  {t("reqFolderChange")}
                </button>
                {custom && (
                  <button className="ghost" onClick={() => saveFolder(key, "")}>
                    {t("reqFolderReset")}
                  </button>
                )}
              </div>
            </div>
          ))}

          {/* Milik aplikasi — ditampilkan supaya bisa ditemukan, tidak untuk
              dipindah lewat sini. */}
          <div style={{ marginTop: 14 }}>
            <div className="req-path" title={req.models_dir}>{t("reqWhereModels")}: {req.models_dir}</div>
            <div className="req-path" title={req.tools_dir}>{t("reqWhereTools")}: {req.tools_dir}</div>
            <div className="req-path" title={req.data_dir}>{t("reqWhereData")}: {req.data_dir}</div>
          </div>
          {req.dev && <div className="meta" style={{ marginTop: 8 }}>{t("reqDevNote")}</div>}
        </div>
      )}

      <button className="ghost" disabled={busy} onClick={load}>
        {busy ? t("loading") : t("reqRefresh")}
      </button>
      </div>
      </div>
    </main>
  );
}

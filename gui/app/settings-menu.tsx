"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Settings } from "lucide-react";
import { eng } from "./engine";
import { LANGUAGES, useI18n } from "./i18n";
import Popover from "./popover";

// Panel setelan yang dibuka ikon gerigi di bilah atas.
//
// Melayang, bukan halaman penuh: isinya disetel sekali lalu dilupakan, dan
// memaksanya jadi halaman berarti pengguna kehilangan tempatnya di halaman klip
// setiap kali ingin menukar bahasa. Yang panjang — pemasangan 2,9 GB dengan
// bilah kemajuan — tetap di halaman Requirements; di sini hanya ringkasannya.

type Component = { id: string; name: string; required: boolean; installed: boolean };

export default function SettingsMenu() {
  const { lang, setLang, t } = useI18n();
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<Component[] | null>(null);
  // Alamat & kunci server LLM. Di SINI, bukan di halaman klip: keduanya disetel
  // sekali lalu dilupakan — persis sifat isi panel ini — sedangkan halaman klip
  // hanya memuat yang diubah tiap job. Menaruhnya di sana juga memaksa kisi
  // mesin skor melipat ke baris kedua, dan baris itu tinggi yang diambil dari
  // pratinjau tiap kali halaman dibuka.
  const [llmUrl, setLlmUrl] = useState("");
  const [llmKey, setLlmKey] = useState("");
  const [hasLlmKey, setHasLlmKey] = useState(false);
  const [llmServer, setLlmServer] = useState("");
  const [saving, setSaving] = useState(false);

  // Status komponen ditarik saat panel DIBUKA — dan SETIAP kali dibuka.
  //
  // Sebelumnya hasil pertama disimpan selamanya (`if (!open || items) return`),
  // jadi daftar yang terbaca sebelum komponennya dipasang tetap berbunyi
  // "missing" sampai aplikasi ditutup — sementara halaman Requirements di
  // belakangnya sudah hijau semua. Dua tempat yang menjawab pertanyaan sama
  // dengan jawaban berbeda, dan yang salah justru yang paling gampang dilihat.
  useEffect(() => {
    if (!open) return;
    fetch(eng(`/api/requirements`))
      .then((r) => r.json())
      .then((d) => setItems(d.components || []))
      .catch(() => setItems([]));
    fetch(eng(`/api/settings`))
      .then((r) => r.json())
      .then((d) => { setLlmUrl(d.llm_url || ""); setHasLlmKey(!!d.has_llm_key); })
      .catch(() => {});
    fetch(eng(`/api/ollama/status`))
      .then((r) => r.json())
      .then((d) => setLlmServer(d.running ? d.server || "" : ""))
      .catch(() => setLlmServer(""));
  }, [open]);

  // Menyimpan lalu MEMERIKSA ULANG. Yang ingin dilihat pengguna sesudah menekan
  // Save bukan kata "tersimpan", melainkan nama server yang menjawab di alamat
  // itu — dan kalau tidak ada, itu harus ketahuan sekarang, bukan saat job
  // pertama berhenti di tengah jalan.
  const saveLLM = async () => {
    setSaving(true);
    try {
      const res = await fetch(eng(`/api/settings`), {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ llm_url: llmUrl, llm_api_key: llmKey }),
      });
      const d = await res.json();
      setHasLlmKey(!!d.has_llm_key);
      setLlmKey("");
      const st = await (await fetch(eng(`/api/ollama/status`))).json();
      setLlmServer(st.running ? st.server || "" : "");
    } catch { setLlmServer(""); } finally { setSaving(false); }
  };

  // Menutup lewat Esc/klik-luar dilayani <Popover>; `open` di sini hanya
  // penanda kapan status komponen perlu ditarik.

  const missing = items?.filter((c) => c.required && !c.installed).length ?? 0;

  return (
    <Popover width={300} buttonClass="rail-tool" side="beside" onOpen={() => setOpen(true)} label={
      <>
        <Settings className="ico" aria-hidden="true" />
        {/* Titik merah hanya muncul bila ada yang WAJIB dan belum ada — kalau
            ia menyala untuk hal opsional, orang berhenti mempercayainya. */}
        {missing > 0 && <span className="dot-bad" aria-hidden="true" />}
      </>
    }>
      {(close) => (
        <div className="settings-pop" role="menu">
          <div className="settings-title">{t("settingsTitle")}</div>

          <div className="settings-group">
            <div className="settings-head">{t("settingsLanguage")}</div>
            <div className="lang-switch">
              {LANGUAGES.map((l) => (
                <button
                  key={l}
                  type="button"
                  className={l === lang ? "active" : ""}
                  aria-pressed={l === lang}
                  onClick={() => setLang(l)}
                >
                  {l === "en" ? "English" : "Indonesia"}
                </button>
              ))}
            </div>
          </div>

          <div className="settings-group">
            <div className="settings-head">{t("llmServer")}</div>
            {/* Nama server yang MENJAWAB, bukan yang diketik: itu satu-satunya
                jawaban yang berguna di sini. Kosong = tidak ada yang menjawab. */}
            <div className="meta" style={{ marginBottom: 6 }}>
              {llmServer ? `✓ ${llmServer}` : t("llmServerNone")}
            </div>
            <div className="path-row">
              <input type="text" value={llmUrl} title={t("llmServerTip")}
                placeholder={t("llmServerAuto")}
                onChange={(e) => setLlmUrl(e.target.value)} />
              <button onClick={saveLLM} disabled={saving}>{t("save")}</button>
            </div>
            <input type="password" value={llmKey} title={t("llmApiKeyTip")}
              style={{ marginTop: 6, width: "100%" }}
              placeholder={hasLlmKey ? t("keyPlaceholderStored") : t("llmApiKeyPlaceholder")}
              onChange={(e) => setLlmKey(e.target.value)} />
          </div>

          <div className="settings-group">
            <div className="settings-head">{t("settingsComponents")}</div>
            {items === null ? (
              <div className="meta">{t("loading")}</div>
            ) : (
              items.slice(0, 6).map((c) => (
                <div className="settings-row" key={c.id}>
                  <span className={"req-dot " + (c.installed ? "on" : c.required ? "off" : "idle")} />
                  <span className="settings-name">{c.name}</span>
                  <span className="meta">{c.installed ? t("settingsReady") : t("settingsMissing")}</span>
                </div>
              ))
            )}
          </div>

          <Link className="settings-more" href="/requirements" onClick={close}>
            {t("settingsOpenFull")}
          </Link>
        </div>
      )}
    </Popover>
  );
}

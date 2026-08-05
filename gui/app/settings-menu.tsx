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

  // Status komponen ditarik saat panel DIBUKA, bukan saat halaman dimuat: ia
  // keterangan sesekali, dan menariknya di awal berarti tiap halaman membayar
  // satu permintaan yang mungkin tidak pernah dilihat.
  useEffect(() => {
    if (!open || items) return;
    fetch(eng(`/api/requirements`))
      .then((r) => r.json())
      .then((d) => setItems(d.components || []))
      .catch(() => setItems([]));
  }, [open, items]);

  // Menutup lewat Esc/klik-luar dilayani <Popover>; `open` di sini hanya
  // penanda kapan status komponen perlu ditarik.

  const missing = items?.filter((c) => c.required && !c.installed).length ?? 0;

  return (
    <Popover width={300} buttonClass="rail-tool" onOpen={() => setOpen(true)} label={
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

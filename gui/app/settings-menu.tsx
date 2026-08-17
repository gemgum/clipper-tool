"use client";

import { useCallback, useEffect, useState } from "react";
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
//
// Alamat & kunci server LLM DIBUANG dari sini 18 Agustus 2026 (notes/39).
// Sesudah mesin cloud masuk, panel ini harus memuat lima mesin dengan tiga
// isian masing-masing — dan panel melayang setinggi 300 px bukan tempatnya.
// Semuanya pindah ke bagian "Mesin AI" di halaman Requirements. Yang tersisa di
// sini adalah apa yang seharusnya: PEMBERI KABAR, bukan tempat menyetel.

type Component = { id: string; name: string; required: boolean; installed: boolean };

export default function SettingsMenu() {
  const { lang, setLang, t } = useI18n();
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<Component[] | null>(null);
  // Versi engine yang SEDANG jalan — bukan angka yang dipaku di GUI. Keduanya
  // dibangun terpisah, jadi angka yang ditulis di sini bisa berbohong.
  const [ver, setVer] = useState("");

  const loadComponents = useCallback(() => {
    fetch(eng(`/api/requirements`))
      .then((r) => r.json())
      .then((d) => setItems(d.components || []))
      .catch(() => setItems([]));
  }, []);

  // Ditarik saat HALAMAN dibuka, bukan hanya saat panel dibuka.
  //
  // Titik merah di tombolnya dihitung dari daftar ini, dan gunanya memberitahu
  // ada yang kurang TANPA membuka panelnya. Selama daftarnya baru diisi setelah
  // panel dibuka sekali, titik itu tidak mungkin muncul lebih dulu — ia
  // memberitahu apa yang sudah dilihat sendiri oleh yang melihatnya.
  useEffect(() => { loadComponents(); }, [loadComponents]);

  // Status komponen ditarik SETIAP kali panel dibuka.
  //
  // Sebelumnya `open` hanya pernah diisi true — tidak ada satu pun yang
  // mengembalikannya ke false — sehingga effect ini berjalan TEPAT SEKALI
  // seumur halaman. Di komputer yang baru dipasang, panel yang dibuka sebelum
  // komponennya diunduh akan terus berbunyi "missing" sampai aplikasi
  // ditutup, sementara halaman Requirements di belakangnya sudah hijau semua.
  // Itu persis laporan yang memicu perbaikan ini — dan yang dibetulkan bukan
  // pemeriksaannya (keduanya memanggil /api/requirements yang sama), melainkan
  // KAPAN ia dipanggil.
  useEffect(() => {
    if (!open) return;
    loadComponents();
    fetch(eng(`/api/health`))
      .then((r) => r.json())
      .then((d) => setVer(d.version || ""))
      .catch(() => {});
  }, [open, loadComponents]);


  // `open` di sini penanda kapan status perlu ditarik ulang, dan ia WAJIB ikut
  // kembali ke false saat panel ditutup — kalau tidak, "dibuka lagi" tidak bisa
  // dibedakan dari "masih terbuka" dan penarikan ulangnya tidak pernah terjadi.
  // Karena itu <Popover> dilanggani lewat onOpenChange (dipanggil pada buka DAN
  // tutup), bukan onOpen (hanya pada buka).

  // EMPAT baris, bukan enam belas.
  //
  // Panel ini melaporkan keadaan, bukan mengelola komponen: yang ingin dilihat
  // orang saat membukanya adalah "siap atau tidak", dan enam belas baris —
  // enam model whisper yang cukup dipunya satu, enam aplikasi LLM yang cukup
  // jalan satu — menjawab pertanyaan itu dengan menyuruhnya membaca dulu.
  // Rinciannya tetap lengkap di halaman Requirements.
  //
  // Kelompoknya juga MEMPERBAIKI hitungan "ada yang kurang": model whisper
  // ditandai `required: false` satu per satu (sebab cukup punya salah satu),
  // jadi mesin yang punya whisper tanpa satu pun model dulu tampil hijau —
  // padahal job pertamanya pasti berhenti.
  const groups = (() => {
    if (!items) return [];
    const some = (f: (c: Component) => boolean) => items.some((c) => f(c) && c.installed);
    const every = (f: (c: Component) => boolean) => {
      const part = items.filter(f);
      return part.length > 0 && part.every((c) => c.installed);
    };
    return [
      { key: "compLibrary", ok: every((c) => c.id === "ffmpeg" || c.id === "ffprobe"), required: true },
      { key: "compTranscribe", ok: some((c) => c.id === "whisper") && some((c) => c.id.startsWith("model:")), required: true },
      { key: "compLLM", ok: some((c) => c.id.startsWith("llm:")), required: false },
      { key: "compChrome", ok: some((c) => c.id === "chrome"), required: false },
    ] as const;
  })();

  const missing = groups.filter((g) => g.required && !g.ok).length;

  return (
    <Popover width={300} buttonClass="rail-tool" side="beside" onOpenChange={setOpen} label={
      <>
        <Settings className="ico" aria-hidden="true" />
        {/* Titik merah hanya muncul bila ada yang WAJIB dan belum ada — kalau
            ia menyala untuk hal opsional, orang berhenti mempercayainya. */}
        {missing > 0 && <span className="dot-bad" aria-hidden="true" />}
      </>
    }>
      {(close) => (
        <div className="settings-pop" role="menu">
          {/* Versi menumpang di baris judul, bukan di kelompok "About" sendiri.
              "Versi berapa yang saya jalankan" memang harus terjawab tanpa
              membuka terminal — tapi jawabannya satu kata, dan judul + baris
              sendiri berarti ~40 px diambil dari panel yang tingginya
              terbatas. Di baris kaki ia sudah dicoba: 12px bersaing dengan
              tautan Requirements dan keduanya melipat jadi dua baris. */}
          <div className="settings-title">
            {t("settingsTitle")}
            <span className="meta" title={t("aboutVersion")}>{ver || "…"}</span>
          </div>

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
              groups.map((g) => (
                <div className="settings-row" key={g.key}>
                  <span className={"req-dot " + (g.ok ? "on" : g.required ? "off" : "idle")} />
                  {/* Judulnya satu kata; ISI kelompoknya di title, sebab yang
                      ditanya orang begitu satu baris merah adalah "yang mana
                      yang kurang". */}
                  <span className="settings-name" title={t(`${g.key}Tip` as "compLibraryTip")}>
                    {t(g.key as "compLibrary")}
                  </span>
                  <span className="meta">{g.ok ? t("settingsReady") : t("settingsMissing")}</span>
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

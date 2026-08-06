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
  // Kotak alamat manual dipisah dari alamat yang SEDANG berlaku.
  //
  // Sebelumnya keduanya satu nilai, jadi memilih "Ollama" dari daftar membuat
  // alamat yang sama muncul tiga kali di panel setinggi 300 px: sebagai baris
  // tersorot, sebagai isi kotak, dan sebagai teks di bawahnya. Kotak ini
  // sekarang hanya untuk alamat yang TIDAK ada di daftar.
  const [manual, setManual] = useState("");
  const [llmKey, setLlmKey] = useState("");
  const [hasLlmKey, setHasLlmKey] = useState(false);
  const [saving, setSaving] = useState(false);
  // Semua server yang menjawab, bukan cuma yang dipakai.
  const [servers, setServers] = useState<{ url: string; kind: string; name: string }[]>([]);
  // Versi engine yang SEDANG jalan — bukan angka yang dipaku di GUI. Keduanya
  // dibangun terpisah, jadi angka yang ditulis di sini bisa berbohong.
  const [ver, setVer] = useState("");

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
      .then((d) => setServers(d.servers || []))
      .catch(() => setServers([]));
    fetch(eng(`/api/health`))
      .then((r) => r.json())
      .then((d) => setVer(d.version || ""))
      .catch(() => {});
  }, [open]);

  // Menyimpan lalu MEMERIKSA ULANG. Yang ingin dilihat pengguna sesudah menekan
  // Save bukan kata "tersimpan", melainkan nama server yang menjawab di alamat
  // itu — dan kalau tidak ada, itu harus ketahuan sekarang, bukan saat job
  // pertama berhenti di tengah jalan.
  const save = async (url: string, key: string) => {
    setSaving(true);
    try {
      // Kunci HANYA ikut terkirim bila memang diketik. Engine membedakan
      // "tidak dikirim" dari "dikirim kosong" (field pointer di postSettings),
      // dan mengirim string kosong berarti MENGHAPUS kunci tersimpan — yang
      // jelas bukan maksud orang yang cuma memilih server lain dari daftar.
      const body: Record<string, string> = { llm_url: url };
      if (key) body.llm_api_key = key;
      const res = await fetch(eng(`/api/settings`), {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      const d = await res.json();
      setHasLlmKey(!!d.has_llm_key);
      setLlmKey("");
      const st = await (await fetch(eng(`/api/ollama/status`))).json();
      setServers(st.servers || []);
    } catch { setServers([]); } finally { setSaving(false); }
  };

  const saveLLM = () => { setLlmUrl(manual); save(manual, llmKey); };

  // Memilih dari daftar LANGSUNG berlaku — tidak ada tombol Save kedua.
  // Kotak alamat di bawahnya punya Save karena teks yang diketik belum tentu
  // selesai diketik; pilihan dari daftar sudah pasti selesai.
  const pickServer = (url: string) => { setLlmUrl(url); setManual(""); save(url, ""); };

  // Alamat tersimpan yang TIDAK ada di daftar berarti pengguna mengetiknya
  // sendiri — kembalikan ke kotaknya supaya ia bisa melihat & menyuntingnya.
  useEffect(() => {
    if (llmUrl && !servers.some((s) => s.url === llmUrl)) setManual(llmUrl);
  }, [llmUrl, servers]);

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
            <div className="settings-head">{t("llmServer")}</div>
            {/* Tidak ada baris "✓ Ollama" lagi: daftarnya sendiri sudah
                menandai mana yang dipakai, dan dua tempat yang menyatakan hal
                yang sama membuat panel ini tinggi tanpa memberi tahu apa pun.
                Pesan hanya muncul saat memang TIDAK ADA yang menjawab. */}
            {servers.length === 0 && (
              <div className="meta" style={{ marginBottom: 6 }}>{t("llmServerNone")}</div>
            )}
            {/* SEMUA server yang menjawab jadi pilihan.
                Di komputer yang punya tiga sekaligus — Ollama, llama.cpp,
                KoboldCpp — menyebut satu lalu menyuruh pengguna mengetik alamat
                dua lainnya dari ingatan adalah pekerjaan yang tidak perlu ada:
                engine sudah tahu ketiganya, sebab ia memang mengetuk semua port
                itu berbarengan. Kotak alamat di bawah tetap ada untuk server di
                port yang tidak dikenal. */}
            {servers.length > 0 && (
              <div className="srv-list">
                {[{ url: "", name: t("llmServerAuto"), kind: "" }, ...servers].map((s) => (
                  <button key={s.url || "auto"} type="button"
                    className={"srv-row" + (llmUrl === s.url ? " on" : "")}
                    onClick={() => pickServer(s.url)} disabled={saving}>
                    <span className="srv-name">{s.name}</span>
                    <span className="meta">{s.url.replace(/^https?:\/\//, "")}</span>
                  </button>
                ))}
              </div>
            )}
            <div className="path-row">
              <input type="text" value={manual} title={t("llmServerTip")}
                placeholder={t("llmServerOther")}
                onChange={(e) => setManual(e.target.value)} />
              <button onClick={saveLLM} disabled={saving || !manual}>{t("save")}</button>
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
              <div className="settings-scroll">
                {items.map((c) => (
                  <div className="settings-row" key={c.id}>
                    <span className={"req-dot " + (c.installed ? "on" : c.required ? "off" : "idle")} />
                    <span className="settings-name">{c.name}</span>
                    <span className="meta">{c.installed ? t("settingsReady") : t("settingsMissing")}</span>
                  </div>
                ))}
              </div>
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

"use client";

import { useCallback, useEffect, useRef, useState } from "react";

const ENGINE = process.env.NEXT_PUBLIC_ENGINE_URL || "http://127.0.0.1:8787";

// Ruang koordinat kartu — sama dengan yang dipakai engine saat merender.
// Pratinjau memakai CSS transform yang identik, jadi menyeret sejauh N piksel
// di sini menggeser foto sejauh N piksel juga di PNG hasil.
const KARTU_W = 1080;
const TINGGI_KARTU: Record<string, number> = { "9:16": 1920, "4:5": 1350, "1:1": 1080 };
// Harus sama dengan tinggiFoto di engine/internal/card/card.go — kalau berbeda,
// kotak pratinjau tidak lagi sebangun dengan bingkai foto pada hasil render.
const PERSEN_FOTO: Record<string, number> = { "9:16": 50, "4:5": 50, "1:1": 48 };

type Artikel = {
  judul: string;
  ringkas: string;
  url: string;
  gambar: string;
  sumber: string;
  domain: string;
  tanggal: string;
  terbit: string;
};

type Feed = { id: string; nama: string; url: string; topik: string };
type Paragraf = { indeks: number; teks: string };
type Peringkat = {
  indeks: number;
  skor: number;
  alasan: string;
  teks: string;
  sumber: "llm" | "heuristik";
};
type Pilihan = {
  kartu: number;
  caption: number;
  peringkat: Peringkat[];
  hashtag: string[];
  mesin: string;
  catatan: string;
};

type Konfig = {
  feeds: Feed[];
  ada_browser: boolean;
  browser: string;
  gaya: string[];
  rasio: string[];
  rata: string[];
};

const KOSONG: Artikel = {
  judul: "", ringkas: "", url: "", gambar: "",
  sumber: "", domain: "", tanggal: "", terbit: "",
};

const LABEL_GAYA: Record<string, string> = {
  gelap: "Gelap", terang: "Terang", kutipan: "Kutipan (tanpa foto)",
};

const LABEL_RATA: Record<string, string> = {
  kiri: "Kiri", tengah: "Tengah", kanan: "Kanan", penuh: "Kiri-kanan (justify)",
};

export default function Berita() {
  const [konfig, setKonfig] = useState<Konfig | null>(null);
  const [pintu, setPintu] = useState<"link" | "jelajah">("link");

  // Pintu 1: tempel link.
  const [link, setLink] = useState("");
  const [ambilSibuk, setAmbilSibuk] = useState(false);

  // Pintu 2: jelajah RSS.
  const [feed, setFeed] = useState("antara");
  const [daftar, setDaftar] = useState<Artikel[]>([]);
  const [daftarSibuk, setDaftarSibuk] = useState(false);

  // Bahan kartu.
  const [art, setArt] = useState<Artikel>(KOSONG);
  const [gaya, setGaya] = useState("gelap");
  const [rasio, setRasio] = useState("9:16");
  const [rata, setRata] = useState("kiri");

  // Analisis LLM.
  const [mesin, setMesin] = useState("ollama");
  const [modelOllama, setModelOllama] = useState("qwen2.5");
  const [modelClaude, setModelClaude] = useState("claude-haiku-4-5");
  const [analisisSibuk, setAnalisisSibuk] = useState(false);
  const [paragraf, setParagraf] = useState<Paragraf[]>([]);
  const [pilihan, setPilihan] = useState<Pilihan | null>(null);
  const [idxKartu, setIdxKartu] = useState<number | null>(null);
  const [idxCaption, setIdxCaption] = useState<number | null>(null);
  const [caption, setCaption] = useState("");
  const [hashtag, setHashtag] = useState("");

  // Bingkai foto: geser & zoom.
  const [fotoX, setFotoX] = useState(0);
  const [fotoY, setFotoY] = useState(0);
  const [zoom, setZoom] = useState(1);
  const seret = useRef<{ x: number; y: number; ax: number; ay: number } | null>(null);
  const kotakRef = useRef<HTMLDivElement>(null);

  const [hasil, setHasil] = useState<{ file: string; zip: string } | null>(null);
  const [buatSibuk, setBuatSibuk] = useState(false);
  const [galat, setGalat] = useState("");

  useEffect(() => {
    fetch(`${ENGINE}/api/news/feeds`)
      .then((r) => r.json())
      .then(setKonfig)
      .catch(() => setGalat("Engine tidak merespons — jalankan ./bin/clipper serve"));
  }, []);

  // Ganti artikel = buang seluruh hasil analisis artikel sebelumnya, supaya
  // paragraf berita lama tidak tertinggal di kartu berita baru.
  const pakaiArtikel = useCallback((a: Artikel) => {
    setArt(a);
    setParagraf([]);
    setPilihan(null);
    setIdxKartu(null);
    setIdxCaption(null);
    setCaption("");
    setHashtag("");
    setHasil(null);
    setFotoX(0);
    setFotoY(0);
    setZoom(1);
  }, []);

  const muatDaftar = useCallback(async (id: string) => {
    setDaftarSibuk(true);
    setGalat("");
    try {
      const r = await fetch(`${ENGINE}/api/news/list?feed=${encodeURIComponent(id)}&maks=24`);
      const d = await r.json();
      if (!r.ok) throw new Error(d.error || "gagal memuat feed");
      setDaftar(d);
    } catch (e: any) {
      setDaftar([]);
      setGalat(e.message);
    } finally {
      setDaftarSibuk(false);
    }
  }, []);

  useEffect(() => {
    if (pintu === "jelajah") muatDaftar(feed);
  }, [pintu, feed, muatDaftar]);

  const ambilLink = useCallback(async () => {
    if (!link.trim()) return;
    setAmbilSibuk(true);
    setGalat("");
    try {
      const r = await fetch(`${ENGINE}/api/news/article`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: link.trim() }),
      });
      const d = await r.json();
      if (!r.ok) throw new Error(d.error || "gagal membaca artikel");
      pakaiArtikel(d);
    } catch (e: any) {
      setGalat(e.message);
    } finally {
      setAmbilSibuk(false);
    }
  }, [link, pakaiArtikel]);

  const analisis = useCallback(async () => {
    if (!art.url) return;
    setAnalisisSibuk(true);
    setGalat("");
    try {
      const r = await fetch(`${ENGINE}/api/news/analisis`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          url: art.url,
          provider: mesin,
          llm_model: modelClaude,
          ollama_model: modelOllama,
        }),
      });
      const d = await r.json();
      if (!r.ok) throw new Error(d.error || "gagal menganalisis artikel");
      const par: Paragraf[] = d.paragraf;
      const pil: Pilihan = d.pilihan;
      setParagraf(par);
      setPilihan(pil);
      terapkanKartu(par, pil.kartu);
      terapkanCaption(par, pil.caption);
      setHashtag(pil.hashtag.join(" "));
      setHasil(null);
    } catch (e: any) {
      setGalat(e.message);
    } finally {
      setAnalisisSibuk(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [art.url, mesin, modelClaude, modelOllama]);

  // Isi kartu & caption SELALU diambil dari teks paragraf artikel — tidak
  // pernah dari karangan model. Itu sebabnya yang disimpan hanya nomornya.
  function terapkanKartu(par: Paragraf[], i: number) {
    const p = par.find((x) => x.indeks === i);
    if (!p) return;
    setIdxKartu(i);
    setArt((a) => ({ ...a, ringkas: p.teks }));
  }
  function terapkanCaption(par: Paragraf[], i: number) {
    const p = par.find((x) => x.indeks === i);
    if (!p) return;
    setIdxCaption(i);
    setCaption(p.teks);
  }

  const buatKartu = useCallback(async () => {
    setBuatSibuk(true);
    setGalat("");
    try {
      const r = await fetch(`${ENGINE}/api/card`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          artikel: art,
          gaya,
          rasio,
          rata,
          caption,
          hashtag: hashtag.split(/\s+/).filter(Boolean),
          foto: { geser_x: fotoX, geser_y: fotoY, zoom },
        }),
      });
      const d = await r.json();
      if (!r.ok) throw new Error(d.error || "gagal membuat kartu");
      setHasil({ file: `${ENGINE}${d.file}?v=${d.id}`, zip: `${ENGINE}${d.zip}` });
    } catch (e: any) {
      setGalat(e.message);
    } finally {
      setBuatSibuk(false);
    }
  }, [art, gaya, rasio, rata, caption, hashtag, fotoX, fotoY, zoom]);

  // Seret di pratinjau → geser foto. Perpindahan piksel layar dikembalikan ke
  // ruang koordinat kartu memakai skala kotak, supaya nilai yang dikirim ke
  // engine tidak bergantung pada seberapa besar pratinjaunya ditampilkan.
  const skalaKotak = useCallback(() => {
    const el = kotakRef.current;
    return el ? el.clientWidth / KARTU_W : 1;
  }, []);

  // Batas geser memakai rumus yang sama dengan engine (batasGeser di card.go):
  // pada zoom Z foto jadi Z kali bingkai, jadi sisa ruangnya (Z-1)/2 tiap sisi.
  // Menyeret lebih jauh hanya akan memunculkan celah kosong di tepi kartu.
  const tinggiBingkai = ((TINGGI_KARTU[rasio] ?? 1920) * (PERSEN_FOTO[rasio] ?? 52)) / 100;
  const batasX = Math.floor((KARTU_W * (zoom - 1)) / 2);
  const batasY = Math.floor((tinggiBingkai * (zoom - 1)) / 2);
  const jepit = (v: number, b: number) => Math.max(-b, Math.min(b, v));

  const mulaiSeret = useCallback((e: React.PointerEvent) => {
    e.preventDefault();
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
    seret.current = { x: e.clientX, y: e.clientY, ax: fotoX, ay: fotoY };
  }, [fotoX, fotoY]);

  const gerakSeret = useCallback((e: React.PointerEvent) => {
    const s = seret.current;
    if (!s) return;
    const k = skalaKotak();
    setFotoX(jepit(Math.round(s.ax + (e.clientX - s.x) / k), batasX));
    setFotoY(jepit(Math.round(s.ay + (e.clientY - s.y) / k), batasY));
  }, [skalaKotak, batasX, batasY]);

  const selesaiSeret = useCallback(() => { seret.current = null; }, []);

  const ubah = (k: keyof Artikel) =>
    (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
      setArt((a) => ({ ...a, [k]: e.target.value }));

  const siap = art.judul.trim().length > 0;

  return (
    <div className="wrap">
      <h1>Kartu berita</h1>
      <p className="sub">
        Ubah artikel jadi gambar siap posting. Isi kartu dan caption diambil{" "}
        <b>apa adanya dari artikel</b> — AI hanya memilih bagian mana yang paling menarik,
        tidak menulis ulang.
      </p>

      {konfig && !konfig.ada_browser && (
        <div className="warnbox">
          Browser tidak ditemukan. Kartu dirender memakai Chrome/Chromium — pasang
          salah satunya, atau set <code>CLIPPER_CHROME</code> ke path chrome.exe.
        </div>
      )}
      {galat && <div className="warnbox err">{galat}</div>}

      {/* --- Pintu masuk --- */}
      <div className="panel">
        <div className="pintu">
          <button className={"ghost" + (pintu === "link" ? " aktif" : "")} onClick={() => setPintu("link")}>
            Tempel link
          </button>
          <button className={"ghost" + (pintu === "jelajah" ? " aktif" : "")} onClick={() => setPintu("jelajah")}>
            Jelajah berita
          </button>
        </div>

        {pintu === "link" ? (
          <div className="row" style={{ alignItems: "flex-end" }}>
            <div className="field" style={{ flex: 3 }}>
              <label>Tautan artikel</label>
              <input
                value={link}
                onChange={(e) => setLink(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && ambilLink()}
                placeholder="https://www.antaranews.com/berita/..."
              />
            </div>
            <div className="field" style={{ flex: "none", minWidth: 0 }}>
              <button onClick={ambilLink} disabled={ambilSibuk || !link.trim()}>
                {ambilSibuk ? "Membaca…" : "Ambil"}
              </button>
            </div>
          </div>
        ) : (
          <>
            <div className="field">
              <label>Sumber berita</label>
              <select value={feed} onChange={(e) => setFeed(e.target.value)}>
                {konfig?.feeds.map((f) => (
                  <option key={f.id} value={f.id}>{f.nama} — {f.topik}</option>
                ))}
              </select>
            </div>
            {daftarSibuk ? (
              <p className="stage">Memuat berita…</p>
            ) : (
              <div className="berita-list">
                {daftar.map((a) => (
                  <button
                    key={a.url}
                    className={"berita-item" + (art.url === a.url ? " aktif" : "")}
                    onClick={() => pakaiArtikel(a)}
                  >
                    {a.gambar ? <img src={a.gambar} alt="" loading="lazy" /> : <div className="berita-nogambar" />}
                    <div className="berita-teks">
                      <div className="berita-judul">{a.judul}</div>
                      <div className="meta">{a.tanggal || a.domain}</div>
                    </div>
                  </button>
                ))}
              </div>
            )}
          </>
        )}
      </div>

      {/* --- Analisis AI --- */}
      {siap && (
        <div className="panel">
          <label className="blok">Pilih bagian paling menarik (AI memilih, bukan menulis)</label>
          <div className="row" style={{ alignItems: "flex-end" }}>
            <div className="field">
              <label>Mesin</label>
              <select value={mesin} onChange={(e) => setMesin(e.target.value)}>
                <option value="ollama">Ollama (lokal, gratis)</option>
                <option value="claude">Claude (API)</option>
              </select>
            </div>
            <div className="field">
              <label>Model</label>
              {mesin === "ollama" ? (
                <input value={modelOllama} onChange={(e) => setModelOllama(e.target.value)} />
              ) : (
                <input value={modelClaude} onChange={(e) => setModelClaude(e.target.value)} />
              )}
            </div>
            <div className="field" style={{ flex: "none", minWidth: 0 }}>
              <button onClick={analisis} disabled={analisisSibuk}>
                {analisisSibuk ? "Menganalisis…" : "Analisis artikel"}
              </button>
            </div>
          </div>
          {analisisSibuk && mesin === "ollama" && (
            <p className="stage">Model lokal membaca artikel — biasanya 20–60 detik.</p>
          )}

          {pilihan && (
            <>
              <p className="stage">
                {pilihan.peringkat.length} paragraf, diurutkan dari yang paling ber-hook. Klik
                untuk memakainya sebagai isi kartu atau caption.
              </p>
              {pilihan.catatan && <div className="warnbox">{pilihan.catatan}</div>}
              <div className="par-list">
                {pilihan.peringkat.map((p) => (
                  <div
                    key={p.indeks}
                    className={
                      "par" +
                      (idxKartu === p.indeks ? " pilih-kartu" : "") +
                      (idxCaption === p.indeks ? " pilih-caption" : "")
                    }
                  >
                    <div
                      className={"par-skor" + (p.sumber === "heuristik" ? " auto" : "")}
                      title={p.sumber === "heuristik" ? "dinilai otomatis oleh engine" : `dinilai ${pilihan.mesin}`}
                    >
                      {p.skor.toFixed(1)}
                      {p.sumber === "heuristik" && <span className="par-auto">auto</span>}
                    </div>
                    <div className="par-isi">
                      <div className="par-teks">{p.teks}</div>
                      {p.alasan && <div className="par-alasan">{p.alasan}</div>}
                      <div className="par-aksi">
                        <button className="tiny ghost" onClick={() => terapkanKartu(paragraf, p.indeks)}>
                          {idxKartu === p.indeks ? "✓ di kartu" : "Pakai di kartu"}
                        </button>
                        <button className="tiny ghost" onClick={() => terapkanCaption(paragraf, p.indeks)}>
                          {idxCaption === p.indeks ? "✓ jadi caption" : "Jadikan caption"}
                        </button>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      )}

      {/* --- Bahan kartu --- */}
      {siap && (
        <div className="panel">
          <label className="blok">Isi kartu (boleh disunting)</label>
          <div className="field">
            <label>Judul</label>
            <textarea rows={2} value={art.judul} onChange={ubah("judul")} />
          </div>
          <div className="field">
            <label>Teks di kartu {idxKartu !== null && <em>(paragraf #{idxKartu} dari artikel)</em>}</label>
            <textarea rows={4} value={art.ringkas} onChange={ubah("ringkas")} />
          </div>
          <div className="row">
            <div className="field">
              <label>Sumber (badge)</label>
              <input value={art.sumber} onChange={ubah("sumber")} />
            </div>
            <div className="field">
              <label>Tanggal</label>
              <input value={art.tanggal} onChange={ubah("tanggal")} />
            </div>
          </div>
          <div className="field">
            <label>URL gambar {gaya === "kutipan" && <em>(tidak dipakai di gaya kutipan)</em>}</label>
            <input value={art.gambar} onChange={ubah("gambar")} />
          </div>

          <div className="field">
            <label>Caption {idxCaption !== null && <em>(paragraf #{idxCaption} dari artikel)</em>}</label>
            <textarea rows={4} value={caption} onChange={(e) => setCaption(e.target.value)} />
          </div>
          <div className="field">
            <label>Hashtag</label>
            <input value={hashtag} onChange={(e) => setHashtag(e.target.value)} placeholder="#Contoh #Tagar" />
          </div>

          {gaya !== "kutipan" && art.gambar.startsWith("http") && (
            <div className="field">
              <label>
                Bingkai foto <em>— seret untuk menggeser, gulir untuk zoom</em>
              </label>
              <div className="foto-atur">
                <div
                  ref={kotakRef}
                  className="foto-kotak"
                  style={{ aspectRatio: `${KARTU_W} / ${(TINGGI_KARTU[rasio] ?? 1920) * (PERSEN_FOTO[rasio] ?? 52) / 100}` }}
                  onPointerDown={mulaiSeret}
                  onPointerMove={gerakSeret}
                  onPointerUp={selesaiSeret}
                  onPointerCancel={selesaiSeret}
                  onWheel={(e) => {
                    const z = Math.min(4, Math.max(1, +(zoom * (e.deltaY < 0 ? 1.06 : 1 / 1.06)).toFixed(3)));
                    // Geseran ikut dijepit: mengecilkan zoom mempersempit ruang
                    // gerak, jadi posisi lama bisa jadi di luar batas baru.
                    const bx = Math.floor((KARTU_W * (z - 1)) / 2);
                    const by = Math.floor((tinggiBingkai * (z - 1)) / 2);
                    setZoom(z);
                    setFotoX((v) => jepit(v, bx));
                    setFotoY((v) => jepit(v, by));
                  }}
                >
                  {/* CSS-nya sengaja disamakan persis dengan template kartu di
                      engine/internal/card — kalau salah satu diubah, ubah keduanya. */}
                  <img
                    src={art.gambar}
                    alt=""
                    draggable={false}
                    style={{
                      transform:
                        `translate(-50%,-50%) translate(${fotoX / KARTU_W * 100}cqw, ${fotoY / KARTU_W * 100}cqw) scale(${zoom})`,
                    }}
                  />
                  <div className="foto-kabut" />
                </div>
                <div className="foto-knop">
                  <label>Zoom {zoom.toFixed(2)}×</label>
                  <input
                    type="range" min={1} max={4} step={0.01}
                    value={zoom}
                    onChange={(e) => {
                      const z = parseFloat(e.target.value);
                      setZoom(z);
                      setFotoX((v) => jepit(v, Math.floor((KARTU_W * (z - 1)) / 2)));
                      setFotoY((v) => jepit(v, Math.floor((tinggiBingkai * (z - 1)) / 2)));
                    }}
                  />
                  {zoom === 1 && <p className="meta">Perbesar dulu agar foto bisa digeser.</p>}
                  <p className="meta">geser: {fotoX}, {fotoY} px</p>
                  <button
                    className="ghost tiny"
                    onClick={() => { setFotoX(0); setFotoY(0); setZoom(1); }}
                  >
                    Kembalikan
                  </button>
                </div>
              </div>
            </div>
          )}

          <div className="row">
            <div className="field">
              <label>Gaya</label>
              <select value={gaya} onChange={(e) => setGaya(e.target.value)}>
                {konfig?.gaya.map((g) => <option key={g} value={g}>{LABEL_GAYA[g] || g}</option>)}
              </select>
            </div>
            <div className="field">
              <label>Rasio</label>
              <select value={rasio} onChange={(e) => setRasio(e.target.value)}>
                {konfig?.rasio.map((r) => (
                  <option key={r} value={r}>
                    {r} {r === "9:16" ? "(Story/Reels)" : r === "4:5" ? "(feed IG)" : "(persegi)"}
                  </option>
                ))}
              </select>
            </div>
            <div className="field">
              <label>Rata teks</label>
              <select value={rata} onChange={(e) => setRata(e.target.value)}>
                {(konfig?.rata ?? ["kiri", "tengah", "kanan", "penuh"]).map((r) => (
                  <option key={r} value={r}>{LABEL_RATA[r] || r}</option>
                ))}
              </select>
            </div>
          </div>
          {rata === "penuh" && (
            <p className="stage">
              Rata kiri-kanan merenggangkan jarak antar kata. Pada judul besar yang cuma
              beberapa kata per baris, celahnya bisa terlihat lebar.
            </p>
          )}

          <button onClick={buatKartu} disabled={buatSibuk || !konfig?.ada_browser}>
            {buatSibuk ? "Merender…" : "Buat kartu"}
          </button>
        </div>
      )}

      {/* --- Hasil --- */}
      {hasil && (
        <div className="panel">
          <label className="blok">Hasil</label>
          <div className="kartu-hasil">
            <img src={hasil.file} alt="kartu berita" />
            <div>
              <p><a className="dl" href={hasil.zip}>⬇ Unduh ZIP (gambar + caption + sumber)</a></p>
              <p><a className="dl" href={hasil.file} download="kartu.png">⬇ Unduh gambar saja</a></p>
              {art.url && (
                <p className="meta" style={{ marginTop: 10 }}>
                  Sumber:{" "}
                  <a className="dl" href={art.url} target="_blank" rel="noreferrer">{art.domain}</a>
                  <br />
                  Cantumkan sumber saat memposting.
                </p>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

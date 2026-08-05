"use client";

// Ikon: lucide-react (ISC) — alasannya di gui/app/page.tsx.
import { Link2, Eye, Download } from "lucide-react";

import { useCallback, useEffect, useRef, useState } from "react";
import { useI18n } from "../i18n";
import { eng, engineURL } from "../engine";
import { useKeep, useRestore } from "../persist";
import Stepper from "../stepper";
import Popover from "../popover";
import Section from "../section";


// Harus sama dengan card.FontSteps di engine: banyaknya langkah ukuran huruf ke
// tiap arah dari ukuran standar.
const FONT_STEPS = 10;
// Harus sama dengan card.HeaderMax di engine: sejauh mana isi boleh digeser turun.
const HEADER_MAX = 400;
// Harus sama dengan card.CardTopMax di engine: setinggi apa pita kosong di atas
// kartu boleh dibuat.
const CARD_TOP_MAX = 400;
type Article = {
  title: string;
  summary: string;
  url: string;
  image: string;
  source: string;
  domain: string;
  date: string;
  published: string;
};

type Feed = { id: string; name: string; url: string; topic: string };
type Paragraph = { index: number; text: string };
type Ranking = {
  index: number;
  score: number;
  reason: string;
  text: string;
  source: "llm" | "heuristic";
};
type Selection = {
  card: number;
  caption: number;
  rankings: Ranking[];
  hashtags: string[];
  engine: string;
  note: string;
};

type Config = {
  feeds: Feed[];
  has_browser: boolean;
  browser: string;
  styles: string[];
  ratios: string[];
  aligns: string[];
  card_colours: string[][] | string[];
};

// colourRows menormalkan daftar warna dari engine.
//
// Bentuknya pernah berubah dari daftar datar jadi daftar per keluarga, dan GUI
// dev bisa memuat ulang lebih dulu daripada engine di-restart. Tanpa penormalan
// ini seluruh halaman mati dengan "row.map is not a function" — satu perbedaan
// bentuk data seharusnya tidak menjatuhkan tab yang sedang dipakai.
function colourRows(v: unknown): string[][] {
  if (!Array.isArray(v)) return [];
  if (v.every((x) => typeof x === "string")) return [v as string[]];
  return v.filter((x): x is string[] => Array.isArray(x));
}

const EMPTY_ARTICLE: Article = {
  title: "", summary: "", url: "", image: "",
  source: "", domain: "", date: "", published: "",
};

export default function News() {
  const { lang, t } = useI18n();

  const [config, setConfig] = useState<Config | null>(null);
  const [entry, setEntry] = useState<"link" | "browse">("link");
  // Daftar berita hidup di dalam <Popover>; halaman ini cuma perlu tahu kapan
  // isinya harus ditarik.
  const [browseOpen, setBrowseOpen] = useState(false);

  // Pintu 1: tempel link.
  const [link, setLink] = useState("");
  const [fetching, setFetching] = useState(false);

  // Pintu 2: jelajah RSS.
  const [feed, setFeed] = useState("antara");
  // Kata kunci yang SUDAH dikirim. Dipisah dari isi kotak ketik supaya daftar
  // tidak memuat ulang di setiap huruf yang diketik.
  const [query, setQuery] = useState("");
  const [typed, setTyped] = useState("");
  const [items, setItems] = useState<Article[]>([]);
  const [listBusy, setListBusy] = useState(false);

  // Bahan kartu.
  const [article, setArticle] = useState<Article>(EMPTY_ARTICLE);
  const [style, setStyle] = useState("dark");
  const [ratio, setRatio] = useState("9:16");
  const [align, setAlign] = useState("left");

  // Analisis LLM.
  const [engine, setEngine] = useState("ollama");
  const [ollamaModel, setOllamaModel] = useState("qwen2.5");
  // Model yang benar-benar terpasang. Sampai kini pengguna harus mengetik
  // namanya dari ingatan — dan satu salah ketik berujung galat dari Ollama
  // yang tidak menyebutkan bahwa masalahnya cuma nama.
  const [ollamaInstalled, setOllamaInstalled] = useState<string[]>([]);
  const [claudeModel, setClaudeModel] = useState("claude-haiku-4-5");
  const [analyzeBusy, setAnalyzeBusy] = useState(false);
  const [paragraphs, setParagraphs] = useState<Paragraph[]>([]);
  const [selection, setSelection] = useState<Selection | null>(null);
  const [cardIndex, setCardIndex] = useState<number | null>(null);
  const [captionIndex, setCaptionIndex] = useState<number | null>(null);
  const [caption, setCaption] = useState("");
  const [hashtags, setHashtags] = useState("");

  // Bingkai foto: hanya cara pas & zoom.
  //
  // Geseran manual (offset_x/offset_y) DIBUANG 6 Agustus 2026 bersama kotak
  // seretnya: ia pratinjau KEDUA di halaman yang sudah punya pratinjau kartu
  // sungguhan, dan dua pratinjau berdampingan yang tidak sama persis lebih
  // membingungkan daripada menolong. Engine tetap menerima medannya — kalau
  // suatu saat geseran diperlukan lagi, kembalikan bersama kendalinya, bukan
  // sebagai state yang selalu nol.
  const [zoom, setZoom] = useState(1);
  // Zoom dibaca RELATIF terhadap titik awal modenya — sumbu yang sama dengan tab
  // klip (notes/15-sumbu-zoom.md). cover: 1 = memenuhi bingkai. whole: 1 =
  // seluruh gambar asli masuk, berapa pun rasionya.
  const [photoFit, setPhotoFit] = useState("cover");
  const [photoFill, setPhotoFill] = useState("blur");

  // Ukuran huruf dalam LANGKAH dari ukuran standar, bukan piksel mutlak: 0
  // berarti template standar apa adanya, dan selalu bisa dikembalikan ke sana.
  const [titleStep, setTitleStep] = useState(0);
  const [paragraphStep, setParagraphStep] = useState(0);
  // Menggeser SELURUH isi turun sebagai satu kesatuan, dalam piksel ruang kartu.
  const [header, setHeader] = useState(0);
  // Menurunkan SELURUH kartu: area foto ikut turun, pita kosong muncul di atas.
  const [cardTop, setCardTop] = useState(0);

  // Warna: dari foto (bawaan) atau dari warna yang kamu tentukan sendiri.
  const [colorSource, setColorSource] = useState("photo");
  const [customColor, setCustomColor] = useState("");
  const [boxMode, setBoxMode] = useState("auto"); // auto | none | custom
  const [boxColor, setBoxColor] = useState("#EFEBE1");

  // Tautan yang baru saja disalin — dipakai untuk mengubah warna tombolnya
  // sebentar, sebagai tanda bahwa klik tadi benar-benar berhasil.
  const [copied, setCopied] = useState("");
  // Tautan yang sedang diresolusi — menandai tombolnya supaya jeda 2-3 detik
  // itu tidak terasa seperti klik yang tidak berfungsi.
  const [copyBusy, setCopyBusy] = useState("");

  const [result, setResult] = useState<{ file: string; zip: string; preview: boolean } | null>(null);
  const [buildBusy, setBuildBusy] = useState(false);
  const [previewBusy, setPreviewBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    fetch(eng(`/api/news/feeds`))
      .then((r) => r.json())
      .then((d: Config) => {
        setConfig(d);
        // Warna awal diambil dari daftar engine, bukan ditulis di sini — supaya
        // tidak ada warna yang muncul di kotak tapi tidak ada di contekannya.
        setCustomColor((v) => v || colourRows(d.card_colours)[0]?.[0] || "");
      })
      .catch(() => setError(t("engineUnreachable", { url: engineURL() })));
  }, [t]);

  // Ganti artikel = buang seluruh hasil analisis artikel sebelumnya, supaya
  // paragraf berita lama tidak tertinggal di kartu berita baru.
  const useArticle = useCallback((a: Article) => {
    setArticle(a);
    setParagraphs([]);
    setSelection(null);
    setCardIndex(null);
    setCaptionIndex(null);
    setCaption("");
    setHashtags("");
    setResult(null);
    setZoom(1);
  }, []);

  const loadList = useCallback(async (feedId: string, search: string) => {
    setListBusy(true);
    setError("");
    try {
      const param = search
        ? `q=${encodeURIComponent(search)}`
        : `feed=${encodeURIComponent(feedId)}`;
      const res = await fetch(eng(`/api/news/list?${param}&max=24&lang=${lang}`));
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || t("errLoadNews"));
      setItems(data);
    } catch (e: any) {
      setItems([]);
      setError(e.message);
    } finally {
      setListBusy(false);
    }
  }, [lang, t]);

  useEffect(() => {
    if (browseOpen) loadList(feed, query);
  }, [browseOpen, feed, query, loadList]);

  // Mencari cukup menyetel kuncinya; daftarnya sendiri ditarik oleh efek di
  // atas begitu popupnya terbuka.
  const runSearch = useCallback(() => {
    setQuery(typed.trim());
    setBrowseOpen(true);
  }, [typed]);

  // Menyalin tautan artikel supaya bisa dicek silang di tab lain.
  //
  // navigator.clipboard hanya tersedia di konteks aman (https atau localhost).
  // Saat GUI dibuka lewat alamat IP mesin, ia tidak ada sama sekali — karena itu
  // ada jalur cadangan memakai textarea sementara.
  const copyLink = useCallback(async (url: string) => {
    if (!url || copyBusy) return;
    try {
      // Hasil pencarian membawa pengalih Google, bukan alamat medianya. Yang
      // berguna untuk dicek silang adalah alamat aslinya, jadi diresolusi dulu.
      // Perlu satu peluncuran browser (~2-3 detik), tapi hasilnya di-cache.
      if (url.includes("news.google.com/")) {
        setCopyBusy(url);
        const res = await fetch(eng(`/api/news/resolve`), {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ url }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || t("errOpenLink"));
        // Simpan supaya kartu ini tidak perlu diresolusi lagi nanti.
        setItems((list) => list.map((a) => (a.url === url ? { ...a, url: data.url } : a)));
        url = data.url;
      }
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(url);
      } else {
        const area = document.createElement("textarea");
        area.value = url;
        area.style.position = "fixed";
        area.style.opacity = "0";
        document.body.appendChild(area);
        area.select();
        document.execCommand("copy");
        document.body.removeChild(area);
      }
      const done = url;
      setCopied(done);
      setTimeout(() => setCopied((c) => (c === done ? "" : c)), 1600);
    } catch (e: any) {
      setError(e?.message || t("errCopy"));
    } finally {
      setCopyBusy("");
    }
  }, [copyBusy, t]);

  const fetchLink = useCallback(async () => {
    if (!link.trim()) return;
    setFetching(true);
    setError("");
    try {
      const res = await fetch(eng(`/api/news/article`), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: link.trim(), lang }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || t("errReadArticle"));
      useArticle(data);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setFetching(false);
    }
  }, [link, useArticle, lang, t]);

  // Isi kartu & caption SELALU diambil dari teks paragraf artikel — tidak
  // pernah dari karangan model. Itu sebabnya yang disimpan hanya nomornya.
  const applyToCard = useCallback((list: Paragraph[], i: number) => {
    const p = list.find((x) => x.index === i);
    if (!p) return;
    setCardIndex(i);
    setArticle((a) => ({ ...a, summary: p.text }));
  }, []);

  const applyToCaption = useCallback((list: Paragraph[], i: number) => {
    const p = list.find((x) => x.index === i);
    if (!p) return;
    setCaptionIndex(i);
    setCaption(p.text);
  }, []);

  const analyze = useCallback(async () => {
    if (!article.url) return;
    setAnalyzeBusy(true);
    setError("");
    try {
      const res = await fetch(eng(`/api/news/analyze`), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          url: article.url,
          provider: engine,
          llm_model: claudeModel,
          ollama_model: ollamaModel,
          lang,
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || t("errAnalyze"));
      const list: Paragraph[] = data.paragraphs;
      const picked: Selection = data.selection;

      // Balasan analisis memuat artikel hasil resolusi — di sinilah alamat asli
      // dan og:image pertama kali tersedia. Hasil pencarian Google News tidak
      // membawa keduanya, jadi tanpa penggabungan ini kartunya terbit tanpa foto
      // dan tautannya tetap menunjuk pengalih Google.
      //
      // Hanya field yang memang lebih baik yang ditimpa. Judul, ringkasan, dan
      // badge dibiarkan apa adanya karena bisa jadi sudah disunting pengguna.
      if (data.article) {
        const fresh: Article = data.article;
        setArticle((a) => ({
          ...a,
          url: fresh.url && !fresh.url.includes("news.google.com/") ? fresh.url : a.url,
          image: a.image || fresh.image || "",
          date: a.date || fresh.date || "",
          published: a.published || fresh.published || "",
          source: a.source || fresh.source || "",
          domain: a.domain || fresh.domain || "",
        }));
        // Daftar hasil ikut diperbarui supaya artikel ini tidak perlu
        // diresolusi lagi kalau nanti dipilih ulang.
        if (fresh.url && !fresh.url.includes("news.google.com/")) {
          setItems((list) => list.map((x) => (x.url === article.url ? { ...x, url: fresh.url } : x)));
        }
      }

      setParagraphs(list);
      setSelection(picked);
      applyToCard(list, picked.card);
      applyToCaption(list, picked.caption);
      setHashtags(picked.hashtags.join(" "));
      setResult(null);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setAnalyzeBusy(false);
    }
  }, [article.url, engine, claudeModel, ollamaModel, lang, applyToCard, applyToCaption, t]);

  // buildCard dipakai untuk pratinjau maupun simpan — bedanya cuma satu flag.
  //
  // Pratinjau menimpa satu folder tetap di engine, jadi menyetel kartu berpuluh
  // kali tidak meninggalkan berpuluh folder. Berkas pendamping (caption &
  // keterangan sumber) baru ditulis saat benar-benar disimpan.
  // Daftar model diambil sekali saat halaman dibuka, lalu setiap kali jendela
  // kembali aktif — pengguna sering memasang model di terminal sebelah.
  useEffect(() => {
    if (engine !== "ollama") return;
    const load = () => {
      fetch(eng(`/api/ollama/status`))
        .then((r) => r.json())
        .then((d: { running: boolean; installed?: { name: string }[]; models?: string[] }) => {
          const names = d.installed?.map((m) => m.name) ?? d.models ?? [];
          setOllamaInstalled(names);
          // Kalau yang terpilih tidak ada di daftar, ambil yang pertama —
          // lebih baik langsung bisa dipakai daripada gagal saat ditekan.
          setOllamaModel((cur) =>
            names.length > 0 && !names.some((n) => n === cur || n.split(":")[0] === cur)
              ? names[0]
              : cur,
          );
        })
        .catch(() => setOllamaInstalled([]));
    };
    load();
    window.addEventListener("focus", load);
    return () => window.removeEventListener("focus", load);
  }, [engine]);

  // Sidik jari dari SEMUA yang memengaruhi gambar kartu. Dipakai untuk memicu
  // pratinjau ulang: mendaftar satu per satu di dependency effect berarti setiap
  // setelan baru harus diingat untuk ditambahkan ke sana — dan yang terlupa
  // berakhir sebagai "kenapa gambarku tidak berubah".
  const cardFingerprint = JSON.stringify([
    article.title, article.url, article.image, article.summary, article.source, article.date,
    style, ratio, align, lang, caption, hashtags,
    zoom, photoFit, photoFill,
    titleStep, paragraphStep, header, cardTop,
    colorSource, customColor, boxMode, boxColor,
  ]);

  const buildCard = useCallback(async (preview: boolean) => {
    if (preview) setPreviewBusy(true); else setBuildBusy(true);
    setError("");
    try {
      const res = await fetch(eng(`/api/card`), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          article,
          style,
          ratio,
          align,
          lang,
          preview,
          caption,
          hashtags: hashtags.split(/\s+/).filter(Boolean),
          photo: { offset_x: 0, offset_y: 0, zoom, fit: photoFit, fill: photoFill },
          fonts: { title: titleStep, paragraph: paragraphStep },
          header,
          card_top: cardTop,
          colors: {
            source: colorSource,
            custom: customColor,
            box: boxMode === "custom" ? boxColor : boxMode,
          },
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || t("errBuildCard"));
      // Pratinjau selalu memakai id yang sama, jadi alamatnya perlu penanda
      // waktu — tanpa itu browser menampilkan gambar lama dari cache dan
      // penyetelanmu terlihat tidak berpengaruh.
      setResult({
        file: eng(`${data.file}?v=${Date.now()}`),
        zip: eng(`${data.zip}`),
        preview,
      });
    } catch (e: any) {
      setError(e.message);
    } finally {
      setPreviewBusy(false);
      setBuildBusy(false);
    }
  }, [article, style, ratio, align, lang, caption, hashtags, zoom,
      photoFit, photoFill, titleStep, paragraphStep, header, cardTop,
      colorSource, customColor, boxMode, boxColor, t]);

  // Isian disimpan & dipulihkan sendiri: di aplikasi desktop halaman bisa
  // termuat ulang tanpa diminta (WebView2 membuang proses penampilnya saat
  // jendela ditinggal atau memori menipis), dan mengetik ulang semuanya karena
  // berpindah jendela adalah hukuman yang tidak masuk akal.
  //
  // Yang disimpan hanya isian, bukan hasil: kartu jadi tetap ada di folder
  // penyimpanan, dan pratinjau dibuat ulang sendiri.
  useKeep("news", {
    article, caption, hashtags, paragraphs, engine, ollamaModel, claudeModel,
    style, ratio, align, zoom, photoFit, photoFill,
    titleStep, paragraphStep, header, cardTop, colorSource, customColor,
    boxMode, boxColor,
  });
  useRestore<Record<string, unknown>>("news", (v) => {
    const set = <T,>(fn: (x: T) => void, val: unknown) => {
      if (val !== undefined && val !== null) fn(val as T);
    };
    set(setArticle, v.article);
    set(setCaption, v.caption);
    set(setHashtags, v.hashtags);
    set(setParagraphs, v.paragraphs);
    set(setEngine, v.engine);
    set(setOllamaModel, v.ollamaModel);
    set(setClaudeModel, v.claudeModel);
    set(setStyle, v.style);
    set(setRatio, v.ratio);
    set(setAlign, v.align);
    set(setZoom, v.zoom);
    set(setPhotoFit, v.photoFit);
    set(setPhotoFill, v.photoFill);
    set(setTitleStep, v.titleStep);
    set(setParagraphStep, v.paragraphStep);
    set(setHeader, v.header);
    set(setCardTop, v.cardTop);
    set(setColorSource, v.colorSource);
    set(setCustomColor, v.customColor);
    set(setBoxMode, v.boxMode);
    set(setBoxColor, v.boxColor);
  });

  // Pratinjau otomatis: setiap perubahan setelan langsung terlihat, tanpa
  // menekan tombol.
  //
  // Ditunda 700 ms sejak perubahan TERAKHIR, bukan dijalankan tiap perubahan:
  // satu pratinjau berarti satu Chrome headless dirender penuh, sedangkan
  // menggeser penggeser ukuran huruf menghasilkan puluhan perubahan per detik.
  // Penundaan mengubahnya jadi satu render setelah tangan berhenti.
  useEffect(() => {
    if (!article.title || !config?.has_browser) return;
    const id = setTimeout(() => buildCard(true), 700);
    return () => clearTimeout(id);
    // Sengaja hanya bergantung pada sidik jari: buildCard berubah identitasnya
    // tiap render, dan cardFingerprint sudah mewakili seluruh isinya.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cardFingerprint, config?.has_browser]);

  const edit = (key: keyof Article) =>
    (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
      setArticle((a) => ({ ...a, [key]: e.target.value }));

  const styleLabels: Record<string, string> = {
    dark: t("styleDark"), light: t("styleLight"), quote: t("styleQuote"),
  };
  const alignLabels: Record<string, string> = {
    left: t("alignLeft"), center: t("alignCenter"),
    right: t("alignRight"), justify: t("alignJustify"),
  };

  const ready = article.title.trim().length > 0;

  return (
    <div className="screen">
      {/* Kepala HANYA untuk peringatan. Tidak ada judul halaman — nama halaman
          sudah ada di rail kiri dalam keadaan terpilih (CLAUDE.md). */}
      {(error || (config && !config.has_browser)) && (
        <div className="screen-head">
          {config && !config.has_browser && (
            <div className="warnbox">
              {t("browserMissing")} <code>CLIPPER_CHROME</code> {t("browserMissingTail")}
            </div>
          )}
          {error && <div className="warnbox err">{error}</div>}
        </div>
      )}

      {/* Kerangka baku halaman klip, disalin apa adanya (CLAUDE.md):
          KIRI  = pratinjau + apa pun yang mengubah rupanya
          KANAN = masukan, pilihan, dan tombol jalan
          Setelan rupa TIDAK pernah di kolom kanan, dan isian sumber TIDAK
          pernah ditempel ke panel pratinjau. */}
      <div className="screen-body two">
        {/* --- KIRI: pratinjau kartu + rupa kartu --- */}
        <div className="screen-main">
          <div className="panel">
            <div className="sub-layout">
              <div className="sub-preview">
                {/* Satu-satunya pratinjau di halaman ini. Dibuat ulang sendiri
                    700 ms sesudah setelan terakhir disentuh. */}
                <div className="card-view">
                  {result ? (
                    /* eslint-disable-next-line @next/next/no-img-element */
                    <img src={result.file} alt="" />
                  ) : (
                    <div className="preview-empty">
                      <div className="pe-icon" aria-hidden="true" />
                      <div className="pe-title">{t("previewEmpty")}</div>
                    </div>
                  )}
                  {(previewBusy || buildBusy) && <div className="card-view-busy">{t("rendering")}</div>}
                </div>

                {result && !result.preview && (
                  <div className="dl-row">
                    <a className="dl" href={result.zip}><Download className="ico" aria-hidden="true" /> {t("downloadZip")}</a>
                    <a className="dl" href={result.file} download="card.png"><Download className="ico" aria-hidden="true" /> {t("downloadImage")}</a>
                  </div>
                )}
              </div>

              {/* Rupa kartu duduk di SEBELAH pratinjaunya, bukan di kolom kanan:
                  tiap kendali di sini langsung mengubah gambar di sebelah kiri,
                  dan itu bukan fitur artikel. */}
              <div className="sub-settings">
                <div className="group">
                  <div className="group-title">{t("groupDesign")}</div>
                  <div className="grid3">
                    <div className="field">
                      <label>{t("style")}</label>
                      <select value={style} onChange={(e) => setStyle(e.target.value)}>
                        {config?.styles.map((s) => <option key={s} value={s}>{styleLabels[s] || s}</option>)}
                      </select>
                    </div>
                    <div className="field">
                      <label>{t("ratio")}</label>
                      <select value={ratio} onChange={(e) => setRatio(e.target.value)}>
                        {config?.ratios.map((r) => (
                          <option key={r} value={r}>
                            {r} {r === "9:16" ? t("ratioStory") : r === "4:5" ? t("ratioFeed") : t("ratioSquare")}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div className="field">
                      <label>{t("textAlign")}</label>
                      <select value={align} onChange={(e) => setAlign(e.target.value)}>
                        {(config?.aligns ?? ["left", "center", "right", "justify"]).map((a) => (
                          <option key={a} value={a}>{alignLabels[a] || a}</option>
                        ))}
                      </select>
                    </div>
                    {/* Warna kartu: satu tombol yang membuka palet, bukan
                        dropdown + deretan contekan yang tumbuh di bawahnya.
                        Contekan inline menggeser seluruh kisi tiap kali mode
                        warna diganti. */}
                    <div className="field">
                      <label>{t("cardColour")}</label>
                      <Popover width={300} buttonClass="ghost swatch-trigger"
                        label={
                          <>
                            <span className="swatch-dot" style={{
                              background: colorSource === "photo" ? undefined : customColor,
                            }} data-auto={colorSource === "photo" ? "" : undefined} />
                            {colorSource === "photo" ? t("colourFromPhoto") : (customColor || t("colourCustom"))}
                          </>
                        }>
                        {(close) => (
                          <>
                            <div className="popover-head">
                              <select value={colorSource} onChange={(e) => setColorSource(e.target.value)}>
                                <option value="photo">{t("colourFromPhoto")}</option>
                                <option value="custom">{t("colourCustom")}</option>
                              </select>
                            </div>
                            {/* Daftar tertutup, bukan pemilih spektrum: engine hanya
                                memakai RONA warna pilihan — terangnya dikunci palet —
                                jadi pemilih spektrum menjanjikan yang tidak dikerjakan. */}
                            {colorSource === "custom" && colourRows(config?.card_colours).map((row, i) => (
                              <div className="swatches" key={i}>
                                {row.map((c) => (
                                  <button
                                    key={c}
                                    type="button"
                                    className={"swatch" + (c === customColor ? " on" : "")}
                                    style={{ background: c }}
                                    title={c}
                                    aria-label={c}
                                    onClick={() => { setCustomColor(c); close(); }}
                                  />
                                ))}
                              </div>
                            ))}
                          </>
                        )}
                      </Popover>
                    </div>
                    <div className="field">
                      <label>{t("cardBoxBackground")}</label>
                      <select value={boxMode} onChange={(e) => setBoxMode(e.target.value)}>
                        <option value="auto">{t("boxAuto")}</option>
                        <option value="none">{t("boxNone")}</option>
                        <option value="custom">{t("boxCustom")}</option>
                      </select>
                    </div>
                    <div className="field">
                      {boxMode === "custom" && (
                        <>
                          <label>{t("boxCustom")}</label>
                          <div className="path-row">
                            <input type="color" className="colour-dot" value={boxColor}
                              onChange={(e) => setBoxColor(e.target.value.toUpperCase())} />
                            <input value={boxColor} spellCheck={false}
                              onChange={(e) => setBoxColor(e.target.value.toUpperCase())} />
                          </div>
                        </>
                      )}
                    </div>
                  </div>

                </div>

                {style !== "quote" && article.image.startsWith("http") && (
                  <div className="group">
                    <div className="group-title">{t("photoFit")}</div>
                    <div className="grid3">
                      <div className="field">
                        <label>{t("photoFitLabel")}</label>
                        <select value={photoFit} onChange={(e) => { setPhotoFit(e.target.value); setZoom(1); }}>
                          <option value="cover">{t("photoFitCover")}</option>
                          <option value="whole">{t("photoFitWhole")}</option>
                        </select>
                      </div>
                      <div className="field">
                        <label>{t("photoFill")}</label>
                        <select value={photoFill} disabled={photoFit !== "whole"}
                          onChange={(e) => setPhotoFill(e.target.value)}>
                          <option value="blur">{t("photoFillBlur")}</option>
                          <option value="solid">{t("photoFillSolid")}</option>
                        </select>
                      </div>
                      <div className="field">
                        <label>{t("photoZoom")}</label>
                        <Stepper value={Math.round(zoom * 100)} onChange={(v) => setZoom(v / 100)}
                          min={100} max={400} step={5} suffix="%" />
                      </div>
                    </div>
                  </div>
                )}

                {/* Ukuran huruf & jarak dalam LANGKAH dari template standar,
                    bukan piksel mutlak: 0 selalu bisa dikembalikan. */}
                <div className="group">
                  {/* Tombol "kembali ke standar" naik ke baris judul: kalau ia ikut
                      di dalam kisi, empat angka + satu tombol tidak pernah muat
                      satu baris dan kelompoknya melipat jadi dua. */}
                  <div className="group-title with-action">
                    <span>{t("groupType")}</span>
                    <button className="ghost tiny"
                      onClick={() => { setTitleStep(0); setParagraphStep(0); setHeader(0); setCardTop(0); setZoom(1); }}
                      disabled={titleStep === 0 && paragraphStep === 0 && header === 0 && cardTop === 0 && zoom === 1}>
                      {t("fontReset")}
                    </button>
                  </div>
                  <div className="grid4">
                    <div className="field">
                      <label>{t("fontTitle")}</label>
                      <Stepper value={titleStep} onChange={setTitleStep} min={-FONT_STEPS} max={FONT_STEPS} />
                    </div>
                    <div className="field">
                      <label>{t("fontParagraph")}</label>
                      <Stepper value={paragraphStep} onChange={setParagraphStep} min={-FONT_STEPS} max={FONT_STEPS} />
                    </div>
                    <div className="field">
                      <label>{t("headerSpace")}</label>
                      <Stepper value={header} onChange={setHeader} min={0} max={HEADER_MAX} step={10} suffix="px" />
                    </div>
                    <div className="field">
                      <label>{t("cardDown")}</label>
                      <Stepper value={cardTop} onChange={setCardTop} min={0} max={CARD_TOP_MAX} step={10} suffix="px" />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* --- KANAN: artikelnya — dari mana, bagian mana, isinya apa --- */}
        <div className="screen-col">
          <div className="panel">
            <Section title={t("groupSource")}>

              {/* Satu baris: dua pintu masuk + pencarian. Pencarian sejajar
                  dengan keduanya sebab ia pintu ketiga, bukan setelan milik
                  salah satunya. */}
              <div className="tabs">
                <button className={"ghost" + (entry === "link" ? " active" : "")}
                  onClick={() => setEntry("link")}>
                  {t("tabPasteLink")}
                </button>
                <Popover width={660} buttonClass="ghost" label={t("tabBrowse")}
                  onOpen={() => setBrowseOpen(true)}>
                  {(close) => (
                    <>
                      <div className="popover-head">
                        {query ? (
                          <>
                            <span>{t("searchResultsFor")} <b>{query}</b></span>
                            <button className="ghost tiny" onClick={() => { setQuery(""); setTyped(""); }}>
                              {t("backToSources")}
                            </button>
                          </>
                        ) : (
                          <select value={feed} onChange={(e) => setFeed(e.target.value)}>
                            {config?.feeds.map((f) => (
                              <option key={f.id} value={f.id}>{f.name} — {f.topic}</option>
                            ))}
                          </select>
                        )}
                      </div>

                      {listBusy ? (
                        <p className="stage">{t("loadingNews")}</p>
                      ) : (
                        <div className="news-list">
                          {items.map((a) => (
                            <div
                              key={a.url}
                              className={"news-item" + (article.url === a.url ? " active" : "")}
                              role="button"
                              tabIndex={0}
                              onClick={() => { useArticle(a); close(); }}
                              onKeyDown={(e) => {
                                if (e.key === "Enter" || e.key === " ") {
                                  e.preventDefault(); useArticle(a); close();
                                }
                              }}
                            >
                              {a.image && <img src={a.image} alt="" loading="lazy" />}
                              <div className="news-text">
                                <div className="news-title">{a.title}</div>
                                <div className="news-foot">
                                  <span className="meta">{a.date || a.domain}</span>
                                  <button
                                    className={"copy-btn" + (copied === a.url ? " ok" : "")}
                                    title={t("copyLinkTitle")}
                                    aria-label={t("copyLinkTitle")}
                                    disabled={copyBusy === a.url}
                                    onClick={(e) => { e.stopPropagation(); copyLink(a.url); }}
                                  >
                                    {copyBusy === a.url ? t("copyOpening")
                                      : copied === a.url ? t("copied")
                                      : <><Link2 className="ico" aria-hidden="true" /> {t("copyLink")}</>}
                                  </button>
                                </div>
                              </div>
                            </div>
                          ))}
                        </div>
                      )}
                    </>
                  )}
                </Popover>
                <div className="search">
                  <input
                    value={typed}
                    onChange={(e) => setTyped(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && runSearch()}
                    placeholder={t("searchPlaceholder")}
                    aria-label={t("search")}
                  />
                  <button onClick={runSearch} disabled={!typed.trim() || listBusy}>
                    {listBusy && query ? t("searching") : t("search")}
                  </button>
                </div>
              </div>

              {entry === "link" && (
                <div className="field">
                  <label>{t("articleLink")}</label>
                  <div className="path-row">
                    <input
                      value={link}
                      onChange={(e) => setLink(e.target.value)}
                      onKeyDown={(e) => e.key === "Enter" && fetchLink()}
                      placeholder="https://www.antaranews.com/berita/..."
                    />
                    <button onClick={fetchLink} disabled={fetching || !link.trim()}>
                      {fetching ? t("fetching") : t("fetch")}
                    </button>
                  </div>
                </div>
              )}

            </Section>

            {ready && (
              <>
                {/* LLM hanya MEMILIH NOMOR paragraf; isinya selalu verbatim dari
                    artikelnya (notes/13). */}
                <Section title={t("groupParagraph")}>
                  <div className="grid3">
                    <div className="field">
                      <label>{t("engine")}</label>
                      <select value={engine} onChange={(e) => setEngine(e.target.value)}>
                        <option value="ollama">{t("engineOllama")}</option>
                        <option value="claude">{t("engineClaude")}</option>
                      </select>
                    </div>
                    <div className="field">
                      <label>{t("model")}</label>
                      {engine === "ollama" ? (
                        // Daftar dipakai bila Ollama menjawab; kalau tidak, kolom
                        // ketik tetap ada supaya pengguna tidak terkunci saat
                        // Ollama sedang mati atau berjalan di mesin lain.
                        ollamaInstalled.length > 0 ? (
                          <select value={ollamaModel} onChange={(e) => setOllamaModel(e.target.value)}>
                            {ollamaInstalled.map((m) => <option key={m} value={m}>{m}</option>)}
                          </select>
                        ) : (
                          <input value={ollamaModel} onChange={(e) => setOllamaModel(e.target.value)}
                            placeholder="qwen2.5" />
                        )
                      ) : (
                        <select value={claudeModel} onChange={(e) => setClaudeModel(e.target.value)}>
                          <option value="claude-haiku-4-5">claude-haiku-4-5</option>
                          <option value="claude-sonnet-4-5">claude-sonnet-4-5</option>
                          <option value="claude-opus-4-1">claude-opus-4-1</option>
                        </select>
                      )}
                    </div>
                    <div className="field field-check">
                      <button onClick={analyze} disabled={analyzeBusy}>
                        {analyzeBusy ? t("analyzing") : t("analyze")}
                      </button>
                    </div>
                  </div>

                  {analyzeBusy && engine === "ollama" && <p className="stage">{t("analyzingLocal")}</p>}
                  {selection?.note && <div className="warnbox">{selection.note}</div>}
                  {selection && (
                    <div className="par-list">
                      {selection.rankings.map((r) => (
                        <div
                          key={r.index}
                          className={
                            "par" +
                            (cardIndex === r.index ? " pick-card" : "") +
                            (captionIndex === r.index ? " pick-caption" : "")
                          }
                        >
                          <div
                            className={"par-score" + (r.source === "heuristic" ? " auto" : "")}
                            title={r.source === "heuristic" ? t("scoredAuto") : t("scoredByEngine", { engine: selection.engine })}
                          >
                            {r.score.toFixed(1)}
                            {r.source === "heuristic" && <span className="par-auto">{t("auto")}</span>}
                          </div>
                          <div className="par-body">
                            <div className="par-text">{r.text}</div>
                            {r.reason && <div className="par-reason">{r.reason}</div>}
                            <div className="par-actions">
                              <button className="tiny ghost" onClick={() => applyToCard(paragraphs, r.index)}>
                                {cardIndex === r.index ? t("onCard") : t("useOnCard")}
                              </button>
                              <button className="tiny ghost" onClick={() => applyToCaption(paragraphs, r.index)}>
                                {captionIndex === r.index ? t("asCaption") : t("useAsCaption")}
                              </button>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </Section>

                <Section title={t("groupContent")}>
                  <div className="field">
                    <label>{t("articleTitle")}</label>
                    <textarea rows={2} value={article.title} onChange={edit("title")} />
                  </div>
                  <div className="field">
                    <label>{t("cardText")} {cardIndex !== null && <em>{t("fromParagraph", { n: cardIndex })}</em>}</label>
                    <textarea rows={3} value={article.summary} onChange={edit("summary")} />
                  </div>
                  <div className="field">
                    <label>{t("caption")} {captionIndex !== null && <em>{t("fromParagraph", { n: captionIndex })}</em>}</label>
                    <textarea rows={3} value={caption} onChange={(e) => setCaption(e.target.value)} />
                  </div>
                  <div className="grid3">
                    <div className="field">
                      <label>{t("sourceBadge")}</label>
                      <input value={article.source} onChange={edit("source")} />
                    </div>
                    <div className="field">
                      <label>{t("date")}</label>
                      <input value={article.date} onChange={edit("date")} />
                    </div>
                    <div className="field">
                      <label>{t("hashtags")}</label>
                      <input value={hashtags} onChange={(e) => setHashtags(e.target.value)} placeholder={t("hashtagsPlaceholder")} />
                    </div>
                  </div>
                  <div className="field">
                    <label>{t("imageURL")} {style === "quote" && <em>{t("imageUnusedInQuote")}</em>}</label>
                    <input value={article.image} onChange={edit("image")} />
                  </div>
                  {/* Pengingat atribusi: isi kartu ini verbatim milik
                      penerbitnya, dan itu kewajiban bukan hiasan (notes/13). */}
                  <p className="meta">{t("creditSource")}</p>
                </Section>
              </>
            )}
          </div>

          <div className="panel start-panel">
            <button onClick={() => buildCard(false)}
              disabled={!ready || previewBusy || buildBusy || !config?.has_browser}>
              {buildBusy ? t("rendering") : t("buildCard")}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

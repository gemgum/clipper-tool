"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useI18n } from "../i18n";
import { eng, engineURL } from "../engine";
import { useKeep, useRestore } from "../persist";


// Ruang koordinat kartu — sama dengan yang dipakai engine saat merender.
// Pratinjau memakai CSS transform yang identik, jadi menyeret sejauh N piksel
// di sini menggeser foto sejauh N piksel juga di PNG hasil.
const CARD_W = 1080;
const CARD_HEIGHT: Record<string, number> = { "9:16": 1920, "4:5": 1350, "1:1": 1080 };
// Harus sama dengan photoHeight di engine/internal/card/card.go — kalau berbeda,
// kotak pratinjau tidak lagi sebangun dengan bingkai foto pada hasil render.
const PHOTO_PERCENT: Record<string, number> = { "9:16": 50, "4:5": 50, "1:1": 48 };
// Harus sama dengan card.FontSteps di engine: banyaknya langkah ukuran huruf ke
// tiap arah dari ukuran standar.
const FONT_STEPS = 10;
// Harus sama dengan card.HeaderMax di engine: sejauh mana isi boleh digeser turun.
const HEADER_MAX = 400;
// Harus sama dengan card.CardTopMax di engine: setinggi apa pita kosong di atas
// kartu boleh dibuat.
const CARD_TOP_MAX = 400;
// photoFrameHeight = tinggi bingkai foto dalam piksel kartu — cerminan rumus di
// engine/internal/card/card.go. Kalau rumusnya berbeda, menyeret foto sejauh N
// piksel di sini tidak lagi berarti N piksel di PNG hasil.
function photoFrameHeight(ratio: string): number {
  const h = CARD_HEIGHT[ratio] ?? 1920;
  const pct = PHOTO_PERCENT[ratio] ?? 50;
  return Math.floor((h * pct) / 100);
}

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

  // Bingkai foto: geser & zoom.
  const [photoX, setPhotoX] = useState(0);
  const [photoY, setPhotoY] = useState(0);
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
  const drag = useRef<{ x: number; y: number; ax: number; ay: number } | null>(null);
  const boxRef = useRef<HTMLDivElement>(null);

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
    setPhotoX(0);
    setPhotoY(0);
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
    if (entry === "browse") loadList(feed, query);
  }, [entry, feed, query, loadList]);

  // Mencari selalu memindahkan tampilan ke daftar — hasilnya muncul di bawah.
  const runSearch = useCallback(() => {
    setQuery(typed.trim());
    setEntry("browse");
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
    photoX, photoY, zoom, photoFit, photoFill,
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
          photo: { offset_x: photoX, offset_y: photoY, zoom, fit: photoFit, fill: photoFill },
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
  }, [article, style, ratio, align, lang, caption, hashtags, photoX, photoY, zoom,
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
    style, ratio, align, photoX, photoY, zoom, photoFit, photoFill,
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
    set(setPhotoX, v.photoX);
    set(setPhotoY, v.photoY);
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

  // Seret di pratinjau → geser foto. Perpindahan piksel layar dikembalikan ke
  // ruang koordinat kartu memakai skala kotak, supaya nilai yang dikirim ke
  // engine tidak bergantung pada seberapa besar pratinjaunya ditampilkan.
  const boxScale = useCallback(() => {
    const el = boxRef.current;
    return el ? el.clientWidth / CARD_W : 1;
  }, []);

  // Batas geser memakai rumus yang sama dengan engine (offsetLimit di card.go):
  // pada zoom Z foto jadi Z kali bingkai, jadi sisa ruangnya (Z-1)/2 tiap sisi.
  // Menyeret lebih jauh hanya akan memunculkan celah kosong di tepi kartu.
  const frameHeight = photoFrameHeight(ratio);
  const limitX = Math.floor((CARD_W * (zoom - 1)) / 2);
  const limitY = Math.floor((frameHeight * (zoom - 1)) / 2);
  const clamp = (v: number, limit: number) => Math.max(-limit, Math.min(limit, v));

  const startDrag = useCallback((e: React.PointerEvent) => {
    e.preventDefault();
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
    drag.current = { x: e.clientX, y: e.clientY, ax: photoX, ay: photoY };
  }, [photoX, photoY]);

  const moveDrag = useCallback((e: React.PointerEvent) => {
    const d = drag.current;
    if (!d) return;
    const scale = boxScale();
    setPhotoX(clamp(Math.round(d.ax + (e.clientX - d.x) / scale), limitX));
    setPhotoY(clamp(Math.round(d.ay + (e.clientY - d.y) / scale), limitY));
  }, [boxScale, limitX, limitY]);

  const endDrag = useCallback(() => { drag.current = null; }, []);

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
    <div className="wrap">
      <h1>{t("newsTitle")}</h1>
      <p className="sub">
        {t("newsIntro")} <b>{t("newsIntroBold")}</b> {t("newsIntroTail")}
      </p>

      {config && !config.has_browser && (
        <div className="warnbox">
          {t("browserMissing")} <code>CLIPPER_CHROME</code> {t("browserMissingTail")}
        </div>
      )}
      {error && <div className="warnbox err">{error}</div>}

      {/* --- Pintu masuk --- */}
      <div className="panel">
        <div className="tabs">
          <button className={"ghost" + (entry === "link" ? " active" : "")} onClick={() => setEntry("link")}>
            {t("tabPasteLink")}
          </button>
          <button className={"ghost" + (entry === "browse" ? " active" : "")} onClick={() => setEntry("browse")}>
            {t("tabBrowse")}
          </button>
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

        {entry === "link" ? (
          <div className="row" style={{ alignItems: "flex-end" }}>
            <div className="field" style={{ flex: 3 }}>
              <label>{t("articleLink")}</label>
              <input
                value={link}
                onChange={(e) => setLink(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && fetchLink()}
                placeholder="https://www.antaranews.com/berita/..."
              />
            </div>
            <div className="field" style={{ flex: "none", minWidth: 0 }}>
              <button onClick={fetchLink} disabled={fetching || !link.trim()}>
                {fetching ? t("fetching") : t("fetch")}
              </button>
            </div>
          </div>
        ) : (
          <>
            {query ? (
              <div className="search-result">
                <span>{t("searchResultsFor")} <b>{query}</b></span>
                <button className="ghost tiny" onClick={() => { setQuery(""); setTyped(""); }}>
                  {t("backToSources")}
                </button>
              </div>
            ) : (
            <div className="field">
              <label>{t("newsSource")}</label>
              <select value={feed} onChange={(e) => setFeed(e.target.value)}>
                {config?.feeds.map((f) => (
                  <option key={f.id} value={f.id}>{f.name} — {f.topic}</option>
                ))}
              </select>
            </div>
            )}
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
                    onClick={() => useArticle(a)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") { e.preventDefault(); useArticle(a); }
                    }}
                  >
                    {/* Hasil pencarian Google News tidak membawa gambar — gambarnya
                        baru ada setelah artikelnya dibuka. Kotak kosong dihilangkan
                        saja daripada menyisakan bidang abu yang terlihat rusak. */}
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
                            : t("copyLink")}
                        </button>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </div>

      {/* --- Analisis AI --- */}
      {ready && (
        <div className="panel">
          <label className="blok">{t("analyzeHeading")}</label>
          <div className="row" style={{ alignItems: "flex-end" }}>
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
                // Daftar dipakai bila Ollama menjawab; kalau tidak, kolom ketik
                // tetap ada supaya pengguna tidak terkunci saat Ollama sedang
                // mati atau berjalan di mesin lain.
                ollamaInstalled.length > 0 ? (
                  <select value={ollamaModel} onChange={(e) => setOllamaModel(e.target.value)}>
                    {ollamaInstalled.map((m) => (
                      <option key={m} value={m}>{m}</option>
                    ))}
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
            <div className="field" style={{ flex: "none", minWidth: 0 }}>
              <button onClick={analyze} disabled={analyzeBusy}>
                {analyzeBusy ? t("analyzing") : t("analyze")}
              </button>
            </div>
          </div>
          {analyzeBusy && engine === "ollama" && (
            <p className="stage">{t("analyzingLocal")}</p>
          )}

          {selection && (
            <>
              <p className="stage">{t("rankingIntro", { n: selection.rankings.length })}</p>
              {selection.note && <div className="warnbox">{selection.note}</div>}
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
            </>
          )}
        </div>
      )}

      {/* --- Bahan kartu --- */}
      {ready && (
        <div className="panel">
          <label className="blok">{t("cardContent")}</label>
          <div className="field">
            <label>{t("articleTitle")}</label>
            <textarea rows={2} value={article.title} onChange={edit("title")} />
          </div>
          <div className="field">
            <label>{t("cardText")} {cardIndex !== null && <em>{t("fromParagraph", { n: cardIndex })}</em>}</label>
            <textarea rows={4} value={article.summary} onChange={edit("summary")} />
          </div>
          <div className="row">
            <div className="field">
              <label>{t("sourceBadge")}</label>
              <input value={article.source} onChange={edit("source")} />
            </div>
            <div className="field">
              <label>{t("date")}</label>
              <input value={article.date} onChange={edit("date")} />
            </div>
          </div>
          <div className="field">
            <label>{t("imageURL")} {style === "quote" && <em>{t("imageUnusedInQuote")}</em>}</label>
            <input value={article.image} onChange={edit("image")} />
          </div>

          {article.url && (
            <div className="field">
              <label>{t("articleLink")} <em>{t("articleLinkCheck")}</em></label>
              <div className="link-row">
                <a href={article.url} target="_blank" rel="noreferrer" title={article.url}>{article.url}</a>
                <button
                  className={"copy-btn" + (copied === article.url ? " ok" : "")}
                  title={t("copyLinkTitle")}
                  aria-label={t("copyLinkTitle")}
                  onClick={() => copyLink(article.url)}
                >
                  {copied === article.url ? t("copied") : t("copyLink")}
                </button>
              </div>
            </div>
          )}

          <div className="field">
            <label>{t("caption")} {captionIndex !== null && <em>{t("fromParagraph", { n: captionIndex })}</em>}</label>
            <textarea rows={4} value={caption} onChange={(e) => setCaption(e.target.value)} />
          </div>
          <div className="field">
            <label>{t("hashtags")}</label>
            <input value={hashtags} onChange={(e) => setHashtags(e.target.value)} placeholder={t("hashtagsPlaceholder")} />
          </div>

          {style !== "quote" && article.image.startsWith("http") && (
            <div className="field">
              <label>
                {t("photoFrame")} <em>{t("photoFrameHint")}</em>
              </label>
              <div className="photo-tools">
                <div
                  ref={boxRef}
                  className="photo-box"
                  style={{ aspectRatio: `${CARD_W} / ${photoFrameHeight(ratio)}` }}
                  onPointerDown={startDrag}
                  onPointerMove={moveDrag}
                  onPointerUp={endDrag}
                  onPointerCancel={endDrag}
                  onWheel={(e) => {
                    const z = Math.min(4, Math.max(1, +(zoom * (e.deltaY < 0 ? 1.06 : 1 / 1.06)).toFixed(3)));
                    // Geseran ikut dijepit: mengecilkan zoom mempersempit ruang
                    // gerak, jadi posisi lama bisa jadi di luar batas baru.
                    const bx = Math.floor((CARD_W * (z - 1)) / 2);
                    const by = Math.floor((frameHeight * (z - 1)) / 2);
                    setZoom(z);
                    setPhotoX((v) => clamp(v, bx));
                    setPhotoY((v) => clamp(v, by));
                  }}
                >
                  {/* CSS-nya sengaja disamakan persis dengan template kartu di
                      engine/internal/card — kalau salah satu diubah, ubah keduanya. */}
                  {photoFit === "whole" && photoFill === "blur" && (
                    <img className="photo-fill" src={article.image} alt="" draggable={false} />
                  )}
                  <img
                    src={article.image}
                    alt=""
                    draggable={false}
                    style={{
                      objectFit: photoFit === "whole" ? "contain" : "cover",
                      transform:
                        `translate(-50%,-50%) translate(${photoX / CARD_W * 100}cqw, ${photoY / CARD_W * 100}cqw) scale(${zoom})`,
                    }}
                  />
                  <div className="photo-fade" />
                </div>
                <div className="photo-knobs">
                  {/* Titik awal zoom, bukan sumbu kedua — persis seperti mode
                      pas di tab klip. "Utuh" memasukkan seluruh gambar asli
                      berapa pun rasionya; ruang yang tak terjangkau gambar diisi
                      salinan buram fotonya atau warna latar kartu. */}
                  <label>{t("photoFit")}</label>
                  <select value={photoFit} onChange={(e) => {
                    setPhotoFit(e.target.value);
                    setZoom(1); setPhotoX(0); setPhotoY(0);
                  }}>
                    <option value="cover">{t("photoFitCover")}</option>
                    <option value="whole">{t("photoFitWhole")}</option>
                  </select>
                  {photoFit === "whole" && (
                    <>
                      <label>{t("photoFill")}</label>
                      <select value={photoFill} onChange={(e) => setPhotoFill(e.target.value)}>
                        <option value="blur">{t("photoFillBlur")}</option>
                        <option value="solid">{t("photoFillSolid")}</option>
                      </select>
                    </>
                  )}
                  <label>{t("photoZoom", { n: zoom.toFixed(2) })}</label>
                  <input
                    type="range" min={1} max={4} step={0.01}
                    value={zoom}
                    onChange={(e) => {
                      const z = parseFloat(e.target.value);
                      setZoom(z);
                      setPhotoX((v) => clamp(v, Math.floor((CARD_W * (z - 1)) / 2)));
                      setPhotoY((v) => clamp(v, Math.floor((frameHeight * (z - 1)) / 2)));
                    }}
                  />
                  {zoom === 1 && <p className="meta">{t("photoZoomHint")}</p>}
                  <p className="meta">{t("photoOffset", { x: photoX, y: photoY })}</p>
                  <button
                    className="ghost tiny"
                    onClick={() => { setPhotoX(0); setPhotoY(0); setZoom(1); }}
                  >
                    {t("photoReset")}
                  </button>
                </div>
              </div>
            </div>
          )}

          <div className="row">
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
          </div>
          {align === "justify" && <p className="stage">{t("justifyNote")}</p>}

          {/* --- Warna kartu --- */}
          <div className="field">
            <label>{t("cardColour")} <em>{t("cardColourHint")}</em></label>
            <div className="row">
              <div className="field">
                <select value={colorSource} onChange={(e) => setColorSource(e.target.value)}>
                  <option value="photo">{t("colourFromPhoto")}</option>
                  <option value="custom">{t("colourCustom")}</option>
                </select>
              </div>
            </div>
            {/* Daftar tertutup, bukan pemilih spektrum. Engine hanya memakai RONA
                warna pilihan — terangnya dikunci palet — jadi pemilih spektrum
                menjanjikan yang tidak dikerjakan: putih & abu-abu tidak mengubah
                apa pun, dan itu terbaca sebagai bug. Warnanya datang dari engine,
                tidak dihitung ulang di sini. */}
            {colorSource === "custom" && (
              <>
                {/* Satu baris per keluarga warna, urutannya dari engine. Sengaja
                    tanpa nama: yang perlu dilihat warnanya, bukan istilahnya. */}
                {colourRows(config?.card_colours).map((row, i) => (
                  <div className="swatches" key={i}>
                    {row.map((c) => (
                      <button
                        key={c}
                        type="button"
                        className={"swatch" + (c === customColor ? " on" : "")}
                        style={{ background: c }}
                        title={c}
                        aria-label={c}
                        onClick={() => setCustomColor(c)}
                      />
                    ))}
                  </div>
                ))}
                <p className="meta">{t("colourSwatchNote", { hex: customColor })}</p>
              </>
            )}
            {colorSource === "photo" && <p className="meta">{t("colourFromPhotoNote")}</p>}
          </div>

          <div className="field">
            <label>{t("cardBoxBackground")}</label>
            <div className="row">
              <div className="field">
                <select value={boxMode} onChange={(e) => setBoxMode(e.target.value)}>
                  <option value="auto">{t("boxAuto")}</option>
                  <option value="none">{t("boxNone")}</option>
                  <option value="custom">{t("boxCustom")}</option>
                </select>
              </div>
              {boxMode === "custom" && (
                <div className="field" style={{ flex: "none" }}>
                  <input type="color" value={boxColor}
                    onChange={(e) => setBoxColor(e.target.value.toUpperCase())} />
                </div>
              )}
              {boxMode === "custom" && (
                <div className="field">
                  <input value={boxColor} spellCheck={false}
                    onChange={(e) => setBoxColor(e.target.value.toUpperCase())} />
                </div>
              )}
            </div>
            {boxMode === "none" && <p className="meta">{t("boxNoneNote")}</p>}
          </div>

          {/* Pratinjau di kiri, kendali ukuran huruf di kanan: keduanya saling
              menjawab, jadi menyetel dan melihat hasilnya tidak perlu berpindah
              tempat. Tombolnya di bawah keduanya. */}
          <div className="card-stage">
            <div className="card-stage-view">
              {result ? (
                <img src={result.file} alt={t("newsTitle")} />
              ) : (
                <p className="meta">{t("previewEmpty")}</p>
              )}
            </div>

            <div className="card-stage-tools">
              <label>{t("fontSizes")}</label>
              {([
                ["title", titleStep, setTitleStep],
                ["paragraph", paragraphStep, setParagraphStep],
              ] as const).map(([key, value, set]) => (
                <div key={key} className="field">
                  <label>
                    {key === "title" ? t("fontTitle") : t("fontParagraph")}{" "}
                    <em>{value === 0 ? t("fontStandard") : `${value > 0 ? "+" : ""}${value}`}</em>
                  </label>
                  <input
                    type="range" min={-FONT_STEPS} max={FONT_STEPS} step={1}
                    value={value} onChange={(e) => set(Number(e.target.value))}
                  />
                </div>
              ))}
              <p className="meta">{t("fontStepsNote")}</p>

              {/* Menggeser isi sebagai SATU kesatuan — judul, guntingan, dan kaki
                  kartu bergerak bersama. Bukan penggeser per blok: itu sudah
                  dicoba dan dicabut karena blok jadi saling tumpang tindih. */}
              <div className="field">
                <label>
                  {t("headerSpace")}{" "}
                  <em>{header === 0 ? t("fontStandard") : `+${header} px`}</em>
                </label>
                <input type="range" min={0} max={HEADER_MAX} step={10}
                  value={header} onChange={(e) => setHeader(Number(e.target.value))} />
              </div>
              <p className="meta">{t("headerSpaceNote")}</p>

              {/* Menurunkan SELURUH kartu — area foto ikut turun, dan pita
                  kosong di atas memakai warna latar kartu. */}
              <div className="field">
                <label>
                  {t("cardDown")}{" "}
                  <em>{cardTop === 0 ? t("fontStandard") : `+${cardTop} px`}</em>
                </label>
                <input type="range" min={0} max={CARD_TOP_MAX} step={10}
                  value={cardTop} onChange={(e) => setCardTop(Number(e.target.value))} />
              </div>
              <p className="meta">{t("cardDownNote")}</p>

              <button className="ghost tiny"
                onClick={() => { setTitleStep(0); setParagraphStep(0); setHeader(0); setCardTop(0); }}
                disabled={titleStep === 0 && paragraphStep === 0 && header === 0 && cardTop === 0}>
                {t("fontReset")}
              </button>
            </div>
          </div>

          {/* Pratinjau dulu, simpan belakangan. Menyetel kartu itu pekerjaan
              puluhan percobaan, dan tiap simpanan meninggalkan satu folder. */}
          <div className="row">
            <button onClick={() => buildCard(true)}
              disabled={previewBusy || buildBusy || !config?.has_browser}>
              {previewBusy ? t("rendering") : t("previewCard")}
            </button>
            <button onClick={() => buildCard(false)}
              disabled={previewBusy || buildBusy || !config?.has_browser}>
              {buildBusy ? t("rendering") : t("buildCard")}
            </button>
          </div>
        </div>
      )}

      {/* --- Hasil --- */}
      {result && (
        <div className="panel">
          <label className="blok">{result.preview ? t("previewResult") : t("result")}</label>
          {/* Gambarnya sudah tampil di atas tombol, jadi di sini cukup tautan
              unduhannya — mengulang gambar yang sama hanya memanjangkan halaman. */}
          <div className="card-result">
            <div>
              {/* Pratinjau tidak punya berkas pendamping — caption & keterangan
                  sumber baru ditulis saat kartunya disimpan. */}
              {result.preview ? (
                <p className="meta">{t("previewNotSaved")}</p>
              ) : (
                <>
                  <p><a className="dl" href={result.zip}>{t("downloadZip")}</a></p>
                  <p><a className="dl" href={result.file} download="card.png">{t("downloadImage")}</a></p>
                </>
              )}
              {article.url && (
                <p className="meta" style={{ marginTop: 10 }}>
                  {t("sourceLabel")}{" "}
                  <a className="dl" href={article.url} target="_blank" rel="noreferrer">{article.domain}</a>
                  <br />
                  {t("creditSource")}
                </p>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

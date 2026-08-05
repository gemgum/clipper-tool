"use client";

// Ikon: lucide-react (ISC) — alasannya di gui/app/page.tsx.
import { Crosshair, Eye, Info, RotateCw, X } from "lucide-react";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useI18n } from "./i18n";
import { eng } from "./engine";
import Stepper from "./stepper";
import Select from "./select";

export const PLAY_W = 1080, PLAY_H = 1920; // ruang koordinat subtitle

// Area yang tertutup UI tiap aplikasi (fraksi tinggi/lebar frame 9:16).
// Angka awal hasil pengukuran kasar — sesuaikan bila UI aplikasi berubah.
//
// Tinggal di sini bersama pratinjau, bukan di <FramePanel>: pembatas ini
// menggambar overlay DI ATAS bingkai dan mengatur ke mana subtitle boleh
// ditaruh — bukan bagaimana video dipasang ke bingkai.
export type Zone = { top: number; bottom: number; right: number };
export const PLATFORMS: Record<string, { label: string; zone: Zone }> = {
  tiktok: { label: "TikTok", zone: { top: 0.08, bottom: 0.20, right: 0.16 } },
  reels: { label: "Instagram Reels", zone: { top: 0.07, bottom: 0.17, right: 0.15 } },
  shorts: { label: "YouTube Shorts", zone: { top: 0.06, bottom: 0.13, right: 0.14 } },
  generic: { label: "", zone: { top: 0.08, bottom: 0.20, right: 0.16 } },
};

// scale = piksel CSS per satuan "size" subtitle. WAJIB dipakai pratinjau: .ass
// mengartikan ukuran sebagai tinggi kotak font, CSS mengartikannya sebagai em,
// dan selisihnya berbeda tiap font (Montserrat 0,725 · Anton 0,577). Engine yang
// menghitungnya dari berkas fontnya — lihat engine/internal/api/fontmetrics.go.
export type Font = { name: string; scale: number };
// Hasil /api/font-check: valid = font benar-benar ada (bawaan atau sistem).
export type FontCheck = { valid: boolean; name: string; family: string; source: string; error: string; scale: number };

// Titik tengah bidang 9:16 + toleransi magnet (dalam koordinat 1080×1920).
// 20 ≈ 5 piksel layar pada preview 270 px, cukup terasa tanpa bikin susah
// menaruh subtitle sedikit di luar tengah.
export const CENTER_X = 540, CENTER_Y = 960;
const MAGNET = 20;
// Pilihan kerapatan grid di ruang 1080×1920. 0 = mati.
//
// 20 jadi bawaan: 54×96 kotak — cukup kasar untuk membuat penempatan bisa
// diulang, cukup halus untuk tidak terasa seperti pagar. Yang lain disediakan
// karena "rasa" grid hanya bisa dinilai dengan mencoba, bukan dari angka.
const GRIDS = [0, 10, 20, 24, 40] as const;
// snap membulatkan ke kelipatan terdekat. g = 0 berarti tidak menempel.
const snap = (v: number, g: number) => (g > 0 ? Math.round(v / g) * g : v);
// Baris contoh di preview — panjangnya sengaja mendekati batas nyata satu baris
// (~22 karakter pada ukuran font bawaan).
const SAMPLE_LINES = ["sampleLine1", "sampleLine2", "sampleLine3"] as const;

const hex = (c: string) => (c === "yellow" ? "#ffdd00" : c === "green" ? "#4ade80" : c === "cyan" ? "#38bdf8" : "#ffffff");

// Berapa baris yang mungkin muncul sekaligus — harus sama dengan Pacing() di
// engine (lambat/normal 2, padat 3; mode word selalu 1 kata per layar).
export const linesFor = (subMode: string, subSpeed: string) =>
  subMode === "word" ? 1 : subSpeed === "dense" ? 3 : 2;

// Bingkai 9:16 + seretan subtitle + kendali huruf dan posisi.
//
// children diletakkan di ujung kolom setelan: di situlah <FramePanel> duduk,
// sebab kendali bingkai memang mengubah gambar di sebelah kiri.
export default function PreviewPanel({
  path, reframe, background, zoom, zone,
  fonts, subFont, setSubFont, fontManual, setFontManual, fontCheck, fontChecking,
  subSize, setSubSize, subColor, setSubColor, subOutline, setSubOutline, subBox, setSubBox,
  subMode, setSubMode, subHighlight, setSubHighlight, subSpeed, setSubSpeed,
  subX, setSubX, subY, setSubY, blockH, centerAnchorY,
  platform, setPlatform, inUnsafe, onPlaceSafe, addLog, children,
}: {
  path: string; reframe: string; background: string; zoom: number; zone?: Zone;
  platform: string; setPlatform: (v: string) => void;
  inUnsafe: boolean; onPlaceSafe: () => void;
  fonts: Font[];
  subFont: string; setSubFont: (v: string) => void;
  fontManual: boolean; setFontManual: (v: boolean) => void;
  fontCheck: FontCheck | null; fontChecking: boolean;
  subSize: number; setSubSize: (v: number) => void;
  subColor: string; setSubColor: (v: string) => void;
  subOutline: number; setSubOutline: (v: number) => void;
  subBox: boolean; setSubBox: (v: boolean) => void;
  subMode: string; setSubMode: (v: string) => void;
  subHighlight: string; setSubHighlight: (v: string) => void;
  subSpeed: string; setSubSpeed: (v: string) => void;
  subX: number; setSubX: (v: number | ((p: number) => number)) => void;
  subY: number; setSubY: (v: number | ((p: number) => number)) => void;
  blockH: number; centerAnchorY: number;
  addLog: (text: string) => void;
  children?: React.ReactNode;
}) {
  const { t } = useI18n();

  const [previewOn, setPreviewOn] = useState(false);
  const [isDragging, setIsDragging] = useState(false); // sedang menggeser subtitle
  const [alwaysGuides, setAlwaysGuides] = useState(false); // paksa grid tetap tampil
  const [grid, setGrid] = useState<number>(20);
  const [previewBusy, setPreviewBusy] = useState(false);
  const [duration, setDuration] = useState(0);
  const [previewTime, setPreviewTime] = useState(5);
  // Dinaikkan tiap muat ulang manual supaya URL frame berubah — kalau URL-nya
  // sama persis, browser memakai gambar lama meski videonya sudah ditimpa.
  const [previewNonce, setPreviewNonce] = useState(0);
  const boxRef = useRef<HTMLDivElement | null>(null);
  const draggingRef = useRef(false);

  // Tinggi bingkai = tinggi kolom setelan di sebelahnya, supaya dasarnya
  // SEJAJAR. Diukur, bukan dirumuskan: tinggi kolom setelan ditentukan isinya
  // (berapa baris kendali yang muat pada lebar saat itu), dan CSS tidak punya
  // cara membacanya. Tidak ada lingkaran umpan balik — kolom setelan sama
  // sekali tidak bergantung pada tinggi bingkai.
  const layoutRef = useRef<HTMLDivElement | null>(null);
  const settingsRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    const el = settingsRef.current, host = layoutRef.current;
    if (!el || !host) return;
    const apply = () => host.style.setProperty("--pv-h", `${Math.round(el.getBoundingClientRect().height)}px`);
    apply();
    const ro = new ResizeObserver(apply);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // reframe ikut dikirim: preview memakai mode yang sama dengan render, jadi
  // koordinat subtitle diatur di atas geometri yang benar (di mode "muat utuh"
  // videonya cuma mengisi pita tengah, bukan seluruh kanvas).
  const frameUrl = useMemo(
    () => eng(`/api/frame?path=${encodeURIComponent(path)}&t=${previewTime.toFixed(2)}`)
      + `&reframe=${encodeURIComponent(reframe)}&background=${encodeURIComponent(background)}`
      + `&zoom=${zoom}`
      + `&n=${previewNonce}`,
    [path, previewTime, reframe, background, zoom, previewNonce]
  );

  // silent = dipicu otomatis oleh video baru; kegagalannya tidak perlu
  // diteriakkan ke log karena path bisa saja masih setengah diketik.
  const loadPreview = useCallback(async (silent = false) => {
    if (!path) return;
    setPreviewBusy(true);
    try {
      const res = await fetch(eng(`/api/probe?path=${encodeURIComponent(path)}`));
      const text = await res.text();
      let data: any;
      try { data = JSON.parse(text); }
      catch { throw new Error(t("errOldEngine")); }
      if (!res.ok) throw new Error(data.error || t("errReadVideo"));
      setDuration(data.duration || 0);
      setPreviewTime(Math.min(5, (data.duration || 10) / 2));
      setPreviewNonce((n) => n + 1);
      setPreviewOn(true);
    } catch (e: any) {
      if (!silent) addLog(t("logPreviewFailed", { error: e.message }));
    } finally { setPreviewBusy(false); }
  }, [path, addLog, t]);

  // Kembali ke keadaan awal: frame dilepas, durasi & waktu direset. Dipakai
  // tombol "reset" dan otomatis saat videonya berganti.
  const resetPreview = useCallback(() => {
    setPreviewOn(false);
    setPreviewBusy(false);
    setDuration(0);
    setPreviewTime(5);
  }, []);

  // Video baru masuk (unggah, seret-lepas, atau path diketik) → preview lama
  // dibuang lalu dimuat ulang sendiri. Ditunda 500 ms supaya path yang sedang
  // diketik tidak memicu satu permintaan per huruf.
  useEffect(() => {
    resetPreview();
    if (!path) return;
    const timer = setTimeout(() => loadPreview(true), 500);
    return () => clearTimeout(timer);
  }, [path, resetPreview, loadPreview]);

  // Geser subtitle di atas frame.
  //
  // Selisih titik pegang disimpan saat tombol ditekan, lalu ikut dijumlahkan
  // tiap gerakan. Dulu posisi langsung disamakan dengan kursor, jadi teks
  // melompat ke bawah kursor begitu disentuh — itu yang terasa "kurang stabil".
  const grab = useRef({ dx: 0, dy: 0 });

  const previewPoint = useCallback((e: React.PointerEvent) => {
    const rect = boxRef.current!.getBoundingClientRect();
    return {
      x: ((e.clientX - rect.left) / rect.width) * PLAY_W,
      y: ((e.clientY - rect.top) / rect.height) * PLAY_H,
    };
  }, []);

  const startDrag = useCallback((e: React.PointerEvent) => {
    if (!boxRef.current) return;
    e.preventDefault();
    const p = previewPoint(e);
    grab.current = { dx: subX - p.x, dy: subY - p.y };
    draggingRef.current = true;
    setIsDragging(true);
    e.currentTarget.setPointerCapture?.(e.pointerId);
  }, [subX, subY, previewPoint]);

  const onPointerMove = useCallback((e: React.PointerEvent) => {
    if (!draggingRef.current || !boxRef.current) return;
    const p = previewPoint(e);
    let nx = p.x + grab.current.dx;
    let ny = p.y + grab.current.dy;
    // Alt = abaikan grid. Grid adalah bawaan, bukan pagar: harus selalu ada
    // jalan menaruh subtitle satu piksel di luar kotaknya.
    const g = e.altKey ? 0 : grid;
    // Magnet MENANG atas grid, dan itu bukan selera: titik tengahnya sering
    // bukan kelipatan grid (pada grid 24, X tengah 540 tidak terjangkau sama
    // sekali), jadi menempelkan ke grid lebih dulu akan membuat "tepat di
    // tengah" mustahil dicapai — persis kemampuan yang tidak boleh hilang.
    nx = Math.abs(nx - CENTER_X) < MAGNET ? CENTER_X : snap(nx, g);
    ny = Math.abs(ny - centerAnchorY) < MAGNET ? centerAnchorY : snap(ny, g);
    setSubX(Math.round(Math.max(0, Math.min(PLAY_W, nx))));
    // Batas bawah dikurangi tinggi blok: yang harus tetap di dalam bingkai
    // adalah seluruh blok, bukan cuma titik jangkarnya.
    setSubY(Math.round(Math.max(0, Math.min(PLAY_H - blockH, ny))));
  }, [previewPoint, blockH, centerAnchorY, grid, setSubX, setSubY]);

  // Tombol panah menggeser 1 piksel (Shift = 10) — penempatan halus tanpa harus
  // mematikan grid, dan satu-satunya cara menaruh subtitle dengan angka yang
  // benar-benar bisa diulang.
  const onKeyDown = useCallback((e: React.KeyboardEvent) => {
    const step = e.shiftKey ? 10 : 1;
    const move: Record<string, [number, number]> = {
      ArrowLeft: [-step, 0], ArrowRight: [step, 0],
      ArrowUp: [0, -step], ArrowDown: [0, step],
    };
    const d = move[e.key];
    if (!d) return;
    e.preventDefault();
    setSubX((v: number) => Math.round(Math.max(0, Math.min(PLAY_W, v + d[0]))));
    setSubY((v: number) => Math.round(Math.max(0, Math.min(PLAY_H - blockH, v + d[1]))));
  }, [blockH, setSubX, setSubY]);

  const endDrag = useCallback(() => {
    draggingRef.current = false;
    setIsDragging(false);
  }, []);

  const atCenterX = subX === CENTER_X;
  const atCenterY = subY === centerAnchorY;
  const guidesVisible = isDragging || alwaysGuides;

  const maxLines = linesFor(subMode, subSpeed);
  // Piksel CSS per satuan ukuran subtitle, dari engine. 0 = belum tahu (daftar
  // font belum sampai, atau font manual belum lolos cek) — dalam keadaan itu
  // dipakai 1, yang berarti pratinjau menggambar seperti sebelumnya.
  const fontScale =
    (fontManual ? fontCheck?.scale : fonts.find((f) => f.name === subFont)?.scale) || 1;
  const colorHex = hex(subColor);
  const highlightHex = hex(subHighlight);

  return (
    <div className="sub-layout" ref={layoutRef}>
      {/* Kiri: bingkai preview. Bingkainya selalu ada — walau frame video
          belum dimuat — supaya posisi subtitle tetap bisa diatur lebih dulu. */}
      <div className="sub-preview">
        <div className="preview9x16" ref={boxRef}>
          {previewOn ? (
            /* eslint-disable-next-line @next/next/no-img-element */
            <img src={frameUrl} alt="preview" draggable={false} />
          ) : (
            <div className="preview-empty">
              <div className="pe-icon" aria-hidden="true" />
              <div className="pe-title">{t("emptyFrame")}</div>
            </div>
          )}
          {zone && (
            <>
              <div className="safezone" style={{ top: 0, left: 0, right: 0, height: `${zone.top * 100}%` }}>
                <span>{t("zoneTop")}</span>
              </div>
              <div className="safezone" style={{ bottom: 0, left: 0, right: 0, height: `${zone.bottom * 100}%` }}>
                <span>{t("zoneBottom")}</span>
              </div>
              <div className="safezone" style={{
                top: `${zone.top * 100}%`, bottom: `${zone.bottom * 100}%`,
                right: 0, width: `${zone.right * 100}%`,
              }}>
                <span className="vert">{t("zoneRight")}</span>
              </div>
            </>
          )}
          {/* Garis tengah: muncul saat digeser (atau dikunci lewat centang).
              Kelas "on" = subtitle sedang menempel di garis itu. */}
          {guidesVisible && grid >= 20 && (
            /* Kotak grid digambar sebagai latar berulang, bukan puluhan elemen:
               pada grid 10 itu 108×192 kotak, dan menggambarnya satu per satu
               berarti 300 elemen yang harus dihitung ulang tiap seretan.

               Di bawah 20 meshnya TIDAK digambar: pratinjau selebar 270 px
               berarti kotak grid 10 hanya 2,5 px di layar, dan yang terlihat
               bukan grid melainkan kabut kelabu di atas frame videonya. Yang
               menempel tetap menempel — cuma gambarnya yang tidak menolong. */
            <div className="gridmesh" style={{
              backgroundSize: `${(grid / PLAY_W) * 100}% ${(grid / PLAY_H) * 100}%`,
            }} />
          )}
          {guidesVisible && (
            <>
              <div className={`guide v${atCenterX ? " on" : ""}`} />
              <div className={`guide h${atCenterY ? " on" : ""}`} />
              {atCenterX && atCenterY && <div className="guide xy" />}
            </>
          )}
          {/* Sumbu pada posisi subtitle SEKARANG, berikut angkanya. Tanpa angka,
              "pasti" hanya terasa — tidak bisa diulang di klip berikutnya. */}
          {isDragging && (
            <>
              <div className="axis v" style={{ left: `${(subX / PLAY_W) * 100}%` }} />
              <div className="axis h" style={{ top: `${(subY / PLAY_H) * 100}%` }} />
              <div className="axis-read" style={{
                left: `${(subX / PLAY_W) * 100}%`, top: `${(subY / PLAY_H) * 100}%`,
              }}>{subX} · {subY}</div>
            </>
          )}
          <div className="suboverlay"
            style={{
              left: `${(subX / PLAY_W) * 100}%`, top: `${(subY / PLAY_H) * 100}%`,
              fontFamily: `"${subFont}", sans-serif`,
              fontSize: `calc(${(subSize * fontScale) / PLAY_H} * var(--pvh))`,
              // Kotak baris harus setinggi ukuran .ass, sedangkan font-size di
              // atas sudah dikecilkan jadi em-nya. 1/scale mengembalikannya.
              lineHeight: 1 / fontScale,
              color: colorHex,
              background: subBox ? "rgba(0,0,0,0.6)" : "transparent",
              borderRadius: subBox ? 8 : 0,
              padding: subBox ? "4px 14px" : "2px 8px",
              textShadow: subBox ? "none" : (subOutline > 0
                ? "-2px -2px 0 #000,2px -2px 0 #000,-2px 2px 0 #000,2px 2px 0 #000,0 0 4px #000"
                : "none"),
            }}
            tabIndex={0}
            onKeyDown={onKeyDown}
            onPointerDown={startDrag}
            onPointerMove={onPointerMove}
            onPointerUp={endDrag}
            onPointerCancel={endDrag}>
            {/* Contoh ditampilkan sebanyak baris terbanyak yang mungkin
                muncul, bukan satu baris pendek. Dulu preview selalu 1 baris
                sehingga pengguna menaruhnya pas, lalu terkejut waktu hasilnya
                2 baris. */}
            {subMode === "word" ? (
              <div style={{ color: highlightHex }}>{t("sampleWord")}</div>
            ) : (
              Array.from({ length: maxLines }, (_, i) => {
                const line = t(SAMPLE_LINES[i]);
                const isLast = i === maxLines - 1;
                if (subMode !== "karaoke" || !isLast) return <div key={i}>{line}</div>;
                // Karaoke: kata terakhir yang sedang disorot.
                const words = line.split(" ");
                const tail = words.pop();
                return (
                  <div key={i}>
                    {words.join(" ")} <span style={{ color: highlightHex }}>{tail}</span>
                  </div>
                );
              })
            )}
          </div>
        </div>

      </div>

      {/* Kanan: setelan dalam kelompok bernama, dan tiap kelompok memakai kisi
          TIGA KOLOM yang sama.

          Sebelumnya tiap baris flex membagi lebarnya sendiri, jadi baris berisi
          dua kendali melebar sementara baris berisi tiga menyempit — tepi
          kanannya bergerigi dan tidak ada satu pun yang sejajar. Dengan kisi,
          kolomnya ditentukan sekali dan semua kendali berdiri di garis yang
          sama, berapa pun isinya. */}
      <div className="sub-settings" ref={settingsRef}>
        <div className="group">
        <div className="group-title">{t("groupSubtitle")}</div>
        <div className="grid3">
          <div className="field"><label>{t("font")}</label>
            <Select
              value={fontManual ? "__manual__" : subFont}
              onChange={(v) => {
                if (v === "__manual__") { setFontManual(true); return; }
                setFontManual(false); setSubFont(v);
              }}
              options={[
                ...fonts.map((f) => ({ value: f.name, label: f.name })),
                { value: "__manual__", label: t("fontOther") },
              ]} />
            {fontManual && (
              <>
                <input style={{ marginTop: 6 }} value={subFont} spellCheck={false}
                  placeholder={t("fontPlaceholder")} onChange={(e) => setSubFont(e.target.value)} />
                <div className="meta">
                  {fontChecking ? t("fontChecking")
                    : !fontCheck ? t("fontHint")
                    : fontCheck.valid ? <span className="ok">{t("fontFound", { family: fontCheck.family, source: fontCheck.source })}</span>
                    : <span className="warn">⚠ {fontCheck.error}</span>}
                </div>
              </>
            )}</div>
          <div className="field"><label>{t("color")}</label>
            <Select value={subColor} onChange={setSubColor} options={[
              { value: "white", label: t("colorWhite") },
              { value: "yellow", label: t("colorYellow") },
            ]} /></div>
          <div className="field"><label>{t("size")}</label>
            <Stepper value={subSize} onChange={setSubSize} min={40} max={140} step={2} /></div>

          <div className="field"><label title={t("subStyleTip")}>{t("subStyle")} <Info className="ico hint" aria-hidden="true" /></label>
            <Select value={subMode} onChange={setSubMode} options={[
              { value: "normal", label: t("subNormal") },
              { value: "karaoke", label: t("subKaraoke") },
              { value: "word", label: t("subWord") },
            ]} /></div>
          <div className="field"><label title={t("subSpeedTip")}>{t("subSpeed")} <Info className="ico hint" aria-hidden="true" /></label>
            <Select value={subSpeed} onChange={setSubSpeed} options={[
              { value: "slow", label: t("speedSlow") },
              { value: "normal", label: t("speedNormal") },
              { value: "dense", label: t("speedDense") },
            ]} /></div>
          <div className="field"><label>{t("outline")}</label>
            <Stepper value={subOutline} onChange={setSubOutline} min={0} max={12} /></div>

          {/* Warna sorot hanya berarti pada gaya yang menyorot. Kotaknya TETAP
              dirender (dinonaktifkan) supaya kolom di bawahnya tidak melompat
              tiap gaya diganti. */}
          <div className="field"><label>{t("highlightColor")}</label>
            <Select value={subHighlight} onChange={setSubHighlight} disabled={subMode === "normal"} options={[
              { value: "yellow", label: t("colorYellow") }, { value: "white", label: t("colorWhite") },
              { value: "green", label: t("colorGreen") }, { value: "cyan", label: t("colorCyan") },
            ]} /></div>
          {/* Centang duduk di dasar selnya supaya sejajar dengan kotak isian di
              kiri-kanannya, bukan melayang di ketinggian labelnya. */}
          <div className="field field-check">
            <label className="chk"><input type="checkbox" checked={subBox}
              onChange={(e) => setSubBox(e.target.checked)} /> {t("boxBackground")}</label>
          </div>
        </div>
        </div>

        {/* Penempatan: di mana subtitle berdiri, dan batas mana yang tidak boleh
            ditabrak. Pembatas platform ada DI SINI, bukan di kelompok bingkai —
            ia mengatur ke mana subtitle boleh ditaruh, bukan bagaimana video
            dipasang. */}
        <div className="group">
          <div className="group-title">{t("groupPlacement")}</div>
          <div className="grid3">
            <div className="field"><label>{t("position")}</label>
              <div className="position-value">
                <span>x {subX} · y {subY}</span>
                <button className="ghost tiny" title={t("resetCentre")} aria-label={t("resetCentre")}
                  onClick={() => { setSubX(CENTER_X); setSubY(CENTER_Y); }}>
                  <Crosshair className="ico" aria-hidden="true" />
                </button>
              </div></div>
            <div className="field"><label title={t("platformGuideTip")}>{t("platformGuide")} <Info className="ico hint" aria-hidden="true" /></label>
              <Select value={platform} onChange={setPlatform} options={[
                ...Object.keys(PLATFORMS).map((k) => ({
                  value: k, label: k === "generic" ? t("platformGeneric") : PLATFORMS[k].label,
                })),
                { value: "off", label: t("platformNone") },
              ]} /></div>
            <div className="field field-check">
              <button className="ghost" disabled={!zone} onClick={onPlaceSafe}>{t("placeSafe")}</button>
            </div>
          </div>
          {inUnsafe && <div className="warn">⚠ {t("unsafeWarning")}</div>}
        </div>

        {children}

        {/* Kendali pratinjau tinggal DI SINI, bukan di bawah bingkainya.
            Alasannya tinggi, bukan kerapian: kolom pratinjau dan kotak log
            berbagi satu jatah tinggi, jadi tiap piksel yang dipakai bilah ini
            di bawah bingkai diambil langsung dari kotak log. Kolom setelan
            punya ruang sisa di dasarnya; bilah ini mengisinya. */}
        {/* Kendali preview ditaruh DI BAWAH gambar: tombolnya mengubah gambar
            itu, jadi urutan bacanya lebih masuk akal daripada di atas. */}
        <div className="preview-actions">
          {!previewOn ? (
            <button className="ghost" disabled={!path || previewBusy} onClick={() => loadPreview()}>
              {previewBusy ? t("loadingPreview") : <><Eye className="ico" aria-hidden="true" /> {t("loadPreview")}</>}
            </button>
          ) : (
            <>
              <button className="ghost tiny icon-only" disabled={previewBusy}
                title={t("reloadPreview")} aria-label={t("reloadPreview")} onClick={() => loadPreview()}>
                <RotateCw className="ico" aria-hidden="true" />
              </button>
              <button className="ghost tiny icon-only" onClick={resetPreview}
                title={t("resetPreview")} aria-label={t("resetPreview")}>
                <X className="ico" aria-hidden="true" />
              </button>
              <label className="chk">{t("grid")}
                <Select value={String(grid)} onChange={(v) => setGrid(Number(v))}
                  options={GRIDS.map((g) => ({ value: String(g), label: g === 0 ? t("gridOff") : String(g) }))} />
              </label>
              <label className="chk"><input type="checkbox" checked={alwaysGuides}
                onChange={(e) => setAlwaysGuides(e.target.checked)} /> {t("guidesAlways")}</label>
            </>
          )}
        </div>

        {/* Penggeser waktu SATU baris: dulu label di atas + penggeser di bawah
            memakan 67 px, dan tiap piksel di kolom ini diambil dari bingkai
            pratinjaunya sendiri. Angkanya pindah ke ujung kanan barisnya. */}
        {previewOn && (
          <div className="frame-time" title={t("previewTime", { t: previewTime.toFixed(1) })}>
            <input type="range" min={0} max={Math.max(1, Math.floor(duration))} step={1}
              aria-label={t("previewTime", { t: previewTime.toFixed(1) })}
              value={previewTime} onChange={(e) => setPreviewTime(Number(e.target.value))} />
            <span className="meta">{previewTime.toFixed(1)}s</span>
          </div>
        )}
      </div>
    </div>
  );
}

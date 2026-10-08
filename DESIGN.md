# Design.md — Perbaikan UI/UX Clipper

> Dokumen desain untuk Clipper (Next.js, `http://127.0.0.1:8787`).
> Tujuan: tampilan lebih nyaman dilihat, lebih profesional, dan lebih mudah dipakai —
> **tanpa mengubah alur kerja yang sudah ada**.
> Disusun: 9 Oktober 2026 · Basis audit: build yang sedang jalan di local.

---

## 1. Ringkasan

Clipper secara fungsional sudah matang: 6 modul (Video clips, News cards, Article writer,
Captions, Watermark, Output history), preview 9:16 real-time, process log, dan history hasil.
Masalahnya bukan fitur — tapi **presentasi**: tipografi terlalu kecil, hierarki visual datar,
token desain tidak bersistem, dan aksi utama (Start processing) justru paling tidak menonjol.

Tiga perubahan dengan dampak terbesar, berurutan:

1. **Naikkan skala tipografi + perbaiki hierarki** (11–12px → 13–14px, tambah judul halaman).
2. **Rapikan token** warna / radius / shadow / spacing jadi satu sistem.
3. **Pindahkan aksi utama ke header sticky** dan beri status job yang selalu terlihat.

Sisanya adalah pemolesan per komponen.

---

## 2. Temuan audit

### 2.1 Tipografi — masalah paling terasa

Distribusi ukuran font pada halaman Video clips:

| Ukuran | Jumlah elemen teks | Dipakai untuk                                   |
| ------ | ------------------ | ----------------------------------------------- |
| 11px   | 62                 | label field, label rail, caption, section title |
| 12px   | 66                 | nilai kontrol, teks log, chip                   |
| 13px   | 3                  | —                                               |
| 14px   | 24                 | tombol                                          |
| 16px   | 11                 | —                                               |
| 18px   | 19                 | —                                               |

- `body` di-set 16px tapi hampir tidak pernah dipakai; **UI sesungguhnya berjalan di 11–12px**.
  Pada layar 1440px+ ini kecil sekali, apalagi untuk sesi kerja panjang.
- **Tidak ada satupun `<h1>`–`<h6>` di seluruh aplikasi.** Semua judul adalah `div.group-title`.
  Akibatnya: tidak ada struktur dokumen, screen reader tidak punya outline, dan secara visual
  semua judul terlihat "setara" — tidak ada yang memimpin.
- Section label (`SUBTITLE`, `PLACEMENT`, `FRAME`, `ENGINE`, `QUALITY`, `OUTPUT`) uppercase 11px
  warna muted → kontras rendah dan terbaca sebagai noise, bukan sebagai pembatas blok.

### 2.2 Warna & kontras

Palet saat ini (`:root`):

```
light: bg #f5f5f5 · panel #fff · panel2 #fafafa · border #e2e2e2
       text #27292a · muted #6b6e70 · accent #0066d6
dark:  bg #0f1115 · panel #171a21 · panel2 #1e222b · border #2a2f3a
       text #e6e9ef · muted #9aa4b2 · accent #4f8cff
```

- Light: `border #e2e2e2` di atas `panel #fff` ≈ **1.3:1** — panel nyaris tidak punya tepi,
  semuanya mengambang. Perlu dua tingkat border (subtle + strong).
- Dark: `panel #171a21` vs `bg #0f1115` ≈ **1.09:1** — pemisahan panel nyaris hilang.
- `accent #0066d6` di atas putih = 5.4:1 → aman. Tapi tombol **Start processing** tampil
  biru pucat (state disabled) sehingga CTA utama kalah menonjol dari tombol **Delete** merah
  solid di Output history. Hierarki aksi terbalik.
- Tidak ada token untuk: focus ring, overlay, surface-inverse (log), selected state, hover.

### 2.3 Sistem yang tidak konsisten

| Aspek           | Kondisi sekarang                               | Target              |
| --------------- | ---------------------------------------------- | ------------------- |
| `border-radius` | 12 nilai: 2, 4, 5, 6, 7, 8, 10, 12, 999px, 50% | 3 nilai             |
| `box-shadow`    | 9 variasi ad-hoc                               | 3 level elevation   |
| `transition`    | .12s / .15s / .3s campur                       | 2 durasi + 1 easing |
| breakpoint      | 700, 820, 900, 980, 1100px                     | 3 breakpoint        |
| spacing         | hardcode campur                                | skala 4px           |

### 2.4 Layout & arsitektur informasi

- **Tidak ada judul halaman.** Saat pindah modul, satu-satunya penanda adalah highlight tipis
  di rail. User kehilangan konteks "saya sedang di mana".
- **Start processing terisolasi** di panel sendiri pojok kanan bawah, jauh dari preview dan
  settings. Saat halaman di-scroll, CTA dan progress bar bisa keluar dari viewport.
- **Process log selalu mengambil ruang besar** meskipun isinya `"Nothing yet. The log fills up
once a job starts."` — di Article writer / Captions / Watermark panel kosong ini memakan
  sepertiga kolom kiri.
- **Kepadatan kontrol tinggi**: Video clips menampilkan ~20 kontrol sekaligus (Font, Colour,
  Size, Style, Pacing, Outline, Highlight, Box, Grid, Position, Platform, Safe area, Fit,
  Background, Zoom, Watermark…). Tidak ada pemisahan antara "yang sering diubah" dan
  "setelan lanjutan".
- **Empty state seadanya**: teks satu baris rata kiri (`"Add up to 5 source articles on the
right, then press Start."`, `"Pick an article to see the card"`) tanpa ikon maupun CTA.
- **News cards**: input "Paste an article link" (lebar penuh) dan "Search news" (lebar
  setengah, baris berikutnya) tidak sejajar; dua tombol biru berdampingan (`Fetch`, `Search`)
  tanpa beda hierarki.
- **Output history**: tombol `Delete` merah solid adalah elemen paling mencolok di toolbar,
  sementara `Download` abu-abu. Destructive action tidak boleh jadi yang paling menonjol.

### 2.5 Aksesibilitas

Yang sudah baik:

- Semua input punya label terasosiasi (`inputsNoLabel: 0`). Bagus.
- Focus ring sudah ada untuk `input`, `select`, `textarea`, `.sel`, `.stepper`, `.news-item`.

Yang perlu diperbaiki:

- **`<button>` tidak punya `:focus-visible`** — 30 tombol per halaman tidak bisa dinavigasi
  dengan keyboard secara terlihat.
- Tidak ada heading → tidak ada landmark/outline.
- Icon-only button di `rail-tools` (bawah kiri) tanpa `aria-label`/tooltip terlihat.
- Process log tidak punya `aria-live` → progres tidak diumumkan.
- Target sentuh beberapa icon button < 32px.

---

## 3. Prinsip desain

1. **Calm by default.** Panel tenang, garis tipis, warna hanya untuk hal yang perlu perhatian
   (status, aksi utama, error). Bukan dekorasi.
2. **Satu aksi utama per layar.** Start processing. Semua yang lain sekunder atau tersier.
3. **Progressive disclosure.** Setelan yang jarang diubah masuk ke accordion "Advanced".
4. **Status selalu terlihat.** Apa yang sedang berjalan, sudah berapa persen, kapan selesai —
   tanpa perlu scroll.
5. **Preview adalah bintangnya.** Area preview 9:16 dapat ruang paling lega dan paling tenang.
6. **Konsisten > kreatif.** Tiga radius, tiga elevasi, satu skala spasi. Tanpa pengecualian.

---

## 4. Design tokens

Ganti blok `:root` di `globals.css` dengan sistem di bawah. Nama token lama (`--bg`, `--panel`,
`--border`, `--text`, `--muted`, `--accent`) **tetap dipertahankan sebagai alias** supaya CSS
yang sudah ada tidak perlu diubah sekaligus.

### 4.1 Warna

```css
:root {
  /* ---- surfaces ---- */
  --color-bg: #f4f5f7; /* kanvas */
  --color-surface: #ffffff; /* panel */
  --color-surface-2: #f9fafb; /* panel bertingkat / header panel */
  --color-surface-sunken: #eef0f3; /* track, well, area preview */
  --color-surface-inv: #0f1115; /* log console */

  /* ---- borders ---- */
  --color-border: #e4e7ec; /* hairline, default */
  --color-border-strong: #cfd4dc; /* tepi panel, divider penting */

  /* ---- text ---- */
  --color-text: #101828; /* primary */
  --color-text-2: #475467; /* secondary / nilai kontrol */
  --color-text-3: #667085; /* tersier / hint, min 12px */

  /* ---- brand & semantic ---- */
  --color-accent: #0b63d6;
  --color-accent-hover: #0a57bb;
  --color-accent-soft: #e8f1fe; /* bg untuk state aktif/selected */
  --color-on-accent: #ffffff;

  --color-good: #12a673;
  --color-good-soft: #e6f6ef;
  --color-warn: #d97706;
  --color-warn-soft: #fef4e6;
  --color-bad: #d92d20;
  --color-bad-soft: #fdecea;

  --color-focus: #0b63d6;
  --color-overlay: rgb(16 24 40 / 48%);

  /* ---- alias kompatibilitas ---- */
  --bg: var(--color-bg);
  --panel: var(--color-surface);
  --panel2: var(--color-surface-2);
  --border: var(--color-border);
  --text: var(--color-text);
  --muted: var(--color-text-3);
  --accent: var(--color-accent);
  --on-accent: var(--color-on-accent);
}

[data-theme="dark"] {
  --color-bg: #0b0d12;
  --color-surface: #14171e;
  --color-surface-2: #1a1e27;
  --color-surface-sunken: #0f1218;
  --color-surface-inv: #06080b;

  --color-border: #252a34;
  --color-border-strong: #353c49;

  --color-text: #e8ebf0;
  --color-text-2: #a9b2c0;
  --color-text-3: #8591a1;

  --color-accent: #5b9bff;
  --color-accent-hover: #76acff;
  --color-accent-soft: #16233a;
  --color-on-accent: #06101f;

  --color-good: #34d399;
  --color-good-soft: #10281f;
  --color-warn: #fbbf24;
  --color-warn-soft: #2a2112;
  --color-bad: #f87171;
  --color-bad-soft: #2b1515;

  --color-focus: #76acff;
  --color-overlay: rgb(0 0 0 / 64%);
}
```

Catatan kontras (semua ≥ WCAG AA):

| Pasangan                | Light            | Dark   |
| ----------------------- | ---------------- | ------ |
| text / surface          | 17.8:1           | 15.0:1 |
| text-2 / surface        | 7.7:1            | 8.4:1  |
| text-3 / surface        | 5.0:1            | 5.6:1  |
| accent / surface        | 5.6:1            | 6.5:1  |
| border-strong / surface | 1.5:1 (non-text) | 1.6:1  |

### 4.2 Tipografi

```css
:root {
  --font-ui: "Inter", system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  --font-mono: "JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, monospace;

  --fs-caption: 12px;
  --lh-caption: 16px; /* hint, meta, timestamp */
  --fs-label: 13px;
  --lh-label: 18px; /* label field, nilai kontrol */
  --fs-body: 14px;
  --lh-body: 20px; /* teks default, tombol */
  --fs-title: 16px;
  --lh-title: 22px; /* judul panel */
  --fs-h2: 20px;
  --lh-h2: 28px; /* judul halaman */
  --fs-display: 28px;
  --lh-display: 36px; /* skor besar di history */

  --fw-regular: 400;
  --fw-medium: 500;
  --fw-semibold: 600;
}

body {
  font-family: var(--font-ui);
  font-size: var(--fs-body);
  line-height: var(--lh-body);
}

/* angka sejajar di stepper, skor, durasi, timestamp */
.stepper input,
.score,
.dur,
.ts,
.logbox {
  font-variant-numeric: tabular-nums;
}
```

Pemetaan naik dari kondisi sekarang:

| Elemen                             | Sekarang             | Jadi                                      |
| ---------------------------------- | -------------------- | ----------------------------------------- |
| Label field (`Font`, `Colour`)     | 11px regular muted   | **13px / 500 / text-2**                   |
| Nilai kontrol (`Montserrat`, `72`) | 12px                 | **13px / 500 / text**                     |
| Section label (`SUBTITLE`)         | 11px uppercase muted | **12px / 600 / 0.06em tracking / text-3** |
| Label rail                         | 11px                 | **12px / 500**                            |
| Teks tombol                        | 14px                 | 14px / 500 (tetap)                        |
| Judul panel                        | —                    | **16px / 600**                            |
| Judul halaman                      | tidak ada            | **20px / 600 `<h1>`**                     |
| Log                                | 12px mono            | 12.5px mono, line-height 1.55             |

Tambahkan Inter lewat `next/font` (sudah ada `@font-face` inline di `.screen`, jadi polanya
sudah dipakai):

```ts
// app/layout.tsx
import { Inter, JetBrains_Mono } from "next/font/google";
const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
});
const mono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jbmono",
  display: "swap",
});
```

### 4.3 Spasi, radius, elevasi, motion

```css
:root {
  --sp-1: 4px;
  --sp-2: 8px;
  --sp-3: 12px;
  --sp-4: 16px;
  --sp-5: 24px;
  --sp-6: 32px;
  --sp-7: 48px;

  --r-sm: 6px; /* input, select, tombol kecil, chip */
  --r-md: 10px; /* panel, card, tombol besar */
  --r-full: 999px; /* pill, badge, avatar */

  --e-1: 0 1px 2px rgb(16 24 40 / 5%); /* panel diam */
  --e-2:
    0 4px 12px rgb(16 24 40 / 8%), 0 1px 2px rgb(16 24 40 / 4%); /* hover card, dropdown */
  --e-3: 0 16px 40px rgb(16 24 40 / 14%); /* modal, popover */

  --dur-fast: 120ms;
  --dur-base: 180ms;
  --ease: cubic-bezier(0.2, 0, 0, 1);

  --ctl-h: 34px; /* dari 32 — lebih nyaman, masih padat */
  --ctl-h-sm: 28px;
  --rail-w: 84px; /* dari 76 */
}

[data-theme="dark"] {
  --e-1: 0 1px 2px rgb(0 0 0 / 40%);
  --e-2: 0 4px 12px rgb(0 0 0 / 48%);
  --e-3: 0 16px 40px rgb(0 0 0 / 60%);
}

@media (prefers-reduced-motion: reduce) {
  * {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

**Aturan radius (tanpa pengecualian):** kontrol & chip → `--r-sm`; panel, card, preview,
log box → `--r-md`; badge/pill/toggle → `--r-full`. Hapus nilai 2/4/5/7/8/12px yang tersisa.

### 4.4 Focus ring global

```css
:where(
  a,
  button,
  [role="button"],
  input,
  select,
  textarea,
  [tabindex]
):focus-visible {
  outline: 2px solid var(--color-focus);
  outline-offset: 2px;
  border-radius: inherit;
}
:where(a, button):focus:not(:focus-visible) {
  outline: none;
}
```

---

## 5. Komponen

### 5.1 Rail (navigasi kiri)

Sekarang: brand teks kecil, 6 item ikon+label 11px, active = tint biru tipis.

```
┌────────────┐
│  ◆ Clipper │   brand: mark 20px + wordmark 14px/600
├────────────┤
│ ┃ ✂ Video  │   active: pill surface-accent-soft, radius md,
│   clips    │           + bar 3px accent di tepi kiri rail
│   ▤ News   │   idle:   ikon text-3, label text-2
│   ✎ Article│   hover:  bg surface-2
│   ▭ Captions│
│   ⬓ Water  │   label 12px/500, ikon 20px stroke 1.75
│   ⟲ History│   padding vertikal 10px, gap 4px antar item
├────────────┤
│  ◑  ⚙  ⓘ   │   tools: 32×32 target, tooltip on hover + aria-label
└────────────┘
```

```css
.rail {
  width: var(--rail-w);
  background: var(--color-surface);
  border-right: 1px solid var(--color-border);
}
.rail-item {
  display: grid;
  gap: var(--sp-1);
  justify-items: center;
  padding: 10px 6px;
  margin: 0 8px;
  border-radius: var(--r-md);
  color: var(--color-text-2);
  font-size: var(--fs-caption);
  font-weight: 500;
  transition:
    background var(--dur-fast) var(--ease),
    color var(--dur-fast) var(--ease);
}
.rail-item:hover {
  background: var(--color-surface-2);
  color: var(--color-text);
}
.rail-item.active {
  background: var(--color-accent-soft);
  color: var(--color-accent);
  font-weight: 600;
}
.rail-item.active::before {
  content: "";
  position: absolute;
  left: 0;
  width: 3px;
  height: 24px;
  border-radius: 0 3px 3px 0;
  background: var(--color-accent);
}
```

Tambahkan `aria-current="page"` pada item aktif.

### 5.2 Page header (BARU — prioritas tinggi)

Setiap screen mendapat header sticky yang memuat judul, status job, dan aksi utama.
Ini menyelesaikan tiga masalah sekaligus: tidak ada judul halaman, CTA terisolasi,
dan status job tidak terlihat.

```
┌──────────────────────────────────────────────────────────────────────────┐
│ Video clips                                   ● Idle    [ Start ▸ ]  ⋯   │
│ Potong video panjang jadi klip vertikal bersubtitle                      │
└──────────────────────────────────────────────────────────────────────────┘
```

Saat job berjalan, header berubah jadi status bar tanpa pindah posisi:

```
┌──────────────────────────────────────────────────────────────────────────┐
│ Video clips                    ◉ Rendering clip 05/08 · 71%   [ Stop ]   │
│ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░  5:12 berjalan · ~2:30 lagi      │
└──────────────────────────────────────────────────────────────────────────┘
```

```css
.page-header {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  align-items: flex-start;
  gap: var(--sp-4);
  padding: var(--sp-4) var(--sp-5);
  background: color-mix(in srgb, var(--color-bg) 85%, transparent);
  backdrop-filter: blur(8px);
  border-bottom: 1px solid var(--color-border);
}
.page-header h1 {
  font-size: var(--fs-h2);
  font-weight: var(--fw-semibold);
  margin: 0;
}
.page-header p {
  font-size: var(--fs-caption);
  color: var(--color-text-3);
  margin: 2px 0 0;
}
```

`.start-panel` yang lama bisa dihapus, atau disisakan sebagai duplikat di bawah settings
untuk layar pendek.

### 5.3 Panel & group

```css
.panel {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--r-md);
  box-shadow: var(--e-1);
  padding: var(--sp-5);
}

.group + .group {
  margin-top: var(--sp-5);
  padding-top: var(--sp-5);
  border-top: 1px solid var(--color-border);
}

.group-title {
  font-size: var(--fs-caption);
  font-weight: var(--fw-semibold);
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--color-text-3);
  margin-bottom: var(--sp-3);
  display: flex;
  align-items: center;
  gap: var(--sp-2);
}
```

Group title mendapat ikon 14px di kiri (opsional) supaya blok lebih cepat dikenali:
Subtitle → `Type`, Placement → `Move`, Frame → `Crop`, Engine → `Cpu`, Quality → `Gauge`,
Output → `Download`.

### 5.4 Field & kontrol

```css
.field {
  display: grid;
  gap: 6px;
}
.field > label {
  font-size: var(--fs-label);
  font-weight: var(--fw-medium);
  color: var(--color-text-2);
}
.field .hint {
  font-size: var(--fs-caption);
  color: var(--color-text-3);
}

input,
select,
textarea,
.sel {
  height: var(--ctl-h);
  padding: 0 10px;
  font-size: var(--fs-label);
  color: var(--color-text);
  background: var(--color-surface);
  border: 1px solid var(--color-border-strong);
  border-radius: var(--r-sm);
  transition:
    border-color var(--dur-fast) var(--ease),
    box-shadow var(--dur-fast) var(--ease);
}
input:hover,
select:hover,
.sel:hover {
  border-color: var(--color-text-3);
}
input:focus-visible,
select:focus-visible,
.sel:focus-visible {
  border-color: var(--color-accent);
  box-shadow: 0 0 0 3px var(--color-accent-soft);
  outline: none;
}
input:disabled,
select:disabled {
  background: var(--color-surface-sunken);
  color: var(--color-text-3);
  cursor: not-allowed;
}
```

Catatan khusus:

- **`Highlight` dan `Background`** saat ini tampil abu (disabled) tanpa penjelasan. Tambahkan
  hint di bawah field: _"Aktif saat Style = Word by word"_ / _"Aktif saat Fit = Fit inside"_.
  Disabled tanpa alasan adalah dead end.
- **Ikon `ⓘ`** (pada Style, Pacing, Platform, FPS, Clip length, Max clips, Save as) → jadikan
  tooltip 240px dengan `--e-3`, `--r-sm`, delay 300ms, dan tetap bisa diakses via keyboard
  (`<button aria-describedby>`).
- **Stepper** sudah bagus. Rapikan: tombol −/+ 28×34, border dalam hairline, angka
  `tabular-nums` rata tengah, dukung ↑/↓ dan Shift+↑/↓ (×10).

### 5.5 Tombol — hierarki yang jelas

| Level         | Dipakai untuk                        | Style                                                                       |
| ------------- | ------------------------------------ | --------------------------------------------------------------------------- |
| **Primary**   | Start processing, Save card, Add     | bg accent, teks on-accent, `--r-sm`, h 36px, weight 600                     |
| **Secondary** | Open…, Load preview, Safe area, Test | bg surface, border border-strong, teks text                                 |
| **Ghost**     | Cancel, Clear, Select all            | transparan, teks text-2, hover bg surface-2                                 |
| **Danger**    | Delete                               | **ghost + teks `--color-bad`**; jadi solid merah hanya di dialog konfirmasi |

```css
.btn-primary {
  background: var(--color-accent);
  color: var(--color-on-accent);
  font-weight: var(--fw-semibold);
}
.btn-primary:hover:not(:disabled) {
  background: var(--color-accent-hover);
}
.btn-primary:disabled {
  background: var(--color-surface-sunken);
  color: var(--color-text-3);
  box-shadow: none;
}

.btn-danger {
  background: transparent;
  color: var(--color-bad);
  border: 1px solid var(--color-bad);
}
.btn-danger:hover {
  background: var(--color-bad-soft);
}
```

Perbaikan konkret:

- **Start processing** harus solid accent saat enabled. Kalau disabled, tampilkan alasan di
  sampingnya: _"Pilih video dulu"_. Jangan tombol pucat tanpa keterangan.
- **Delete di Output history** turun jadi danger-ghost; `Download` naik jadi primary saat ada
  item terpilih. Toolbar: `3 dipilih · [Download] [Hapus] [Batal]`.
- **Fetch vs Search** di News cards: `Fetch` primary (aksi utama), `Search` secondary.

### 5.6 Preview 9:16

```
┌─────────────────────┐
│ 9:16 · 1080×1920    │  ← toolbar: ratio, safe-area toggle, zoom, reload
├─────────────────────┤
│  ╎ COVERED BY UI  ╎ │  overlay guide: text 10px/600 tracking .08em
│  ╎               ╎ │  warna: rgb(255 255 255 / 55%)
│  ╎  [ preview ]   ╎ │  garis safe-area: dashed 1px rgb(255 255 255 / 28%)
│  ╎               ╎ │
│  ╎ Begini tampilan╎ │  ← contoh subtitle, render dengan font & ukuran asli
│  ╎  subtitle-mu   ╎ │
│  ╎ CAPTION & NAME ╎ │
└─────────────────────┘
         ◯━━━━━━━━●     scrubber + timecode 00:42 / 39:51
```

- Latar area preview: `--color-surface-sunken` di light, `#000` di dalam frame video.
  Hatch diagonal sekarang terlalu ramai — ganti jadi solid + border-strong.
- Empty state: ikon film 32px + _"Belum ada preview"_ + tombol **Open a video** (bukan hanya
  teks "No preview").
- Label overlay rotated (`ACTION BUTTONS`) dipertahankan — konsepnya bagus — tapi naikkan
  kontras ke 55% dan beri `text-shadow: 0 1px 2px rgb(0 0 0 / 60%)`.
- Tambah toggle **Show guides** supaya overlay bisa disembunyikan saat mengevaluasi hasil.

### 5.7 Process log → drawer

Masalah: panel hitam besar di tengah UI terang, dan tetap memakan ruang saat kosong.

Jadikan **drawer bawah yang bisa dilipat**, lebar penuh kolom utama:

```
┌──────────────────────────────────────────────────────────────┐
│ ⌃ Process log            ✓ Selesai · 8 klip · 7:44   [Salin] │  ← tinggi 40px saat tertutup
└──────────────────────────────────────────────────────────────┘
```

Saat terbuka: tinggi default 240px, resizable, auto-scroll ke bawah dengan tombol
"Jump to latest" saat user scroll ke atas.

```css
.logbox {
  background: var(--color-surface-inv);
  color: #cfd6e4;
  font-family: var(--font-mono);
  font-size: 12.5px;
  line-height: 1.55;
  border-radius: var(--r-md);
  padding: var(--sp-3) var(--sp-4);
}
.logbox .ts {
  color: #6b7688;
} /* timestamp redup */
.logbox .ok {
  color: var(--color-good);
}
.logbox .warn {
  color: var(--color-warn);
}
.logbox .err {
  color: var(--color-bad);
  background: rgb(217 45 32 / 12%);
  display: block;
  margin: 0 -16px;
  padding: 2px 16px;
}
.logbox .stage {
  color: #8ab4ff;
} /* nama tahap */
```

Tambahkan `role="log" aria-live="polite" aria-atomic="false"` pada container.

Blok "Job summary" (tabel ASCII dengan bar) **dipertahankan** — itu salah satu bagian terbaik
dari aplikasi ini. Cukup pastikan lebarnya tidak terpotong dan bar-nya diberi warna accent.

### 5.8 Daftar berita (News cards & Article writer)

- Satukan `Paste an article link` dan `Search news` jadi satu baris segaris dengan tombol
  menempel (input group), bukan dua baris dengan lebar berbeda.
- Item berita: thumbnail 56×56 `--r-sm`, judul 13px/500 maks 2 baris (`-webkit-line-clamp: 2`),
  meta 12px text-3 (`ANTARA · 9 Okt 2026`), tombol copy jadi icon-button yang muncul saat hover
  (sekarang chip "⚭ copy" selalu tampil → ramai).
- Hover: `background: var(--color-surface-2)`; selected: `background: var(--color-accent-soft)`
  - border kiri 2px accent.
- **Skeleton loading** menggantikan teks `"Loading news…"`: 6 baris placeholder beranimasi.
- Scroll list: `scrollbar-gutter: stable`, dan beri fade mask 24px di atas/bawah.

### 5.9 Kartu hasil (Output history)

Yang sudah bagus: grid klip, skor besar, timeline, tombol Download + .txt.

Perbaikan:

- Skor: `--fs-display` 28px/600 dengan warna semantik — ≥75 good, 60–74 warn, <60 text-3.
  `/100` jadi 12px text-3.
- Header grup job: status dot + `08 Okt 2026, 05:31` 14px/600 + `8 klip · selesai · 7:44`
  12px text-3. Dot: good/warn/bad sesuai hasil.
- Judul klip 13px/500 clamp 2 baris; baris meta (`hook 78 · emo 78 · clear 78`) jadi chip kecil
  `--r-full` 11px, bukan teks mentah yang terpotong `…`.
- Checkbox select muncul di pojok kiri atas thumbnail saat hover / saat mode select aktif.
- Tab `Results (4)` / `News cards (2)` jadi segmented control `--r-full`, bukan tab polos.

### 5.10 Empty, loading, error

Pola seragam untuk semua modul:

```
          ◻ (ikon 32px, text-3)
      Belum ada artikel
  Tempel maksimal 5 link di panel kanan,
        lalu tekan Start.
        [ Tambah link ]          ← CTA opsional
```

```css
.empty {
  display: grid;
  justify-items: center;
  gap: var(--sp-2);
  padding: var(--sp-7) var(--sp-5);
  text-align: center;
}
.empty h3 {
  font-size: var(--fs-body);
  font-weight: 600;
  color: var(--color-text-2);
  margin: 0;
}
.empty p {
  font-size: var(--fs-caption);
  color: var(--color-text-3);
  max-width: 38ch;
  margin: 0;
}
```

Error: banner inline di atas panel terkait — `--color-bad-soft` bg, border `--color-bad`,
ikon, pesan, dan satu tombol aksi (`Coba lagi`). Jangan hanya menulis error ke log.

Toast untuk hasil aksi singkat (tersalin, tersimpan, job selesai) — pojok kanan bawah,
auto-dismiss 4s, `--e-3`.

---

## 6. Layout per screen

Grid utama (tetap 2 kolom seperti sekarang, dirapikan):

```
rail 84px │ main 1fr (min 560px) │ side 380px
          │ gap 20px · padding 20px · max-width 1680px
```

### Video clips

```
┌ Page header ────────────────────────────────────────────────┬──────────────┐
│ Video clips                              ● Idle   [Start ▸] │              │
├─────────────────────────────────────────────────────────────┤  ENGINE      │
│ ┌ Source ─────────────────────────────────────────────────┐ │  Whisper     │
│ │ [ Upload a file              ] [ Open… ]                │ │  Engine      │
│ └─────────────────────────────────────────────────────────┘ │  Model +Test │
│ ┌ Preview ───────────┐ ┌ Settings ────────────────────────┐ │  ☑ Correct   │
│ │                    │ │ SUBTITLE                         │ │  Names&terms │
│ │   [ 9:16 frame ]   │ │  Font  Colour  Size              │ │              │
│ │                    │ │  Style Pacing Outline            │ │  QUALITY     │
│ │                    │ │ PLACEMENT                        │ │  Res Qual FPS│
│ │                    │ │  Position  Platform  Safe area   │ │              │
│ │                    │ │ FRAME                            │ │  OUTPUT      │
│ │                    │ │  Fit  Background  Zoom           │ │  Len Max Save│
│ │  ◯━━━━━━●  00:42   │ │ ⌄ Advanced (Highlight, Box, Grid)│ │              │
│ └────────────────────┘ └──────────────────────────────────┘ │              │
├─────────────────────────────────────────────────────────────┴──────────────┤
│ ⌃ Process log                       ✓ Selesai · 8 klip · 7:44      [Salin] │
└────────────────────────────────────────────────────────────────────────────┘
```

Perubahan: `Highlight`, `Box`, `Grid`, `Outline`, `Zoom` masuk accordion **Advanced**
(tertutup secara default). Layar utama turun dari ~20 jadi ~11 kontrol terlihat.

### Article writer / Captions / Watermark

Pakai kerangka yang sama: header → main (input/preview) → side (engine/settings) →
log drawer. Saat log kosong, drawer hanya setinggi 40px — tidak ada lagi kotak hitam besar
yang kosong.

### Output history

Header: `Output history` + filter (`Semua / Selesai / Gagal`) + search + segmented tab.
Toolbar selection muncul sebagai bar sticky hanya saat ada item terpilih.

---

## 7. Responsive

Ganti 5 breakpoint acak (700/820/900/980/1100) dengan 3:

```css
/* ≥1280px — layout penuh: rail + main + side */
@media (max-width: 1279px) {
  /* side panel turun ke bawah main, grid 3 kolom tetap di dalamnya */
  .screen-body.two {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 1023px) {
  /* preview dan settings menumpuk; preview sticky di atas */
  .sub-layout {
    grid-template-columns: 1fr;
  }
  .sub-preview {
    position: sticky;
    top: 64px;
  }
}
@media (max-width: 767px) {
  /* rail jadi bottom bar ikon-only 56px */
  :root {
    --rail-w: 0px;
  }
  .rail {
    position: fixed;
    inset: auto 0 0 0;
    width: 100%;
    height: 56px;
    flex-direction: row;
    border-right: none;
    border-top: 1px solid var(--color-border);
  }
  .rail-item span {
    display: none;
  }
  .grid3 {
    grid-template-columns: 1fr 1fr;
  }
}
```

Target minimum yang harus tetap jalan: **1280×800** (laptop) tanpa scroll horizontal.
Saat ini `document.scrollWidth` = 1534 pada viewport 1534 — aman, tapi belum diuji di bawah 1100.

---

## 8. Aksesibilitas — checklist

- [ ] `<h1>` di setiap screen (boleh `.sr-only` kalau tidak ingin terlihat), `<h2>` untuk judul
      panel, `<h3>` untuk group title.
- [ ] `:focus-visible` global untuk semua elemen interaktif (termasuk 30 `<button>`).
- [ ] `aria-label` + tooltip untuk semua icon-only button (rail-tools, copy, reload, popover).
- [ ] `role="log" aria-live="polite"` pada process log; `aria-live="polite"` pada teks status
      di header.
- [ ] `<progress>` atau `role="progressbar"` + `aria-valuenow` untuk progress bar.
- [ ] Kontras minimum 4.5:1 untuk semua teks (lihat tabel di §4.1).
- [ ] Target interaktif minimum 32×32px.
- [ ] `aria-current="page"` pada rail item aktif.
- [ ] Hint tekstual untuk setiap field disabled (jangan hanya diredupkan).
- [ ] `prefers-reduced-motion` dihormati.
- [ ] Navigasi keyboard penuh: Tab urut logis, Esc menutup popover, Enter men-submit.

---

## 9. Rencana implementasi

### Fase 0 — Fondasi (±2 jam, dampak terbesar)

Semua di `globals.css`, nol perubahan komponen:

1. Ganti blok `:root` + `[data-theme="dark"]` dengan token di §4.1 (alias lama tetap ada).
2. Tambah token tipografi, spasi, radius, elevasi, motion (§4.2–4.3).
3. Naikkan `--ctl-h` 32 → 34, `--rail-w` 76 → 84.
4. Naikkan ukuran font: `.field > label` 13px, nilai kontrol 13px, `.group-title` 12px/600,
   `.rail-item span` 12px.
5. Pasang focus ring global (§4.4).
6. Normalisasi radius: cari-ganti semua nilai ke `--r-sm` / `--r-md` / `--r-full`.
7. Normalisasi shadow ke `--e-1/2/3`.

> Hasil fase ini saja sudah membuat aplikasi terasa jauh lebih rapi dan profesional,
> tanpa menyentuh satu pun komponen React.

### Fase 1 — Struktur (±1 hari)

8. Komponen `<PageHeader title subtitle status actions>`, dipasang di 6 screen.
9. Pindahkan Start/Cancel + progress ke header; `.start-panel` jadi fallback.
10. Process log jadi drawer yang bisa dilipat + pewarnaan baris (ok/warn/err/stage).
11. Komponen `<EmptyState icon title description action>`, ganti semua teks placeholder.
12. Hierarki tombol: primary / secondary / ghost / danger-ghost (§5.5).

### Fase 2 — Polish (±1 hari)

13. Accordion **Advanced** di Video clips.
14. Toolbar preview + empty state preview + toggle guides.
15. Skeleton loading untuk list berita; input group untuk Paste/Search.
16. Kartu Output history: skor semantik, chip meta, checkbox on-hover, segmented tab.
17. Tooltip komponen untuk semua `ⓘ`.
18. Toast + error banner inline.

### Fase 3 — Aksesibilitas & responsif (±½ hari)

19. Heading hierarchy, aria-label, aria-live, progressbar.
20. Konsolidasi breakpoint jadi 3; uji 1280 / 1024 / 768.
21. Sapu kontras dengan Lighthouse + axe DevTools.

---

## 10. Definition of done

- [ ] Tidak ada teks UI di bawah 12px.
- [ ] Hanya 3 nilai `border-radius` dan 3 level shadow di seluruh CSS.
- [ ] Setiap screen punya `<h1>` dan judul yang terlihat.
- [ ] Aksi utama terlihat tanpa scroll di viewport 1280×800.
- [ ] Status job terbaca dari header di semua screen.
- [ ] Setiap state kosong punya ikon, penjelasan, dan (kalau relevan) CTA.
- [ ] Setiap kontrol disabled punya alasan tertulis.
- [ ] Lighthouse Accessibility ≥ 95.
- [ ] Light dan dark sama-sama lolos kontras AA.
- [ ] Tidak ada scroll horizontal di 1280px.

---

## Lampiran — ringkasan sebelum/sesudah

| Aspek                          | Sebelum                          | Sesudah                      |
| ------------------------------ | -------------------------------- | ---------------------------- |
| Ukuran teks dominan            | 11–12px                          | 13–14px                      |
| Judul halaman                  | tidak ada                        | `<h1>` 20px di header sticky |
| Heading HTML                   | 0                                | h1 per screen, h2 per panel  |
| Aksi utama                     | panel terpisah, biru pucat       | header sticky, solid accent  |
| Status job                     | tersembunyi di log               | selalu terlihat di header    |
| Process log                    | panel hitam besar, selalu tampil | drawer lipat 40px, berwarna  |
| Kontrol terlihat (Video clips) | ~20                              | ~11 + Advanced               |
| `border-radius`                | 12 nilai                         | 3 nilai                      |
| `box-shadow`                   | 9 variasi                        | 3 level                      |
| Breakpoint                     | 5                                | 3                            |
| Destructive action             | merah solid, paling menonjol     | ghost merah, konfirmasi dulu |
| Empty state                    | teks satu baris                  | ikon + penjelasan + CTA      |
| Focus ring                     | sebagian elemen                  | semua elemen interaktif      |

---

## Keputusan pemilik (9 Oktober 2026)

Dicatat saat dokumen ini dipakai sebagai arah (antislop R-37):

- **Header sticky + `<h1>` (§5.2): DIPAKAI.** Aturan "tidak ada `<h1>` / bilah atas" di
  `CLAUDE.md` diganti keputusan ini.
- **Responsif (§7): DIBUANG.** Acuan tetap desktop 900×600 dan 1240×860, jendela tidak
  bergulir (`CLAUDE.md`).
- **Huruf: Inter + JetBrains Mono, pilihan pemilik.** Alasan: angka tabular yang rapi untuk
  stepper, skor, durasi, dan log; dibundel saat build lewat `next/font` (tidak diunduh saat
  aplikasi jalan).
- **Logo: teks "Clipper" saja.** Tanpa simbol ◆.
- **Warna status sebagai TEKS** memakai varian `-text` yang sudah teruji kontrasnya
  (hijau/oranye/merah §4.1 hanya 2,9–4,4:1 di atas putih). Nilai §4.1 dipakai untuk titik,
  bilah, dan latar status.

Dial: ENERGY 1 / RHYTHM 1 / MOTION 1 ("Calm by default", §3).

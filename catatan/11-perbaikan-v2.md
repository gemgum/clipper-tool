# 11 — Perbaikan v2 (flow pengembangan)

Empat catatan dari uji pakai MVP, beserta akar masalah yang sudah diverifikasi
di kode dan di output job `data/job_0002_2026-07-28_02-18-04`.

## Ringkasan temuan

| # | Keluhan | Akar masalah |
|---|---|---|
| 1 | Belum ada pembatas UI tiap sosmed | Fitur belum ada di GUI |
| 2 | Subtitle "tunggal" jadi kuning & tak jalan | `subtitle.go:37` memaksa kuning; `{\k}` hanya menyapu warna, bukan mode 1 kata; timing per kata dibagi rata |
| 3 | Subtitle terlalu cepat pada teks panjang | `subtitle.go:181` membagi halaman proporsional tanpa durasi minimum (ditemukan halaman 0,45 dtk) |
| 4 | Pemotong selalu ~30 dtk (maks 36) | `segment.go:31` memotong begitu durasi ≥ `targetMin`; preset `auto` = min 30 dtk |

Bukti #4 — durasi 15 klip job terakhir: 30, 31, 31, 32, 32, 32, 32, 33, 34, 34,
35, 36, 37, 30, 30 detik. Tidak ada variasi karena selalu berhenti di ambang bawah.

Bukti #2/#3 — potongan `clip_01.ass`:

```
Style: Default,Montserrat,84,&H0000FFFF,...   ← kuning dipaksa
Dialogue: 0:00:04.00,0:00:07.55,...{\k45}Hari-hari {\k45}aku ... (3 baris/3,5 dtk)
Dialogue: 0:00:07.55,0:00:08.00,...{\k44}itu.                     ← tampil 0,45 dtk
```

## Status: SEMUA TAHAP SELESAI & TERUJI (28 Juli 2026)

Bukti uji akhir (`clipper run` 5 menit, model base, preset 60, mode karaoke,
`-save both`):

| Yang diuji | Sebelum | Sesudah |
|---|---|---|
| Durasi klip | 30–37 dtk (semua) | 60 / 59 / 60 dtk (ikut preset) |
| Tampilan subtitle terpendek | 0,45 dtk | 1,23 dtk (rata 2,0–2,6 dtk) |
| Warna dasar saat karaoke | dipaksa kuning | putih (pilihan pengguna), sorot kuning per kata |
| Mode satu kata | tidak ada | 1 kata per tampilan, ikut timestamp asli |
| Berkas per klip | 1 | `clip_01.mp4` + `clip_01_polos.mp4` + `clip_01.srt` |

Uji otomatis: `engine/internal/segment/segment_test.go` (durasi kandidat) dan
`engine/internal/subtitle/subtitle_test.go` (tak ada tampilan kilat, kata tidak
hilang, warna tidak dipaksa, mode word/karaoke).

## Keputusan desain

- Gaya subtitle jadi **tiga pilihan**: `normal` · `karaoke` (highlight per kata) ·
  `word` (satu kata per layar).
- Warna utama **selalu** ikut pilihan pengguna. Warna sorot jadi setelan
  terpisah (default kuning).
- Pembatas sosmed = **panduan di preview + preset posisi**, tidak ikut ter-render.

## Urutan pengerjaan

```
Tahap 0 (fondasi)  ──┬──> Tahap 2 (gaya subtitle)
timestamp per kata   └──> Tahap 3 (kecepatan subtitle)

Tahap 1 (durasi klip)      ← independen, bisa duluan
Tahap 4 (pembatas sosmed)  ← independen, GUI saja
Tahap 5 (verifikasi end-to-end)
```

---

### Tahap 0 — Timestamp per kata (fondasi #2 & #3)

`engine/internal/transcribe/whisper.go`, `types/types.go`

- Tambah flag `-ojf` (`--output-json-full`) + `-sow` pada pemanggilan whisper-cli
  → JSON memuat array `tokens` beserta `offsets` tiap token. Satu kali jalan,
  tanpa tambahan waktu transkripsi.
- Parse token → `TranscriptSegment.Words []Word{Start, End, Text}`; buang token
  khusus (`[_BEG_]`, `[_TT_..]`) dan gabungkan sub-token menjadi kata utuh.
- Fallback: bila `Words` kosong (model/JSON lama), bagi rata seperti sekarang —
  semua kode di bawah harus tetap jalan tanpa word timing.
- Opsional lanjutan: `-dtw <model>` untuk timestamp yang jauh lebih presisi
  (biaya CPU kecil). Ditaruh di belakang opsi, tidak default.

**Selesai bila:** transkrip JSON internal punya `words` dengan waktu masuk akal
(cek manual 1 segmen vs audio).

### Tahap 1 — Durasi klip (#4)

`engine/internal/segment/segment.go`, `config/config.go`

- Kenalkan **durasi ideal** = titik tengah rentang (mis. auto → 60–90 dtk).
  Jangan potong hanya karena sudah lewat `targetMin`.
- Aturan potong baru, berurutan:
  1. Kumpulkan segmen sampai `d >= ideal`.
  2. Dari situ, potong di akhir kalimat **pertama** yang ditemukan, selama
     `d <= targetMax`.
  3. Bila lewat `targetMax` tanpa akhir kalimat → mundur ke akhir kalimat
     terakhir, sisanya dibawa ke kandidat berikutnya (perilaku sekarang, tetap).
  4. Bonus: jeda diam panjang (>1,5 dtk antar segmen) jadi titik potong yang
     lebih disukai daripada sekadar tanda titik.
- Ubah preset `auto` dari `30–180` menjadi `45–120` dengan ideal 70, supaya
  label GUI ("auto") benar-benar menghasilkan variasi.
- Terapkan juga ke jalur LLM: momen dari Claude/Ollama yang durasinya di bawah
  `targetMin` digabung/diperluas ke batas segmen tetangga sebelum di-render.

**Selesai bila:** satu video uji menghasilkan durasi klip yang bervariasi dan
mengikuti preset (preset 60 → mayoritas 48–75 dtk, bukan 30-an).

### Tahap 2 — Gaya subtitle & warna (#2)

`config/config.go`, `subtitle/subtitle.go`, `gui/app/page.tsx`

- `config.Subtitle`: ganti `Karaoke bool` → `Mode string` (`normal|karaoke|word`)
  \+ `HighlightColor string`. Pertahankan pembacaan `karaoke: true` lama sebagai
  alias `karaoke` supaya preset lama tidak rusak.
- `styleColors()`: **hapus** pemaksaan kuning. `PrimaryColour` selalu = warna
  pengguna.
- Karaoke TIDAK memakai tag `\k` (yang menyapu warna permanen dan membuat
  seluruh baris berakhir kuning). Sebagai gantinya: satu `Dialogue` per kata,
  seluruh tampilan tetap warna dasar, hanya kata aktif dibungkus `{\c…&}` —
  hasilnya "kata aktif disorot", persis yang diminta.
- Renderer baru `renderWordMode()`: satu `Dialogue` per kata memakai
  `Words[i].Start/End` (fallback bagi rata), teks 1 kata, ukuran font boleh
  dinaikkan otomatis (mode ini biasanya lebih besar).
- GUI: ganti checkbox "Karaoke per-kata" jadi dropdown **Gaya subtitle**
  (Normal / Highlight per-kata / Satu kata per layar) + pemilih **warna sorot**
  yang hanya muncul untuk karaoke/word. Preview ikut mode (jangan lagi memaksa
  `#ffdd00` di `page.tsx:499`).

**Selesai bila:** pilih putih → hasil putih; mode `word` benar-benar menampilkan
satu kata bergantian; mode `karaoke` menyorot kata aktif tanpa mengubah warna dasar.

### Tahap 3 — Kecepatan subtitle (#3)

`subtitle/subtitle.go`, `config`, GUI

- Batas kata per halaman dihitung dari **kecepatan baca**, bukan hanya lebar
  baris: `maxChars ≈ cps × durasiHalaman` dengan `cps` ±14 char/dtk (bisa diatur).
- **Durasi minimum halaman** 1,2 dtk. Bila halaman lebih pendek:
  - perpanjang ke jeda kosong sebelum segmen berikutnya (bila ada), atau
  - gabung ke halaman sebelumnya/berikutnya (hilangkan halaman "itu." 0,45 dtk).
- Batas halaman memakai `Words` (Tahap 0) supaya waktu tampil = waktu ucap
  sebenarnya, bukan proporsi jumlah kata.
- GUI: setelan **Kecepatan subtitle** (Lambat / Normal / Padat) yang memetakan ke
  `cps` + `minDur`, dan opsi maksimum baris (2 atau 3).

**Selesai bila:** tidak ada `Dialogue` di bawah 1,0 dtk pada video uji, dan teks
panjang terbaca nyaman saat ditonton.

### Tahap 4 — Pembatas area aman sosmed (#1)

`gui/app/page.tsx`, `gui/app/globals.css` (GUI saja, tanpa perubahan engine)

- Dropdown **Platform**: TikTok · Instagram Reels · YouTube Shorts · Umum · Mati.
- Overlay zona tak-aman di preview 9:16 (persen terhadap tinggi/lebar frame):

  | Platform | Atas | Bawah | Kanan |
  |---|---|---|---|
  | TikTok | 8% | 20% | 16% |
  | Reels | 7% | 17% | 15% |
  | Shorts | 6% | 13% | 14% |
  | Umum | 8% | 20% | 16% |

  (angka awal, dikalibrasi dengan screenshot asli tiap aplikasi)
- Zona digambar sebagai arsiran gelap + label kecil ("caption", "tombol aksi").
- Tombol **"Taruh di area aman"** → set `subY` ke tengah area aman (dan `subX`
  bergeser kiri bila zona kanan aktif).
- Peringatan halus bila posisi subtitle sekarang masuk zona tak-aman.
- Platform ikut tersimpan di preset `localStorage` bersama setelan lain.

**Selesai bila:** ganti platform → arsiran berubah, tombol memindahkan subtitle
ke posisi yang tidak tertutup UI.

### Tahap 6 — Opsi simpan klip (permintaan tambahan)

`config`, `pipeline`, `ffmpeg`, `api`, GUI

- `Options.SubtitleOutput`: `burn` (default, bersubtitle) · `clean` (klip polos)
  · `both` (dua berkas: `clip_01.mp4` + `clip_01_polos.mp4`).
- Mode `clean`/`both` sekalian menulis `clip_01.srt` di folder output, supaya
  klip polos bisa disubtitle ulang di CapCut/Premiere.
- API: `GET /api/jobs/{id}/clips/{clip}/file?varian=polos|srt`.
- GUI: dropdown "Simpan klip" + tautan unduh terpisah (bersubtitle / polos / .srt).
- CLI: `-save burn|clean|both`.

Catatan biaya: mode `both` meng-encode dua kali → waktu render ±2x.

### Tahap 5 — Verifikasi

- Render ulang 1 video uji (±10 menit, model `base`) untuk tiap kombinasi:
  preset durasi 60 + gaya normal/karaoke/word.
- Cek: distribusi durasi klip, tidak ada halaman subtitle <1 dtk, warna sesuai,
  subtitle di dalam area aman platform terpilih.
- Perbarui `catatan/08-status-mvp.md`.

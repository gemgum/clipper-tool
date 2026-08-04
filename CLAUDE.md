# CLAUDE.md — Panduan untuk agen/sesi berikutnya

Proyek **Clipper**: memotong video panjang (1–4 jam) jadi klip pendek 9:16
bersubtitle otomatis + skor "berpotensi viral". Konten target: **bahasa Indonesia**.

## Arsitektur (2 lapis, terpisah)

gui/ (Next.js) --HTTP+SSE--> engine/ (Go)

`````````````````````````````|
````````````````````````````+-- exec --> whisper.cpp, ffmpeg

````

- **engine/** (Go) — otak: HTTP API, orkestrasi pipeline, scoring, panggil
  whisper.cpp/ffmpeg. Standard library saja. (tanpa dependency eksternal).
- **worker/** (C++) — native: `features` (RMS energi audio). `reframe`/face-follow
  (OpenCV) = milestone berikut. Kontrak: stdin JSON → stdout NDJSON.
- **gui/** (Next.js) — dua tab: `/` klip video (form → progress SSE → daftar klip)
  dan `/news` kartu berita (tempel link / jelajah RSS → kartu PNG). Ada pemilih
  bahasa antarmuka EN/ID di bilah navigasi (default EN, disimpan di localStorage).
- **notes/** — diskusi & keputusan desain (baca 01–07 untuk konteks).

## Perintah penting

```bash
./setup.sh [base|small|medium|large-v3|large-v3-turbo]   # build whisper.cpp + unduh model
./build.sh                       # build engine (Go)
./bin/clipper run <video> [-mode -model -duration -max -min-score \
                           -reframe center|fit -background blur|black -zoom 5..200 \
                           -sub-mode normal|karaoke|word -sub-speed slow|normal|dense \
                           -transcript-fix on|off -terms "Londo Ireng,Mahfud MD" \
                           -save burn|clean|both]                # CLI
./bin/clipper serve              # HTTP API di 127.0.0.1:8787 (port acak bila terpasang)
./bin/clipper serve -token on    # + kunci sesi; buka GUI dengan ?token=<kunci>
cd gui && npm run dev            # GUI di 3000 — tiga tab: klip, kartu, Requirements
````

Tab kartu berita butuh Chrome/Chromium (Edge bawaan Windows juga bisa). Engine
mencarinya sendiri; timpa dengan `CLIPPER_CHROME=/path/ke/chrome`.

Di tab itu LLM hanya **memilih nomor paragraf**, tidak pernah menulis: isi kartu
& caption selalu verbatim dari artikel. Lihat `notes/13-kartu-berita.md`.

## Alur pipeline (engine/internal/pipeline)

extract audio (ffmpeg) → features (RMS di engine) → transkripsi (whisper.cpp) →
**deteksi loop halusinasi** → **koreksi transkrip (LLM)** → segmentasi kandidat →
**LLM MEMILIH NOMOR kandidat** (bukan mengarang timestamp — lihat notes/18) +
scoring → buang kandidat dari cuplikan pembuka → render klip 9:16 + burn .ass.

Setiap job diakhiri **ringkasan waktu per tahap** (tabel monospace di terminal &
kotak log GUI), lengkap dengan rasio realtime. Itu angka pembanding antar
percobaan — model, mesin skor, CPU vs GPU.

Tiap klip juga menghasilkan `<clip>.txt`: ucapan klip itu tanpa timestamp dan
tanpa nomor, satu kalimat per baris. Bukan pengganti `.srt` (yang untuk editor
video dan hanya ada di mode clean/both) — berkas ini untuk ditempel ke LLM mana
pun saat membuat caption, jadi selalu ditulis apa pun mode simpannya.

## Peta paket engine

| Paket           | Isi                                                                           |
| --------------- | ----------------------------------------------------------------------------- |
| config          | Options, mode, Layout (folder), ResolvePaths                                  |
| ffmpeg          | ekstrak audio, clip + zoom/reframe, burn subtitle                             |
| transcribe      | wrapper whisper-cli (-ojf), parse segmen + kata bertimestamp                  |
| audio           | baca WAV (PCM 16-bit) + hitung RMS energi per hop                             |
| segment         | bangun kandidat klip (incar durasi ideal, potong di kalimat/jeda)             |
| score/heuristic | aturan Indonesia (emosi, hook, energi, durasi)                                |
| score/llm       | Claude API (pilih momen, skor + judul + hashtag)                              |
| score/ollama    | LLM lokal via Ollama (pilih momen, mode offline)                              |
| subtitle        | tulis .ass (normal/karaoke/word) & .srt                                       |
| correct         | koreksi transkrip via LLM + penyejajaran ulang timestamp per kata             |
| pipeline        | orkestrasi + progress callback                                                |
| job             | store job in-memory + SSE broadcast                                           |
| api             | HTTP handlers + SSE, pemilih berkas, kunci sesi                               |
| setup           | status komponen + unduh/pasang whisper.cpp, ffmpeg, model                     |
| capture         | foto layar halaman web via Chrome headless (exec, + terjemah path WSL)        |
| news            | RSS + metadata artikel (Open Graph) + ekstraksi paragraf + pemilih hook (LLM) |
| card            | kartu berita: data artikel → template HTML → PNG 1080x1920 + caption/sumber   |

## Folder data: dua bentuk, dipilih sendiri

`config.Locate` menentukan di mana engine menulis, dan tidak ada flag untuk
memilihnya — penandanya keberadaan `engine/go.mod`:

- **checkout sumber** → `<repo>/data`, `<repo>/models`, `<repo>/bin`,
  `<repo>/.env`. Persis seperti sebelumnya, jadi model & cache yang sudah ada
  tetap terpakai.
- **terpasang** → folder data per pengguna (`%LOCALAPPDATA%\Clipper` ·
  `~/Library/Application Support/Clipper` · `$XDG_DATA_HOME/clipper`). Wajib,
  sebab `Program Files` dan `/Applications` hanya-baca.

Font selalu dicari di `assets/fonts` **di sebelah biner** (ia dibundel, tidak
pernah berubah). Jangan menulis apa pun ke sana.

Akibatnya untuk kode baru: **jangan pernah menyusun path dari root proyek.**
Pakai `Paths.DataDir` / `ModelsDir` / `ToolsDir` / `EnvFile`. Timpaan env
(`CLIPPER_DATA_DIR`, `CLIPPER_MODELS_DIR`, `CLIPPER_TOOLS_DIR`,
`CLIPPER_FONTS_DIR`, `CLIPPER_ENV_FILE`) menang atas keduanya. Lihat
`notes/23-aplikasi-desktop.md`.

## Desktop: tiga hal yang sudah berlaku

Rinciannya di `notes/23`–`26`; yang wajib diingat saat menulis kode baru:

- **Berkas lokal tidak diunggah.** GUI punya pemilih berkas sendiri lewat
  `/api/browse`, dan berkas yang di-drop dicari di tempatnya lewat `/api/locate`.
  Jangan menambah alur yang menyalin video ke `data/uploads` (notes/24).
- **Komponen dipasang engine.** `internal/setup` mengunduh whisper.cpp, ffmpeg,
  dan model ke `ToolsDir`/`ModelsDir`; halaman Requirements memanggilnya. Kalau
  menambah komponen, tambahkan resepnya di sana — bukan instruksi di README
  (notes/25).
- **Kunci sesi.** Saat terpasang, engine memakai port acak + kunci yang wajib
  ada di setiap permintaan, ditulis ke `<DataDir>/engine.json`. Di GUI, SEMUA
  URL engine dibentuk lewat `eng()` di `gui/app/engine.ts` — jangan menyusun
  `${ENGINE}/api/...` sendiri, kuncinya tidak akan ikut (notes/26).

## Mode & mesin skor

offline (gratis: Ollama lokal atau heuristik) · hybrid (Claude) · online (belum).
Mode hybrid butuh `ANTHROPIC_API_KEY` di `.env` atau lewat GUI.

**Tanpa fallback**: mesin yang dipilih pengguna dipakai apa adanya. Bila gagal,
job berhenti dengan pesan akar masalah — engine TIDAK diam-diam pindah ke
heuristik. Lihat `notes/12-kebijakan-mesin-skor.md`.

## Transkripsi: flag yang TIDAK boleh diubah sembarangan

whisper dipanggil dengan **`-mc 0`** (jangan suapkan teks sebelumnya sebagai
konteks) dan **`-sns`**. Tanpa `-mc 0`, model besar bisa terjebak mengulang satu
kalimat sampai audio habis — pernah terjadi: 1603 dari 1644 segmen isinya
kalimat yang sama.

Konsekuensinya: **`--prompt` whisper mati total** saat `-mc 0`, jadi kosakata
tidak bisa dibias di tingkat decoding. Itu sebabnya nama & istilah daerah
dibenahi di tahap koreksi lewat `-terms`. Jangan mencoba menghidupkan `--prompt`
tanpa membaca `notes/21-flag-whisper.md` lebih dulu.

Flag decoding ikut menentukan kunci cache: **naikkan awalan versi di
`transcriptCacheKey`** (kini `v2`) setiap kali flag berubah, kalau tidak
transkrip lama dipakai ulang dan perubahannya seolah tak berefek.

Ada penjaga `detectRepetitionLoop` sebelum tahap koreksi: job berhenti dengan
pesan yang menyebut kalimat pengulangnya, bukan diam-diam menghasilkan klip
sampah.

## Koreksi transkrip

Keluaran mentah whisper membawa tanda hubung dialog, tanda baca salah tempat, dan
kata salah dengar. Semuanya ikut terbakar ke subtitle DAN menyesatkan segmentasi
(`BuildCandidates` memotong di akhir kalimat). Karena itu transkrip dikoreksi LLM
lebih dulu — menyala secara default, matikan dengan `-transcript-fix off`.

Butuh LLM walau mesin skornya heuristik: Claude di mode hybrid, selain itu
Ollama. Bila tak terjangkau, job berhenti dengan pesan sebabnya.

Paket `correct` MENGOREKSI, bukan menulis ulang: tiap segmen dijaga empat pagar
deterministik (panjang, jatah kata isi yang berubah, tanda kutip hilang, balasan
kosong), dan koreksi yang ditolak dilaporkan di `Report.Rejected`. Timestamp per
kata disejajarkan ulang lewat jarak edit supaya mode karaoke/word tetap tepat.
Hasilnya di-cache di `data/cache/corrected`. Lihat `notes/14-koreksi-transkrip.md`.

**Daftar istilah (`-terms`)** memperbaiki nama & kata daerah yang salah dengar
("Londo Irang" → "Londo Ireng"). Diperlukan karena `--prompt` whisper mati (lihat
di atas), DAN karena prompt koreksi justru melarang menyentuh kata daerah —
daftar ini pengecualiannya. Ikut jadi bahan kunci cache koreksi, jadi menambah
istilah memicu koreksi ulang. Untuk tugas ini **llama3.1 jelas lebih baik
daripada qwen2.5**. Lihat `notes/22-daftar-istilah.md`.

Transkrip panjang dipecah bertumpang-tindih sebelum dikirim ke LLM (12 mnt
Ollama / 25 mnt Claude); momen yang terbelah batas disambung lewat tanda
`"continues"`. Transkrip di-cache di `data/cache/transcripts` (kunci = sidik
jari isi video + model + bahasa).

## Konvensi

- **Komentar** kode dalam **bahasa Indonesia**.
- **Semua yang lain dalam bahasa Inggris**: nama folder, berkas, paket, fungsi,
  variabel, konstanta, nama test, field JSON API, nama event SSE, nilai flag CLI,
  dan seluruh pesan yang dilihat pengguna.
- Teks antarmuka GUI lewat kamus `gui/app/i18n.tsx` (EN sumber kebenaran; kunci
  bahasa Inggris wajib punya pasangan Indonesia, dijaga oleh TypeScript).
- Teks yang engine tulis ke kartu (tanggal, kaki kartu, berkas pendamping)
  mengikuti parameter `lang` dari klien; default `en`.
- Biner eksternal & model besar TIDAK di-commit (lihat .gitignore).
- Model whisper default engine: `small` (produksi). Uji cepat: `base`.
- Video besar: audio di-stream via ffmpeg (tidak dimuat ke RAM).

## Cara mengarahkan pemilik proyek (mode belajar)

Pemilik proyek mengerjakan sendiri perubahan kodenya dan memakai agen sebagai
pengarah, bukan pengetik. Kalau ia minta "arahkan", "aku mau belajar", atau
menyebut "jangan rubah code": **jangan menyunting berkas apa pun** — termasuk
setelah beberapa giliran berlalu; larangan itu berlaku sampai ia mencabutnya.

Bentuk arahan yang dipakai — **satu bagian per suntingan**, urut dari baris
paling bawah ke paling atas supaya nomor baris tidak bergeser saat ada baris
yang dihapus:

1. nomor baris
2. blok kode **Dari:**
3. blok kode **Jadi:** (atau "hapus seluruh barisnya")
4. **Kenapa:** 1–3 kalimat — alasannya, bukan sekadar apa yang berubah

Tutup dengan perintah verifikasi (build/vet/test/grep) dan angka yang harus
muncul kalau benar. Sebutkan angka harapan itu **sebelum** perintahnya
dijalankan; itu yang membedakan "kelihatan jalan" dari "terbukti jalan".

Yang bikin arahan gagal, dari pengalaman sesi pemindahan worker C++ ke Go:

- **penjelasan konsep panjang dulu, daftar suntingan belakangan.** Sertakan
  alasan di dalam tiap suntingan, jangan dipisah jadi esai tersendiri.
- **memberi banyak langkah sekaligus saat ia bilang bingung.** Turunkan jadi
  satu suntingan, minta hasil build, baru lanjut.
- **kalimat yang bisa ditafsir lebih luas dari maksudnya.** "`ctx` hilang dari
  pemanggilan" terbaca sebagai "buang `ctx` dari tanda tangan fungsi" dan
  memecahkan tujuh tempat lain. Sebut ruang lingkupnya dengan tegas.

Kalau ia melaporkan error, **baca keadaan berkasnya lebih dulu** (build, diff,
baris yang disebut) sebelum menebak sebabnya — dua kali dalam sesi itu tebakan
meleset karena suntingannya ternyata belum sesuai yang dikira.

## Status: MVP JALAN

CLI + HTTP + GUI semua berfungsi (mode offline diverifikasi
end-to-end). Lihat notes/08-status-mvp.md untuk yang sudah/belum.
`````````````````````````````

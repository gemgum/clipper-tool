# CLAUDE.md — Panduan untuk agen/sesi berikutnya

Proyek **Clipper**: memotong video panjang (1–4 jam) jadi klip pendek 9:16
bersubtitle otomatis + skor "berpotensi viral". Konten target: **bahasa Indonesia**.

## Arsitektur (3 lapis, terpisah)

```
gui/ (Next.js)  --HTTP+SSE-->  engine/ (Go)  --stdin/NDJSON-->  worker/ (C++)
                                    |
                                    +-- exec --> whisper.cpp, ffmpeg
```

- **engine/** (Go) — otak: HTTP API, orkestrasi pipeline, scoring, panggil
  whisper.cpp/ffmpeg/worker. Standard library saja (tanpa dependency eksternal).
- **worker/** (C++) — native: `features` (RMS energi audio). `reframe`/face-follow
  (OpenCV) = milestone berikut. Kontrak: stdin JSON → stdout NDJSON.
- **gui/** (Next.js) — dua tab: `/` klip video (form → progress SSE → daftar klip)
  dan `/news` kartu berita (tempel link / jelajah RSS → kartu PNG). Ada pemilih
  bahasa antarmuka EN/ID di bilah navigasi (default EN, disimpan di localStorage).
- **notes/** — diskusi & keputusan desain (baca 01–07 untuk konteks).

## Perintah penting

```bash
./setup.sh [base|small|medium]   # build whisper.cpp + unduh model + build worker (sekali)
./build.sh                       # build engine (Go) + worker (C++)
./bin/clipper run <video> [-mode -model -duration -max -min-score \
                           -reframe center|fit -background blur|black -zoom 5..200 \
                           -sub-mode normal|karaoke|word -sub-speed slow|normal|dense \
                           -transcript-fix on|off \
                           -save burn|clean|both]                # CLI
./bin/clipper serve              # HTTP API di 127.0.0.1:8787
cd gui && npm run dev            # GUI di 3000
```

Tab kartu berita butuh Chrome/Chromium (Edge bawaan Windows juga bisa). Engine
mencarinya sendiri; timpa dengan `CLIPPER_CHROME=/path/ke/chrome`.

Di tab itu LLM hanya **memilih nomor paragraf**, tidak pernah menulis: isi kartu
& caption selalu verbatim dari artikel. Lihat `notes/13-kartu-berita.md`.

## Alur pipeline (engine/internal/pipeline)

extract audio (ffmpeg) → features (worker C++) → transkripsi (whisper.cpp) →
**koreksi transkrip (LLM)** → segmentasi kandidat → scoring (heuristik ± Claude)
→ render klip 9:16 + burn .ass.

Tiap klip juga menghasilkan `<clip>.txt`: ucapan klip itu tanpa timestamp dan
tanpa nomor, satu kalimat per baris. Bukan pengganti `.srt` (yang untuk editor
video dan hanya ada di mode clean/both) — berkas ini untuk ditempel ke LLM mana
pun saat membuat caption, jadi selalu ditulis apa pun mode simpannya.

## Peta paket engine

| Paket           | Isi                                                                           |
| --------------- | ----------------------------------------------------------------------------- |
| config          | Options, mode, ResolvePaths                                                   |
| ffmpeg          | ekstrak audio, clip + zoom/reframe, burn subtitle                             |
| transcribe      | wrapper whisper-cli (-ojf), parse segmen + kata bertimestamp                  |
| worker          | client subprocess ke clipper-worker (NDJSON)                                  |
| segment         | bangun kandidat klip (incar durasi ideal, potong di kalimat/jeda)             |
| score/heuristic | aturan Indonesia (emosi, hook, energi, durasi)                                |
| score/llm       | Claude API (pilih momen, skor + judul + hashtag)                              |
| score/ollama    | LLM lokal via Ollama (pilih momen, mode offline)                              |
| subtitle        | tulis .ass (normal/karaoke/word) & .srt                                       |
| correct         | koreksi transkrip via LLM + penyejajaran ulang timestamp per kata             |
| pipeline        | orkestrasi + progress callback                                                |
| job             | store job in-memory + SSE broadcast                                           |
| api             | HTTP handlers + SSE                                                           |
| capture         | foto layar halaman web via Chrome headless (exec, + terjemah path WSL)        |
| news            | RSS + metadata artikel (Open Graph) + ekstraksi paragraf + pemilih hook (LLM) |
| card            | kartu berita: data artikel → template HTML → PNG 1080x1920 + caption/sumber   |

## Mode & mesin skor

offline (gratis: Ollama lokal atau heuristik) · hybrid (Claude) · online (belum).
Mode hybrid butuh `ANTHROPIC_API_KEY` di `.env` atau lewat GUI.

**Tanpa fallback**: mesin yang dipilih pengguna dipakai apa adanya. Bila gagal,
job berhenti dengan pesan akar masalah — engine TIDAK diam-diam pindah ke
heuristik. Lihat `notes/12-kebijakan-mesin-skor.md`.

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

## Status: MVP JALAN

CLI + HTTP + GUI + worker C++ semua berfungsi (mode offline diverifikasi
end-to-end). Lihat notes/08-status-mvp.md untuk yang sudah/belum.

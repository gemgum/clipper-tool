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
- **gui/** (Next.js) — halaman tunggal: form → progress SSE → daftar klip.
- **catatan/** — diskusi & keputusan desain (baca 01–07 untuk konteks).

## Perintah penting

```bash
./setup.sh [base|small|medium]   # build whisper.cpp + unduh model + build worker (sekali)
./build.sh                       # build engine (Go) + worker (C++)
./bin/clipper run <video> [-mode -model -duration -max -min-score \
                           -sub-mode normal|karaoke|word -sub-speed lambat|normal|padat \
                           -save burn|clean|both]                # CLI
./bin/clipper serve              # HTTP API di 127.0.0.1:8787
cd gui && npm run dev            # GUI di 3000
```

## Alur pipeline (engine/internal/pipeline)

extract audio (ffmpeg) → features (worker C++) → transkripsi (whisper.cpp) →
segmentasi kandidat → scoring (heuristik ± Claude) → render klip 9:16 + burn .ass.

## Peta paket engine

| Paket | Isi |
|---|---|
| config | Options, mode, ResolvePaths |
| ffmpeg | ekstrak audio, clip+reframe center, burn subtitle |
| transcribe | wrapper whisper-cli (-ojf), parse segmen + kata bertimestamp |
| worker | client subprocess ke clipper-worker (NDJSON) |
| segment | bangun kandidat klip (incar durasi ideal, potong di kalimat/jeda) |
| score/heuristic | aturan Indonesia (emosi, hook, energi, durasi) |
| score/llm | Claude API (pilih momen, skor + judul + hashtag) |
| score/ollama | LLM lokal via Ollama (pilih momen, mode offline) |
| subtitle | tulis .ass (normal/karaoke/word) & .srt |
| pipeline | orkestrasi + progress callback |
| job | store job in-memory + SSE broadcast |
| api | HTTP handlers + SSE |

## Mode & mesin skor

offline (gratis: Ollama lokal atau heuristik) · hybrid (Claude) · online (belum).
Mode hybrid butuh `ANTHROPIC_API_KEY` di `.env` atau lewat GUI.

**Tanpa fallback**: mesin yang dipilih pengguna dipakai apa adanya. Bila gagal,
job berhenti dengan pesan akar masalah — engine TIDAK diam-diam pindah ke
heuristik. Lihat `catatan/12-kebijakan-mesin-skor.md`.

Transkrip panjang dipecah bertumpang-tindih sebelum dikirim ke LLM (12 mnt
Ollama / 25 mnt Claude); momen yang terbelah batas disambung lewat tanda
`"berlanjut"`. Transkrip di-cache di `data/cache/transcripts` (kunci = sidik
jari isi video + model + bahasa).

## Konvensi

- Komentar & pesan pengguna dalam **bahasa Indonesia**.
- Biner eksternal & model besar TIDAK di-commit (lihat .gitignore).
- Model whisper default engine: `small` (produksi). Uji cepat: `base`.
- Video besar: audio di-stream via ffmpeg (tidak dimuat ke RAM).

## Status: MVP JALAN

CLI + HTTP + GUI + worker C++ semua berfungsi (mode offline diverifikasi
end-to-end). Lihat catatan/08-status-mvp.md untuk yang sudah/belum.

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
./bin/clipper run <video> [-mode -model -style -max -min-score]  # CLI
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
| transcribe | wrapper whisper-cli (-oj), parse segmen |
| worker | client subprocess ke clipper-worker (NDJSON) |
| segment | bangun kandidat klip dari transkrip |
| score/heuristic | aturan Indonesia (emosi, hook, energi, durasi) |
| score/llm | Claude API (skor + judul + hashtag) |
| subtitle | tulis .ass (plain/viral) |
| pipeline | orkestrasi + progress callback |
| job | store job in-memory + SSE broadcast |
| api | HTTP handlers + SSE |

## Mode

offline (whisper+heuristik, gratis) · hybrid (whisper+Claude) · online (belum).
Mode hybrid/online butuh `ANTHROPIC_API_KEY` di `.env`.

## Konvensi

- Komentar & pesan pengguna dalam **bahasa Indonesia**.
- Biner eksternal & model besar TIDAK di-commit (lihat .gitignore).
- Model whisper default engine: `small` (produksi). Uji cepat: `base`.
- Video besar: audio di-stream via ffmpeg (tidak dimuat ke RAM).

## Status: MVP JALAN

CLI + HTTP + GUI + worker C++ semua berfungsi (mode offline diverifikasi
end-to-end). Lihat catatan/08-status-mvp.md untuk yang sudah/belum.

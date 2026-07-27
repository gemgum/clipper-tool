# Clipper

Alat memotong video panjang jadi klip pendek 9:16, otomatis subtitle, dan
memberi skor "berpotensi viral". Fokus konten bahasa Indonesia.

## Arsitektur

```
Next.js (GUI web lokal)  --HTTP+SSE-->  Go (engine)  --stdin/NDJSON-->  C++ worker
                                             |
                                             +-- exec --> whisper.cpp, ffmpeg
```

- **engine/** — Go: orkestrasi, HTTP API, scoring, panggil whisper.cpp & ffmpeg.
- **worker/** — C++: analisis audio (`features`) & reframe (nanti face-follow).
- **gui/** — Next.js: antarmuka pengguna.
- **catatan/** — dokumentasi & keputusan desain.

## Mode

| Mode | Transkripsi | Scoring |
|---|---|---|
| offline | whisper.cpp (lokal) | heuristik |
| hybrid  | whisper.cpp (lokal) | Claude API |
| online  | STT API | Claude API |

## Prasyarat

- Go 1.22+, Node 18+, ffmpeg, g++ & cmake (untuk build worker & whisper.cpp).

## Setup

```bash
# 1. Build whisper.cpp + unduh model + build worker C++
./setup.sh

# 2. Build engine
cd engine && go build -o ../bin/clipper ./cmd/clipper

# 3. Coba lewat CLI (mode offline)
./bin/clipper run /path/video.mp4

# 4. Jalankan server + GUI
./bin/clipper serve
cd gui && npm install && npm run dev
```

## Konfigurasi

Salin `.env.example` ke `.env`. Untuk mode hybrid/online set `ANTHROPIC_API_KEY`.

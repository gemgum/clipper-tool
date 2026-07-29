# Kontrak API — Next.js ↔ Go ↔ C++

Keputusan: **Metode C** (worker C++ sendiri, aditif — whisper.cpp & ffmpeg tetap dipakai).
Bahasa sumber: **Indonesia** → default `language: "id"`, model Whisper `small`/`medium` (CPU).

## Alur data

```
Next.js (GUI) --HTTP+SSE--> Go (engine/API/orkestra) --stdin/NDJSON--> C++ worker (native)
                                  |
                                  +-- exec --> whisper.cpp, ffmpeg (binary C/C++ jadi)
```

- Go: manajemen job, panggil whisper.cpp & ffmpeg, heuristik, Claude API, setir worker C++.
- C++ worker: reframe 9:16, face-follow (OpenCV), analisis energi audio.
- ffmpeg: burn subtitle .ass + encode/mux final. Worker fokus visual, ffmpeg fokus codec.

---

## Kontrak 1 — Next.js ↔ Go (REST + SSE)

Base: `http://127.0.0.1:8787`

| Method | Endpoint | Fungsi |
|---|---|---|
| GET | /api/health | cek engine hidup |
| GET | /api/config | mode & default didukung |
| GET | /api/models | daftar model Whisper |
| POST | /api/models/download | unduh model `{ "name": "small" }` |
| POST | /api/jobs | buat job → `{ "id": "job_..." }` |
| GET | /api/jobs | daftar job |
| GET | /api/jobs/{id} | status + progress + hasil |
| GET | /api/jobs/{id}/events | SSE progress realtime |
| POST | /api/jobs/{id}/cancel | batalkan job |
| GET | /api/jobs/{id}/clips | daftar klip + skor |
| PATCH | /api/clips/{id}/subtitles | edit subtitle sebelum render |
| POST | /api/clips/{id}/render | render final klip |
| GET | /api/clips/{id}/file | unduh mp4 |
| GET | /api/clips/{id}/preview | streaming preview |

### POST /api/jobs (request)
```json
{
  "source": { "type": "path", "value": "/abs/video.mp4" },
  "mode": "hybrid",
  "language": "id",
  "transcription": { "model": "small", "device": "cpu" },
  "output": {
    "aspect": "9:16",
    "reframe": "center",
    "subtitle_style": "viral",
    "max_clips": 10,
    "target_duration": [20, 60]
  },
  "scoring": { "engine": "heuristic_llm", "llm_model": "claude-haiku-4-5", "min_score": 60 }
}
```
- `mode`: `offline` | `hybrid` | `online`
- `reframe`: `center` | `face_follow`
- `subtitle_style`: `plain` | `viral`
- `scoring.engine`: `heuristic` | `heuristic_llm` | `llm`

### GET /api/jobs/{id} (response)
```json
{
  "id": "job_a1b2",
  "status": "running",
  "stage": "scoring",
  "progress": 0.62,
  "error": null,
  "clips": []
}
```
- `status`: `queued` | `running` | `done` | `error` | `canceled`
- `stage`: `extracting` | `transcribing` | `segmenting` | `scoring` | `rendering`

### Clip
```json
{
  "id": "clip_09",
  "job_id": "job_a1b2",
  "start": 123.4, "end": 158.9, "duration": 35.5,
  "score": 82,
  "reasons": { "hook": 90, "emotion": 75, "clarity": 80, "shareability": 85, "standalone": 78 },
  "title": "Rahasia yang jarang diketahui...",
  "hashtags": ["#fyp", "#podcast"],
  "transcript": "...",
  "video_path": "clip_09.mp4",
  "video_path_raw": "clip_09_polos.mp4",
  "subtitle_srt": "clip_09.srt",
  "status": "rendered"
}
```
- `status`: `scored` | `rendering` | `rendered`
- Tidak ada `subtitle_path`: `.ass` cuma berkas antara di `tmp/` yang dihapus
  setelah dibakar ke video. Yang bertahan `.srt` (mode `clean`/`both`).
- `video_path_raw` & `subtitle_srt` hanya terisi bila mode simpan memintanya.

### PATCH /api/clips/{id}/subtitles (request)
```json
{ "segments": [ { "start": 0.0, "end": 1.8, "text": "Halo semuanya" } ] }
```

### SSE — GET /api/jobs/{id}/events
```
event: progress
data: {"stage":"transcribing","progress":0.30}

event: clip
data: {"id":"clip_09","score":82,"status":"scored"}

event: done
data: {"job_id":"job_a1b2","clips":8}

event: error
data: {"message":"ffmpeg gagal: ..."}
```

---

## Kontrak 2 — Go ↔ C++ worker (stdin JSON → stdout NDJSON)

Worker = subprocess. 1 request JSON di stdin, balas NDJSON di stdout, exit 0 = sukses.
Subprocess (bukan cgo) supaya Go tetap single-binary & cross-compile mudah.

### cmd: features (heuristik energi audio)
Request:
```json
{ "cmd": "features", "input": "/tmp/job_a1b2/audio.wav", "hop_ms": 100 }
```
Response (stdout):
```json
{"type":"result","rms":[0.12,0.34],"hop_ms":100}
```

### cmd: reframe (potong + 9:16)
Request:
```json
{
  "cmd": "reframe",
  "input": "/abs/video.mp4",
  "output": "/tmp/job_a1b2/clip_09_916.mp4",
  "start": 123.4, "end": 158.9,
  "target": { "w": 1080, "h": 1920 },
  "mode": "center",
  "options": { "smoothing": 0.3 }
}
```
Response (streaming NDJSON):
```json
{"type":"progress","value":0.5}
{"type":"result","output":"/tmp/job_a1b2/clip_09_916.mp4"}
```
- `mode: center` → crop tengah (ringan; bisa ffmpeg murni juga).
- `mode: face_follow` → OpenCV, lintasan crop ikut wajah. Bagian C++ utama.

### Error worker
```json
{"type":"error","message":"tidak bisa buka video"}
```

Pembagian render: worker hasilkan mp4 9:16 → ffmpeg (dipanggil Go) burn subtitle .ass + mux.
Worker fokus geometri/visual; codec diserahkan ke ffmpeg (jangan tulis ulang codec).

---

## Default bahasa Indonesia
- `transcription.language = "id"`
- Model: CPU → `small` (seimbang) atau `medium` (lebih akurat, lebih lambat).
  `large-v3` paling akurat tapi berat untuk CPU.
- Heuristik & prompt scoring perlu disesuaikan untuk bahasa Indonesia
  (kata emosi/hook dalam bahasa Indonesia).

# 43 — Transkripsi lewat Google AI Studio

Diminta pemilik 9 Oktober 2026: selain whisper lokal, transkripsi bisa
dikerjakan Gemini lewat Google AI Studio. Dipilih per job, bukan cadangan.

## Cara memakai

- `.env`: `AI_STUDIO_KEY=<kunci>` (wajib), `AI_STUDIO_MODEL=` (opsional,
  bawaan `gemini-2.5-flash`).
- CLI: `clipper run video.mp4 -transcriber aistudio`.
- API: `"transcriber": "aistudio"` di `options` job (`whisper` = bawaan).
- `GET /api/requirements` membawa `ai_studio: {key_set, model}`; kuncinya
  sendiri tidak pernah dikirim.

## Cara kerjanya

1. Audio diekstrak seperti biasa (WAV 16 kHz mono).
2. Dipotong per 10 menit, tiap potongan dikodekan Opus mono 24 kbps
   (≈ 1,8 MB per 10 menit) dan dikirim INLINE ke `generateContent`, berurutan.
   Jauh di bawah batas 20 MB per permintaan, dan balasan JSON satu potongan
   masih muat di jatah keluaran model.
3. Model diminta JSON berskema: `segments: [{start, end, text}]`, detik relatif
   potongan. Engine menjepitnya ke batas potongan, membuang teks kosong,
   mengurutkan, memotong tumpang-tindih, lalu menggeser ke waktu mutlak.
4. Hasilnya `types.Transcript` yang sama dengan whisper, jadi penjaga loop,
   koreksi, segmentasi, dan subtitle tidak berubah.

## Batas yang diketahui

- **Waktu per kata hanya PERKIRAAN.** Gemini tidak memberi timestamp per kata
  yang bisa dipercaya, jadi durasi tiap segmen dibagi ke katanya sebanding
  jumlah huruf. Mode karaoke/word dengan transkrip ini lebih kasar sinkronnya
  daripada whisper.
- Kalimat yang terbelah batas 10 menit bisa jadi dua segmen.
- Tanpa cadangan: kunci kosong, kunci ditolak, atau balasan tak terbaca
  menghentikan job dengan pesan sebabnya (notes/12). 429/5xx diulang lewat
  `httpx.Retry`.
- Kunci cache transkrip memakai model `aistudio:<model>`; cache whisper lama
  tetap terpakai (kuncinya tidak berubah).
- Halaman Requirements belum punya isian untuk `AI_STUDIO_KEY`; isi lewat `.env`.

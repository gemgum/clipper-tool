# Aplikasi desktop & pemasangan engine sekali klik

Sasaran berikutnya, ditetapkan 4 Agustus 2026.

## Bentuk yang dituju

Pengguna membuka satu berkas aplikasi. Kalau ada komponen yang belum ada —
whisper, model, Ollama — muncul tombol untuk memasangnya. Tidak ada terminal,
tidak ada `setup.sh`, tidak ada cmake.

Itu perbedaan pokoknya dengan keadaan sekarang: hari ini pemasangan adalah
pekerjaan pengembang; besok ia harus jadi pekerjaan aplikasi.

## Yang sudah diputuskan, jangan dibuka lagi

- Desktop tiga OS. `.exe` Windows sasaran utama, AppImage Linux untuk uji cepat,
  macOS menyusul. **Mode web ter-deploy ditolak.**
- Engine tetap Go, UI tetap Next.js, komunikasi tetap REST + SSE.
- Worker C++ sudah dibuang (selesai 4 Agustus 2026, lihat `20-buang-worker-cpp.md`).
  Build tiga OS sekarang cukup `go build`.

## Yang belum diputuskan

Shell jendelanya. Dua kandidat, keduanya masih hidup:

- **Chrome `--app=`** — nol dependensi tambahan, dan pencari browser sudah ada
  di `capture.go` untuk kartu berita.
- **Tauri + Go sidecar** — pengemasan, penandatanganan, dan notarisasi paling
  matang.

Tidak mendesak: fondasinya sama untuk keduanya, jadi pekerjaan di bawah ini
tidak perlu menunggu keputusan itu.

## Tiga penghalang yang sudah diketahui

Ada di kode sekarang dan menghalangi bentuk desktop apa pun:

1. **Folder data ditulis di sebelah biner.** Mustahil di `/Applications` dan
   `Program Files` yang hanya-baca. Harus pindah ke folder data per pengguna.
2. **Port 8787 dipaku, CORS `*`, tanpa token.** Di mesin pengguna itu berarti
   aplikasi lain bisa memerintah engine. Perlu port acak + token sesi.
3. **Berkas lokal disalin lewat HTTP.** Video 3,84 GB digandakan ke
   `data/uploads/`. Di desktop, berkasnya sudah ada di mesin yang sama — cukup
   kirim path-nya.

## Pemasangan engine sekali klik

Bagian baru, dan yang paling menentukan rasanya sebagai aplikasi.

Sekarang `setup.sh` mengerjakan empat hal: membangun whisper.cpp dari sumber
(butuh cmake + g++), mengunduh model, mengunduh font. Tidak satu pun boleh
dituntut dari pengguna desktop.

Per komponen:

| Komponen | Rencana |
| --- | --- |
| whisper | Jangan bangun dari sumber. Pakai rilis biner resmi whisper.cpp per OS, unduh saat dibutuhkan. |
| model whisper | Unduh dari HuggingFace saat pengguna memilihnya, dengan progres. `small` ≈ 466 MB. |
| ffmpeg | Wajib ada. Pilihannya: bundel biner statis, atau unduh sekali di pemakaian pertama. |
| Ollama | Aplikasi terpisah, tidak bisa dibundel. Deteksi keberadaannya; kalau tidak ada, tombol yang membuka halaman unduhnya. Kalau ada, deteksi model dan tawarkan `ollama pull`. |
| font subtitle | Kecil. Bundel saja, jangan diunduh. |
| Chrome | Sudah ditangani `capture.go`. Hanya perlu bagi tab kartu berita. |

Bentuk antarmukanya: satu halaman **Requirements** berisi daftar komponen
dengan status (ada / belum ada / sedang mengunduh) dan tombol per baris.

Yang dibutuhkan dari engine: satu endpoint status komponen, dan satu endpoint
pemasangan yang mengalirkan progres. Pola SSE-nya sudah ada di `job` — pakai
ulang, jangan bikin mekanisme kedua.

## Urutan yang disarankan

Penghalang nomor 1 lebih dulu. Selama folder data masih menempel ke biner,
tidak ada tempat sah untuk menaruh model dan biner yang diunduh — jadi ia
memblokir seluruh pekerjaan pemasangan sekali klik.

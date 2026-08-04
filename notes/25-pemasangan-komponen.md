# Pemasangan komponen sekali klik

Selesai 4 Agustus 2026 (bagian pertama). Rencananya di `23-aplikasi-desktop.md`.

## Bentuknya

Satu halaman **Requirements** (tab ketiga di GUI): tiap komponen satu baris,
lampu hijau/merah, ukuran unduhan, dan tombol. Engine yang tahu apa yang ada dan
apa yang kurang — GUI hanya menampilkan jawabannya.

| Endpoint | Isi |
| --- | --- |
| `GET /api/requirements` | status semua komponen + daftar yang wajib tapi belum ada |
| `POST /api/requirements/install` | pasang satu komponen, alirkan progres (SSE) |
| `POST /api/requirements/remove` | hapus model yang sudah diunduh |

Alirannya memakai format SSE yang sama dengan progres job — satu mekanisme untuk
seluruh engine, sesuai catatan 23. Bedanya ia dibaca dengan `fetch` + reader,
bukan `EventSource`: EventSource hanya bisa GET, sedangkan memasang komponen
jelas mengubah keadaan mesin.

## Dari mana binernya diambil

Paket `internal/setup`. Resep per OS ada di `recipe.go`:

| Komponen | Windows | Linux | macOS |
| --- | --- | --- | --- |
| whisper.cpp | rilis resmi `whisper-bin-x64.zip` | `whisper-bin-ubuntu-x64.tar.gz` (arm64 juga) | tidak ada biner resmi → `brew install whisper-cpp` |
| ffmpeg | gyan.dev `ffmpeg-release-essentials.zip` | rilis statisnya `.tar.xz` → pakai paket sistem | evermeet.cx `.zip` |
| model whisper | HuggingFace `ggml-<nama>.bin` | sama | sama |

Tiga keputusan di baliknya:

- **Hanya zip dan tar.gz.** Keduanya ada di pustaka standar Go. Rilis `.tar.xz`
  (ffmpeg statis untuk Linux) sengaja tidak dipakai: menambah dependensi
  eksternal cuma untuk membongkarnya melanggar aturan "standard library saja".
  Linux memang sasaran uji cepat, bukan sasaran distribusi utama.
- **Versi whisper.cpp dipaku (`v1.9.1`), bukan "latest".** Rilis baru bisa
  mengubah nama berkas di dalam arsip, dan yang gagal karenanya adalah
  pemasangan di mesin pengguna — tempat paling buruk untuk menemukan kejutan.
- **Semua isi arsip diratakan ke satu folder** (`ToolsDir`). whisper.cpp membawa
  pustaka bersamanya, dan engine memang menunjuk `LD_LIBRARY_PATH` ke folder
  binernya. Symlink di dalam tar ikut dibuat ulang — rantai
  `libwhisper.so → .so.1 → .so.1.9.1` itu yang membuat binernya bisa jalan.

Selain itu: unduhan ditulis ke `.part` lalu diganti nama (berkas setengah jadi
tidak boleh terlihat seperti berkas jadi), biner diberi bit x sendiri (rilis
Windows tidak membawanya), dan satu komponen tidak bisa dipasang dua kali
sekaligus.

## Yang tidak dipasang engine

Ollama dan Chrome. Keduanya aplikasi utuh milik orang lain dengan pemasangnya
sendiri; yang bisa dilakukan hanyalah mendeteksi dan menunjukkan halaman
unduhnya. Ollama yang sudah jalan ikut dilaporkan model apa saja yang ada.

## Bukti jalannya

Dari folder `bin`/`models` yang benar-benar kosong, lewat API:

- `whisper.cpp` terunduh & dibongkar → `whisper-cli --help` jalan;
- model `tiny` terunduh (8 detik, 39 laporan progres);
- `clipper run` di folder itu menghasilkan klip 9:16 bersubtitle — jadi biner
  hasil unduhan benar-benar menggerakkan pipeline, bukan sekadar ada.

## Sisa pekerjaan

Tombol pasang ffmpeg belum bisa diuji di Linux (resepnya memang tidak ada di
sana). Perlu dicoba di Windows sebelum rilis pertama — bersama seluruh alur,
sebab Windows-lah sasaran utamanya.

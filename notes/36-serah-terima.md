# Serah terima — 6 Agustus 2026 (malam)

Ditulis sebelum mesin dimatikan. **Baca ini dulu**, lalu `CLAUDE.md`. Rinciannya
di `notes/32`–`35`; ini peta dan daftar kerjanya.

Keadaan repo saat catatan ini ditulis: pohon kerja **bersih**, rilis terakhir
**v0.6.1**, `tauri.conf.json` juga 0.6.1.

## Yang sudah dikerjakan (dan angkanya)

**Tampilan tidak bergerak** (`notes/32`)

- Lingkaran umpan balik ukuran pratinjau diputus. Bingkai **262×465 pada subSize
  72 DAN 140** — angka yang sama persis.
- `.screen-head` dibuang: galat & peringatan jadi lapisan melayang
  (`alerts.tsx`), kemajuan job pindah ke panel Start yang tempatnya permanen.
- Peringatan di dalam panel jadi lambang di label (`warn.tsx`); di sel tanpa
  label ia melayang. Terukur: tombol **top 91 sebelum & sesudah**, panel
  **420 px** di kedua keadaan.
- Semua kendali sebaris memakai satu tinggi (`--ctl-h: 32px`).
- Jendela dikunci **1240×860** (satu-satunya ukuran yang 0/0 di semua halaman).

**Riwayat & keluaran**

- Riwayat job bertahan: satu JSON per job di `<DataDir>/jobs/`, klip yang
  berkasnya hilang dibuang saat dibaca.
- Kartu jadi kisi yang mengalir ke bawah; klip tetap deret mendatar per job.
- Pilih dengan klik / Ctrl+klik / Shift+klik; **unduh massal** satu zip lewat
  `GET /api/download`.
- Halaman Results dibuang (isinya bagian dari Output history).

**LLM lokal** (`notes/33`, `notes/35`)

- Alamatnya DICARI sendiri: `OLLAMA_HOST` → localhost → gerbang WSL →
  `host.docker.internal`, plus port server lain. Alamat publik ditolak.
- Engine tahu sistem asalnya: `Windows` · `WSL` · `Linux` · `macOS` · `remote`,
  tampil di label dan di tiap baris daftar model.
- Bukan Ollama saja: satu klien untuk semua server bergaya OpenAI (llama.cpp,
  LocalAI, llamafile, vLLM, Aphrodite, LiteLLM, Exo).
- Model dinilai dari metadata (parameter, konteks, kemampuan, lokal vs cloud);
  yang kecil DITANDAI, bukan ditolak. Model `*-cloud` ditolak.
- Koreksi transkrip menyesuaikan diri: potongan yang balasannya tak terbaca
  **dipecah dua** sampai model sanggup; `num_ctx` mengikuti konteks model.

**Pesan & komponen** (`notes/34`)

- Tidak ada lagi `./setup.sh` di pesan pengguna — semuanya menunjuk halaman
  Requirements. Nama baris = nama berkas sebenarnya (`whisper-cli.exe`).
- `applyPaths()` dipanggil setelah pemasangan DAN tiap `/api/requirements`
  ditanya: tidak perlu restart, dan "Check again" benar-benar memeriksa ulang.
- Requirements MENJALANKAN `ffmpeg -version`/`ffprobe -version`.
- Notifikasi "ada yang kurang" membawa tombol **Install N missing**.
- Video tanpa trek suara dikenali sebelum ekstraksi + lencana **⚠ no sound** di
  pratinjau. (Ini yang dulu muncul sebagai "Invalid argument" pada audio.wav.)

## Yang berikutnya, berurut menurut manfaat

Daftar ini disaring ulang **6 Agustus 2026** dan tiap butirnya diperiksa lagi ke
kode — yang tertulis di bawah adalah keadaan sesudah pemeriksaan itu, bukan
keadaan saat catatan ini pertama ditulis.

### 1. Uji klien OpenAI-compatible dengan server SUNGGUHAN — BELUM

Masih terbukti dengan server tiruan saja: `openai_test.go` seluruhnya
`httptest`. Yang belum diketahui: dukungan `json_schema` tiap server, penamaan
model, batas `max_tokens`. Paling murah dicoba: LM Studio (port 1234) atau
`llama-server` dari llama.cpp (8080) — keduanya sudah ada di daftar kandidat
`discover.go`.

### 2. Kotak alamat server LLM + kunci API di GUI — BELUM

Jalur alamatnya di engine **sudah jadi**, yang kurang cuma pengisinya:

| Sudah menerima alamat            | Berkas                                  |
| -------------------------------- | --------------------------------------- |
| `POST /api/run` → `ollama_url`   | `engine/internal/api/api.go:284`        |
| `GET /api/ollama/status?url=`    | `engine/internal/api/api.go:686`        |
| `GET /api/requirements?ollama_url=` | `engine/internal/api/requirements.go:33` |

Yang kurang ada empat, dan yang kedua paling menentukan:

- **GUI tidak pernah mengirimnya.** `gui/app/setup-panel.tsx:257` cuma punya
  `Select` model; `ollama_url` selalu `""`.
- **Kunci API dipaku `Bearer local`** di DUA tempat: `openai.go:92` (permintaan)
  dan `discover.go:202` (**pemeriksaan**). Yang kedua yang fatal — LiteLLM
  dengan auth menyala membalas 401, `probeJSON` mengembalikan `false`, dan
  servernya dilaporkan "tidak jalan", bukan "salah kunci". Jadi kotak kunci itu
  bukan kenyamanan: tanpanya servernya tidak terlihat sama sekali.
- **Di mana disimpan — KEPUTUSAN PEMILIK.** Polanya sudah ada (kunci Claude ke
  `.env` lewat `postSettings`, `api.go:667`); `LLM_URL` + `LLM_API_KEY` di sana
  mengikuti pola itu. Menyentuh format data, jangan diputuskan sendiri.
- **Alamat publik — KEPUTUSAN PEMILIK.** `hostURL()` (`discover.go:159`)
  sengaja MENOLAK IP publik; kotak isian membobol pagar itu. Perlu diputuskan
  apakah alamat yang diketik pengguna sendiri boleh publik (itu satu-satunya
  cara memakai LLM di server sendiri) atau tetap ditolak.

### 3. Layar 1366×768 — BELUM

`desktop/src-tauri/tauri.conf.json` masih `minWidth: 1240`, `minHeight: 860`;
860 + bilah judul + taskbar ≈ 940, tidak muat. Kalau jadi masalah nyata, yang
dikerjakan adalah membuat halaman klip muat di jendela lebih pendek — BUKAN
menurunkan angka minimumnya; tabel ukur di `notes/33` menunjukkan akibatnya
kalau diturunkan. Bahannya sudah ada: `<Section>` (`gui/app/section.tsx`) sudah
bisa diciutkan, tinggal dipakai untuk kelompok setelan.

### 4. Deploy situs otomatis — DITUTUP pemilik

Dianggap selesai; deploy dijalankan sebagaimana adanya. Catatan faktual supaya
sesi berikutnya tidak bingung: `.github/workflows/desktop.yml` di repo ini
BERHENTI di langkah "Publish release" — tidak ada `repository_dispatch`, jadi
kalau otomatisasinya ada, ia hidup di sisi `gemgum.github.io`, bukan di sini.

### 5. Galat ffmpeg lama di Windows — SETENGAH JALAN

Akar yang bisa dibuktikan sudah ditutup: `checkRuns`
(`engine/internal/setup/setup.go:215`) BENAR-BENAR menjalankan
`ffmpeg -version` / `ffprobe -version`, dan yang gagal dilaporkan tidak
terpasang dengan pesan sistemnya apa adanya (maks 300 karakter) di `Detail`.

Dua hal tersisa:

- **Teks galatnya masih belum ada.** Belum ada yang membuka halaman Requirements
  di mesin Windows itu. Satu tangkapan layar baris ffmpeg/ffprobe sudah cukup;
  sebelum itu, apa pun yang dikerjakan hanya tebakan (`notes/31` butir 1).
- **`whisperStatus` (`setup.go:235`) BELUM ikut.** Ia `os.Stat`, ketemu,
  `return` di baris 246–249 — tidak pernah memanggil `checkRuns`. Padahal
  laporan aslinya berbunyi *"untuk whisper tadi..."*, jadi kalau yang tidak bisa
  jalan justru `whisper-cli.exe`, Requirements MASIH bilang hijau. Bedanya:
  `-version` tidak dijamin ada di whisper-cli — cek dulu flag mana yang keluar
  bersih (`-h` mungkin lebih aman).

## Yang DIBUANG dari daftar (6 Agustus 2026)

Bukan ditunda — diputuskan tidak dikerjakan. Jangan dihidupkan lagi tanpa alasan
baru:

- **Menghapus satu JOB utuh dari riwayat.** Menghapus per klip sudah cukup.
- **Sumber private + repo rilis publik.** Repo tetap seperti sekarang.

## Cara memeriksa sebelum menyerahkan apa pun

```bash
./bin/clipper serve                       # terminal lain
~/.cache/ms-playwright/chromium-*/chrome-linux64/chrome \
  --headless --remote-debugging-port=9333 --user-data-dir=/tmp/cdp about:blank &
node scripts/measure-ui.mjs http://127.0.0.1:8787 /tmp/shots
# SIZES='[["1240x860",1240,765]]' untuk ukuran lain
```

**Lihat potretnya, bukan cuma angkanya.** Lalu: `go vet ./...` ·
`go test ./...` · `npx tsc --noEmit --noUnusedLocals --noUnusedParameters` ·
`npm run build`.

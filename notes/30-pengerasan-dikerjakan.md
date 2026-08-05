# Pengerasan: yang sudah dikerjakan

Dikerjakan 5 Agustus 2026, menutup daftar kerja di
`28-pengerasan-sebelum-pentest.md` nomor 1–5. Berkas ini mencatat apa yang
berubah, **keputusan yang diambil di persimpangan**, dan cara membuktikannya
masih benar nanti.

Berkas 28 tetap berlaku sebagai duduk perkara dan sebagai daftar "sengaja
dibiarkan". Yang di sini adalah hasilnya.

## 1. Biner unduhan diperiksa sidik jarinya

`internal/setup/recipe.go`, `setup.go`

Dulu: engine mengunduh `ffmpeg.exe` dan `whisper-cli.exe` lalu
**menjalankannya**, dengan HTTPS sebagai satu-satunya penjaga.

Sekarang tiap alamat unduhan berpasangan dengan sha256 yang dipaku
(tipe `source`), dan berkasnya diperiksa **sebelum** namanya berubah dari
`.part` jadi nama berkas jadi. Gagal cocok = berkas dibuang, bukan diperbaiki
diam-diam. Sumber tanpa sidik jari **ditolak sebelum satu byte pun diunduh** —
penjaga itu ada di jalur pemakaian, bukan cuma di uji, supaya cermin baru yang
lupa dipaku gagal seketika.

Sidik jarinya diambil dengan mengunduh berkasnya sendiri lalu menghitungnya
(model: lewat `x-linked-etag` HuggingFace, yang memang sha256 objek LFS-nya).

### Cacat yang kita buat sendiri, dan cara menutupnya

Lanjutan-unduhan membiarkan `.part` dari cermin pertama **disambung** oleh cermin
kedua. Isinya beda build, ukurannya wajar, hasilnya rusak — dan tetap
dijalankan. Sekarang alamat asal ditulis ke `<berkas>.part.from`; pindah cermin
berarti potongannya dibuang dan mulai dari nol.

Sidik jari sebenarnya menangkap ini juga, tapi baru **setelah** seluruh sisanya
diunduh. Penanda asal menghentikannya di byte pertama. Keduanya dipakai.

### Keputusan: cermin yang tidak bisa dipaku dibuang

`https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip` **dicabut**.
Alamatnya selalu menunjuk "rilis terbaru" tanpa nomor versi, jadi isinya berganti
sendiri dan sidik jarinya mustahil dipaku. Cermin GitHub GyanD yang tetap dipakai
adalah build yang sama dari pengunggah yang sama, hanya bernomor.

`BtbN/FFmpeg-Builds` tetap ada tapi **pindah dari tag `latest` ke satu tag
autobuild** (`autobuild-2026-08-04-21-26`) — dengan alasan yang sama.

Kenapa cermin kedua dipertahankan meski keduanya sama-sama di GitHub: yang
ditakutkan bukan "GitHub mati" (kalau GitHub mati, aplikasinya sendiri tak bisa
diunduh), melainkan **satu pengunggah menghapus rilisnya**. Untuk itu pengunggah
yang berbeda memang menolong.

### Model: dipaku ke commit, bukan ke cabang

Alamat model berpindah dari `resolve/main/` ke
`resolve/5359861c739e955e79d9a303bcbc70fb988958b1/`. Cabang bisa dipindahkan
kapan saja oleh pemilik repo, dan berkas yang isinya boleh berubah tidak bisa
dipaku sidik jarinya. Dua cermin (HuggingFace + hf-mirror) berbagi satu sha256 —
cermin yang menyajikan isi berbeda dari aslinya bukan cermin, dan justru itu yang
ingin ketahuan.

**Menaikkan `modelRevision`, `whisperVersion`, `ffmpegVersion`, atau `btbnBuild`
berarti menghitung ulang sha256-nya.** Keduanya satu paket; menaikkan versi tanpa
sidik jari baru menghasilkan pemasangan yang gagal di mesin pengguna.

## 2. Permintaan yang mustahil datang dari GUI ditolak

`internal/api/guard.go` (baru) — tiga pemeriksaan, berlaku di **semua** mode,
jadi mode pengembangan ikut terlindung tanpa harus menyalakan kunci di sana:

| Pemeriksaan | Menutup apa | Jawaban |
| --- | --- | --- |
| `Host` harus alamat mesin ini | DNS rebinding — di serangan itu Host berisi nama si penyerang, berapa pun IP yang akhirnya dituju | 403 |
| Badan permintaan wajib `application/json` | POST lintas-asal yang tidak butuh preflight. Begitu jenisnya wajib JSON, browser wajib minta izin dulu — dan izin itu yang ditolak CORS | 415 |
| `Sec-Fetch-Site: cross-site` ditolak | Bentuk yang tidak membawa Origin sama sekali: `<img src>`, `<script src>`, `<iframe>` | 403 |

Urutan lapisannya sekarang `withGuard(withCORS(withToken(mux)))` — dari yang
paling murah & luas ke yang paling sempit.

Tiga hal yang sengaja **tidak** ditolak, dan alasannya:

- **`same-site`**, bukan hanya `same-origin`: GUI pengembangan di `:3000`
  menghubungi engine di `:8787`, dan bagi browser itu same-site. Menolaknya
  berarti mematikan alur pengembangan demi serangan yang sudah tertutup Origin.
- **`OPTIONS`** tidak dinilai dari Content-Type — preflight memang tanpa badan,
  dan menolaknya berarti menolak permintaan yang justru sedang meminta izin.
- **`/api/upload`** tetap multipart: videonya bisa bergiga-giga dan harus
  dialirkan, bukan dimuat ke memori sebagai JSON.

`-addr` yang disebut pengguna sendiri ikut diterima (`Server.AllowHost`, dipanggil
dari `main.go`). Mengikat ke alamat non-loopback adalah keputusan sadar, dan
menolaknya di sini berarti flag itu diam-diam tidak berfungsi. `0.0.0.0`/`::`
tidak ikut — itu cara mendengar, bukan nama yang bisa dituju.

### Yang ikut berubah di GUI

`gui/app/page.tsx`: pembatalan job dulu `fetch(..., { method: "POST" })` tanpa
badan dan tanpa jenis. Sekarang menyebut `application/json`. Semua POST lain
sudah menyebutnya sejak awal.

## 3. Kunci sesi pindah ke cookie

`internal/api/token.go`, `gui/app/engine.ts`

Query adalah tempat paling mudah bocor: Referer, riwayat browser, tangkapan
layar, salin-tempel. Sejak GUI disajikan engine sendiri, `EventSource`,
`<video src>`, dan tautan unduh semuanya **satu asal** — dan cookie ikut terkirim
sendiri oleh ketiganya. Itulah yang dulu memaksa kuncinya ditaruh di query.

Sekarang: `clipper_session`, `HttpOnly; SameSite=Strict; Path=/`, tanpa masa
berlaku (cookie sesi — hilang saat jendela ditutup, sama seperti kuncinya
sendiri yang dibuat baru tiap engine dijalankan).

**`/api/` tidak lagi menerima `?token=` sama sekali**, bahkan ketika kuncinya
benar. Query hanya dibaca sekali, di halaman non-`/api/` yang dibuka shell, untuk
ditukar jadi cookie. Kunci salah di alamat halaman tidak menghasilkan cookie apa
pun — kalau tidak, siapa pun bisa memasang cookie pilihannya sendiri.

`eng()` di `gui/app/engine.ts` **berhenti menempelkan kunci** ke setiap alamat.
Ia tetap wajib dipakai untuk semua pemanggilan engine, tapi alasannya sekarang
tinggal satu: alamat engine baru diketahui saat halaman dibuka, sebab portnya
acak begitu aplikasi terpasang. `sessionStorage` untuk kunci ikut dibuang —
cookie yang menyimpannya, dan JavaScript memang tidak boleh bisa membacanya.

**Akibat yang harus diketahui:** menjalankan `clipper serve -token on` dari
checkout lalu memakai GUI `npm run dev` di `:3000` tidak lagi berfungsi penuh —
cookie `SameSite=Strict` tidak terkirim lintas-asal. Itu tidak menghalangi
apa pun: dari checkout kuncinya memang mati secara bawaan, dan alur yang dipakai
aplikasi jadi adalah satu asal. Bila suatu saat perlu, jalurnya lewat header
`X-Clipper-Token` yang tetap diterima.

## 4. Path berawalan tanda hubung

`internal/ffmpeg/ffmpeg.go` — `CLIPath()`.

Tidak ada shell di seluruh engine (`os/exec` tanpa `sh -c`), jadi injeksi shell
memang tidak berlaku. Yang berlaku: ffmpeg membaca argumen berawalan `-` sebagai
flagnya sendiri, dan path itu datang dari klien (`/api/probe?path=…`,
`/api/frame?path=…`, yang keduanya tidak mem-`Stat` berkasnya lebih dulu).

ffmpeg tidak mengenal `--` sebagai penanda akhir flag, jadi yang dipakai adalah
bentuk yang selalu dipahami: **path mutlak**. `-report` jadi
`/folder/kerja/-report` — tidak lagi diawali tanda hubung, menunjuk berkas yang
sama persis. Path yang tidak diawali `-` tidak disentuh sama sekali, jadi tidak
ada perilaku lama yang berubah.

Semua argumen path ke ffmpeg/ffprobe lewat sana. whisper **tidak** ikut: path
yang diterimanya (WAV cache, prefix keluaran, model) semuanya disusun engine,
bukan datang dari klien — dan mengimpor paket `ffmpeg` dari `transcribe` hanya
untuk itu justru menambah keterikatan yang tidak perlu.

## 5. Pengetatan lapis kedua

- **CSP** untuk halaman yang disajikan engine (`internal/api/webui.go`), plus
  `X-Content-Type-Options: nosniff` dan `Referrer-Policy: no-referrer`.
- **Semua badan JSON lewat `readJSON`** (batas 1 MB). Enam handler tadinya
  memakai `json.NewDecoder(r.Body)` langsung — tanpa batas ukuran sama sekali.

### CSP: satu kelonggaran, dan kenapa

`script-src` terpaksa memuat `'unsafe-inline'`. Ini **diperiksa, bukan dikira**:
`next build` menaruh tujuh `<script>` inline di `index.html` untuk data
hidrasinya. Nonce tidak bisa dipakai (berkasnya statis, engine tidak menyusun
HTML-nya) dan hash berganti tiap build. Yang tetap didapat: skrip dari **host
luar** ditolak, dan `'unsafe-eval'` tetap tidak ada — keduanya sudah diperiksa
tidak dibutuhkan pada `gui/out` hari ini.

`img-src` mengizinkan http/https: gambar artikel di tab kartu memang datang dari
medianya. Itu **isi**, bukan tampilan — pengecualian yang disebut `notes/29`
secara khusus.

Nilai sampingannya untuk `notes/29`: aturan "nol alamat luar di antarmuka"
berhenti bergantung pada ingatan. Satu tautan CDN yang tak sengaja ikut akan
langsung gagal memuat, bukan diam-diam bekerja di mesin yang punya internet lalu
putus di mesin yang tidak.

## Cara membuktikannya masih benar

```bash
cd engine && go test ./...          # semua paket lulus
./build.sh && ./bin/clipper serve -token on
```

Lalu, dengan `T` = token dari `data/engine.json` dan `B=http://127.0.0.1:8787`:

| Permintaan | Harus |
| --- | --- |
| `curl -D- "$B/?token=$T"` | `Set-Cookie: clipper_session=…; HttpOnly; SameSite=Strict` |
| `curl -b "clipper_session=$T" "$B/api/config"` | 200 |
| `curl "$B/api/config?token=$T"` | **401** — query tidak diterima lagi |
| `curl "$B/api/config"` | 401 |
| `curl -H 'Host: penyerang.example' -b "clipper_session=$T" "$B/api/config"` | 403 |
| `curl -X POST -H 'Content-Type: text/plain' -b "clipper_session=$T" -d '{}' "$B/api/jobs"` | 415 |
| `curl -X POST -H 'Content-Type: application/json' -b "clipper_session=$T" -d '{}' "$B/api/jobs"` | 400 — lolos penjaga, ditolak validasi |
| `curl -H 'Sec-Fetch-Site: cross-site' -b "clipper_session=$T" "$B/api/config"` | 403 |
| `curl -H 'Origin: https://asing.example' -b "clipper_session=$T" "$B/api/config"` | 403 |
| `curl -D- "$B/"` | memuat `Content-Security-Policy` |
| `curl "$B/api/health"` | 200 — tetap terbuka untuk shell |

Kesebelasnya sudah dijalankan pada 5 Agustus 2026 dan hasilnya sesuai.

## Yang BELUM diuji, dan harus

**GUI belum pernah benar-benar dibuka di browser dengan CSP dan cookie yang
baru** — di mesin pengembangan ini tidak ada Chrome yang terpasang. Yang sudah
dibuktikan hanyalah yang bisa diperiksa tanpa merender: `gui/out` tidak memuat
`eval`/`new Function`, tidak memuat alamat luar selain di teks pesan galat, dan
tidak lagi menyusun `token=` ke URL mana pun.

Uji yang menutup sisanya, dan yang harus dijalankan sebelum rilis:

1. `./build.sh && ./bin/clipper serve -token on`, buka alamat "open:" di browser;
2. buka konsol pengembang — **tidak boleh ada pelanggaran CSP**;
3. jalankan satu job sampai selesai: itu menyentuh SSE (`EventSource`),
   `<video src>`, dan tautan unduh sekaligus — tepat ketiga hal yang dulu memaksa
   kunci ditaruh di query;
4. buka tab kartu berita: itu menguji `img-src` terhadap gambar dari media;
5. **cabut jaringan, buka aplikasi** — uji "nol alamat luar" dari `notes/29`.

## Yang tetap dibiarkan

Tabel "sengaja dibiarkan" di `28-pengerasan-sebelum-pentest.md` tidak berubah:
`/api/browse` menelusuri disk, `/api/requirements/path` menunjuk program apa pun,
engine menghubungi alamat dari klien, pemasang tidak ditandatangani. Semuanya
punya alasan yang ditulis di sana, dan tiga yang pertama ada **di balik kunci
sesi** — yang sejak sekarang tidak lagi lewat bilah alamat.

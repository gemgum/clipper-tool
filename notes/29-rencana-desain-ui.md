# Rencana perombakan tampilan

Ditulis 5 Agustus 2026, **belum dikerjakan**. MVP sudah jalan di Windows, jadi
pekerjaan berikutnya adalah tampilan. Berkas ini menyimpan keputusan yang sudah
diambil dan batasan yang harus diketahui sebelum ada yang menggambar.

## Keadaan sekarang

```
gui/           Next.js, ekspor statis, disajikan engine
               797 baris CSS tulisan tangan · 180 kelas · 294 className
               0 dependensi UI (hanya next + react)
               font: system-ui (berbeda-beda di tiap komputer)
               ikon: emoji
               tidak ada gui/public — nol aset lokal
```

## Keputusan: pindah ke Tailwind

Alasannya bukan Tailwind lebih baik dari CSS tulisan tangan, melainkan
**ekosistem referensinya**. Contoh tampilan yang bisa ditiru hari ini hampir
selalu berbentuk kelas Tailwind; kalau proyeknya CSS sendiri, tiap referensi
harus diterjemahkan dulu — sama mahalnya dengan membuat dari nol. Repo situs
(`gemgum.github.io`) juga sudah memakainya, jadi satu cara berpikir untuk dua
proyek.

**Yang mahal bukan memasangnya, melainkan berhenti di tengah.** Dua sistem yang
hidup berdampingan memaksa tiap suntingan berikutnya memilih "pakai yang mana",
sementara CSS lama terus tumbuh dan aturan mati menumpuk.

Karena itu: **kerjakan bersamaan dengan perombakan tampilan, sekali jalan.**
Perombakan menulis ulang markup-nya toh, jadi migrasinya nyaris gratis.
Memigrasikan lebih dulu berarti menerjemahkan gaya yang sebentar lagi dibuang.

Satu langkah yang membuat masa peralihan bertahan: **pindahkan 10 variabel warna
di `globals.css` jadi token `@theme` lebih dulu**, dengan nilai yang sekarang.
Halaman lama dan baru tetap bicara warna yang sama, jadi tidak terlihat seperti
dua aplikasi. Nilainya tinggal ditimpa angka dari Figma nanti.

## Batasan yang harus diketahui desainernya

Empat hal ini sering dilanggar referensi, dan ketiganya baru ketahuan setelah
kode ditulis:

1. **Jendela bisa diubah ukurannya** — minimal 900×600 (`tauri.conf.json`).
   Frame lebar tetap perlu aturan: apa yang melar, apa yang tetap, apa yang
   turun.
2. **Dua bahasa.** Semua teks dari kamus i18n, dan bahasa Indonesia rata-rata
   15–20% lebih panjang. Tombol yang pas untuk "Install" harus tetap muat untuk
   "Pasang komponen".
3. **Tidak ada internet.** Tidak boleh ada satu pun alamat luar di antarmuka —
   bukan font, ikon, gambar, maupun skrip. Ujinya sederhana dan menangkap
   seluruh kelasnya: **cabut jaringan, buka aplikasi.** Kalau ada yang berubah,
   masih ada yang menggantung. (Pengecualian sah: gambar artikel di tab kartu —
   itu isi, bukan tampilan.)
4. **Dua area terkunci ke engine.** Penyeretan jangkar subtitle di ruang
   1080×1920 dan pratinjau kartu harus mencerminkan perhitungan engine PERSIS;
   kalau tidak, yang terlihat berbeda dengan hasil render. Bingkai dan kontrol
   di sekitarnya bebas diubah, angkanya tidak.

## Bentuk serahan Figma yang terpakai

Tautan Figma tidak bisa dibaca agen (butuh login, kanvasnya bukan halaman). Yang
bekerja:

- **PNG 2x per layar**, termasuk keadaan: kosong, sedang berjalan, galat, teks
  panjang. Aplikasi ini sebagian besar waktunya "sedang berjalan" — desain dari
  layar diam biasanya melupakan itu.
- **Token sebagai teks** (warna hex, radius, skala jarak, ukuran & tebal huruf),
  bukan diukur dari gambar.
- **Pemetaan frame → halaman**: `/` klip, `/news` kartu, `/requirements`.

Cakupan yang paling berhasil: **token + dua-tiga layar acuan**, lalu sistemnya
diterapkan ke sisanya. Menyalin pixel-perfect tiap halaman menghasilkan tampilan
yang kaku begitu jendelanya diubah ukuran.

Mulai dari **`/requirements`**: halaman itu paling banyak berisi keadaan
(hijau/merah/sedang mengunduh/gagal), jadi sistem desainnya teruji sejak awal.
Mulai dari halaman paling sepi berarti masalahnya baru ketahuan di halaman
terakhir.

## Aset: harus lokal

Belum ada `gui/public` sama sekali. Yang akan perlu diunduh ke sana:

| Aset | Catatan |
| --- | --- |
| Font antarmuka | `.woff2` + `@font-face`. Jangan tertukar dengan `assets/fonts` — itu font subtitle untuk libass di dalam video. |
| Ikon | lihat di bawah |
| Gambar/ilustrasi | SVG dari Figma |

Isi `gui/public/` ikut ke `gui/out` saat build, lalu disajikan engine — jadi
otomatis masuk ke pemasang.

## Ikon: emoji harus diganti

Bukan soal selera. Emoji ✂️ dan 📁 tampil sebagai **kotak kosong** di jendela
Tauri Linux karena tidak ada font emoji — sudah terbukti, bukan dugaan. Emoji
bergantung pada font sistem, persis hal yang tidak boleh diandalkan aplikasi
yang harus tampil sama di mana-mana.

**Kit yang sudah dikumpulkan** (`~/documents/design_ui`): Xmind Desktop UI Kit
(Community), 172 SVG + 15 PNG. Hasil pembacaan 5 Agustus 2026:

- Hanya **±19 ikon** yang relevan untuk Clipper (play, pause, folder, camera,
  close, check, warning, update, reset, add, clock). 119 berkas di antaranya
  soal peta pikiran — cabang, boundary, struktur — yang tidak ada padanannya di
  sini.
- Warna dipaku `stroke="#27292A"` (abu gelap): di antarmuka gelap Clipper nyaris
  tak terlihat. Perlu diganti `currentColor`.
- Ukuran 16×16, stroke 1.5, konsisten — asal ikon dari sumber lain mengikuti.
- **Lisensi belum diperiksa.** Ini kit milik produk lain dari Figma Community.
  Untuk belajar wajar; untuk dibagikan sebagai aplikasi publik, ketentuannya
  perlu dibaca dulu.

Pembanding: **lucide-react**, lisensi ISC (bebas komersial), satu dependensi,
gaya seragam, menutup seluruh kebutuhan Clipper (gunting, unduh, folder, berkas,
putar, setelan, centang, peringatan, galat, muat-ulang, tautan-luar, panah,
hapus, tambah), dan sudah dipakai repo situs.

Saran: pakai set lengkap sebagai dasar, ambil dari kit lain hanya bila ada
bentuk yang benar-benar pas dan tidak ada padanannya.

## Gaya & warna (ditetapkan 5 Agustus 2026)

Acuannya kit Xmind. Warna diambil dengan mengukur gambarnya, bukan dikira-kira.

### Gayanya: flat 2.0

Bukan flat murni, bukan Material. Buktinya dari gambar acuan:

- **ada bayangan, tapi sangat halus** — tepi kartu melandai
  `#FEFEFE → #FDFDFD → #FBFBFB → #F7F7F7 → #F3F3F3 → #ECECEC` selebar ±12 px
  sebelum garisnya; panel biasa tidak berbayang sama sekali;
- **warna solid tanpa gradien** — tombol biru tetap `#0072EC` dari atas ke bawah.

Ciri lain yang ikut: dasar hampir tanpa warna (putih 71%, abu 11%), ikon garis
16×16 stroke 1,5, pemisah berupa garis rambut 1 px, dan warna dipakai untuk
MAKNA — biru = interaktif, deret warna = kategori.

Satu kalimat untuk desainer: *flat 2.0, dasar netral, satu aksen biru, ikon
garis, kedalaman hanya untuk elemen yang melayang.*

Cocok untuk Clipper bukan karena selera: aplikasi ini penuh **keadaan**
(hijau/merah/sedang mengunduh, skor, bilah kemajuan). Dasar yang netral membuat
warna jadi sinyal — kalau ada yang hijau, itu berarti sesuatu.

### Palet terang

| Token | Gelap (sekarang) | Terang |
| --- | --- | --- |
| `--bg` | `#0f1115` | `#F5F5F5` |
| `--panel` | `#171a21` | `#FFFFFF` |
| `--panel2` | `#1e222b` | `#FAFAFA` |
| `--border` | `#2a2f3a` | `#E2E2E2` |
| `--text` | `#e6e9ef` | `#27292A` |
| `--muted` | `#9aa4b2` | `#6B6E70` |
| `--accent` | `#4f8cff` | `#0072EC` |
| `--good` | `#3ecf8e` | `#20C07D` |
| `--warn` | `#f5a623` | `#FF9D43` |
| galat | `#ff6b6b` | `#FF4747` |

Warna kategori (dari deret Tag & Priority): `#FF4747` `#FF9D43` `#F7CF00`
`#20C07D` `#577CFF` `#6A4CCC`.

**Merah `#F9423A` adalah LOGO Xmind, bukan warna aksinya.** Yang dipakai untuk
tombol aktif dan isian terpilih di kit itu adalah biru `#0072EC`. Memakai merah
sebagai aksen utama akan bentrok dengan warna galat — pengguna tidak bisa
membedakan "tombol utama" dari "ada yang salah".

### Kontras — yang tidak boleh jadi teks

Diukur di atas putih (ambang 4,5:1 untuk teks biasa):

| | Rasio | Boleh untuk |
| --- | --- | --- |
| `#27292A` | 14,6:1 | semua |
| `#6B6E70` | 5,1:1 | semua |
| `#0072EC` | 4,6:1 | semua (pas-pasan) |
| `#FF4747` | 3,4:1 | teks besar / ikon |
| `#A8A9A9` | 2,4:1 | **jangan untuk teks** |
| `#20C07D` | 2,4:1 | **jangan untuk teks** |
| `#FF9D43` | 2,1:1 | **jangan untuk teks** |

Di tema gelap hijau & oranye aman sebagai teks; di tema terang **tidak**. Di
sana keduanya hanya boleh jadi lampu status — titik, bilah, latar. Ini jenis
kesalahan yang lolos dari mata saat mendesain dan baru terasa setelah dipakai
lama.

### Keputusan

- **Dua tema, terang dan gelap.** Ditetapkan sejak awal supaya tidak perlu
  menyisir ulang semua halaman nanti; struktur tokennya sudah siap.
- **Area pratinjau belum diputuskan.** Kalau nanti harus dibedakan dari
  sekitarnya, coba dulu bentuk melayang (kartu berbayang) sebelum memaksanya
  tetap gelap.

## Bug yang ditemukan, dikerjakan bersama pembaruan tampilan

Dilaporkan 5 Agustus 2026 dari hasil render sungguhan di Windows.
**Nomor 1, 2, dan 3 sudah SELESAI hari yang sama.**

Cara mengukurnya, supaya bisa diulang: `.ass` dibuat lewat penulis engine
sendiri, dirender `ffmpeg -f lavfi -i color=black:s=1080x1920 -vf
subtitles=…:fontsdir=…`, dan pratinjaunya direproduksi 1:1 di Chrome headless
pada kanvas 1080×1920 yang sama. Keduanya lalu dibandingkan per piksel. Tanpa
mengukur di ruang yang SAMA, semua selisih di bawah ini tidak akan terlihat.

### 1. Subtitle hasil render tidak memakai Montserrat SAMA SEKALI — SELESAI

Dugaan awal di catatan ini ("libass merender instance bawaan Thin") **meleset,
dan yang sebenarnya lebih buruk.** Diukur 5 Agustus 2026:

`assets/fonts/Montserrat.ttf` memang font variabel (wght 100…900, bawaan 100),
tapi masalahnya bukan bobot melainkan **nama**: berkas itu menyebut dirinya
family **"Montserrat Thin"**, sedangkan engine menulis `Fontname: Montserrat` ke
`.ass`. libass karena itu tidak pernah mengenalinya.

Buktinya tidak bisa dibantah: render dengan berkas itu di `fontsdir` **identik
byte demi byte** dengan render tanpa font sama sekali.

```
fontsdir berisi Montserrat[wght].ttf → sha256 1ff7985c…
fontsdir kosong                      → sha256 1ff7985c…
tanpa fontsdir                       → sha256 1ff7985c…
```

Jadi setiap subtitle yang pernah dirender proyek ini memakai **font cadangan
sistem**, bukan Montserrat — dan di mesin pengguna yang font sistemnya berbeda,
hasilnya berbeda pula.

**Perbaikan yang dikerjakan:** `fetch-fonts.sh` kini mengunduh **dua face
statis** (`Montserrat-Regular.ttf` + `Montserrat-Bold.ttf`, dipaku ke tag
`v7.222` repo hulu) di samping yang variabel. Yang variabel TETAP ada dan tetap
dipakai kartu berita — kartu dirender Chrome, yang memang memahami sumbu
variabel. Ketiganya hidup berdampingan di satu folder tanpa saling ganggu
(diuji).

**Kenapa dua face, bukan satu** — diukur pada teks yang sama:

| Isi `assets/fonts` | Bold=1 | Bold=0 |
| --- | --- | --- |
| Regular saja | 3.755 tinta (penebalan buatan, terlalu tipis) | — |
| Bold saja | 6.056 | 6.056 (togglenya mati) |
| **Regular + Bold** | **6.056** | **3.097** |

**Jangan memakai `Montserrat-SemiBold`**: family di dalamnya "Montserrat
SemiBold", jadi ia tidak akan pernah cocok dengan "Montserrat" — persoalan yang
sama dengan yang variabel.

Anton dan Bebas Neue sudah statis, dan keduanya memang hanya terbit satu bobot.

### 2. Pratinjau menggambar subtitle 38% TERLALU BESAR — SELESAI

Urutan "perbaiki font dulu, baru ukur" ternyata benar dan menyelamatkan: setelah
nomor 1 beres, selisihnya bukan "sedikit lebih naik" melainkan **selisih
UKURAN**, dan besarnya berbeda tiap font.

Sebabnya satu kalimat: **`.ass` mengartikan ukuran font sebagai tinggi KOTAK
font (`winAscent + winDescent`), CSS mengartikannya sebagai ukuran em.**

Diukur, bukan diturunkan dari teori — `Fontsize` di `.ass` disapu sampai lebar
tintanya sama dengan `font-size: 72px` di Chrome:

```
Fontsize 72 →  lebar 352      Fontsize 96 →  lebar 470
Fontsize 92 →  lebar 450      Fontsize 99 →  lebar 484   ← 72px Chrome = 485
```

72/99 = 0,727, dan `upem/(winAscent+winDescent)` Montserrat = 1000/1379 = 0,725.
Cocok. Angka itu berbeda tiap font: **Montserrat 0,725 · Bebas Neue 0,769 ·
Anton 0,577 · DejaVu Sans 0,859** — jadi ia tidak bisa jadi satu tetapan di GUI.

Akibat kedua dari definisi yang sama: **jarak antarbaris libass persis sebesar
`Fontsize`**, berapa pun fontnya (diukur: dua baris ukuran 72 berjarak 72 px).
Faktor `1.17` yang dulu ada di GUI diukur ketika font bawaan tidak pernah
benar-benar dipakai — jadi ia mengukur font cadangan sistem.

**Perbaikan yang dikerjakan:** engine membaca `head.unitsPerEm` dan
`OS/2.usWinAscent/Descent` dari berkas fontnya sendiri
(`engine/internal/api/fontmetrics.go`, tanpa pustaka luar) dan melaporkannya
sebagai `scale` di `/api/fonts` dan `/api/font-check` — untuk font bawaan MAUPUN
font sistem. GUI memakainya: `font-size = size × scale`,
`line-height = 1/scale`, `blockH = size × barisan` (bukan `× 1.17`).

Hasilnya, diukur dengan render libass dan render Chrome berdampingan di ruang
1080×1920 yang sama:

| | sebelum | sesudah |
| --- | --- | --- |
| selisih lebar tinta | +133 px | **0 px** |
| selisih tepi kiri/kanan | −65 / +68 px | **0 / 0 px** |
| selisih tepi atas | — | −4 px |

**Sisa −4 px belum ditutup**, dan sengaja tidak ditambal: itu selisih cara
browser dan libass menaruh garis dasar di dalam kotak baris. Pada pratinjau
setinggi 480 px, 4 px di ruang 1920 = **1 piksel layar**. Kalau nanti mau
dihabiskan, jalannya menjangkarkan teks pratinjau pada garis dasar (butuh
`winAscent` dikirim juga), bukan menambah offset karangan.

### 2b. Pratinjau memuat face tebal yang sama dengan render

Bagian dari nomor 2 dan ikut selesai: GUI dulu memasang SATU `@font-face` tanpa
penyebut bobot, lalu memakai `font-weight: 800`. Browser karena itu menebalkan
sendiri face tegaknya — penebalan buatan yang tidak sama dengan face Bold
sungguhan yang dipakai libass.

Sekarang `/api/font-file` menerima `?weight=`, GUI memasang dua `@font-face`
(400 dan 700), dan `.suboverlay` memakai `font-weight: 700` — sama dengan arti
`Bold=1` di `.ass`.

### 3. Grid untuk menggeser subtitle — SELESAI

Permintaan: menggeser subtitle terasa tidak pasti. Tambahkan grid supaya
penempatannya bisa diulang, **tetap dengan sumbu X dan Y yang terlihat**.

Ukurannya tidak diputuskan di sini melainkan **dijadikan pilihan**, sebab "rasa"
grid cuma bisa dinilai dengan mencoba: `mati · 10 · 20 · 24 · 40`, bawaan **20**
(54×96 kotak). Pilihannya ada di bilah tombol pratinjau, sebelah "garis tengah
terus".

Yang berlaku sekarang:

- **magnet ke garis tengah MENANG atas grid.** Ini bukan selera: titik tengahnya
  sering bukan kelipatan grid — pada grid 24, X tengah 540 tidak terjangkau sama
  sekali (540/24 = 22,5), dan pada grid 20 jangkar tengah Y 888 juga tidak. Kalau
  grid dipasang lebih dulu, "tepat di tengah" jadi mustahil dicapai — persis
  kemampuan yang catatan ini minta jangan dibuang;
- **Alt = abaikan grid**, dan **tombol panah menggeser 1 px** (Shift = 10).
  Overlay subtitle karena itu `tabIndex={0}` — panahnya butuh fokus;
- **sumbu X/Y digambar pada posisi subtitle sekarang**, warna aksen supaya
  berbeda dari garis tengah yang putih, **berikut angkanya** (`540 · 888`) selama
  diseret. Tanpa angka, "pasti" hanya terasa dan tidak bisa diulang di klip
  berikutnya.

Dua keputusan kecil yang diambil saat mengerjakan:

- **Mesh grid tidak digambar di bawah 20.** Pratinjau selebar 270 px berarti
  kotak grid 10 hanya 2,5 px di layar — yang terlihat bukan grid melainkan kabut
  kelabu di atas frame videonya. Menempelnya tetap jalan; cuma gambarnya yang
  tidak menolong.
- **Mesh-nya satu elemen berlatar berulang**, bukan puluhan `<div>`: pada grid 10
  itu 108×192 kotak, dan menggambarnya satu per satu berarti ratusan elemen yang
  dihitung ulang tiap gerakan seretan.

## Yang bisa dikerjakan tanpa menunggu desain

Tiga hal ini tidak menuntut keputusan desain, dan nilainya tidak hangus walau
rancangannya berubah:

1. ~~pasang Tailwind + pindahkan 10 warna jadi token `@theme`~~ — **SELESAI**;
2. ~~ganti emoji dengan ikon sungguhan~~ — **SELESAI**;
3. buat `gui/public/` beserta aturan "nol alamat luar" — **ditunda, lihat bawah**.

### 1. Tailwind + token warna — selesai

Tailwind v4 lewat `@tailwindcss/postcss`; tidak ada `tailwind.config.js`,
seluruh setelannya di CSS. Sepuluh warna jadi token `@theme`
(`--color-bg`, `--color-panel`, …) dan nama pendek lama (`--bg`, `--panel`, …)
dipetakan ke token itu — satu tempat, satu kebenaran, dan CSS tulisan tangan di
bawahnya tidak disentuh sama sekali. Itu memang maksudnya: perombakan akan
menulis ulang markupnya, jadi menerjemahkan gaya lama sekarang adalah pekerjaan
yang hangus dua kali.

**Tailwind diimpor TANPA preflight**, dan ini ditemukan lewat pengukuran, bukan
kehati-hatian: `@import "tailwindcss"` yang biasa menggeser **12,38% piksel**
halaman (tombol kehilangan tebalnya, seluruh kolom turun ~12 px). Dengan
mengimpor `theme.css` + `utilities.css` saja, selisihnya **0,00% — nol piksel**.
Preflight dinyalakan nanti bersama perombakan, ketika markupnya memang ditulis
ulang dan pergeseran itu tidak jadi kejutan.

### 2. Emoji → ikon — selesai

`lucide-react` (ISC, 1 dependensi) seperti yang direncanakan. Yang berubah:
`gui/app` kini memuat **nol** lambang yang butuh font emoji.

Yang penting diketahui sebelum menambah lambang baru: **tidak semua emoji
bermasalah.** Yang butuh font emoji adalah lambang dengan `Emoji_Presentation=Yes`
— ✅ ⛔ ⬇ ⬆ 🔄 👁 🔎 📋 📁 🎬 🔗 ✏️ ✂️. Sedangkan **⚠ ✓ ✕ → ↗ ↓ ↑ ✗ adalah simbol
teks biasa** yang ada di font mana pun, dan semuanya DIBIARKAN. Mengganti yang
tidak perlu hanya menambah komponen tanpa memperbaiki apa pun.

Karena itu pekerjaannya terbelah dua, dan pembelahan itu bukan pilihan
melainkan keharusan:

- **Label** (tombol, status) → komponen ikon di JSX, lambangnya dibuang dari
  kamus i18n;
- **Baris log** → tetap teks, sebab log adalah `string[]` yang dirender sebagai
  `<div>{teks}</div>` dan tidak bisa memuat komponen. Lambangnya diganti yang
  aman: ✅→✓, ⬇→↓, ⬆→↑, 🔄→↻, dan 🔎/📋/🎬 dibuang begitu saja;
- **Isi `<option>`** tidak bisa memuat komponen sama sekali (`fontOther`,
  daftar model) — di situ lambangnya dibuang atau memakai ✓/✗ yang aman.

Biayanya 2 kB pada bundel halaman utama.

Cara memeriksa aturannya masih dipegang — hasil kosong berarti aman:

```bash
grep -rnP '[\x{1F300}-\x{1FAFF}\x{2705}\x{26D4}\x{2B06}\x{2B07}\x{270F}\x{2702}]' \
  gui/app --include=*.tsx --include=*.css
```

### 3. `gui/public/` — ditunda, dan alasannya

Belum dibuat, sebab **tidak ada yang perlu ditaruh di sana**: ikon sekarang
datang dari lucide (komponen JS, bukan berkas), dan tidak ada gambar. Folder
kosong tidak bisa dilacak git, jadi membuatnya sekarang berarti menambah nol.

Aturan "nol alamat luar" sendiri **sudah dijaga mesin** sejak `notes/30`: header
`Content-Security-Policy` yang disajikan engine menolak skrip, gaya, dan font
dari host luar, jadi satu tautan CDN yang tak sengaja ikut akan gagal memuat
alih-alih diam-diam bekerja di mesin yang punya internet lalu putus di mesin
yang tidak.

Yang membuat folder ini benar-benar diperlukan adalah **font antarmuka**, dan
itu belum bisa diputuskan agen: antarmuka masih memakai `system-ui`, yang
persis penyakit yang sama dengan emoji — tampilannya berbeda di tiap komputer.
Begitu fontnya dipilih (dari Figma atau sekarang), `gui/public/` dibuat bersama
berkas `.woff2`-nya, tidak sebelum itu.

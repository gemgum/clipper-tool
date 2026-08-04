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

### 1. Subtitle hasil render lebih kurus daripada pratinjau — SEBAB SUDAH PASTI

`assets/fonts/Montserrat.ttf` adalah **font variabel**:

```
sumbu wght: 100 … 900,  BAWAAN 100
```

libass tidak menerapkan sumbu font variabel, jadi ia merender **instance bawaan,
yaitu Thin (100)**. Pratinjau di browser menerapkan bold (700) karena browser
memahami font variabel. Itulah selisihnya.

Perbaikan: unduh berkas **statis** (`Montserrat-Bold.ttf` / `SemiBold`) di
`fetch-fonts.sh`, jangan yang `Montserrat[wght].ttf`. Periksa juga dua font
lain — Anton dan Bebas Neue — apakah statis atau variabel.

### 2. Subtitle hasil render sedikit lebih naik daripada pratinjau

Engine menjangkarkan dengan `\an8` — tepi ATAS blok teks — di titik (x, y),
sedangkan pratinjau menghitung tinggi baris dengan angka tetap
`subSize × 1.17 × jumlahBaris`.

Kemungkinan besar ini **akibat lanjutan dari nomor 1**: Thin dan Bold punya
metrik vertikal berbeda, jadi tinggi baris sebenarnya tidak sama dengan yang
diperkirakan pratinjau. **Perbaiki font dulu, baru ukur ulang selisihnya** —
jangan menambal angka 1.17 sebelum penyebab pertama hilang, nanti tambalannya
justru jadi salah.

### 3. Grid untuk menggeser subtitle

Permintaan: menggeser subtitle terasa tidak pasti. Tambahkan grid supaya
penempatannya bisa diulang, **tetap dengan sumbu X dan Y yang terlihat**.

Yang perlu dicoba dulu ukurannya (ruang 1080×1920):

| Grid | Kolom × baris | Rasa |
| --- | --- | --- |
| 10 px | 108 × 192 | halus, nyaris seperti sekarang |
| 20 px | 54 × 96 | kandidat awal |
| 24 px | 45 × 80 | pas untuk kelipatan 8 |
| 40 px | 27 × 48 | kasar, penempatan sangat pasti |

Catatan rancangan:

- **magnet ke garis tengah yang sudah ada jangan dibuang** — itu yang membuat
  "tepat di tengah" bisa dicapai tanpa membidik;
- sediakan jalan keluar untuk penempatan halus: tahan Alt = abaikan grid, atau
  tombol panah menggeser 1 px. Grid harus jadi bawaan, bukan pagar;
- sumbu X/Y ditampilkan sebagai garis, dan sebaiknya angkanya ikut terlihat saat
  diseret — tanpa angka, "pasti" hanya terasa, tidak bisa diulang.

## Yang bisa dikerjakan tanpa menunggu desain

Tiga hal ini tidak menuntut keputusan desain, dan nilainya tidak hangus walau
rancangannya berubah:

1. pasang Tailwind + pindahkan 10 warna jadi token `@theme` (nilai sekarang);
2. ganti emoji dengan ikon sungguhan — ini perbaikan bug, bukan penataan;
3. buat `gui/public/` beserta aturan "nol alamat luar".

# Kartu Berita — Keputusan Desain

Fitur tab kedua: mengubah artikel berita jadi gambar siap posting.
Tanggal keputusan: 30 Juli 2026.

## Yang dipilih: "mode kartu", bukan "mode asli"

Ada dua cara membuat gambar dari berita:

| | Mode asli | **Mode kartu (dipilih)** |
|---|---|---|
| Cara | foto layar situs + potong | data artikel → template sendiri |
| Perlu crop | ya, geser manual | **tidak** |
| Iklan/popup | ikut terbawa | tidak ada |
| Teks di layar HP | sering kecil/terpotong | besar & utuh |
| Kesan otentik | kuat | lemah |

Percobaan nyata memotret artikel ANTARA membuktikan masalah mode asli: banner
iklan memakan ~14% bagian atas, judul terpotong di kanan, keterangan foto ikut
terpotong. Sebabnya situs berita menata halaman untuk lebar desktop — dipaksa ke
lebar HP, isinya meluber, dan tiap situs meluber dengan cara berbeda.

Mode kartu juga **lebih murah dibangun**: tidak butuh antarmuka geser-crop, yang
justru bagian paling makan waktu.

Mode asli belum dibuat. Bila nanti diperlukan (untuk kesan otentik), fondasinya
sama — paket `capture` yang sama — jadi tidak ada yang terbuang.

## Pencarian kata kunci: Google News RSS, bukan Puppeteer

Diuji langsung sebelum diputuskan:

| Jalur | Hasil uji |
|---|---|
| Scrape `google.com/search` tanpa JS | HTTP 200, **nol hasil** — cangkang `enablejs` |
| Scrape dengan Chrome asli (menjalankan JS) | **CAPTCHA di permintaan pertama**, dari IP rumah |
| `news.google.com/rss/search?q=` | **100 artikel**, media Indonesia, gratis, tanpa key |

Jadi Puppeteer tidak menyelesaikan apa pun di sini: yang menghalangi bukan
ketiadaan JavaScript, melainkan deteksi otomasi. Melawannya berarti merawat
lapisan penyamaran yang jadwal rusaknya ditentukan pihak lain — dan matinya
diam-diam, tanpa error, cuma "0 hasil".

Operator pencarian tetap berlaku di endpoint RSS (sudah diuji): `site:`,
tanda kutip untuk frasa persis, dan `when:7d` untuk batas waktu.

### Ganjalan: tautannya bukan alamat artikel

Google News mengembalikan pengalih `news.google.com/rss/articles/CBMi…`.
Dua jalan buntu yang sudah dicoba:

- **Ikuti redirect HTTP** → mendarat di halaman Google News, bukan artikelnya
- **Bongkar base64-nya** → **0 dari 6**; Google mengubah format, URL-nya tidak
  lagi ada di dalam kode itu

Yang berhasil: `chrome --headless --dump-dom`. Sekali buka, DOM-nya sudah memuat
`og:url` (alamat asli), tag og: lainnya, **dan** badan artikelnya — jadi satu
panggilan menggantikan dua pengunduhan yang biasanya dibutuhkan alur tempel-link.

`DumpDOM` mencoba **dua kali**: pengalihan JavaScript kadang belum selesai saat
anggaran waktu virtual habis, terutama pada Chrome yang baru dingin. Gejalanya
khas — keluar sukses tapi DOM nyaris kosong. Percobaan kedua dengan anggaran dua
kali lipat menyelesaikannya; ini ditemukan dari kegagalan sesekali saat uji.

### Resolusi di-cache, dan itu dipakai dua kali

Tombol "salin tautan" di daftar hasil meresolusi pengalihnya lebih dulu, supaya
yang tersalin adalah alamat medianya — bukan URL panjang milik Google yang tak
berguna saat dibagikan ke orang lain.

Hasil resolusi disimpan di `data/cache/resolusi` (kunci = sidik jari tautan
Google). Efeknya berlapis:

| | Waktu |
|---|---|
| Resolusi pertama (browser) | ~3–8 detik |
| Resolusi berikutnya (cache) | **0,01 detik** |
| Analisis tautan yang sudah diresolusi | **melewati browser sepenuhnya** — cukup unduh HTTP biasa |

Yang terakhir itu bukan kebetulan: `AmbilIsi` memeriksa cache resolusi lebih
dulu, dan bila ada, ia memakai jalur HTTP normal alih-alih memanggil Chrome
untuk kedua kalinya.

Catatan tampilan: hasil pencarian **tidak membawa gambar** (Google News tidak
menyertakannya). Gambarnya baru muncul setelah artikel diresolusi, jadi kartu
daftar dibiarkan tanpa bidang gambar alih-alih menampilkan kotak abu kosong.

## Kenapa RSS, bukan Google dorking

Rencana awal: menyisir hasil Google dengan operator pencarian. Ditolak karena
**scraping google.com/search pasti diblokir** — beberapa kueri otomatis, muncul
CAPTCHA, lalu IP kena batas. Memakai headless browser justru mempercepat
terdeteksi.

Yang dipakai: RSS media Indonesia (`encoding/xml`, stdlib). Feed memang
disediakan untuk dibaca mesin, formatnya stabil, dan tidak memblokir.

Bila nanti butuh pencarian kata kunci, pintunya adalah search API resmi (Google
CSE 100/hari gratis, atau Brave 2rb/bulan) — sintaks `site:` tetap bisa dipakai
di sana. **Jangan** scrape halaman pencarian.

## Kenapa Chrome di-exec, bukan Playwright

Mengikuti pola ffmpeg & whisper.cpp: engine memanggil biner yang sudah ada,
tidak memasang pustaka sendiri. Playwright/Puppeteer menyeret Chromium ~300 MB
sebagai unduhan wajib — bertabrakan dengan sasaran distribusi (lihat memori
proyek: idealnya satu `.exe` tanpa pemasangan).

Chrome/Edge praktis selalu ada; Edge bawaan Windows. Tambahan ukuran: 0 MB.

## Ganjalan WSL: chrome.exe tidak paham path Linux

Di WSL sering tidak ada Chromium Linux — yang ada Chrome/Edge **Windows**.
Program Windows tidak bisa menulis ke path Linux, jadi paket `capture`
menerjemahkan path lewat `wslpath`:

- Berkas keluaran ditulis ke `%TEMP%` Windows, lalu disalin ke tujuan Linux.
- Halaman HTML sementara juga ditaruh di `%TEMP%` Windows.
- **Folder profil (`--user-data-dir`) wajib di disk Windows.** Menaruhnya di
  `/tmp` Linux membuat Chrome mati dengan `LockFileEx: Incorrect function` —
  jalur `\\wsl.localhost` tidak mendukung penguncian berkas.

## Font ditanam sebagai data URI

Montserrat & Anton di-base64 ke dalam HTML kartu, bukan dirujuk lewat path.
Dua alasan: Chrome bisa berjalan di sisi Windows sementara font ada di sisi
Linux, dan supaya kartu tampil sama persis di komputer siapa pun.

## Nama media diambil dari daftar kurasi

Judul channel RSS tidak bisa dipercaya untuk badge kartu:

- ANTARA: `"Berita Terkini - ANTARA News"` → nama media di **belakang**
- CNN: `"CNN Indonesia | Berita Terkini"` → nama media di **depan**

Jadi `news.SumberBawaan` menyimpan nama kurasi, dan itu yang menang. Tebakan
dari judul channel hanya dipakai untuk feed yang ditempel sendiri oleh pengguna.

## LLM memilih, tidak menulis

Isi kartu dan caption **wajib verbatim** dari artikel. Alasannya: kartu membawa
badge nama media. Kalau LLM memparafrase dan maknanya bergeser, kita menerbitkan
klaim yang berubah di bawah nama media itu.

Penegakannya bukan lewat instruksi prompt, melainkan lewat **bentuk data**:

```
LLM membalas  : nomor paragraf  ({"kartu": 3, "caption": 7})
Engine ambil  : teks paragraf nomor itu dari artikel
```

Karena LLM tidak pernah memegang teks keluaran, mengarang jadi mustahil — bukan
sekadar dilarang. Nomor di luar jangkauan dibuang (`susun` di `news/pilih.go`);
bila tidak ada satu pun nomor sah, permintaan gagal, tidak menebak.

Hashtag satu-satunya yang tidak bisa berupa kutipan utuh. Jalan tengahnya: LLM
memilih **kata kunci yang memang tertulis di artikel**, lalu engine memeriksa
kata itu benar-benar ada sebelum mengubahnya jadi tagar (`Isi.Tagar`).

Yang boleh berupa teks bebas dari LLM hanyalah "alasan" pada peringkat — itu
hanya penjelasan untuk manusia di GUI, tidak pernah ikut terbit.

## Model lokal sering setengah mengerjakan

Uji 8 artikel ANTARA dengan qwen2.5: **4 gagal total**. Balasannya berbentuk
JSON sah, `kartu` & `caption` terisi nomor yang benar, tetapi `"peringkat": []`
— model melewatkan tugas panjang "nilai SEMUA paragraf".

Engine dulu menolak balasan seperti itu, padahal jawabannya sudah bisa dipakai.
Sekarang `susun()` **tidak pernah mengembalikan galat**; tiga lapis penanganan:

1. `minItems: 1` di JSON Schema — memaksa Ollama mengisi peringkat di hulu.
2. Paragraf yang tidak dinilai model dilengkapi skor heuristik (`hook.go`) dan
   ditandai `Sumber: "heuristik"`. Efek sampingnya bagus: **seluruh** paragraf
   kini muncul di GUI, jadi pengguna bisa memilih yang mana pun — sebelumnya
   paragraf yang dilewati model tidak bisa dijangkau sama sekali.
3. Nomor kartu/caption yang ngawur jatuh ke peringkat teratas.

Penggantian ini **tidak senyap**: `Pilihan.Catatan` menjelaskan berapa paragraf
yang dinilai model dan berapa yang otomatis, dan GUI menampilkannya. Inilah
batas yang dipakai terhadap notes/12 — yang dilarang adalah berpindah mesin
diam-diam, bukan melengkapi urutan tampilan secara terbuka.

Setelah perbaikan: **8 dari 8 berhasil**, dengan 2–8 paragraf dinilai model.

Catatan lain: qwen2.5 menjawab `kartu=0, caption=0` di 7 dari 8 artikel,
sehingga teks kartu dan captionnya kembar dan postingan jadi mubazir. Bila
kembar, caption digeser ke paragraf berperingkat berikutnya — tetap dipilih
dari artikel yang sama, dan pengguna bisa menggantinya sekali klik.

## Ekstraksi badan artikel

Go stdlib tidak punya parser HTML dan proyek ini tanpa dependency eksternal.
Yang dipakai: buang blok `<script>/<nav>/<footer>/…` beserta isinya, lalu petik
teks `<p>` (turun ke `<div>` berisi teks polos bila `<p>` terlalu sedikit), buang
blok pendek dan frasa sampah seperti "Baca juga".

Catatan RE2: mesin regexp Go tidak mendukung backreference, jadi pola
`<(a|b)>...</\1>` tidak bisa dipakai — dibuat satu regex per tag.

## Bingkai foto: geser & zoom

Hanya **fotonya** yang bisa diatur; tata letak teks di bawahnya tetap. Nilainya
disimpan di `card.Foto{GeserX, GeserY, Zoom}`, satuan piksel di ruang koordinat
kartu (lebar 1080).

Pratinjau di GUI memakai CSS yang **sama persis** dengan template render. Ini
harus dijaga: kalau `.foto img` di `card.go` diubah, `.foto-kotak img` di
`globals.css` wajib ikut diubah.

Pelajaran mahal saat membangunnya: versi pertama memakai
`min-width:100%; min-height:100%; width:auto; height:auto` untuk meniru
`object-fit:cover`. Hasilnya **pratinjau tidak cocok dengan render** — dengan
aturan itu penskalaan bergantung pada ukuran asli gambar relatif terhadap
kotaknya, sehingga kotak 1080px (render) dan kotak ±380px (pratinjau) memberi
zoom berbeda untuk nilai yang sama. `object-fit:cover` tidak punya masalah itu:
selalu memenuhi bingkai dengan rasio terjaga, apa pun ukuran kotak dan gambar.

Urutan transform `translate(...) scale(...)` disengaja — CSS menerapkannya dari
kanan, jadi jarak geser tidak ikut terkalikan zoom dan menyeret N piksel di
pratinjau sama dengan N piksel di hasil.

Zoom minimum 1 dan geseran dijepit ke `(zoom-1)/2 × ukuran bingkai`, di kedua
sisi. Di bawah itu foto lebih kecil dari bingkainya dan menyisakan celah kosong.
Konsekuensinya: pada zoom 1 foto tidak bisa digeser — GUI menyebutkannya
("Perbesar dulu agar foto bisa digeser") daripada membiarkan seretan terasa rusak.

## Rata teks

Judul & ringkasan bisa dirata kiri / tengah / kanan / kiri-kanan (`card.Rata`).
Dua hal yang sengaja diputuskan:

- **Garis aksen merah ikut berpindah** mengikuti perataan. Kalau tidak, ia
  menggantung sendirian di kiri sementara teksnya rata kanan.
- **Kaki kartu tidak ikut.** Nama domain di kiri dan ajakan baca di kanan tetap
  dua kolom, apa pun perataannya — itu bukan bagian dari blok teks.

Rata kiri-kanan pada judul besar memang merenggangkan jarak antar kata secara
mencolok (satu baris kadang cuma 2–3 kata). Itu sifat justify, bukan bug; GUI
memperingatkannya saat opsi itu dipilih daripada diam-diam melarang.

## Atribusi

Foto dan teks tetap milik media. Badge sumber dan domain di kaki kartu
**selalu ditampilkan**, bukan opsional — benar secara etika, sekaligus membuat
kartunya terlihat kredibel.

## Kenapa sinkron, bukan lewat antrian job

Satu kartu selesai ~1 detik. Mesin job + SSE dirancang untuk pekerjaan hitungan
menit (clipping video). Menambahkan kartu ke sana hanya menambah rumit tanpa
manfaat.

## Pita bawah: dicoba, lalu DICABUT

Masalahnya nyata: kartu menjangkarkan isinya ke bawah, sementara TikTok/Reels/
Shorts menimpa bagian bawah dengan caption, nama akun, dan bilah navigasi.
Diukur pada render 1080x1920, stempel sumber duduk di 93% dan kaki kartu di 99%
— keduanya tertutup saat diposting, padahal atribusi adalah janji fitur ini.

Percobaan perbaikannya: `safeBottomPercent = 15`, satu `padding-bottom` di
`body`, dan pitanya diambil seluruhnya dari tinggi foto supaya area teks tidak
ikut menyusut.

**Dicabut.** Kartu harus tetap terisi penuh sampai tepi 1080x1920 — ukuran
layar TikTok apa adanya. Pita kosong itu menukar masalah yang tidak terlihat
(isi tertutup UI aplikasi) dengan masalah yang terlihat di mana-mana: kartu
yang berongga, termasuk saat dilihat di luar aplikasi. Kanvas kembali diisi
penuh; `.photo` kembali memakai persen dan `layout()` dibuang.

Kalau nanti digarap lagi, arahnya bukan mengosongkan kanvas: **padatkan blok
isinya** (ukuran font paragraf, padding, jarak antar-elemen) supaya isi yang
penting naik ke atas 76% tanpa menyisakan rongga. Foto tetap 50%.

## Warna kartu diambil dari FOTO, bukan dari warna situsnya

Palet kartu dulu tetap: latar `#14171C`, kertas `#EFEBE1`, kuning `#E4B429`.
Semua kartu jadi kembar, dan itu keluhan yang wajar — yang membedakan dua berita
bukan cuma teksnya.

Dugaan pertama, ambil warna dari HTML situsnya, ternyata tidak bisa dipakai.
Dari lima media besar hanya satu yang menulis `<meta name="theme-color">`:

| kumparan | detik | CNN Indonesia | Kompas | Tempo |
| --- | --- | --- | --- | --- |
| `#00A5AF` | — | — | — | — |

Empat dari lima kartu akan jatuh ke warna bawaan — keseragaman yang sama, hanya
dengan lebih banyak kode. Fotonya selalu ada, dan warnanya memang milik berita
itu sendiri.

**Yang dipinjam hanya RONA dan KEPEKATANNYA, tidak pernah TERANGNYA.** Terang
ditetapkan palet (latar 9,4%, kertas 91%), jadi foto malam dan foto siang sama
terbacanya. `TestTextStaysReadableForEveryHue` menyapu seluruh lingkaran warna
karena ronanya datang dari foto orang lain — kita tidak bisa memilih yang aman
saja.

Kuning penanda TIDAK ikut berubah. Ia satu-satunya warna tetap, dan itulah yang
membuat kartu-kartu ini tetap terlihat satu keluarga walau latarnya berbeda.

Tiga keputusan di `toneOf` yang tidak kelihatan tapi menentukan:

- **Piksel kelabu, terlalu gelap, dan terlalu terang dibuang.** Yang paling
  banyak di sebuah foto justru yang tidak punya rona — bayangan, sorot lampu,
  langit putih. Tanpa saringan ini hampir semua foto "dominan abu-abu".
- **Tiap piksel menyumbang sebesar kepekatannya.** Satu jaket merah menyala
  lebih menentukan kesan sebuah foto daripada seluas apa pun dinding krem.
- **Kelompok rona dinilai bersama tetangganya.** Rona hangat sebuah ruangan
  tersebar di 0-45 derajat; tanpa ini ia bisa kalah oleh satu bilah sempit yang
  pekat, dan warna kartu diambil dari hal terkecil di fotonya.

Foto hitam putih tidak punya rona yang jujur bisa dipinjam, jadi ia kembali ke
palet bawaan — bukan ditebak.

Catatan yang mengejutkan: kartu kumparan keluar toska karena bilah merek
kumparan menyatu di dalam og:image-nya, dan bilah itu memang bidang warna
terbesar di foto. Kebetulan hasilnya sama dengan `theme-color` mereka.

## Ukuran huruf paragraf: tangga lama, anak tangga teratas ditutup

Tangganya (≤110 → 62, ≤170 → 56, ≤240 → 50, ≤320 → 44, selebihnya 38) hasil
percobaan sejak kartu ini lahir dan memang enak dilihat. Yang salah cuma satu:
anak tangga teratas **terbuka** — di atas 320 huruf ukurannya berhenti mengecil,
jadi paragraf jauh lebih panjang tumpah keluar kartu.

Luapan itu lama tersembunyi karena pita bawah 15% memendekkan foto dan
meminjamkan ruangnya ke teks. Begitu pita dicabut dan kanvas kembali penuh,
paragraf 511 huruf langsung menabrak tepi bawah.

**Seluruh tangga sempat saya ganti dengan rumus, dan itu keliru.** Rumusnya
membesarkan paragraf 120 huruf dari 56 ke 62 px — lebih besar daripada yang
pernah diminta siapa pun, dan merusak jaraknya terhadap judul yang tetap 38 px.
Pengguna yang menemukannya, bukan tes saya: tes "ukuran lama dipertahankan"
waktu itu memakai toleransi ±6 px, persis sebesar selisih terburuknya. Toleransi
di tes yang menjaga regresi harus lebih ketat dari regresi yang mungkin terjadi;
kalau tidak, tesnya cuma hiasan. Sekarang perbandingannya sama persis.

Jadi penghitungan hanya bekerja **di atas anak tangga teratas**. Di bawahnya
tangga yang menang.

### Ruang yang tersedia: diukur, bukan dijumlahkan

Angka ruang pertama (550 px) saya jumlahkan dari padding, tinggi foto, stempel,
dan kaki kartu. Terlalu kecil, karena blok isi boleh **tumbuh ke atas menutupi
bagian bawah foto** — itu tidak terlihat dari angka padding mana pun. Akibatnya
kartu yang sebenarnya baik-baik saja ikut dikecilkan.

Diukur ulang dengan mematikan pengecilan lalu memeriksa apakah kertas guntingan
menyentuh tepi bawah kanvas:

| paragraf | ukuran | taksiran tinggi | hasil render |
| --- | --- | --- | --- |
| 320 huruf | 44 px | 649 px | muat |
| 460 huruf | 38 px | 713 px | muat |
| 500 huruf | 38 px | 764 px | **terpotong** |

`heroRoomWithPhoto = 720` diambil di antara 713 dan 764, condong ke sisi aman.
Kontrolnya: paragraf asli 511 huruf (taksiran 764) memang terpotong — cocok.

## Ukuran huruf bisa disetel, dalam LANGKAH bukan piksel

Judul dan paragraf punya kendalinya sendiri, masing-masing −2…+2 langkah (10%
per langkah) dari ukuran standar. Angka piksel bebas sengaja tidak dipakai: ia
membuang tangga, pengecilan otomatis, dan penskalaan rasio sekaligus, lalu
memaksa pengguna menyetel ulang tiap kartu. Dengan langkah relatif, standar
selalu jadi titik nol yang bisa dikembalikan.

Urutannya penting: **tangga → langkah pengguna → pengecilan agar muat**.
Membalik urutannya berarti pilihan pengguna dimakan lebih dulu oleh penyesuaian
yang sebetulnya tidak perlu. Dan pengecilan tetap berlaku terakhir, jadi
memperbesar paragraf panjang tidak bisa mendorongnya keluar kartu.

±20% dipilih sebagai batas karena lebih dari itu judul mulai bersaing dengan
paragraf. Hierarki kartu ini dibalik dengan sengaja — paragraf yang jadi
bintang, judul turun pangkat jadi keterangan — dan itu dijaga tes
(`TestTitleStaysSmallerThanTheParagraph`).

## Kartu jadi bisa disetel — dan pagar yang menjaganya

Lima kendali ditambahkan sekaligus. Yang menyatukan rancangannya satu kalimat:
**otomatis tetap jadi bawaan, manual hanya menimpa.** Tanpa itu, dua sistem
berebut mengatur hal yang sama dan pengguna kehilangan jaminan yang sudah ada.

### Foto utuh: titik awal, bukan langit-langit

`clampZoom` batas bawahnya 1 — artinya titik awalnya "penuhi bingkai", persis
*Center of the Picture* di tab klip. Mode `whole` memindahkan titik awal itu ke
"seluruh gambar asli masuk". Bukan menaikkan batas atas: pelajaran nomor 4-5 di
`notes/15-sumbu-zoom.md` dibayar mahal sekali, tidak perlu diulang.

Gambar artikel landscape (mis. 640x336) di bingkai 1080x960 menyisakan ±393 px
kosong. Isiannya salinan buram fotonya sendiri, bukan warna polos, supaya
bingkai tetap terbaca sebagai satu gambar.

Isian itu **tidak boleh** memakai `z-index:-1`. `.photo` tidak membuat konteks
penumpukan, jadi anak ber-z-index negatif jatuh ke belakang latar induknya dan
isiannya hilang sama sekali — sempat terjadi. Yang menentukan urutan HTML-nya:
isian ditulis sebelum gambar utama.

### Geseran blok: dibuat, lalu DICABUT

Judul, paragraf, dan kaki kartu sempat diberi penggeser sendiri-sendiri. Itu
salah baca dari saya: pengguna sudah mengganti permintaan itu ("**daripada
dibuat bergeser** kita buat batas header dan footer bisa diatur"), penggantinya
belum diputuskan, dan saya menghidupkan lagi versi yang sudah dicabut.

Alasan mengapa itu memang tidak diinginkan, terlepas dari salah bacanya: ketiga
blok itu **satu kesatuan**. Judul menjelaskan kutipan, kutipan membawa
stempelnya, kaki kartu menutup. Memberi masing-masing penggeser membuat mereka
bisa saling tumpang tindih, dan yang didapat pengguna bukan tata letak lain
melainkan kartu rusak.

Yang menggantikannya: **ukuran huruf**, bukan posisi. Itu menjawab kebutuhan
aslinya ("supaya ketiganya pas") tanpa memecah kesatuannya.

### Satu warna menyetel semuanya

Warna pilihan pengguna lewat `toneOfHex` → `paletteFor`, jalur yang sama persis
dengan warna dari foto. Jadi "sesuaikan seluruh warna dari satu warna" bukan
fitur terpisah, melainkan pintu masuk kedua ke mesin yang sudah ada. Pagar
keterbacaannya ikut berlaku otomatis.

Warna pilihan yang kelabu jatuh ke palet bawaan, sama seperti foto hitam putih —
bukan dipaksa jadi rona yang sebetulnya tidak ada.

Kotak paragraf bisa dihilangkan (`box: none`). Teksnya lalu memakai bayangan
sebagai pengganti kertas; tanpa itu, menghilangkan kotak = kartu yang tidak
terbaca di atas foto ramai.

### Pratinjau: satu folder yang menimpa dirinya sendiri

Menyetel kartu itu pekerjaan puluhan percobaan, dan tiap percobaan dulu
meninggalkan satu folder permanen — terukur **27 folder / 24 MB dalam sehari**.
Pratinjau memakai id tetap `card-preview` dan melewati berkas pendamping;
caption & keterangan sumber baru ditulis saat kartunya benar-benar disimpan.

Alamat pratinjau WAJIB membawa penanda waktu. Id-nya selalu sama, jadi tanpa itu
browser menampilkan gambar lama dari cache dan penyetelan terlihat tidak
berpengaruh sama sekali.

Kartu tersimpan disapu sampai tersisa 50 terbaru (`keepCards`). Fungsi ini
menghapus folder secara rekursif, jadi ia diuji tersendiri: pratinjau tidak ikut
tersapu, dan apa pun yang bukan folder kartu tidak disentuh.

### Yang BELUM dijaga

Tanpa kotak paragraf, lebar barisnya jadi 952 px sementara `heroSizeFor` masih
menghitung dengan 824 px. Ukurannya karena itu sedikit lebih kecil dari
seharusnya — melesetnya ke arah aman (tetap muat), jadi dibiarkan.

Paragraf tidak dibatasi panjangnya di hulu — `parseParagraphs` hanya punya batas
minimum kata. Ukuran huruf berhenti mengecil di 22 px karena di bawah itu tidak
terbaca di layar ponsel, jadi paragraf yang sangat panjang tetap bisa terpotong.
Itu batas yang disengaja: pertanda paragrafnya perlu dipilih ulang, bukan
kartunya yang perlu mengalah lagi.

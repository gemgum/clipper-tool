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
batas yang dipakai terhadap catatan/12 — yang dilarang adalah berpindah mesin
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

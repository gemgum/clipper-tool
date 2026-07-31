# Sumbu zoom: dari ukuran kotak jadi pembesaran gambar

Laporan: *"Pada zoom salah, seharusnya mulai fill the whole frame salah logika
seharusnya fit aseli itu 0% baru zoomnya pakai zoom semakin naik dan gambar
semakin diperbesar."*

Benar, dan sebabnya bukan salah hitung melainkan salah sumbu.

## Yang dulu terjadi

`ReframeFilter` memakai zoom untuk menskalakan **kotak** tempat video ditaruh,
bukan isi gambarnya:

```go
fw := floorEven(targetW * zoom / 100)   // kotak, bukan gambar
fh := floorEven(targetH * zoom / 100)
```

Akibatnya sumbunya terbalik dari yang orang harapkan:

| zoom | yang terjadi dulu |
| --- | --- |
| 100 | kotak = seluruh bingkai → video memenuhi bingkai |
| 50 | kotak = separuh bingkai → **video mengambang kecil di tengah** |
| 5 | kotak nyaris tak ada → video sebesar prangko |

Jadi menurunkan zoom mengecilkan gambar, dan tidak ada satu pun nilai yang
berarti "tampilkan frame asli apa adanya". Yang berarti begitu justru kendali
**lain**: `reframe: fit`.

Itulah akar masalahnya — **dua kendali menyatakan hal yang sama**. `reframe`
memilih antara `fit` (frame utuh) dan `center` (isi penuh), sementara `zoom`
juga bergerak di sumbu besar-kecil yang sama. Wajar terasa salah.

## Yang berlaku sekarang

Satu sumbu menerus, `0..100`:

- **0** = *contain* — seluruh frame asli terlihat, tidak ada yang terpotong,
  sisa ruangnya diisi `background` (blur atau hitam);
- **100** = *cover* — video memenuhi bingkai, tepi yang berlebih dipotong;
- di antaranya diinterpolasi linear dari skala contain ke skala cover.

```
skala(z) = contain + (cover - contain) × z/100
  contain = min(TW/iw, TH/ih)
  cover   = max(TW/iw, TH/ih)
```

Diukur sungguhan (sumber 1920×1080 → bingkai 1080×1920):

| zoom | ukuran gambar | |
| --- | --- | --- |
| 0 | 1080×608 | seluruh frame terlihat, ada ruang kosong |
| 25 | 1662×934 | |
| 50 | 2246×1262 | |
| 75 | 2830×1590 | |
| 100 | 3413×1920 | bingkai penuh, sisi terpotong |

`reframe` turun jadi urusan **letak** saja (`center`, kelak `face_follow`).
`fit` dipertahankan sebagai alias: `Validate` menerjemahkannya jadi
`center` + `zoom 0`, jadi preset & permintaan lama tidak rusak.

## Kenapa ekspresi ffmpeg, bukan ffprobe

Interpolasi butuh rasio aspek sumber. Memprobe lebih dulu berarti menambah
panggilan ffprobe di dua jalur (render klip & preview satu frame) plus jalur
kegagalan baru bila probe gagal. Ekspresi `min()/max()` dihitung ffmpeg sendiri
dari `iw`/`ih`, jadi tidak ada yang perlu diketahui di muka.

Dua jebakan yang sudah kena:

1. **Kutip, jangan backslash.** Koma di dalam `min(1080/iw,1920/ih)` akan dibaca
   sebagai pemisah filter kalau dibiarkan telanjang. Di dalam kutip tunggal
   ffmpeg TIDAK menafsirkan backslash, jadi `\,` justru menyisakan backslash
   literal dan merusak ekspresinya. Yang benar: kutip, tanpa escape.
2. **Ujungnya tidak memakai ekspresi.** Pembulatan `trunc(...)*2` bisa meleset
   satu piksel; pada zoom 100 satu piksel meleset berarti segaris latar terlihat
   di tepi. Karena itu zoom 0 dan 100 tetap memakai
   `force_original_aspect_ratio=decrease/increase` yang dihitung ffmpeg secara
   tepat, dan ekspresi hanya dipakai di antaranya.

## Dua sumbu, bukan satu

Versi pertama perbaikan ini menyatukan semuanya jadi SATU sumbu, lalu membuang
dropdown "Cara pas". Dua hal langsung dilaporkan:

1. *"kok fitur 9:16 fullnya hilang?"* — preset bernama lenyap. Hasil rendernya
   tidak berubah sedikit pun, tapi keluaran yang paling sering dipakai kehilangan
   namanya dan jadi harus dihafal sebagai sebuah angka.
2. *"yang salah tadi kan ada fitur hilang"* — dan yang benar-benar hilang adalah
   tampilan **video mengecil di tengah dengan latar mengelilingi keempat sisinya**.
   Satu sumbu tidak bisa menyatakannya, sebab itu memang pertanyaan yang berbeda.

Akar keduanya sama: dulu SATU kendali mencampur DUA pertanyaan yang berbeda —
"berapa banyak frame asli yang terlihat" dan "seberapa besar gambarnya duduk di
bingkai". Menyatukannya jadi satu sumbu memperbaiki kebingungan arah, tapi
sekaligus membuang salah satu pertanyaannya.

Jadi sekarang keduanya berdiri sendiri:

| | 0 | 100 |
| --- | --- | --- |
| `frame_visible` | gambar memenuhi kotaknya, tepi terpotong | seluruh frame asli terlihat |
| `picture_size` | (min 5) gambar kecil di tengah | gambar memenuhi bingkai |

Empat kombinasi yang berarti:

| frame_visible | picture_size | hasil |
| --- | --- | --- |
| 0 | 100 | isi penuh 9:16 — default, tidak berubah sejak awal |
| 0 | 50 | potongan penuh yang mengecil di tengah — **tampilan yang sempat hilang** |
| 100 | 100 | frame utuh dengan pita latar |
| 100 | 60 | frame utuh yang dikecilkan — kombinasi yang dulu MUSTAHIL |

Baris terakhir itu bonus yang jatuh sendiri begitu sumbunya dipisah: dulu "fit"
dan "perkecil" adalah kendali yang sama, jadi keduanya tidak bisa dipakai
bersamaan.

`TestOldShrunkLookIsReproducibleExactly` mengunci rantai filter tampilan lama
sebagai string persis, jadi ia tidak bisa hilang lagi tanpa ada tes yang gagal.

## Arah angka: dibalik sekali lagi

Pesan pertama meminta `fit = 0%`. Setelah melihat pratinjaunya, pilihannya
dibalik: **100% = frame utuh, 0% = isi penuh** — angka dibaca sebagai "berapa
persen frame asli yang terlihat".

Karena itu nama `zoom` jadi menyesatkan di kode: nilai 100 justru yang paling
TIDAK di-zoom. Itu persis jenis jebakan yang sedang diperbaiki di catatan ini,
jadi field-nya dinamai `frame_visible`. Label di layar tetap "Zoom" karena itu
kata yang dipakai penggunanya.

## Perubahan yang memutus kompatibilitas

`zoom` diganti `frame_visible` + `picture_size`. Nilai nol keduanya diperlakukan
berbeda, dan itu disengaja:

- `frame_visible` **0 SAH** (= isi penuh) dan kebetulan itu juga defaultnya, jadi
  permintaan yang tidak mengirim field ini tetap mendapat keluaran andalan;
- `picture_size` **0 TIDAK sah** (gambar tanpa ukuran), jadi dipakai sebagai
  penanda "belum diisi" dan jatuh ke 100.

Di GUI, preset tersimpan ikut dimigrasikan: `reframe:"fit"` jadi frame utuh, dan
nilai `zoom` dari versi sebelumnya dibalik. Preset baru menyimpan penanda
`zoomAxis:"visible"` supaya pembalikan itu tidak terjadi dua kali.

## Verifikasi

`reframe_test.go` tidak berhenti di mencocokkan string filter — mencocokkan
string tidak membuktikan apa pun soal geometri. Ia menjalankan ffmpeg sungguhan
atas sumber sintetis, lalu ffprobe mengukur hasilnya:

- zoom 0 tidak melebihi bingkai (tidak ada yang terpotong);
- zoom 100 menutupi bingkai sepenuhnya;
- ukurannya **naik monoton** di tiap langkah zoom;
- dimensinya selalu genap (h264 menolak yang ganjil);
- benar juga untuk sumber potret, saat peran lebar & tinggi bertukar.

Tesnya dilewati sendiri bila ffmpeg/ffprobe tidak ada.

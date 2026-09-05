"use client";

// Watermark: banner PNG milik pengguna + teks di atasnya, dibakar ke setiap klip.
//
// Bentuknya satu objek, bukan belasan state terpisah, karena ia melintasi tiga
// berkas — halaman yang memegangnya, panel setelan yang mengubahnya, dan
// pratinjau yang menggambarnya. Tiga belas prop yang diteruskan satu per satu
// adalah tiga belas kesempatan untuk lupa satu.
//
// Koordinatnya memakai ruang 1080x1920 yang sama dengan subtitle, jadi angka di
// pratinjau berarti hal yang sama dengan angka yang dikirim ke engine.

export type Watermark = {
  image: string;   // path lokal; kosong = watermark mati
  x: number;       // titik TENGAH banner
  y: number;
  // KOTAK tempat gambar diletakkan, persen lebar & tinggi bingkai. Gambarnya
  // dimuat utuh ke dalamnya — tidak digepengkan, tidak dipotong.
  width: number;
  height: number;
  at: number;      // detik muncul, relatif awal klip
  dur: number;     // berapa detik tampil; 0 = sampai klip habis
  // Sumber teks. "text" = satu teks untuk semua klip, bisa dipratinjau apa
  // adanya. "llm" = judul yang dipilihkan LLM untuk tiap klip — beda tiap klip,
  // jadi pratinjau hanya bisa menampilkan contoh.
  hlSource: "text" | "llm";
  hlText: string;
  hlSize: number;
  hlColor: string;
  hlOutline: number;
  hlX: number;     // jangkar tepi ATAS blok teks (sama dengan subtitle)
  hlY: number;
};

// Harus sama dengan config.DefaultWatermark() di engine.
export const DEFAULT_WATERMARK: Watermark = {
  image: "", x: 540, y: 700, width: 25, height: 25, at: 0, dur: 0,
  hlSource: "text", hlText: "", hlSize: 64, hlColor: "white", hlOutline: 3,
  hlX: 540, hlY: 640,
};

// watermarkOn = ada yang akan tergambar. Headline tanpa banner sah: teks tetap di
// atas video adalah bentuk watermark yang paling murah.
export const watermarkOn = (b: Watermark) =>
  !!b.image || !!b.hlText.trim() || b.hlSource === "llm";

// watermarkToAPI menerjemahkan ke bentuk yang dibaca config.Watermark di engine.
//
// font datang dari pilihan font SUBTITLE, dan itu disengaja: headline memakai
// keluarga font yang sama supaya pratinjau bisa memakai metrik yang sudah
// dihitung engine untuk font itu (lihat fontScale di preview-panel). Pemilih
// font kedua berarti pengukuran kedua, dan pratinjau yang meleset dari hasil
// render adalah persis kegagalan yang sudah pernah terjadi (notes/29).
export const watermarkToAPI = (b: Watermark, font: string) => ({
  image: b.image,
  x: Math.round(b.x), y: Math.round(b.y),
  width: Math.round(b.width),
  height: Math.round(b.height),
  at: Number(b.at) || 0,
  for: Number(b.dur) || 0,
  headline: {
    source: b.hlSource,
    font,
    text: b.hlSource === "llm" ? "" : b.hlText,
    size: Math.round(b.hlSize),
    color: b.hlColor,
    bold: true,
    outline: Math.round(b.hlOutline),
    x: Math.round(b.hlX), y: Math.round(b.hlY),
  },
});

// wrapHeadline meniru headlineLines() di engine (internal/subtitle).
//
// Ada di dua tempat, dan itu disengaja: engine yang menulis .ass, tapi
// pratinjau harus menunjukkan PEMENGGALAN YANG SAMA. Pratinjau yang menampilkan
// satu baris sementara hasil rendernya tiga adalah bentuk kebohongan yang
// paling mahal — ketahuannya setelah klipnya jadi.
//
// Angka-angkanya harus sama dengan sisi Go: margin 60 di ruang 1080, dan faktor
// lebar huruf 0,6.
const HL_MARGIN = 60;

export function wrapHeadline(text: string, size: number): string[] {
  const usable = 1080 - 2 * HL_MARGIN;
  const maxChars = Math.max(6, Math.floor(usable / ((size || 64) * 0.6)));
  const out: string[] = [];
  for (const para of text.split("\n")) {
    let line = "";
    for (const word of para.split(/\s+/).filter(Boolean)) {
      if (!line) line = word;
      else if (line.length + 1 + word.length <= maxChars) line += " " + word;
      else { out.push(line); line = word; }
    }
    if (line) out.push(line);
  }
  return out;
}

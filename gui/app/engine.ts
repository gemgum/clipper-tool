"use client";

// Alamat engine + kunci sesi.
//
// Dua hal yang harus diketahui setiap permintaan ke engine, dan keduanya baru
// diketahui SAAT HALAMAN DIBUKA — bukan saat GUI dibangun:
//
//   alamat  engine memakai port acak begitu ia terpasang, jadi memaku
//           "127.0.0.1:8787" ke dalam berkas hasil build membuat aplikasi
//           bertanya ke port yang kosong;
//   kunci   dibuat baru setiap engine dijalankan, dan sampai ke halaman lewat
//           "?token=…" pada alamat yang dibuka jendela aplikasi.

const DEV_ENGINE = "http://127.0.0.1:8787";

// Port server pengembangan Next. Halaman yang disajikan dari sini BUKAN berasal
// dari engine, jadi hanya dalam keadaan itulah alamat engine perlu ditebak.
const DEV_PORT = "3000";

const KEY = "clipper.token";

let base: string | null = null;
let token = "";
let ready = false;

/** engineBase menentukan ke mana permintaan API dikirim. */
function engineBase(): string {
  if (base !== null) return base;

  const fromEnv = process.env.NEXT_PUBLIC_ENGINE_URL;
  if (fromEnv) {
    base = fromEnv;
    return base;
  }
  if (typeof window !== "undefined") {
    const { protocol, port, origin } = window.location;
    // Halaman ini disajikan engine sendiri (aplikasi jadi): API-nya ada di
    // alamat yang sama persis, berapa pun portnya. Alamat relatif akan lebih
    // sederhana, tapi origin penuh dipakai supaya nilainya bisa dicetak apa
    // adanya saat mencari masalah.
    if (protocol.startsWith("http") && port !== DEV_PORT) {
      base = origin;
      return base;
    }
  }
  // Mode pengembangan: GUI di :3000, engine di tempat biasanya.
  base = DEV_ENGINE;
  return base;
}

function init() {
  if (ready || typeof window === "undefined") return;
  ready = true;
  try {
    const fromURL = new URLSearchParams(window.location.search).get("token");
    if (fromURL) {
      token = fromURL;
      sessionStorage.setItem(KEY, fromURL);
      // Kunci dibersihkan dari bilah alamat setelah dibaca: ia tidak perlu
      // terlihat, dan tidak perlu ikut tersalin saat pengguna menyalin URL.
      const clean = new URL(window.location.href);
      clean.searchParams.delete("token");
      window.history.replaceState({}, "", clean.toString());
      return;
    }
    // sessionStorage, bukan localStorage: kunci berganti tiap engine
    // dijalankan, jadi menyimpannya lebih lama dari satu tab hanya membuat
    // kunci basi terkirim dan permintaan ditolak tanpa sebab yang jelas.
    token = sessionStorage.getItem(KEY) || process.env.NEXT_PUBLIC_ENGINE_TOKEN || "";
  } catch {
    token = "";
  }
}

/**
 * eng membentuk URL lengkap ke engine, lengkap dengan kunci sesi bila ada.
 *
 * Kuncinya dititipkan di query, bukan di header, karena tidak semua permintaan
 * dibuat oleh kode kita: <video src>, tautan unduh, dan EventSource dibentuk
 * browser dan tidak bisa membawa header. Satu jalur untuk semuanya lebih baik
 * daripada dua yang harus diingat mana dipakai di mana.
 */
export function eng(path: string): string {
  init();
  const url = engineBase() + path;
  if (!token) return url;
  return url + (url.includes("?") ? "&" : "?") + "token=" + encodeURIComponent(token);
}

/** ENGINE = alamat engine yang sedang dipakai. Untuk ditampilkan, bukan disusun. */
export function engineURL(): string {
  return engineBase();
}

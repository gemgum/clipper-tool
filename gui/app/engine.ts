"use client";

// Alamat engine + kunci sesi.
//
// Engine menolak permintaan tanpa kunci ketika Clipper jalan sebagai aplikasi
// terpasang (lihat engine/internal/api/token.go). Jendela aplikasi membuka GUI
// dengan "?token=…", sebab halaman yang baru dibuka tidak punya cara lain
// menerimanya — ia tidak bisa membaca berkas engine.json.
//
// Kuncinya disimpan di sessionStorage, bukan localStorage: ia dibuat baru
// setiap engine dijalankan, jadi menyimpannya lebih lama dari satu tab hanya
// membuat kunci basi terkirim dan permintaan ditolak tanpa sebab yang jelas.

export const ENGINE = process.env.NEXT_PUBLIC_ENGINE_URL || "http://127.0.0.1:8787";

const KEY = "clipper.token";

let token = "";
let ready = false;

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
  const url = ENGINE + path;
  if (!token) return url;
  return url + (url.includes("?") ? "&" : "?") + "token=" + encodeURIComponent(token);
}

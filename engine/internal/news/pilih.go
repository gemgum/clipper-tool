package news

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Asal sebuah nilai peringkat.
const (
	SumberLLM       = "llm"
	SumberHeuristik = "heuristik"
)

// Peringkat satu paragraf beserta nilai hook-nya.
type Peringkat struct {
	Indeks int     `json:"indeks"`
	Skor   float64 `json:"skor"`
	Alasan string  `json:"alasan"`
	Teks   string  `json:"teks"`   // diisi engine dari indeks — TIDAK dari balasan LLM
	Sumber string  `json:"sumber"` // llm | heuristik — supaya terlihat di GUI
}

// Pilihan hasil analisis satu artikel.
type Pilihan struct {
	Kartu     int         `json:"kartu"`   // indeks paragraf usulan untuk kartu
	Caption   int         `json:"caption"` // indeks paragraf usulan untuk caption
	Peringkat []Peringkat `json:"peringkat"`
	Hashtag   []string    `json:"hashtag"`
	Mesin     string      `json:"mesin"`
	Catatan   string      `json:"catatan"` // penjelasan bila LLM tidak melengkapi
}

// Mesin adalah satu panggilan ke LLM. Dibuat berupa fungsi, bukan antarmuka,
// supaya paket news tidak perlu mengenal paket llm/ollama — lapisan api yang
// merangkainya sesuai mesin yang dipilih pengguna.
type Mesin func(ctx context.Context, system, user string) (string, error)

// balasan = bentuk JSON yang diminta dari LLM.
//
// Perhatikan: tidak ada satu pun field teks bebas untuk isi kartu maupun
// caption — hanya NOMOR paragraf. LLM memilih, engine yang mengambil kalimat
// aslinya. Alasan boleh berupa teks karena itu hanya penjelasan untuk manusia,
// tidak pernah ikut terbit.
type balasan struct {
	Kartu     int `json:"kartu"`
	Caption   int `json:"caption"`
	Peringkat []struct {
		Indeks int     `json:"indeks"`
		Skor   float64 `json:"skor"`
		Alasan string  `json:"alasan"`
	} `json:"peringkat"`
	KataKunci []string `json:"kata_kunci"`
}

const systemPilih = `Kamu membantu memilih bagian menarik dari artikel berita Indonesia untuk dijadikan konten media sosial.

ATURAN PALING PENTING: kamu TIDAK menulis, TIDAK merangkum, dan TIDAK memparafrase.
Kamu hanya MEMILIH NOMOR paragraf yang sudah diberikan. Dilarang mengarang kalimat baru.

Tugasmu:
1. "kartu"   — nomor paragraf terbaik untuk ditaruh di gambar. Pilih yang berdiri
               sendiri (dimengerti tanpa membaca paragraf lain), padat, dan memuat
               inti beritanya. Hindari paragraf yang diawali kata rujukan seperti
               "Ia", "Hal itu", "Sementara itu".
2. "caption" — nomor paragraf paling memancing rasa ingin tahu untuk keterangan
               unggahan. Boleh sama dengan "kartu" bila memang paragraf itu yang terbaik.
3. "peringkat" — nilai SEMUA paragraf dari sisi daya tarik (hook), skor 0-10.
               Sertakan "alasan" singkat dalam bahasa Indonesia, maksimal 12 kata.
4. "kata_kunci" — 5 sampai 8 kata atau frasa PENTING yang benar-benar tertulis di
               artikel (nama orang, lembaga, tempat, peristiwa). Salin persis apa
               adanya. Jangan menambah kata yang tidak ada di artikel.

Yang membuat sebuah paragraf ber-hook: ada angka mengejutkan, konflik, pernyataan
langsung yang tegas, akibat yang menyentuh orang banyak, atau fakta yang tidak terduga.

Balas HANYA JSON.`

// SkemaPilih = JSON Schema untuk parameter "format" Ollama, supaya model lokal
// tidak perlu diandalkan kepatuhannya pada instruksi bentuk balasan.
func SkemaPilih() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"kartu":   map[string]any{"type": "integer"},
			"caption": map[string]any{"type": "integer"},
			"peringkat": map[string]any{
				"type": "array",
				// minItems memaksa model lokal benar-benar mengisi peringkat.
				// Tanpa ini qwen2.5 kerap membalas "peringkat": [] — bentuk JSON
				// sah, isinya kosong. Ini pencegahan di hulu; susun() tetap
				// menambal sisanya kalau model masih membandel.
				"minItems": 1,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"indeks": map[string]any{"type": "integer"},
						"skor":   map[string]any{"type": "number"},
						"alasan": map[string]any{"type": "string"},
					},
					"required": []string{"indeks", "skor"},
				},
			},
			"kata_kunci": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
		"required": []string{"kartu", "caption", "peringkat", "kata_kunci"},
	}
}

// maksKataPrompt membatasi panjang artikel yang dikirim ke model. 1200 kata
// masih muat di konteks 8k milik model lokal, dan berita Indonesia jarang
// melampauinya.
const maksKataPrompt = 1200

// Pilih meminta LLM menilai paragraf mana yang paling layak jadi kartu & caption.
//
// Kegagalan dikembalikan apa adanya — tidak ada perpindahan diam-diam ke mesin
// lain maupun ke tebakan heuristik (lihat catatan/12).
func Pilih(ctx context.Context, isi Isi, jalan Mesin, namaMesin, cacheDir string) (Pilihan, error) {
	if len(isi.Paragraf) == 0 {
		return Pilihan{}, fmt.Errorf("artikel tidak punya paragraf untuk dinilai")
	}
	kunciCache := kunci(isi.Artikel.URL, namaMesin)
	if p, ok := muatCache(cacheDir, kunciCache); ok {
		return p, nil
	}

	user := fmt.Sprintf("Judul: %s\n\nParagraf:\n\n%s", isi.Artikel.Judul, isi.Bernomor(maksKataPrompt))
	mentah, err := jalan(ctx, systemPilih, user)
	if err != nil {
		return Pilihan{}, err
	}

	var b balasan
	if err := json.Unmarshal([]byte(petikJSON(mentah)), &b); err != nil {
		return Pilihan{}, fmt.Errorf("%s membalas JSON yang tidak bisa dibaca: %w — balasan: %s",
			namaMesin, err, potong(strings.ReplaceAll(mentah, "\n", " "), 300))
	}

	hasil := susun(isi, b, namaMesin)
	simpanCache(cacheDir, kunciCache, hasil)
	return hasil, nil
}

// susun mengubah balasan LLM jadi hasil yang aman: setiap nomor diperiksa
// jangkauannya, dan teksnya diambil dari artikel — bukan dari balasan.
//
// Tidak pernah mengembalikan galat. Model lokal sering hanya mengisi sebagian
// (mis. memberi "kartu" & "caption" tapi "peringkat": []), dan menolak hasil
// seperti itu berarti membuang jawaban yang sebenarnya sudah bisa dipakai.
// Paragraf yang tidak dinilai model dilengkapi penilaian heuristik, dan
// ditandai supaya pengguna tahu mana yang berasal dari mana.
func susun(isi Isi, b balasan, namaMesin string) Pilihan {
	if len(isi.Paragraf) == 0 {
		return Pilihan{Mesin: namaMesin}
	}
	var per []Peringkat
	dinilai := map[int]bool{}
	for _, r := range b.Peringkat {
		teks, ok := isi.Teks(r.Indeks)
		if !ok || dinilai[r.Indeks] {
			continue // nomor tidak ada atau ganda — abaikan, jangan tebak
		}
		dinilai[r.Indeks] = true
		per = append(per, Peringkat{
			Indeks: r.Indeks, Skor: r.Skor, Alasan: r.Alasan,
			Teks: teks, Sumber: SumberLLM,
		})
	}

	// Lengkapi paragraf yang terlewat. Selain menambal kemalasan model, ini
	// juga membuat SELURUH paragraf bisa dipilih pengguna di GUI — sebelumnya
	// paragraf yang tak dinilai model tidak muncul sama sekali.
	var tertinggal int
	for _, p := range isi.Paragraf {
		if dinilai[p.Indeks] {
			continue
		}
		tertinggal++
		s, alasan := skorHook(p, len(isi.Paragraf))
		per = append(per, Peringkat{
			Indeks: p.Indeks, Skor: s, Alasan: alasan,
			Teks: p.Teks, Sumber: SumberHeuristik,
		})
	}
	UrutPeringkat(per)

	catatan := ""
	switch {
	case len(dinilai) == 0:
		catatan = fmt.Sprintf("%s tidak memberi peringkat sama sekali — urutan di bawah disusun otomatis oleh engine.", namaMesin)
	case tertinggal > 0:
		catatan = fmt.Sprintf("%s menilai %d dari %d paragraf; sisanya diberi nilai otomatis oleh engine.",
			namaMesin, len(dinilai), len(isi.Paragraf))
	}

	// Nomor kartu/caption yang di luar jangkauan diganti peringkat teratas —
	// ini bukan menebak isi, hanya memilih di antara paragraf artikel sendiri.
	kartu := b.Kartu
	if _, ok := isi.Teks(kartu); !ok {
		kartu = per[0].Indeks
	}
	caption := b.Caption
	if _, ok := isi.Teks(caption); !ok {
		caption = per[0].Indeks
	}
	// Model lokal cenderung menjawab paragraf 0 untuk dua-duanya, sehingga teks
	// kartu dan captionnya kembar dan postingannya jadi mubazir. Bila kembar,
	// caption digeser ke paragraf berperingkat berikutnya — masih dipilih dari
	// artikel yang sama, dan pengguna tetap bisa menggantinya sekali klik.
	if caption == kartu && len(per) > 1 {
		for _, r := range per {
			if r.Indeks != kartu {
				caption = r.Indeks
				break
			}
		}
	}

	return Pilihan{
		Kartu:     kartu,
		Caption:   caption,
		Peringkat: per,
		Hashtag:   isi.Tagar(b.KataKunci, 8),
		Mesin:     namaMesin,
		Catatan:   catatan,
	}
}

// petikJSON mengambil blok objek JSON dari balasan. Model lokal kadang
// membungkus JSON dengan pagar kode atau kalimat pengantar.
func petikJSON(s string) string {
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}

// --- cache ---

// kunci = sidik jari URL + mesin. Artikel yang sama dinilai ulang hanya bila
// mesinnya berganti, sehingga bereksperimen dengan gaya kartu tidak memanggil
// LLM berulang kali.
func kunci(url, mesin string) string {
	h := sha256.Sum256([]byte(url + "\x00" + mesin))
	return hex.EncodeToString(h[:16])
}

func jalurCache(dir, k string) string {
	return filepath.Join(dir, "cache", "artikel", k+".json")
}

func muatCache(dir, k string) (Pilihan, bool) {
	if dir == "" {
		return Pilihan{}, false
	}
	raw, err := os.ReadFile(jalurCache(dir, k))
	if err != nil {
		return Pilihan{}, false
	}
	var p Pilihan
	if err := json.Unmarshal(raw, &p); err != nil || len(p.Peringkat) == 0 {
		return Pilihan{}, false
	}
	return p, true
}

func simpanCache(dir, k string, p Pilihan) {
	if dir == "" {
		return
	}
	jalur := jalurCache(dir, k)
	if err := os.MkdirAll(filepath.Dir(jalur), 0o755); err != nil {
		return // cache itu percepatan, bukan keharusan — gagal simpan tidak fatal
	}
	if raw, err := json.Marshal(p); err == nil {
		_ = os.WriteFile(jalur, raw, 0o644)
	}
}

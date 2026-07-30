package news

import (
	"regexp"
	"strings"
)

// Penilaian hook tanpa LLM.
//
// Gunanya dua: (1) melengkapi paragraf yang tidak ikut dinilai model — model
// lokal kerap membalas "peringkat": [] karena menilai selusin paragraf itu
// tugas panjang yang gampang dilewatkan; (2) menjadi jaring supaya fitur ini
// tidak pernah gagal total hanya karena model malas.
//
// Ini BUKAN pengganti mesin skor yang dipilih pengguna: yang dinilai di sini
// cuma urutan tampilan paragraf, sedangkan isinya tetap verbatim dari artikel.
// Setiap paragraf yang skornya berasal dari sini ditandai Sumber="heuristik"
// agar terlihat di GUI — bukan penggantian diam-diam (catatan/12).

var (
	reAngka   = regexp.MustCompile(`\d`)
	reAngkaBs = regexp.MustCompile(`\d[\d.,]{2,}|\d+\s*(persen|%|juta|miliar|triliun|ribu)`)
	reKutip   = regexp.MustCompile(`["“”']`)
)

// rujukanAwal = pembuka yang menandakan paragraf ini menyambung paragraf
// sebelumnya, jadi tidak dimengerti bila berdiri sendiri di sebuah kartu.
var rujukanAwal = []string{
	"ia ", "dia ", "beliau ", "hal itu", "hal tersebut", "sementara itu",
	"selain itu", "menurutnya", "ujarnya", "katanya", "lebih lanjut",
	"adapun ", "sedangkan ", "kemudian ", "selanjutnya", "pihaknya",
}

// kataKuat = penanda kabar yang biasanya menarik perhatian pembaca Indonesia.
var kataKuat = []string{
	"tewas", "meninggal", "korban", "korupsi", "ditangkap", "tersangka",
	"darurat", "bencana", "gagal", "rugi", "anjlok", "melonjak", "rekor",
	"tertinggi", "terbesar", "pertama kali", "terungkap", "diduga", "protes",
	"tolak", "kecam", "viral", "dilarang", "denda", "sanksi",
}

// skorHook menilai satu paragraf 0..10 beserta alasan singkat.
func skorHook(p Paragraf, jumlah int) (float64, string) {
	teks := p.Teks
	low := strings.ToLower(teks)
	kata := len(strings.Fields(teks))

	skor := 4.0
	var sebab []string

	if reAngkaBs.MatchString(low) {
		skor += 2
		sebab = append(sebab, "memuat angka")
	} else if reAngka.MatchString(teks) {
		skor += 0.8
	}
	if reKutip.MatchString(teks) {
		skor += 1.8
		sebab = append(sebab, "kutipan langsung")
	}
	for _, k := range kataKuat {
		if strings.Contains(low, k) {
			skor += 1.2
			sebab = append(sebab, "kata bermuatan kuat")
			break
		}
	}

	// Paragraf pembuka berita Indonesia hampir selalu memuat intinya
	// (piramida terbalik), jadi diberi keunggulan kecil.
	if p.Indeks == 0 {
		skor += 1.5
		sebab = append(sebab, "paragraf pembuka")
	} else if jumlah > 0 && p.Indeks < jumlah/3 {
		skor += 0.5
	}

	switch {
	case kata >= 15 && kata <= 45:
		skor += 1
	case kata > 70:
		skor -= 1.5
		sebab = append(sebab, "terlalu panjang untuk kartu")
	case kata < 15:
		skor -= 1
		sebab = append(sebab, "terlalu pendek")
	}

	for _, r := range rujukanAwal {
		if strings.HasPrefix(low, r) {
			skor -= 1.8
			sebab = append(sebab, "menyambung paragraf sebelumnya")
			break
		}
	}

	if skor < 0 {
		skor = 0
	}
	if skor > 10 {
		skor = 10
	}
	alasan := "dinilai otomatis"
	if len(sebab) > 0 {
		alasan = strings.Join(sebab, ", ")
	}
	return skor, alasan
}

package pipeline

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gemgum/clipper/engine/internal/score/llm"
	"github.com/gemgum/clipper/engine/internal/types"
)

const (
	// chunkOverlap: detik terakhir tiap potongan diulang di awal potongan
	// berikutnya, supaya momen yang jatuh tepat di perbatasan tetap terlihat
	// utuh oleh model minimal sekali.
	chunkOverlap = 120.0
	// mergeTol: toleransi (detik) saat menyambung/menyamakan momen antar potongan.
	mergeTol = 2.0
	// minMomentDur: momen lebih pendek dari ini dianggap balasan ngawur.
	minMomentDur = 5.0
)

// chunkPart satu potongan transkrip beserta posisinya di video utuh.
type chunkPart struct {
	tr   types.Transcript
	info llm.Chunk
}

// chunkSeconds panjang potongan menurut mesin: model lokal punya jendela
// konteks jauh lebih kecil (num_ctx 8192) daripada Claude.
func chunkSeconds(provider string) float64 {
	if provider == "ollama" {
		return 12 * 60
	}
	return 25 * 60
}

// chunkTranscript memecah transkrip jadi potongan bertumpang-tindih. Timestamp
// TIDAK digeser (tetap detik asli video) supaya model membalas angka absolut.
func chunkTranscript(tr types.Transcript, size, overlap float64) []chunkPart {
	if len(tr.Segments) == 0 {
		return nil
	}
	first := tr.Segments[0].Start
	last := tr.Segments[len(tr.Segments)-1].End
	if last-first <= size {
		return []chunkPart{{
			tr:   tr,
			info: llm.Chunk{Index: 1, Total: 1, Start: first, End: last},
		}}
	}
	if overlap >= size {
		overlap = size / 4
	}

	var parts []chunkPart
	for start := first; start < last; start += size - overlap {
		end := start + size
		if end > last {
			end = last
		}
		var sub types.Transcript
		sub.Language = tr.Language
		for _, s := range tr.Segments {
			if s.End > start && s.Start < end {
				sub.Segments = append(sub.Segments, s)
			}
		}
		if len(sub.Segments) > 0 {
			parts = append(parts, chunkPart{
				tr:   sub,
				info: llm.Chunk{Start: start, End: end},
			})
		}
		if end >= last {
			break
		}
	}
	for i := range parts {
		parts[i].info.Index = i + 1
		parts[i].info.Total = len(parts)
	}
	return parts
}

// mergeMoments menggabungkan momen dari semua potongan:
//   - momen bertanda "berlanjut" yang ujungnya menempel dengan momen berikutnya
//     disambung jadi satu klip (momen yang terbelah batas potongan);
//   - duplikat dari area tumpang-tindih dibuang, skor terbaik dipertahankan;
//   - tumpang-tindih sebagian dirapikan agar klip tidak saling memakan.
func mergeMoments(ms []llm.Moment) []llm.Moment {
	if len(ms) == 0 {
		return nil
	}
	sorted := make([]llm.Moment, len(ms))
	copy(sorted, ms)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })

	out := []llm.Moment{sorted[0]}
	for _, m := range sorted[1:] {
		last := &out[len(out)-1]
		switch {
		case m.Start >= last.End-mergeTol && !last.Berlanjut:
			// Terpisah bersih → klip baru.
			out = append(out, m)

		case last.Berlanjut && m.Start <= last.End+mergeTol && m.End > last.End:
			// Sambungan momen yang terbelah batas potongan.
			last.End = m.End
			last.Berlanjut = m.Berlanjut
			if m.Score > last.Score {
				last.Score = m.Score
			}

		case m.End <= last.End+mergeTol:
			// Duplikat / terkandung (biasanya dari area tumpang-tindih).
			if m.Score > last.Score {
				last.Score = m.Score
				last.Title = m.Title
				last.Hashtags = m.Hashtags
				last.Reasons = m.Reasons
			}

		default:
			// Tumpang-tindih sebagian tanpa tanda berlanjut → geser awalnya.
			m.Start = last.End
			if m.End-m.Start >= minMomentDur {
				out = append(out, m)
			}
		}
	}
	return out
}

// validateMoments membuang momen yang batasnya tidak masuk akal. Bila TIDAK ADA
// yang tersisa, job digagalkan dengan pesan yang menunjukkan balasan model —
// engine tidak lagi diam-diam beralih ke heuristik.
func validateMoments(ms []llm.Moment, tr types.Transcript, engine string) ([]llm.Moment, []string, error) {
	if len(tr.Segments) == 0 {
		return nil, nil, fmt.Errorf("transkrip kosong")
	}
	lo := tr.Segments[0].Start
	hi := tr.Segments[len(tr.Segments)-1].End

	var ok []llm.Moment
	var ditolak []string
	kosong := 0
	for _, m := range ms {
		switch {
		case m.End <= m.Start:
			ditolak = append(ditolak, fmt.Sprintf("start %.1f >= end %.1f", m.Start, m.End))
		case m.Start < lo-1 || m.End > hi+1:
			ditolak = append(ditolak, fmt.Sprintf("%.1f-%.1f di luar durasi video (%.1f-%.1f)", m.Start, m.End, lo, hi))
		case m.End-m.Start < minMomentDur:
			ditolak = append(ditolak, fmt.Sprintf("%.1f-%.1f terlalu pendek (%.1f dtk)", m.Start, m.End, m.End-m.Start))
		case m.Score <= 0 && strings.TrimSpace(m.Title) == "":
			// Model lemah kadang memenuhi skema dengan isian kosong.
			kosong++
			ditolak = append(ditolak, fmt.Sprintf("%.1f-%.1f tanpa judul & skor 0", m.Start, m.End))
		default:
			ok = append(ok, m)
		}
	}
	if len(ok) == 0 {
		switch {
		case len(ditolak) == 0:
			return nil, nil, fmt.Errorf("%s tidak memilih satu momen pun dari transkrip — model kemungkinan terlalu kecil untuk tugas ini, coba model yang lebih kuat (mis. `ollama pull qwen2.5`)", engine)
		case kosong == len(ditolak):
			return nil, nil, fmt.Errorf("%s hanya membalas isian kosong (%d momen tanpa judul & skor 0) — model terlalu kecil untuk prompt ini, coba model yang lebih kuat (mis. `ollama pull qwen2.5`)", engine, kosong)
		}
		return nil, nil, fmt.Errorf("%s membalas batas waktu yang tidak valid, semua ditolak: %s",
			engine, strings.Join(ditolak, "; "))
	}
	return ok, ditolak, nil
}

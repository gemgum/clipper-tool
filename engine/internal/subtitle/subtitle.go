// Package subtitle menghasilkan berkas subtitle (.ass) untuk klip.
package subtitle

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/types"
)

const (
	playResX = 1080
	playResY = 1920
	margin   = 60 // margin kiri/kanan (ruang aman)
	maxLines = 3  // maksimum baris per tampilan; sisanya jadi halaman berikut
)

func assColor(c string) string {
	switch strings.ToLower(c) {
	case "yellow":
		return "&H0000FFFF"
	case "white", "":
		return "&H00FFFFFF"
	case "black":
		return "&H00000000"
	}
	if strings.HasPrefix(c, "#") && len(c) == 7 {
		r, g, b := c[1:3], c[3:5], c[5:7]
		return "&H00" + strings.ToUpper(b+g+r)
	}
	return "&H00FFFFFF"
}

func styleColors(s config.Subtitle) (primary, secondary string) {
	if s.Karaoke {
		hl := s.Color
		if strings.EqualFold(hl, "white") || hl == "" {
			hl = "yellow"
		}
		return assColor(hl), assColor("white")
	}
	return assColor(s.Color), assColor("white")
}

func assHeader(s config.Subtitle) string {
	bold := 0
	if s.Bold {
		bold = 1
	}
	font := s.Font
	if font == "" {
		font = "Montserrat"
	}
	primary, secondary := styleColors(s)
	outline := s.Outline
	if outline < 0 {
		outline = 4
	}
	borderStyle, shadow := 1, 2
	back := "&H90000000"
	if s.Box {
		borderStyle = 3
		shadow = 0
		back = "&HA0000000"
	}
	// WrapStyle 2 = hanya baris manual (\N); kita atur pembungkusan sendiri.
	return fmt.Sprintf(`[Script Info]
ScriptType: v4.00+
PlayResX: %d
PlayResY: %d
WrapStyle: 2

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,%s,%d,%s,%s,%s,%s,%d,0,0,0,100,100,0,0,%d,%d,%d,5,%d,%d,40,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
`, playResX, playResY, font, s.Size, primary, secondary, assColor(s.OutlineColor), back, bold, borderStyle, outline, shadow, margin, margin)
}

func tc(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	h := int(sec) / 3600
	m := (int(sec) % 3600) / 60
	s := int(sec) % 60
	cs := int((sec - float64(int(sec))) * 100)
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}

func escapeText(t string) string {
	t = strings.ReplaceAll(t, "\n", " ")
	t = strings.ReplaceAll(t, "{", "(")
	t = strings.ReplaceAll(t, "}", ")")
	return strings.TrimSpace(t)
}

// maxCharsPerLine memperkirakan jumlah karakter yang muat 1 baris berdasarkan
// ukuran font & lebar aman. Faktor 0.6 (over-estimate) agar tidak meluber.
func maxCharsPerLine(size int) int {
	if size <= 0 {
		size = 72
	}
	usable := playResX - 2*margin
	n := int(float64(usable) / (float64(size) * 0.6))
	if n < 6 {
		n = 6
	}
	return n
}

// wrapWords membungkus daftar kata menjadi baris (tiap baris <= maxChars).
func wrapWords(words []string, maxChars int) [][]string {
	var lines [][]string
	var cur []string
	curLen := 0
	for _, w := range words {
		add := len(w)
		if curLen > 0 {
			add++ // spasi
		}
		if curLen > 0 && curLen+add > maxChars {
			lines = append(lines, cur)
			cur = []string{w}
			curLen = len(w)
		} else {
			cur = append(cur, w)
			curLen += add
		}
	}
	if len(cur) > 0 {
		lines = append(lines, cur)
	}
	return lines
}

// WriteASS menulis .ass; teks panjang dibungkus (maks maxLines baris/tampilan),
// kelebihannya dipecah jadi halaman berurutan dalam rentang waktu segmen.
func WriteASS(path string, segs []types.TranscriptSegment, clipStart float64, sub config.Subtitle) error {
	x, y := sub.X, sub.Y
	if x <= 0 {
		x = playResX / 2
	}
	if y <= 0 {
		y = playResY / 2
	}
	pos := fmt.Sprintf("{\\an5\\pos(%d,%d)}", x, y)
	maxChars := maxCharsPerLine(sub.Size)

	var b strings.Builder
	b.WriteString(assHeader(sub))

	for _, s := range segs {
		start := s.Start - clipStart
		end := s.End - clipStart
		if end <= 0 {
			continue
		}
		if start < 0 {
			start = 0
		}
		text := escapeText(s.Text)
		if text == "" {
			continue
		}
		words := strings.Fields(text)
		if len(words) == 0 {
			continue
		}
		lines := wrapWords(words, maxChars)

		// Bagi baris menjadi halaman berukuran <= maxLines.
		total := len(words)
		wordsSeen := 0
		segDur := end - start
		for i := 0; i < len(lines); i += maxLines {
			pageLines := lines[i:min(i+maxLines, len(lines))]
			pageWords := 0
			for _, ln := range pageLines {
				pageWords += len(ln)
			}
			pStart := start + segDur*float64(wordsSeen)/float64(total)
			wordsSeen += pageWords
			pEnd := start + segDur*float64(wordsSeen)/float64(total)

			body := renderPage(pageLines, sub.Karaoke, pEnd-pStart, pageWords)
			fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s%s\n", tc(pStart), tc(pEnd), pos, body)
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// renderPage membangun teks satu halaman (baris digabung \N).
func renderPage(lines [][]string, karaoke bool, durSec float64, pageWords int) string {
	if !karaoke {
		rendered := make([]string, len(lines))
		for i, ln := range lines {
			rendered[i] = strings.Join(ln, " ")
		}
		return strings.Join(rendered, `\N`)
	}
	// Karaoke: durasi per kata dibagi rata dalam halaman.
	totalCS := int(math.Round(durSec * 100))
	if pageWords < 1 {
		pageWords = 1
	}
	if totalCS < pageWords {
		totalCS = pageWords
	}
	per := totalCS / pageWords
	rem := totalCS - per*pageWords
	idx := 0
	rendered := make([]string, len(lines))
	for i, ln := range lines {
		var sb strings.Builder
		for _, w := range ln {
			cs := per
			if idx < rem {
				cs++
			}
			idx++
			fmt.Fprintf(&sb, "{\\k%d}%s ", cs, w)
		}
		rendered[i] = strings.TrimSpace(sb.String())
	}
	return strings.Join(rendered, `\N`)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

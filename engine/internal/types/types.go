// Package types memuat tipe domain bersama agar paket lain tidak saling impor.
package types

// TranscriptSegment satu potongan transkrip dengan timing (detik).
type TranscriptSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// Transcript hasil transkripsi lengkap.
type Transcript struct {
	Language string              `json:"language"`
	Segments []TranscriptSegment `json:"segments"`
}

// Candidate kandidat klip (jendela waktu) sebelum dinilai.
type Candidate struct {
	Start float64             `json:"start"`
	End   float64             `json:"end"`
	Text  string              `json:"text"`
	Segs  []TranscriptSegment `json:"-"`
}

// Duration klip dalam detik.
func (c Candidate) Duration() float64 { return c.End - c.Start }

// Reasons rincian skor per dimensi (0-100).
type Reasons struct {
	Hook         int `json:"hook"`
	Emotion      int `json:"emotion"`
	Clarity      int `json:"clarity"`
	Shareability int `json:"shareability"`
	Standalone   int `json:"standalone"`
}

// Clip hasil akhir satu klip.
type Clip struct {
	ID           string   `json:"id"`
	JobID        string   `json:"job_id"`
	Start        float64  `json:"start"`
	End          float64  `json:"end"`
	Duration     float64  `json:"duration"`
	Score        int      `json:"score"`
	Reasons      Reasons  `json:"reasons"`
	Title        string   `json:"title"`
	Hashtags     []string `json:"hashtags"`
	Transcript   string   `json:"transcript"`
	SubtitlePath string   `json:"subtitle_path"`
	VideoPath    string   `json:"video_path"`
	Status       string   `json:"status"` // scored | rendering | rendered
}

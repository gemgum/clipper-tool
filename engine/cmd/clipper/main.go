// Command clipper: engine pemotong video (CLI + server).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/api"
	"github.com/gemgum/clipper/engine/internal/caption"
	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/correct"
	"github.com/gemgum/clipper/engine/internal/job"
	"github.com/gemgum/clipper/engine/internal/pipeline"
	"github.com/gemgum/clipper/engine/internal/writer"
)

// version diisi saat rilis lewat -ldflags "-X main.version=…" dari nomor tag,
// jadi banner engine dan nama pemasang selalu menyebut versi yang sama. Nilai
// di bawah ini yang terpakai saat dibangun sendiri.
var version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	layout := config.Locate(projectRoot())
	loadDotEnv(layout.EnvFile)

	switch os.Args[1] {
	case "run":
		cmdRun(layout, os.Args[2:])
	case "write":
		cmdWrite(layout, os.Args[2:])
	case "caption":
		cmdCaption(layout, os.Args[2:])
	case "serve":
		cmdServe(layout, os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println("clipper", version)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `clipper `+version+`

Usage:
  clipper run <video> [flags]    Process a single video (CLI)
  clipper write <url>... [flags] Write one article from up to 5 source articles
  clipper caption <video>... [flags]  Write scroll-stopping captions for one or
                                more videos, one .txt next to each video's name
  clipper serve [flags]          Run the HTTP API for the GUI
  clipper version

'write' flags:
  -engine      ollama|claude|openai|gemini|deepseek (default ollama). Cloud engines
               need their key in .env — see the Requirements page in the app.
  -model       model name; empty uses the engine's own default
  -write-engine  engine for the WRITING stage, when it should differ from the one
               that reads the facts. Reading is five cheap calls; writing is one
               call where model quality shows. Empty uses -engine for both.
  -write-model   model for -write-engine
  -url         address of the local server (default: found automatically)
  -lang        en|id — language of the fixed text the engine writes (default en)
  -out         where to write the article folder (default <data>/posts)
  -max-words   article body budget sent to the model per source (default 1500)

'caption' flags:
  -engine      ollama|claude|openai|gemini|deepseek (default ollama). Cloud engines
               need their key in .env — see the Requirements page in the app.
  -model       model name; empty uses the engine's own default
  -minutes     how much of each video to listen to (default 5). Longer videos are
               not refused — only the first minutes are read, and the .txt says so.
  -variants    how many captions to write per video (default 3). You pick one;
               that choice is the only guard a machine-written caption has.
  -lang        id|en — language of the captions (default id)
  -terms       comma-separated correct spellings of names and regional words, the
               same list the clip page uses. Whisper writes down the nearest word
               it knows instead, and a wrong name shows far more in a caption.
  -whisper     whisper model: tiny|base|small|medium|large-v3 (default small).
               This is the single biggest lever on caption quality: a caption can
               only be as good as the words it was written from.
  -transcript-fix on|off — let the same engine fix the transcript's misheard words
               before the captions are written (default on). Without it the captions
               copy whatever Whisper misheard, hashtags included.
  -out         one folder for every .txt. The default writes each .txt next to
               its own video, which is where you go looking for it.

'run' flags:
  -model       whisper model: tiny|base|small|medium|large-v3 (default small)
  -reframe     how the video is fitted into the 9:16 frame (default center):
                 center      Center of the Picture — crop to fill
                 fit         Whole Picture — the entire video, nothing cropped
  -background  blur|black — fills the space the video cannot reach. This is what
               "fit" exists for; it also shows up when -zoom is below 100.
  -zoom        in steps of 5, read relative to the starting point of -reframe.
               Both modes stop at 100, where the picture already fills the frame:
                 fit     0 = the whole video fits — the starting point, and the
                             only direction is up (sides get cropped as it grows)
                       100 = the video fills the frame
                 center 100 = the centre crop fills the frame — the starting
                             point, and the only direction is down
                         5 = the centre crop shrinks inside the frame
               Defaults to the starting point of the mode you chose.
  -sub-mode    normal|karaoke|word — subtitle style (default normal)
  -sub-speed   slow|normal|dense — subtitle pacing (default normal)
  -save        burn|clean|both — burned-in / clean / both (default burn)
  -duration    auto|30|60|90|120|180 — clip length (default auto)
  -provider    moment selector: ollama (default), heuristic, claude, or any
               engine configured on the Requirements page
  -ollama-model  local model for the ollama provider (default qwen2.5)
  -transcript-fix on|off — let an LLM fix the transcript's punctuation, sentence
               structure and misheard words before clips are cut (default on).
               Needs an LLM even when -provider is heuristic: Claude in hybrid
               mode, Ollama otherwise. Turn it off to use the raw transcript.
  -terms       comma-separated correct spellings of the words this video uses
               that Whisper is unlikely to know: people's names, words from a
               regional language, and abbreviations. Whisper writes down the
               nearest word it does know instead, so the correction step uses
               this list to put them back. Needs -transcript-fix on.
  -max         maximum number of clips (default 10)
  -min-score   minimum score 0-100 (default 0)
  -llm-model   Claude model (default claude-haiku-4-5)
  -out         clip output folder (default data/cli)

'serve' flags:
  -addr        listen address. Default: 127.0.0.1:8787 from a source checkout,
               a random free port when installed (the port and the session key
               are written to engine.json in the data folder).
  -token       auto|on|off — require a session key on every request. "auto"
               turns it on for an installed app and off in a source checkout,
               where the GUI dev server has no way to receive the key.
`)
}

func cmdRun(layout config.Layout, args []string) {
	// Pisahkan path video (positional) dari flag agar urutan argumen bebas
	// (paket flag standar berhenti di positional pertama).
	input, flagArgs := splitInput(args)

	fs := flag.NewFlagSet("run", flag.ExitOnError)
	opts := config.DefaultOptions()
	model := fs.String("model", opts.WhisperModel, "")
	reframe := fs.String("reframe", string(opts.Reframe), "")
	background := fs.String("background", opts.Background, "")
	zoom := fs.Int("zoom", opts.Zoom, "")
	fps := fs.Int("fps", opts.FPS, "")
	resolution := fs.String("resolution", opts.Resolution, "")
	quality := fs.String("quality", opts.Quality, "")
	subMode := fs.String("sub-mode", opts.Subtitle.Mode, "")
	subSpeed := fs.String("sub-speed", opts.Subtitle.Speed, "")
	save := fs.String("save", opts.SubtitleOutput, "")
	provider := fs.String("provider", opts.Provider, "")
	ollamaModel := fs.String("ollama-model", opts.OllamaModel, "")
	transcriptFix := fs.String("transcript-fix", opts.TranscriptFix, "")
	terms := fs.String("terms", "", "")
	duration := fs.String("duration", opts.DurationPreset, "")
	maxClips := fs.Int("max", opts.MaxClips, "")
	minScore := fs.Int("min-score", opts.MinScore, "")
	llmModel := fs.String("llm-model", opts.LLMModel, "")
	outDir := fs.String("out", opts.OutputDir, "")
	_ = fs.Parse(flagArgs)

	if input == "" {
		fmt.Fprintln(os.Stderr, "error: the video path is required. Example: clipper run video.mp4")
		os.Exit(1)
	}

	opts.WhisperModel = *model
	opts.Reframe = config.Reframe(*reframe)
	opts.Background = *background
	opts.Zoom = *zoom
	opts.FPS = *fps
	opts.Resolution = *resolution
	opts.Quality = *quality
	opts.Subtitle.Mode = *subMode
	opts.Subtitle.Speed = *subSpeed
	opts.SubtitleOutput = *save
	opts.Provider = *provider
	opts.OllamaModel = *ollamaModel
	opts.TranscriptFix = *transcriptFix
	opts.Terms = correct.ParseTerms(*terms)
	opts.DurationPreset = *duration
	opts.MaxClips = *maxClips
	opts.MinScore = *minScore
	opts.LLMModel = *llmModel
	opts.OutputDir = *outDir
	// Nilai bawaan -zoom hanya cocok untuk mode center. Bila pengguna memilih
	// mode lain TANPA menyebut -zoom, dipakai titik awal mode itu — kalau tidak,
	// "clipper run -reframe fit" justru menghasilkan gambar yang terpotong
	// penuh, kebalikan dari arti modenya.
	zoomGiven := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "zoom" {
			zoomGiven = true
		}
	})
	if !zoomGiven {
		opts.Zoom = config.NaturalZoom(opts.Reframe)
	}

	if err := opts.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if err := layout.Ensure(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	paths := config.ResolvePaths(layout, opts)
	if opts.Provider == "claude" && paths.APIKey == "" {
		fmt.Fprintln(os.Stderr, "warning: -provider claude but ANTHROPIC_API_KEY is empty — the job will stop when the engine is called")
	}

	dir := "cli_" + time.Now().Format("2006-01-02_15-04-05")
	workDir := filepath.Join(paths.DataDir, dir)
	destDir := workDir
	if opts.OutputDir != "" {
		destDir = filepath.Join(opts.OutputDir, dir)
	}
	p := pipeline.New(paths, opts)

	clips, err := p.Run(context.Background(), "cli", input, workDir, destDir, func(pr pipeline.Progress) {
		if pr.Clip == nil {
			fmt.Fprintf(os.Stderr, "[%3.0f%%] %-12s %s\n", pr.Value*100, pr.Stage, pr.Message)
		}
		// Ringkasan dicetak apa adanya: ia tabel multi-baris, jadi awalan
		// "[100%] done" di depannya justru merusak perataan kolomnya.
		if pr.Summary != "" {
			fmt.Fprintln(os.Stderr, pr.Summary)
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAILED:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\n%d clips written to %s:\n", len(clips), destDir)
	for _, c := range clips {
		title := c.Title
		if title == "" {
			title = firstWords(c.Transcript, 8)
		}
		fmt.Fprintf(os.Stderr, "  %s  score %3d  %6.1fs-%6.1fs  %s\n", c.ID, c.Score, c.Start, c.End, title)
	}
	// JSON lengkap ke stdout (bisa di-pipe).
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(clips)
}

// cmdWrite menjalankan pembuat berita dari CLI.
//
// Ada supaya fitur ini bisa dipakai dan diukur sebelum tab GUI-nya dibuat —
// dan supaya ada satu jalan yang tidak bergantung pada browser sama sekali.
func cmdWrite(layout config.Layout, args []string) {
	urls, flagArgs := splitPositional(args, map[string]bool{
		"-engine": true, "-model": true, "-url": true,
		"-write-engine": true, "-write-model": true,
		"-lang": true, "-out": true, "-max-words": true,
	})

	fs := flag.NewFlagSet("write", flag.ExitOnError)
	engine := fs.String("engine", "ollama", "")
	model := fs.String("model", "", "")
	writeEngine := fs.String("write-engine", "", "")
	writeModel := fs.String("write-model", "", "")
	llmURL := fs.String("url", "", "")
	lang := fs.String("lang", "en", "")
	outDir := fs.String("out", "", "")
	maxWords := fs.Int("max-words", 0, "")
	_ = fs.Parse(flagArgs)

	if len(urls) == 0 {
		fmt.Fprintln(os.Stderr, "error: at least one source article URL is required. Example: clipper write https://… https://…")
		os.Exit(1)
	}
	if err := layout.Ensure(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	paths := config.ResolvePaths(layout, config.DefaultOptions())

	// Alamat ditimpa lewat lingkungan, bukan lewat parameter: pemilih mesin
	// membaca .env sebagai satu-satunya sumber kebenaran (notes/39), dan satu
	// jalur berarti CLI dan GUI tidak bisa berbeda pendapat.
	if *llmURL != "" {
		_ = os.Setenv("OLLAMA_HOST", *llmURL)
	}
	read, readName, err := api.EngineFor(*engine, *model)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	var write writer.Completer
	writeName := readName
	if *writeEngine != "" {
		write, writeName, err = api.EngineFor(*writeEngine, *writeModel)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}
	out := *outDir
	if out == "" {
		out = filepath.Join(paths.DataDir, "posts")
	}
	if writeName != readName {
		fmt.Fprintf(os.Stderr, "engine: read %s · write %s\n", readName, writeName)
	} else {
		fmt.Fprintf(os.Stderr, "engine: %s\n", readName)
	}

	res, err := writer.Run(context.Background(),
		writer.Options{URLs: urls, Lang: *lang, MaxWords: *maxWords},
		writer.Deps{
			Read:        read,
			ReadEngine:  readName,
			Write:       write,
			WriteEngine: writeName,
			Browse:      api.Browser(paths.Chrome),
			CacheDir:    paths.DataDir,
			OutDir:      out,
		},
		func(p writer.Progress) {
			// Ringkasan waktu itu tabel multi-baris; awalan "[100%] done" di
			// depannya merusak perataan kolomnya.
			if p.Stage == "done" {
				fmt.Fprintln(os.Stderr, p.Message)
				return
			}
			fmt.Fprintf(os.Stderr, "[%3.0f%%] %-10s %s\n", p.Value*100, p.Stage, p.Message)
		})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAILED:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\nWritten to %s\n", res.Post.Dir)
	if n := len(res.Draft.Violations); n > 0 {
		fmt.Fprintf(os.Stderr, "%d unverified item(s) — listed at the top of artikel.md\n", n)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
}

// cmdCaption membuat caption dari CLI, untuk satu video atau sekaligus banyak.
//
// Bulk BUKAN jalan kedua: satu berkas cuma daftar sepanjang satu. Kegagalan
// satu video dilaporkan lalu antriannya lanjut — 29 berkas tidak dibuang karena
// yang ke-30 tidak punya suara.
func cmdCaption(layout config.Layout, args []string) {
	videos, flagArgs := splitPositional(args, map[string]bool{
		"-engine": true, "-model": true, "-minutes": true, "-variants": true,
		"-lang": true, "-terms": true, "-whisper": true, "-out": true,
		"-transcript-fix": true,
	})

	fs := flag.NewFlagSet("caption", flag.ExitOnError)
	defaults := config.DefaultOptions()
	engine := fs.String("engine", "ollama", "")
	model := fs.String("model", "", "")
	minutes := fs.Float64("minutes", caption.DefaultMaxSeconds/60, "")
	variants := fs.Int("variants", caption.DefaultVariants, "")
	lang := fs.String("lang", defaults.Language, "")
	terms := fs.String("terms", "", "")
	whisperModel := fs.String("whisper", defaults.WhisperModel, "")
	transcriptFix := fs.String("transcript-fix", "on", "")
	outDir := fs.String("out", "", "")
	_ = fs.Parse(flagArgs)

	if len(videos) == 0 {
		fmt.Fprintln(os.Stderr, "error: at least one video path is required. Example: clipper caption clip_01.mp4 clip_02.mp4")
		os.Exit(1)
	}
	if err := layout.Ensure(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// Transkripsi memakai tahap yang sama dengan pipeline klip, jadi opsinya
	// pun opsi pipeline — hanya model whisper & bahasanya yang berlaku di sini.
	opts := defaults
	opts.WhisperModel = *whisperModel
	opts.Language = *lang
	paths := config.ResolvePaths(layout, opts)

	complete, engineName, err := api.EngineFor(*engine, *model)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "engine: %s · whisper %s · first %.0f min per video\n", engineName, opts.WhisperModel, *minutes)

	fix := *transcriptFix != "off"
	p := pipeline.New(paths, opts)
	work := filepath.Join(paths.DataDir, "cache", "caption")
	res, err := caption.Run(context.Background(),
		caption.Options{
			Videos:     videos,
			MaxSeconds: *minutes * 60,
			Lang:       *lang,
			Terms:      correct.ParseTerms(*terms),
			Variants:   *variants,
			Fix:        &fix,
			OutDir:     *outDir,
			EngineName: engineName,
		},
		caption.Deps{
			Complete: complete,
			Engine:   engineName,
			Transcribe: caption.Whisper(p, work, func(msg string) {
				fmt.Fprintln(os.Stderr, "         ", msg)
			}),
		},
		func(pr caption.Progress) {
			fmt.Fprintf(os.Stderr, "[%3.0f%%] %-12s %s\n", pr.Value*100, pr.Stage, pr.Message)
		})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAILED:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\n%d of %d video(s):\n", res.Done(), len(res.Files))
	for _, f := range res.Files {
		if f.Error != "" {
			fmt.Fprintf(os.Stderr, "  %-30s FAILED: %s\n", f.Name, f.Error)
			continue
		}
		fmt.Fprintf(os.Stderr, "  %-30s %d caption(s)  %s\n", f.Name, len(f.Variants), f.TXT)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
}

// splitPositional memisahkan argumen positional (alamat artikel, path video)
// dari flag, sehingga urutan argumen bebas — paket flag standar berhenti di
// positional pertama.
//
// valueFlags menyebut flag yang MEMBAWA nilai terpisah ("-model llama3.1"),
// sebab nilai itu tidak boleh terbaca sebagai positional.
func splitPositional(args []string, valueFlags map[string]bool) (positional []string, flagArgs []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if valueFlags[a] {
			flagArgs = append(flagArgs, a)
			if i+1 < len(args) {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			flagArgs = append(flagArgs, a)
			continue
		}
		positional = append(positional, a)
	}
	return positional, flagArgs
}

func cmdServe(layout config.Layout, args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	// Kosong = pilih sendiri: 8787 di checkout (GUI pengembangan menunjuk ke
	// sana), port acak saat terpasang. Port tetap di mesin pengguna berarti
	// program lain tahu persis ke mana harus mengetuk.
	addr := fs.String("addr", "", "")
	jobsN := fs.Int("jobs", 1, "")
	// Kunci sesi: "auto" (menyala saat terpasang), "on", atau "off".
	tokenMode := fs.String("token", "auto", "")
	// -shell dipakai jendela aplikasi: satu baris yang bisa dibaca mesin,
	// dicetak SEBELUM banner, berisi alamat lengkap + kunci. Jendela membacanya
	// dari stdout lalu membuka alamat itu. Sengaja bukan berkas: shell adalah
	// induk proses engine, jadi pipa stdout satu-satunya jalur yang pasti sudah
	// siap dan tidak bisa tertukar dengan engine lain yang kebetulan jalan.
	shell := fs.Bool("shell", false, "")
	_ = fs.Parse(args)

	opts := config.DefaultOptions()
	if err := layout.Ensure(); err != nil {
		fmt.Fprintln(os.Stderr, "data dir error:", err)
		os.Exit(1)
	}
	paths := config.ResolvePaths(layout, opts)
	mgr := job.NewManager(layout, paths, *jobsN)
	srv := api.NewServer(mgr, layout)
	api.Version = version // dilaporkan lewat /api/health, dipakai panel setelan

	listenAddr := *addr
	if listenAddr == "" {
		listenAddr = "127.0.0.1:8787"
		if !layout.Dev {
			listenAddr = "127.0.0.1:0"
		}
	}
	// Didengarkan lebih dulu, baru dilaporkan: dengan port 0, nomor portnya
	// baru ada setelah sistem operasi memberikannya.
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot listen on", listenAddr+":", err)
		os.Exit(1)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	// Engine hanya menjawab alamatnya sendiri (lihat guard.go). Bila pengguna
	// menyebut -addr sendiri, alamat itu ikut diterima — mengikat ke alamat lain
	// adalah keputusan sadar, dan menolaknya di sini berarti flagnya diam-diam
	// tidak berfungsi.
	if host, _, err := net.SplitHostPort(listenAddr); err == nil {
		srv.AllowHost(host)
	}

	token := ""
	if *tokenMode == "on" || (*tokenMode == "auto" && !layout.Dev) {
		token = api.NewToken()
		srv.SetToken(token)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	handshake, err := api.WriteHandshake(paths.DataDir, api.Handshake{
		URL: url, Port: port, Token: token, PID: os.Getpid(), Version: version,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot write the handshake file:", err)
		os.Exit(1)
	}

	if *shell {
		fmt.Printf("%s%s\n", api.ShellURLPrefix, api.AppURL(url, token))
	}

	fmt.Printf("clipper engine %s\n", version)
	fmt.Printf("  whisper : %s\n", paths.Whisper)
	fmt.Printf("  model   : %s\n", paths.Model)
	fmt.Printf("  data    : %s%s\n", paths.DataDir, devNote(layout))
	fmt.Printf("  API key : %s\n", maskKey(paths.APIKey))
	fmt.Printf("  gui     : %s\n", api.GUIStatus(layout.GUIDir))
	fmt.Printf("  key     : %s\n", keyNote(token))
	fmt.Printf("  address : %s\n", handshake)
	// Satu alamat yang tinggal dibuka — inilah yang dipakai jendela aplikasi,
	// dan yang bisa ditempel sendiri ke browser saat mengembangkan.
	fmt.Printf("\n  open    : %s\n\n", api.AppURL(url, token))

	if err := http.Serve(ln, srv.Handler()); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

// keyNote menerangkan keadaan kunci sesi dalam satu baris.
func keyNote(token string) string {
	if token == "" {
		return "(off — anything on this computer can talk to the engine)"
	}
	return "on (a new key for every run, written to the address file)"
}

// splitInput memisahkan argumen positional (path video) dari flag, sehingga
// pengguna bebas menaruh path sebelum atau sesudah flag.
func splitInput(args []string) (input string, flagArgs []string) {
	valueFlags := map[string]bool{
		"-mode": true, "-model": true, "-reframe": true,
		"-max": true, "-min-score": true, "-llm-model": true, "-out": true, "-fps": true,
		"-resolution": true, "-quality": true, "-background": true, "-zoom": true,
		"-sub-mode": true, "-sub-speed": true, "-save": true, "-duration": true,
		"-provider": true, "-ollama-model": true, "-transcript-fix": true,
		"-terms": true,
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if valueFlags[a] { // bentuk "-flag nilai"
			flagArgs = append(flagArgs, a)
			if i+1 < len(args) {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(a, "-") { // "-flag=nilai" atau flag lain
			flagArgs = append(flagArgs, a)
			continue
		}
		if input == "" {
			input = a
			continue
		}
		flagArgs = append(flagArgs, a)
	}
	return input, flagArgs
}

// --- util ---

func projectRoot() string {
	if v := os.Getenv("CLIPPER_ROOT"); v != "" {
		return v
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if filepath.Base(dir) == "bin" {
			return filepath.Dir(dir)
		}
	}
	wd, _ := os.Getwd()
	return wd
}

// loadDotEnv memuat KEY=VALUE dari .env (tanpa menimpa env yang sudah ada).
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

// devNote menandai bahwa folder datanya ada di dalam checkout, bukan di folder
// data pengguna. Sengaja dicetak: dua bentuk itu tidak dipilih dengan flag, jadi
// baris inilah satu-satunya cara mengetahui yang mana yang sedang berlaku.
func devNote(l config.Layout) string {
	if l.Dev {
		return "  (source checkout)"
	}
	return ""
}

func maskKey(k string) string {
	if k == "" {
		return "(empty — offline mode)"
	}
	if len(k) > 10 {
		return k[:8] + "…"
	}
	return "(set)"
}

func firstWords(s string, n int) string {
	f := strings.Fields(s)
	if len(f) > n {
		f = f[:n]
	}
	return strings.Join(f, " ")
}

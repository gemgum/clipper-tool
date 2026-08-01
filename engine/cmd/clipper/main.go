// Command clipper: engine pemotong video (CLI + server).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gemgum/clipper/engine/internal/api"
	"github.com/gemgum/clipper/engine/internal/config"
	"github.com/gemgum/clipper/engine/internal/job"
	"github.com/gemgum/clipper/engine/internal/pipeline"
)

const version = "0.1.0-mvp"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	root := projectRoot()
	loadDotEnv(filepath.Join(root, ".env"))

	switch os.Args[1] {
	case "run":
		cmdRun(root, os.Args[2:])
	case "serve":
		cmdServe(root, os.Args[2:])
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
  clipper serve [flags]          Run the HTTP API for the GUI
  clipper version

'run' flags:
  -mode        offline|hybrid|online (default offline)
  -model       whisper model: tiny|base|small|medium|large-v3 (default small)
  -reframe     how the video is fitted into the 9:16 frame (default center):
                 center      Center of the Picture — crop to fill
                 fit         Whole Picture — the entire video, nothing cropped
                 face_follow Follow Face (not available yet)
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
  -style       plain|viral (default plain)
  -sub-mode    normal|karaoke|word — subtitle style (default normal)
  -sub-speed   slow|normal|dense — subtitle pacing (default normal)
  -save        burn|clean|both — burned-in / clean / both (default burn)
  -duration    auto|30|60|90|120|180 — clip length (default auto)
  -provider    claude|ollama|heuristic — moment selector (default follows -mode)
  -ollama-model  local model for the ollama provider (default qwen2.5)
  -transcript-fix on|off — let an LLM fix the transcript's punctuation, sentence
               structure and misheard words before clips are cut (default on).
               Needs an LLM even when -provider is heuristic: Claude in hybrid
               mode, Ollama otherwise. Turn it off to use the raw transcript.
  -max         maximum number of clips (default 10)
  -min-score   minimum score 0-100 (default 0)
  -llm-model   Claude model (default claude-haiku-4-5)
  -out         clip output folder (default data/cli)

'serve' flags:
  -addr        listen address (default 127.0.0.1:8787)
`)
}

func cmdRun(root string, args []string) {
	// Pisahkan path video (positional) dari flag agar urutan argumen bebas
	// (paket flag standar berhenti di positional pertama).
	input, flagArgs := splitInput(args)

	fs := flag.NewFlagSet("run", flag.ExitOnError)
	opts := config.DefaultOptions()
	mode := fs.String("mode", string(opts.Mode), "")
	model := fs.String("model", opts.WhisperModel, "")
	reframe := fs.String("reframe", string(opts.Reframe), "")
	background := fs.String("background", opts.Background, "")
	zoom := fs.Int("zoom", opts.Zoom, "")
	fps := fs.Int("fps", opts.FPS, "")
	resolution := fs.String("resolution", opts.Resolution, "")
	quality := fs.String("quality", opts.Quality, "")
	style := fs.String("style", opts.SubtitleStyle, "")
	subMode := fs.String("sub-mode", opts.Subtitle.Mode, "")
	subSpeed := fs.String("sub-speed", opts.Subtitle.Speed, "")
	save := fs.String("save", opts.SubtitleOutput, "")
	provider := fs.String("provider", opts.Provider, "")
	ollamaModel := fs.String("ollama-model", opts.OllamaModel, "")
	transcriptFix := fs.String("transcript-fix", opts.TranscriptFix, "")
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

	opts.Mode = config.Mode(*mode)
	opts.WhisperModel = *model
	opts.Reframe = config.Reframe(*reframe)
	opts.Background = *background
	opts.Zoom = *zoom
	opts.FPS = *fps
	opts.Resolution = *resolution
	opts.Quality = *quality
	opts.SubtitleStyle = *style
	opts.Subtitle.Mode = *subMode
	opts.Subtitle.Speed = *subSpeed
	opts.SubtitleOutput = *save
	opts.Provider = *provider
	opts.OllamaModel = *ollamaModel
	opts.TranscriptFix = *transcriptFix
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

	paths := config.ResolvePaths(root, opts)
	if opts.Mode != config.ModeOffline && paths.APIKey == "" {
		fmt.Fprintln(os.Stderr, "warning: mode", opts.Mode, "but ANTHROPIC_API_KEY is empty — the job will stop when the selected engine is called")
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

func cmdServe(root string, args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8787", "")
	jobsN := fs.Int("jobs", 1, "")
	_ = fs.Parse(args)

	opts := config.DefaultOptions()
	paths := config.ResolvePaths(root, opts)
	if err := os.MkdirAll(paths.DataDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "data dir error:", err)
		os.Exit(1)
	}
	mgr := job.NewManager(root, paths, *jobsN)
	srv := api.NewServer(mgr, root)

	fmt.Printf("clipper engine %s\n", version)
	fmt.Printf("  whisper : %s\n", paths.Whisper)
	fmt.Printf("  model   : %s\n", paths.Model)
	fmt.Printf("  worker  : %s\n", paths.Worker)
	fmt.Printf("  API key : %s\n", maskKey(paths.APIKey))
	fmt.Printf("  listen  : http://%s\n", *addr)

	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

// splitInput memisahkan argumen positional (path video) dari flag, sehingga
// pengguna bebas menaruh path sebelum atau sesudah flag.
func splitInput(args []string) (input string, flagArgs []string) {
	valueFlags := map[string]bool{
		"-mode": true, "-model": true, "-reframe": true, "-style": true,
		"-max": true, "-min-score": true, "-llm-model": true, "-out": true, "-fps": true,
		"-resolution": true, "-quality": true, "-background": true, "-zoom": true,
		"-sub-mode": true, "-sub-speed": true, "-save": true, "-duration": true,
		"-provider": true, "-ollama-model": true, "-transcript-fix": true,
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

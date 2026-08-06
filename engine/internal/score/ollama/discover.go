package ollama

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// Menemukan Ollama SENDIRI, bukan menuntut pengguna tahu di mana ia berada.
//
// Alamat bawaan `http://localhost:11434` benar hanya untuk satu susunan: Ollama
// dan Clipper di sistem yang sama. Susunan yang benar-benar dipakai bisa
// berbeda, dan pengguna tidak punya cara tahu mana yang berlaku:
//
//   - Clipper di Windows, Ollama di WSL — dari Windows ia terlihat di localhost
//     (WSL2 meneruskan port), tapi TIDAK selalu: pada WSL tanpa penerusan port
//     ia hanya ada di IP WSL-nya sendiri;
//   - Clipper di WSL, Ollama di Windows — localhost WSL BUKAN localhost Windows;
//     alamatnya IP gerbang default (biasanya 172.x.x.1);
//   - Ollama disetel mendengarkan alamat lain lewat OLLAMA_HOST.
//
// Discover mencoba semuanya sekaligus dan memakai yang menjawab lebih dulu.
// Hasilnya di-cache: penjelajahan berlangsung di bawah satu detik, tapi ia
// dipanggil dari halaman yang menyegarkan status tiap 15 detik.

// probeTimeout: satu alamat yang tidak menjawab tidak boleh menahan sisanya.
// 1,2 detik cukup untuk jaringan loopback dan gerbang WSL, dan tetap terasa
// seketika bagi pengguna.
const probeTimeout = 1200 * time.Millisecond

// cacheFor: berapa lama alamat yang ditemukan dipercaya tanpa dicek ulang.
const cacheFor = 30 * time.Second

var (
	found struct {
		ep   Endpoint
		when time.Time
	}
	foundMu sync.Mutex
)

// Discover mengembalikan alamat server LLM lokal yang menjawab, atau "" bila
// tidak satu pun kandidat hidup.
func Discover(ctx context.Context) string { return DiscoverEndpoint(ctx).URL }

// DiscoverEndpoint mencari server LLM lokal — Ollama ATAU yang bergaya OpenAI —
// dan menyebutkan jenisnya.
func DiscoverEndpoint(ctx context.Context) Endpoint {
	foundMu.Lock()
	if found.ep.URL != "" && time.Since(found.when) < cacheFor {
		ep := found.ep
		foundMu.Unlock()
		return ep
	}
	foundMu.Unlock()

	cands := Candidates()
	type hit struct {
		i  int
		ep Endpoint
	}
	results := make(chan hit, len(cands))
	var wg sync.WaitGroup
	for i, u := range cands {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			if kind := kindOf(ctx, u); kind != "" {
				results <- hit{i, Endpoint{URL: u, Kind: kind}}
			}
		}(i, u)
	}
	wg.Wait()
	close(results)

	// Yang paling awal di daftar menang, bukan yang paling cepat menjawab:
	// urutan kandidat itu urutan kemungkinan, dan hasil balapan jaringan bukan
	// hal yang boleh menentukan konfigurasi mana yang dipakai (dua alamat bisa
	// menunjuk server berbeda, dengan model yang berbeda).
	best := hit{i: len(cands)}
	for h := range results {
		if h.i < best.i {
			best = h
		}
	}
	if best.ep.URL == "" {
		return Endpoint{}
	}
	foundMu.Lock()
	found.ep, found.when = best.ep, time.Now()
	foundMu.Unlock()
	return best.ep
}

// resetDiscoverCache dipakai uji: cache alamat membuat dua kasus uji dalam satu
// proses saling memengaruhi.
func resetDiscoverCache() {
	foundMu.Lock()
	found.ep, found.when = Endpoint{}, time.Time{}
	foundMu.Unlock()
}

// Candidates menyusun daftar alamat yang mungkin, dari yang paling mungkin.
func Candidates() []string {
	out := []string{}
	add := func(u string) {
		u = strings.TrimRight(strings.TrimSpace(u), "/")
		if u == "" {
			return
		}
		if slices.Contains(out, u) {
			return
		}
		out = append(out, u)
	}

	// 1. Yang disetel pengguna sendiri menang atas tebakan apa pun.
	if h := os.Getenv("OLLAMA_HOST"); h != "" {
		if !strings.HasPrefix(h, "http") {
			h = "http://" + h
		}
		// "0.0.0.0" berarti "mendengarkan semua"; sebagai alamat TUJUAN ia tidak
		// berarti apa-apa, jadi diterjemahkan ke loopback.
		h = strings.Replace(h, "0.0.0.0", "127.0.0.1", 1)
		add(h)
	}
	// 2. Sistem yang sama. Selain port Ollama, port yang lazim dipakai server
	// lokal bergaya OpenAI: llama.cpp & llamafile & LocalAI (8080), vLLM &
	// Aphrodite (8000), LiteLLM (4000), Exo (52415). Semuanya hanya dianggap
	// LLM bila benar-benar menjawab /v1/models — lihat probeJSON.
	add(defaultURL)
	add("http://127.0.0.1:11434")
	for _, port := range []string{"8080", "8000", "4000", "52415", "1234"} {
		add("http://127.0.0.1:" + port)
	}

	// 3. Menyeberangi batas WSL <-> Windows.
	if runtime.GOOS == "linux" {
		// Dari WSL menuju Windows: IP gerbang default, dan nameserver di
		// resolv.conf (keduanya menunjuk host Windows pada susunan NAT bawaan).
		add(hostURL(gatewayIP()))
		add(hostURL(resolvNameserver()))
	}
	// Docker Desktop & sebagian susunan WSL menyediakan nama ini.
	add("http://host.docker.internal:11434")
	return out
}

// hostURL hanya menerima alamat PRIVAT.
//
// Kandidat di sini diambil dari resolv.conf, dan di WSL tanpa penerusan DNS
// isinya bisa jadi DNS publik (1.1.1.1). Mengetuk port 11434 di alamat internet
// milik orang lain bukan hal yang boleh dilakukan aplikasi ini diam-diam —
// entah untuk mencari Ollama atau untuk apa pun.
func hostURL(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil || !(parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsLinkLocalUnicast()) {
		return ""
	}
	return "http://" + ip + ":11434"
}

// kindOf memeriksa satu alamat dan menyebutkan jenis servernya ("" = bukan
// server LLM, atau tidak menjawab).
//
// Ollama diperiksa lebih dulu: ia MENYEDIAKAN /v1 juga, dan lewat API-nya
// sendiri bentuk balasan bisa dipaksa dengan JSON Schema penuh — kemampuan yang
// hilang kalau ia diperlakukan sebagai server OpenAI biasa.
func kindOf(ctx context.Context, url string) string {
	if probeJSON(ctx, url+"/api/tags", "models") {
		return KindOllama
	}
	if probeJSON(ctx, url+"/v1/models", "data") {
		return KindOpenAI
	}
	return ""
}

// probeJSON menganggap sebuah alamat "milik kita" hanya bila ia menjawab 200
// DENGAN JSON yang memuat kunci yang diharapkan.
//
// Bukan sekadar "port 8080 terbuka": kandidat di sini mencakup port yang lazim
// dipakai server lain (8000, 8080), dan menyapa server pengembangan orang lalu
// menyimpulkan ia LLM adalah cara paling cepat membuat aplikasi ini salah.
func probeJSON(ctx context.Context, url, key string) bool {
	c, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(c, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("authorization", "Bearer local")
	resp, err := (&http.Client{Timeout: probeTimeout}).Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var probe map[string]json.RawMessage
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if json.Unmarshal(raw, &probe) != nil {
		return false
	}
	_, ok := probe[key]
	return ok
}

// gatewayIP membaca gerbang default lewat `ip route` — di WSL2 itu host Windows.
func gatewayIP() string {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "via" && i+1 < len(fields) {
			if net.ParseIP(fields[i+1]) != nil {
				return fields[i+1]
			}
		}
	}
	return ""
}

// resolvNameserver membaca /etc/resolv.conf. Pada WSL2 bawaan, nameserver-nya
// adalah host Windows — dan itu tetap benar pada susunan yang gerbangnya tidak
// bisa dibaca.
func resolvNameserver() string {
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[0] == "nameserver" && net.ParseIP(fields[1]) != nil {
			return fields[1]
		}
	}
	return ""
}

// Where menyebut DI SISTEM MANA Ollama itu berjalan, bukan sekadar alamatnya.
//
// Perlu, dan bukan kemewahan: pada mesin Windows+WSL, `http://localhost:11434`
// bisa berarti Ollama Windows ATAU Ollama WSL yang portnya diteruskan — dan
// keduanya berperilaku berbeda (yang kedua ikut mati saat WSL dimatikan).
// Membedakannya butuh satu petunjuk lagi, dan petunjuk itu ada: kalau tidak ada
// ollama.exe terpasang di Windows, yang menjawab pasti bukan Ollama Windows.
func Where(url string) string {
	if url == "" {
		return "not found"
	}
	local := strings.Contains(url, "127.0.0.1") || strings.Contains(url, "localhost")
	switch runtime.GOOS {
	case "windows":
		if !local {
			return url
		}
		if windowsOllama() {
			return "Windows, on this computer"
		}
		return "WSL (Linux), forwarded to this computer's localhost"
	case "linux":
		if local {
			if inWSL() {
				return "WSL (this Linux system)"
			}
			return "Linux, on this computer"
		}
		if inWSL() {
			return "the Windows host, from WSL"
		}
		return url
	case "darwin":
		if local {
			return "macOS, on this computer"
		}
		return url
	}
	return url
}

// OS menyebut sistemnya dalam SATU kata, untuk ditempel di sebelah label di
// GUI. Kalimat panjangnya (Where) tinggal di tooltip — label yang memuat
// "WSL (Linux), forwarded to this computer's localhost" akan melipat, dan baris
// yang melipat mengubah tinggi barisnya.
func OS(url string) string {
	if url == "" {
		return ""
	}
	local := strings.Contains(url, "127.0.0.1") || strings.Contains(url, "localhost")
	switch runtime.GOOS {
	case "windows":
		if !local {
			return "remote"
		}
		if windowsOllama() {
			return "Windows"
		}
		return "WSL"
	case "linux":
		if !local {
			if inWSL() {
				return "Windows"
			}
			return "remote"
		}
		if inWSL() {
			return "WSL"
		}
		return "Linux"
	case "darwin":
		if local {
			return "macOS"
		}
		return "remote"
	}
	return "remote"
}

// inWSL: kernel WSL selalu menyebut "microsoft" di /proc/version.
func inWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	b, err := os.ReadFile("/proc/version")
	return err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft")
}

// windowsOllama melaporkan apakah Ollama benar-benar TERPASANG di Windows.
func windowsOllama() bool {
	if _, err := exec.LookPath("ollama.exe"); err == nil {
		return true
	}
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		if _, err := os.Stat(dir + `\Programs\Ollama\ollama.exe`); err == nil {
			return true
		}
	}
	return false
}

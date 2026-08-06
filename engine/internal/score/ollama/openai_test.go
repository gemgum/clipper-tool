package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Server bergaya OpenAI (llama.cpp, LocalAI, llamafile, vLLM, Aphrodite,
// LiteLLM, Exo) harus dikenali DAN dipakai tanpa setelan apa pun.
func TestOpenAIServerDiscoveredAndUsed(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags": // BUKAN Ollama
			w.WriteHeader(404)
		case "/v1/models":
			w.Write([]byte(`{"object":"list","data":[{"id":"Qwen2.5-1.5B-Instruct"},{"id":"SmolLM2-360M"}]}`))
		case "/v1/chat/completions":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.Write([]byte(`{"choices":[{"message":{"content":"{\"segments\":[]}"}}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	t.Setenv("OLLAMA_HOST", srv.URL)
	resetDiscoverCache()

	ep := DiscoverEndpoint(context.Background())
	if ep.Kind != KindOpenAI || ep.URL != srv.URL {
		t.Fatalf("endpoint = %+v, mau kind=openai url=%s", ep, srv.URL)
	}

	st := Status(context.Background(), "")
	if !st.Running || len(st.Installed) != 2 || st.Installed[0].Name != "Qwen2.5-1.5B-Instruct" {
		t.Fatalf("status = %+v", st)
	}

	c := New(srv.URL, "Qwen2.5-1.5B-Instruct")
	c.Kind = KindOpenAI
	out, err := c.Complete(context.Background(), "sys", "user", map[string]any{"type": "object"}, 512)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if out != `{"segments":[]}` {
		t.Fatalf("isi balasan = %q", out)
	}
	// Bentuk permintaannya harus yang disepakati bersama, bukan bentuk Ollama.
	if gotBody["model"] != "Qwen2.5-1.5B-Instruct" {
		t.Fatalf("model tidak ikut terkirim: %v", gotBody["model"])
	}
	if _, ok := gotBody["response_format"]; !ok {
		t.Fatal("response_format tidak dikirim — bentuk balasan jadi tidak dijamin")
	}
	if _, ok := gotBody["messages"]; !ok {
		t.Fatal("messages tidak dikirim")
	}
}

// Port yang kebetulan terbuka TAPI bukan server LLM tidak boleh dianggap LLM.
func TestNonLLMServerIsIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>halaman biasa</html>`))
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_HOST", srv.URL)
	resetDiscoverCache()
	if ep := DiscoverEndpoint(context.Background()); ep.URL == srv.URL {
		t.Fatalf("server biasa dikira LLM: %+v", ep)
	}
}

// Ukuran model dari NAMANYA — satu-satunya petunjuk yang ada pada server yang
// tidak memberi metadata.
func TestParamsFromName(t *testing.T) {
	cases := map[string]float64{
		"Qwen2.5-1.5B-Instruct": 1.5,
		"SmolLM2-360M":          0.36,
		"Llama-3.2-3b":          3,
		"llama-3.1-8B-instruct": 8,
		"DeepSeek-R1-1.5B":      1.5,
		"Phi-4-Mini-3.8B":       3.8,
		"gpt-oss:120b-cloud":    120,
		"models/gemma-3-4b-it":  4,
		"Llama-3.2":             0, // versi, bukan ukuran
		"tanpa-angka":           0,
	}
	for name, want := range cases {
		if got := paramsFromName(name); got != want {
			t.Errorf("paramsFromName(%q) = %v, mau %v", name, got, want)
		}
	}
	if got := formatParams(0.36); got != "360M" {
		t.Errorf("formatParams(0.36) = %q", got)
	}
	if got := formatParams(1.5); got != "1.5B" {
		t.Errorf("formatParams(1.5) = %q", got)
	}
}

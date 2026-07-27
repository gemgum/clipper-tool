// clipper-worker — worker native C++ untuk engine Go.
//
// Kontrak: baca 1 request JSON dari stdin, balas baris-baris NDJSON ke stdout,
// exit code 0 = sukses. Tanpa dependensi eksternal (untuk MVP).
//
// Perintah MVP:
//   features : baca WAV 16kHz mono PCM16, hitung RMS energi per hop.
//   reframe  : (placeholder) center-crop dilakukan ffmpeg oleh engine;
//              face-follow (OpenCV) menyusul di milestone berikutnya.

#include <cstdint>
#include <cstdio>
#include <cmath>
#include <fstream>
#include <iostream>
#include <sstream>
#include <string>
#include <vector>

// --- util JSON minimal (khusus field yang kita butuhkan) ---

static std::string readAll(std::istream& in) {
    std::ostringstream ss;
    ss << in.rdbuf();
    return ss.str();
}

// Ambil nilai string untuk sebuah key: "key":"value"
static std::string jsonString(const std::string& j, const std::string& key) {
    std::string pat = "\"" + key + "\"";
    size_t p = j.find(pat);
    if (p == std::string::npos) return "";
    p = j.find(':', p);
    if (p == std::string::npos) return "";
    p = j.find('"', p);
    if (p == std::string::npos) return "";
    size_t start = p + 1;
    std::string out;
    for (size_t i = start; i < j.size(); ++i) {
        if (j[i] == '\\' && i + 1 < j.size()) { out += j[i + 1]; ++i; continue; }
        if (j[i] == '"') break;
        out += j[i];
    }
    return out;
}

// Ambil nilai integer untuk sebuah key: "key":123
static long jsonInt(const std::string& j, const std::string& key, long fallback) {
    std::string pat = "\"" + key + "\"";
    size_t p = j.find(pat);
    if (p == std::string::npos) return fallback;
    p = j.find(':', p);
    if (p == std::string::npos) return fallback;
    ++p;
    while (p < j.size() && (j[p] == ' ' || j[p] == '\t')) ++p;
    size_t start = p;
    while (p < j.size() && (isdigit(j[p]) || j[p] == '-')) ++p;
    if (p == start) return fallback;
    try { return std::stol(j.substr(start, p - start)); } catch (...) { return fallback; }
}

static void emitError(const std::string& msg) {
    std::cout << "{\"type\":\"error\",\"message\":\"" << msg << "\"}\n";
}

// --- pembacaan WAV (RIFF/PCM16 mono) ---

static uint32_t rd32(const unsigned char* p) {
    return p[0] | (p[1] << 8) | (p[2] << 16) | (uint32_t(p[3]) << 24);
}
static uint16_t rd16(const unsigned char* p) { return p[0] | (p[1] << 8); }

// Baca sampel PCM16 mono. Mengisi 'samples' (dinormalisasi -1..1) & sampleRate.
static bool readWav(const std::string& path, std::vector<float>& samples, int& sampleRate) {
    std::ifstream f(path, std::ios::binary);
    if (!f) return false;
    std::vector<unsigned char> buf((std::istreambuf_iterator<char>(f)),
                                   std::istreambuf_iterator<char>());
    if (buf.size() < 44) return false;
    if (buf[0] != 'R' || buf[1] != 'I' || buf[2] != 'F' || buf[3] != 'F') return false;

    size_t pos = 12; // lewati RIFF header
    int channels = 1, bits = 16;
    sampleRate = 16000;
    size_t dataOff = 0, dataLen = 0;
    while (pos + 8 <= buf.size()) {
        char id[5] = {char(buf[pos]), char(buf[pos + 1]), char(buf[pos + 2]), char(buf[pos + 3]), 0};
        uint32_t sz = rd32(&buf[pos + 4]);
        size_t body = pos + 8;
        if (std::string(id) == "fmt ") {
            channels = rd16(&buf[body + 2]);
            sampleRate = (int)rd32(&buf[body + 4]);
            bits = rd16(&buf[body + 14]);
        } else if (std::string(id) == "data") {
            dataOff = body;
            dataLen = sz;
        }
        pos = body + sz + (sz & 1); // chunk di-align genap
    }
    if (dataOff == 0 || bits != 16) return false;
    if (dataOff + dataLen > buf.size()) dataLen = buf.size() - dataOff;

    size_t n = dataLen / 2;
    samples.reserve(n / (channels > 0 ? channels : 1));
    for (size_t i = 0; i + 1 < dataLen; i += 2 * (channels > 0 ? channels : 1)) {
        int16_t s = (int16_t)rd16(&buf[dataOff + i]); // kanal pertama
        samples.push_back(s / 32768.0f);
    }
    return true;
}

// --- perintah ---

static int cmdFeatures(const std::string& input, int hopMs) {
    std::vector<float> samples;
    int sr = 16000;
    if (!readWav(input, samples, sr)) {
        emitError("gagal baca WAV: " + input);
        return 1;
    }
    if (hopMs <= 0) hopMs = 100;
    size_t hop = (size_t)((long long)sr * hopMs / 1000);
    if (hop == 0) hop = 1;

    std::ostringstream rms;
    rms << "[";
    bool first = true;
    for (size_t i = 0; i < samples.size(); i += hop) {
        double sum = 0.0;
        size_t end = std::min(i + hop, samples.size());
        for (size_t k = i; k < end; ++k) sum += (double)samples[k] * samples[k];
        double val = std::sqrt(sum / (double)(end - i));
        if (!first) rms << ",";
        char b[32];
        std::snprintf(b, sizeof(b), "%.5f", val);
        rms << b;
        first = false;
    }
    rms << "]";

    std::cout << "{\"type\":\"result\",\"rms\":" << rms.str()
              << ",\"hop_ms\":" << hopMs << "}\n";
    return 0;
}

static int cmdReframe(const std::string& input, const std::string& output) {
    // Placeholder: engine (Go) menangani center-crop via ffmpeg untuk MVP.
    // Milestone berikut: baca frame dengan OpenCV, hitung crop ikut wajah.
    (void)input;
    emitError("reframe belum diimplementasi di worker (dipakai ffmpeg oleh engine); output=" + output);
    return 2;
}

int main() {
    std::string req = readAll(std::cin);
    std::string cmd = jsonString(req, "cmd");
    if (cmd == "features") {
        std::string input = jsonString(req, "input");
        int hopMs = (int)jsonInt(req, "hop_ms", 100);
        if (input.empty()) { emitError("field 'input' kosong"); return 1; }
        return cmdFeatures(input, hopMs);
    } else if (cmd == "reframe") {
        return cmdReframe(jsonString(req, "input"), jsonString(req, "output"));
    }
    emitError("perintah tidak dikenal: " + cmd);
    return 1;
}

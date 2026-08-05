"use client";

// Kamus antarmuka. Bahasa Inggris adalah sumber kebenaran: kunci diambil dari
// objek `en`, sehingga TypeScript menolak kompilasi bila terjemahan Indonesia
// kehilangan satu entri pun.
//
// Yang diterjemahkan hanya teks antarmuka. Isi artikel, judul klip, dan pesan
// galat dari engine tetap apa adanya — itu data, bukan label.

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

export const LANGUAGES = ["en", "id"] as const;
export type Language = (typeof LANGUAGES)[number];

const STORAGE_KEY = "clipper.lang";

const en = {
  // --- shared ---
  tabClips: "Video clips",
  tabNews: "News cards",
  tabRequirements: "Requirements",
  save: "Save",
  downloading: "downloading…",
  loading: "Loading…",
  engineUnreachable: "Cannot reach the engine at {url}. If you started the app from a terminal, check that window for an error.",

  // --- source panel ---
  uploadingPct: "Uploading… {pct}%",
  pickerFileTitle: "Choose a video on this computer",
  pickerFolderTitle: "Choose a folder",
  pickerUp: "Up",
  pickerGo: "Open",
  pickerEmpty: "This folder is empty",
  pickerTruncated: "Long folder — only the first items are shown",
  pickerUseFolder: "Use this folder",
  pickerCancel: "Close",
  pickerFileHint: "Click a file to use it. Nothing is copied.",
  pickerFolderHint: "Open a folder, then press \u201cUse this folder\u201d.",
  videoPath: "Source video",
  outputDir: "Output folder",
  outputDirPlaceholder: "Default: data/<job>",

  // --- render settings ---
  groupEngine: "Engine",
  mode: "Mode",
  modeOffline: "Offline (free)",
  modeHybrid: "Hybrid (Claude API)",
  whisperModel: "Whisper model",
  modelNotDownloaded: "not downloaded",

  groupQuality: "Quality",
  resolution: "Resolution",
  quality: "Quality",
  qualityDraft: "Draft (fast)",
  qualityHd: "HD (balanced)",
  qualityMax: "Maximum (slow)",
  fps: "FPS",
  fpsTip: "Motion smoothness, not sharpness",
  fpsSource: "Match source",

  fitMode: "Fit",
  fitModeTip: "How the video is placed into the upright 9:16 frame",
  fitCenter: "Crop to fill",
  fitWhole: "Whole video",
  zoom: "Zoom",
  zoomTip:
    "How far the video is zoomed in from the starting point of the mode above. Steps of 5%.",
  background: "Background",
  backgroundTip: "Used when the video does not cover the whole frame",
  backgroundBlur: "Blurred video",
  backgroundBlack: "Black",

  groupClips: "Output",
  clipDuration: "Clip length",
  clipDurationTip: "How long each clip is",
  durationAuto: "Automatic",
  durationAbout: "± {n}",
  maxClips: "Max clips",
  maxClipsTip: "Upper bound on clips produced from one video",
  saveClips: "Save as",
  saveClipsTip:
    "A clean clip is useful if you want to redo the subtitles in another editor",
  saveBurn: "With subtitles",
  saveClean: "Without subtitles",
  saveBoth: "Both files",

  // --- AI engine panel ---
  apiKeyClaude: "Claude API key",
  keyStored: "✓ stored",
  keyPlaceholderStored: "•••• (type to replace)",
  claudeModel: "Claude model",
  claudeHaiku: "Haiku 4.5 (fast)",
  claudeSonnet: "Sonnet 5 (balanced)",
  claudeOpus: "Opus 4.8 (best)",
  noKeyWarning:
    "No API key yet — hybrid mode will fail. Enter a key, or switch to offline mode.",
  offlineEngine: "Scoring engine",
  offlineOllama: "Local AI (Ollama)",
  offlineHeuristic: "Rules only (no AI)",
  transcriptFix: "Correct transcript with AI",
  transcriptFixTip:
    "Speech recognition leaves dialogue dashes, misplaced punctuation and misheard words behind",
  transcriptFixNeedsLLM:
    "Needs an LLM. If unreachable the job stops — untick to use the raw transcript.",
  terms: "Names & terms",
  termsPlaceholder: "People's names, local words, abbreviations",
  localModel: "Ollama model",
  modelNeedsDownload: "needs download",
  modelReady: "✓ ready",
  modelNotCapable: "⚠ not capable enough",
  ollamaNotDetected:
    "⚠ Ollama not detected. Install it from ollama.com, then run",
  recheck: "check again",
  downloadModel: "download model",
  downloadSelected: "download the selected model",

  // --- subtitle settings ---
  groupVideoInFrame: "Frame",
  emptyFrame: "No preview",
  loadPreview: "Load preview",
  loadingPreview: "Loading preview…",
  reloadPreview: "Reload",
  resetPreview: "Clear",
  guidesAlways: "Centre guides",
  settingsTitle: "Settings",
  settingsLanguage: "Language",
  settingsComponents: "Components",
  settingsReady: "ready",
  settingsMissing: "missing",
  settingsOpenFull: "Open the full Requirements page →",
  tabResults: "Results",
  tabHistory: "Output history",
  resultsSub: "Clips from the run that just finished.",
  resultsEmpty: "No clips yet — start a run on the Video clips page.",
  historySub: "Every run so far, newest first.",
  historyEmpty: "Nothing has been rendered yet.",
  historyShow: "Show clips",
  historyHide: "Hide clips",
  grid: "Grid",
  gridOff: "Off",
  previewTime: "Frame at {t}s",
  zoneTop: "covered by top UI",
  zoneBottom: "caption & account name",
  zoneRight: "action buttons",

  font: "Font",
  fontOther: "Other…",
  fontPlaceholder: "e.g. Poppins",
  fontChecking: "Checking font…",
  fontHint:
    "Type a font family name — letters/digits, spaces, dots, ' & or -",
  fontFound: "✓ {family} found ({source})",
  color: "Colour",
  colorWhite: "White",
  colorYellow: "Yellow",
  colorGreen: "Green",
  colorCyan: "Light blue",
  groupSubtitle: "Subtitle",
  boxBackground: "Background box",
  size: "Size",
  outline: "Outline",
  subStyle: "Style",
  subStyleTip: "How words appear on screen",
  subNormal: "Whole sentences",
  subKaraoke: "Word highlight",
  subWord: "One word at a time",
  highlightColor: "Highlight",
  subSpeed: "Pacing",
  subSpeedTip: "Slower = less text at once, held longer",
  speedSlow: "Slow",
  speedNormal: "Normal",
  speedDense: "Dense",
  groupPlacement: "Placement",
  position: "Position",
  resetCentre: "Centre",
  platformGuide: "Platform",
  platformGuideTip: "Areas covered by the app's buttons and caption",
  platformNone: "None",
  platformGeneric: "Generic",
  placeSafe: "Move to safe area",
  unsafeWarning: "The subtitle sits under the app's own buttons",
  sampleWord: "Sample",
  // Contoh subtitle: sebanyak baris terbanyak yang bisa muncul sekaligus.
  // Panjangnya dijaga <= 19 karakter — batas satu baris di engine pada ukuran
  // font bawaan sekitar 22, dan baris contoh tidak boleh melipat sendiri.
  sampleLine1: "This is how your",
  sampleLine2: "subtitle will look",
  sampleLine3: "at this pacing",

  // --- run + results ---
  modelMissingWarn: "Model {model} has not been downloaded. Run",
  modelMissingWarnTail: "then reload.",
  start: "Start processing",
  processing: "Processing…",
  cancel: "Cancel",
  statusDone: "Done",
  statusStopped: "Stopped",
  log: "Process log",
  logEmpty: "Nothing yet — the log fills up once a job starts.",
  clipCount: "{n} clips",
  noTitle: "(untitled)",
  reasonHook: "hook",
  reasonEmotion: "emo",
  reasonClarity: "clear",
  reasonShare: "share",
  reasonStandalone: "standalone",
  downloadWithSubs: "with subtitles",
  downloadPlain: "download",
  downloadTxtTip:
    "The spoken words, no timestamps — paste it into any AI to write the caption",
  downloadClean: "clean",

  // --- log lines ---
  logReconnect: "↻ Reconnecting to a running job: {id}",
  logKeySaved: "✓ API key saved",
  logKeyEmpty: "⚠ API key is empty",
  logKeyFailed: "⚠ Could not save the API key",
  logPullStart: "↓ Downloading model {model} via Ollama (may take minutes)…",
  logPullDone: "✓ Model {model} is ready",
  logPullFailed: "⚠ Model download failed: {error}",
  logLocating: "Looking for {name} on this computer…",
  logLocated: "✓ Found on this computer: {path} — used in place, nothing copied",
  logLocateMiss: "{name} was not found on this computer — uploading a copy instead",
  logUploadStart: "↑ Uploading {name} ({size} MB)…",
  logUploadDone: "✓ Upload finished: {name}",
  logUploadFailed: "⚠ Upload failed: {error}",
  logUploadInvalid: "⚠ Invalid upload response",
  logUploadNetwork: "⚠ Upload failed (network)",

  // --- requirements page ---
  reqTitle: "Requirements",
  reqSubtitle: "What Clipper needs to work, and what is already on this computer.",
  reqGroupTools: "Programs Clipper runs",
  reqGroupModels: "Speech recognition models",
  reqGroupApps: "Separate applications",
  reqRequired: "required",
  reqInstall: "Install",
  reqInstalling: "Installing…",
  reqRemove: "Remove",
  reqRemoved: "Removed.",
  reqRefresh: "Check again",
  reqOpenDownload: "Open the download page",
  reqPathPick: "Point to it myself",
  reqPathChange: "Use another file",
  reqPathSaved: "Saved — this file will be used from now on.",
  reqMissing: "Not ready yet: {list}. Clips cannot be made until these are installed.",
  reqAllReady: "Everything needed for making clips is installed.",
  reqWhereTitle: "Where things are kept",
  reqFolderClips: "Finished clips",
  reqFolderCards: "News cards",
  reqFolderChange: "Change…",
  reqFolderReset: "Back to default",
  reqFolderDefault: "Default — inside the app's data folder.",
  reqWhereModels: "Models",
  reqWhereTools: "Programs",
  reqWhereData: "Clips & cache",
  reqDevNote: "Running from the source checkout, so these stay inside the project folder.",

  logFontInvalid: "⚠ Font \"{font}\" is not valid yet — fix the name first",
  logJobStart: "▶ Job — {resolution}/{quality}, duration={duration}, font={font}",
  logJobCreated: "Job {id}",
  logCancelling: "✕ Cancelling…",
  logPreviewFailed: "⚠ Preview failed: {error}",
  logClip: "{id} (score {score})",
  logFinished: "✓ Finished",
  errCreateJob: "could not create the job",
  errReadVideo: "could not read the video",
  errOldEngine:
    "the response is not JSON — old engine? Stop it and run ./bin/clipper serve again",

  // --- news card page ---
  newsTitle: "News cards",
  newsIntro:
    "Turn an article into a ready-to-post image. The card text and caption are taken",
  newsIntroBold: "verbatim from the article",
  newsIntroTail: "— the AI only picks which part is most interesting, it never rewrites.",
  browserMissing:
    "No browser found. Cards are rendered with Chrome/Chromium — install one, or set",
  browserMissingTail: "to the chrome.exe path.",
  tabPasteLink: "Paste a link",
  tabBrowse: "Browse news",
  searchPlaceholder: "Search news — e.g. jakarta flood",
  search: "Search",
  searching: "Searching…",
  articleLink: "Article link",
  fetch: "Fetch",
  fetching: "Reading…",
  searchResultsFor: "Search results for",
  backToSources: "✕ back to sources",
  newsSource: "News source",
  loadingNews: "Loading news…",
  copyLink: "copy",
  copied: "✓ copied",
  copyOpening: "… opening",
  copyLinkTitle: "Copy the article link so you can check it yourself",

  analyzeHeading: "Pick the most interesting part (the AI selects, it does not write)",
  engine: "Engine",
  engineOllama: "Ollama (local, free)",
  engineClaude: "Claude (API)",
  model: "Model",
  analyze: "Analyse article",
  analyzing: "Analysing…",
  analyzingLocal: "The local model is reading the article — usually 20–60 seconds.",
  rankingIntro:
    "{n} paragraphs, ordered by hook strength. Click one to use it as the card text or the caption.",
  scoredByEngine: "scored by {engine}",
  scoredAuto: "scored automatically by the engine",
  auto: "auto",
  useOnCard: "Use on the card",
  onCard: "✓ on the card",
  useAsCaption: "Use as caption",
  asCaption: "✓ as caption",

  cardContent: "Card content (editable)",
  articleTitle: "Title",
  cardText: "Text on the card",
  fromParagraph: "(paragraph #{n} of the article)",
  sourceBadge: "Source (badge)",
  date: "Date",
  imageURL: "Image URL",
  imageUnusedInQuote: "(unused in the quote style)",
  articleLinkCheck: "— to check for yourself before posting",
  caption: "Caption",
  hashtags: "Hashtags",
  hashtagsPlaceholder: "#Example #Tag",
  photoFrame: "Photo frame",
  photoFrameHint: "— drag to move, scroll to zoom",
  photoZoom: "Zoom {n}×",
  photoZoomHint: "Zoom in first so the photo can be moved.",
  photoOffset: "offset: {x}, {y} px",
  photoReset: "Reset",
  style: "Style",
  styleDark: "Dark",
  styleLight: "Light",
  styleQuote: "Quote (no photo)",
  ratio: "Ratio",
  ratioStory: "(Story/Reels)",
  ratioFeed: "(IG feed)",
  ratioSquare: "(square)",
  textAlign: "Text alignment",
  alignLeft: "Left",
  alignCenter: "Centre",
  alignRight: "Right",
  alignJustify: "Justified",
  justifyNote:
    "Justified text stretches the gaps between words. On a large heading with only a few words per line the gaps can look wide.",
  photoFit: "How the photo fits",
  photoFitCover: "Fill the frame — sides cropped",
  photoFitWhole: "Whole photo — nothing cropped",
  photoFill: "Fill the leftover space with",
  photoFillBlur: "A blurred copy of the photo",
  photoFillSolid: "The card background colour",

  cardColour: "Card colour",
  cardColourHint: "— sets the background, the paper and the supporting text at once",
  colourFromPhoto: "Take it from the photo",
  colourCustom: "Choose it myself",
  colourSwatchNote:
    "Only the hue is used — brightness stays locked so the text is always readable. That is why the choice is a fixed set rather than a full colour picker. Selected: {hex}",
  colourFromPhotoNote:
    "The hue comes from the article photo. Brightness stays fixed so the text is always readable.",
  cardBoxBackground: "Paragraph box",
  boxAuto: "Paper, matching the card colour",
  boxNone: "No box — text sits on the photo",
  boxCustom: "Paper, my own colour",
  boxNoneNote:
    "Without the box the text gets a shadow instead. On a busy photo it is still harder to read.",


  fontSizes: "Font sizes",
  fontTitle: "Article title",
  fontParagraph: "Paragraph",
  fontStandard: "standard",
  fontStepsNote:
    "Steps away from the standard template — 0 is the standard. A long paragraph still shrinks on its own so it cannot run off the card.",
  fontReset: "Back to standard",
  previewEmpty: "No preview yet — press Preview.",
  headerSpace: "Push the content down",
  headerSpaceNote:
    "Moves the title, the paragraph box and the footer down together, as one block — the text keeps its size. It stops once the content reaches the bottom edge, so how far it can go depends on how long the paragraph is.",
  cardDown: "Lower the whole card",
  cardDownNote:
    "Moves the photo down as well, leaving an empty band at the top in the card colour. Use this when the app crops the top of your card.",
  previewCard: "Preview",
  previewResult: "Preview",
  previewNotSaved:
    "Preview only — nothing has been saved yet. Press “Create card” to keep this one.",
  buildCard: "Create card",
  rendering: "Rendering…",
  result: "Result",
  downloadZip: "Download ZIP (image + caption + source)",
  downloadImage: "Download the image only",
  sourceLabel: "Source:",
  creditSource: "Credit the source when posting.",
  errLoadNews: "could not load the news",
  errReadArticle: "could not read the article",
  errAnalyze: "could not analyse the article",
  errBuildCard: "could not create the card",
  errOpenLink: "could not open the link",
  errCopy:
    "Could not copy automatically — copy it manually from the source link.",
} as const;

export type MessageKey = keyof typeof en;

const id: Record<MessageKey, string> = {
  tabClips: "Klip video",
  tabNews: "Kartu berita",
  tabRequirements: "Kebutuhan",
  save: "Simpan",
  downloading: "mengunduh…",
  loading: "Memuat…",
  engineUnreachable: "Tidak bisa menghubungi engine di {url}. Kalau aplikasi dijalankan dari terminal, lihat jendela itu — pesannya ada di sana.",

  uploadingPct: "Mengunggah… {pct}%",
  pickerFileTitle: "Pilih video di komputer ini",
  pickerFolderTitle: "Pilih folder",
  pickerUp: "Naik",
  pickerGo: "Buka",
  pickerEmpty: "Folder ini kosong",
  pickerTruncated: "Folder panjang — hanya sebagian awal yang ditampilkan",
  pickerUseFolder: "Pakai folder ini",
  pickerCancel: "Tutup",
  pickerFileHint: "Klik satu berkas untuk memakainya. Tidak ada yang disalin.",
  pickerFolderHint: "Buka foldernya, lalu tekan \u201cPakai folder ini\u201d.",
  videoPath: "Video sumber",
  outputDir: "Folder hasil",
  outputDirPlaceholder: "Bawaan: data/<job>",

  groupEngine: "Mesin",
  mode: "Mode",
  modeOffline: "Offline (gratis)",
  modeHybrid: "Hybrid (Claude API)",
  whisperModel: "Model Whisper",
  modelNotDownloaded: "belum diunduh",

  groupQuality: "Kualitas",
  resolution: "Resolusi",
  quality: "Kualitas",
  qualityDraft: "Draf (cepat)",
  qualityHd: "HD (seimbang)",
  qualityMax: "Maksimum (lambat)",
  fps: "FPS",
  fpsTip: "Kehalusan gerak, bukan ketajaman",
  fpsSource: "Ikut sumber",

  fitMode: "Pemasangan",
  fitModeTip: "Cara video dimasukkan ke bingkai tegak 9:16",
  fitCenter: "Potong penuh",
  fitWhole: "Video utuh",
  zoom: "Zoom",
  zoomTip:
    "Seberapa jauh video di-zoom dari titik awal mode di atas. Kelipatan 5%.",
  background: "Latar",
  backgroundTip: "Dipakai saat video tidak menutupi seluruh bingkai",
  backgroundBlur: "Blur videonya",
  backgroundBlack: "Hitam",

  groupClips: "Hasil",
  clipDuration: "Durasi klip",
  clipDurationTip: "Panjang tiap klip",
  durationAuto: "Otomatis",
  durationAbout: "± {n}",
  maxClips: "Maks. klip",
  maxClipsTip: "Batas atas jumlah klip dari 1 video",
  saveClips: "Simpan sebagai",
  saveClipsTip:
    "Klip polos berguna bila subtitle mau diatur ulang di editor lain",
  saveBurn: "Dengan subtitle",
  saveClean: "Tanpa subtitle",
  saveBoth: "Dua-duanya",

  apiKeyClaude: "API Key Claude",
  keyStored: "✓ tersimpan",
  keyPlaceholderStored: "•••• (isi untuk mengganti)",
  claudeModel: "Model Claude",
  claudeHaiku: "Haiku 4.5 (cepat)",
  claudeSonnet: "Sonnet 5 (seimbang)",
  claudeOpus: "Opus 4.8 (terbaik)",
  noKeyWarning:
    "Belum ada API key — mode hybrid akan gagal. Masukkan key, atau pindah ke mode offline.",
  offlineEngine: "Mesin skor",
  offlineOllama: "AI lokal (Ollama)",
  offlineHeuristic: "Aturan saja (tanpa AI)",
  transcriptFix: "Koreksi transkrip dengan AI",
  transcriptFixTip:
    "Pengenalan suara menyisakan tanda hubung dialog, tanda baca salah tempat, dan kata salah dengar",
  transcriptFixNeedsLLM:
    "Butuh LLM. Bila tak terjangkau job berhenti — hilangkan centang untuk transkrip mentah.",
  terms: "Nama & istilah",
  termsPlaceholder: "Nama orang, kata daerah, singkatan",
  localModel: "Model Ollama",
  modelNeedsDownload: "perlu unduh",
  modelReady: "✓ siap",
  modelNotCapable: "⚠ kurang memadai",
  ollamaNotDetected:
    "⚠ Ollama tak terdeteksi. Pasang dari ollama.com lalu jalankan",
  recheck: "cek ulang",
  downloadModel: "unduh model",
  downloadSelected: "unduh model terpilih",

  groupVideoInFrame: "Bingkai",
  emptyFrame: "Belum ada pratinjau",
  loadPreview: "Muat pratinjau",
  loadingPreview: "Memuat preview…",
  reloadPreview: "Muat ulang",
  resetPreview: "Bersihkan",
  guidesAlways: "Garis tengah",
  settingsTitle: "Setelan",
  settingsLanguage: "Bahasa",
  settingsComponents: "Komponen",
  settingsReady: "siap",
  settingsMissing: "belum ada",
  settingsOpenFull: "Buka halaman Requirements lengkap →",
  tabResults: "Hasil",
  tabHistory: "Riwayat keluaran",
  resultsSub: "Klip dari pekerjaan yang baru saja selesai.",
  resultsEmpty: "Belum ada klip — mulai satu pekerjaan di halaman Video clips.",
  historySub: "Semua pekerjaan sejauh ini, terbaru di atas.",
  historyEmpty: "Belum ada yang pernah dirender.",
  historyShow: "Tampilkan klip",
  historyHide: "Sembunyikan klip",
  grid: "Kisi",
  gridOff: "Mati",
  previewTime: "Frame pada {t}s",
  zoneTop: "ketutup UI atas",
  zoneBottom: "caption & nama akun",
  zoneRight: "tombol aksi",

  font: "Font",
  fontOther: "Lainnya…",
  fontPlaceholder: "mis. Poppins",
  fontChecking: "Memeriksa font…",
  fontHint: "Ketik nama family font — huruf/angka, spasi, titik, ' & atau -",
  fontFound: "✓ {family} ditemukan ({source})",
  color: "Warna",
  colorWhite: "Putih",
  colorYellow: "Kuning",
  colorGreen: "Hijau",
  colorCyan: "Biru muda",
  groupSubtitle: "Subtitle",
  boxBackground: "Kotak latar",
  size: "Ukuran",
  outline: "Garis tepi",
  subStyle: "Gaya",
  subStyleTip: "Cara kata ditampilkan di layar",
  subNormal: "Kalimat utuh",
  subKaraoke: "Sorot per kata",
  subWord: "Satu kata sekali tampil",
  highlightColor: "Sorotan",
  subSpeed: "Kerapatan",
  subSpeedTip: "Makin lambat = makin sedikit teks sekaligus & tampil lebih lama",
  speedSlow: "Lambat",
  speedNormal: "Normal",
  speedDense: "Padat",
  groupPlacement: "Penempatan",
  position: "Posisi",
  resetCentre: "Tengah",
  platformGuide: "Platform",
  platformGuideTip: "Area yang tertutup tombol & caption aplikasi",
  platformNone: "Tidak ada",
  platformGeneric: "Umum",
  placeSafe: "Pindah ke area aman",
  unsafeWarning: "Subtitle tertutup tombol aplikasinya sendiri",
  sampleWord: "Contoh",
  sampleLine1: "Beginilah tampilan",
  sampleLine2: "subtitle Anda nanti",
  sampleLine3: "pada kecepatan ini",

  modelMissingWarn: "Model {model} belum diunduh. Jalankan",
  modelMissingWarnTail: "lalu muat ulang.",
  start: "Mulai proses",
  processing: "Memproses…",
  cancel: "Batalkan",
  statusDone: "Selesai",
  statusStopped: "Berhenti",
  log: "Log proses",
  logEmpty: "Belum ada — log terisi begitu job dijalankan.",
  clipCount: "{n} klip",
  noTitle: "(tanpa judul)",
  reasonHook: "hook",
  reasonEmotion: "emo",
  reasonClarity: "jelas",
  reasonShare: "share",
  reasonStandalone: "mandiri",
  downloadWithSubs: "bersubtitle",
  downloadPlain: "unduh",
  downloadTxtTip:
    "Ucapan klipnya tanpa timestamp — tempel ke AI mana pun untuk dibuatkan caption",
  downloadClean: "polos",

  logReconnect: "↻ Menyambung ke job berjalan: {id}",
  logKeySaved: "✓ API key tersimpan",
  logKeyEmpty: "⚠ API key kosong",
  logKeyFailed: "⚠ Gagal menyimpan API key",
  logPullStart: "↓ Mengunduh model {model} via Ollama (bisa beberapa menit)…",
  logPullDone: "✓ Model {model} siap",
  logPullFailed: "⚠ Unduh model gagal: {error}",
  logLocating: "Mencari {name} di komputer ini…",
  logLocated: "✓ Ketemu di komputer ini: {path} — dipakai di tempat, tanpa salinan",
  logLocateMiss: "{name} tidak ditemukan di komputer ini — diunggah sebagai salinan",
  logUploadStart: "↑ Unggah {name} ({size} MB)…",
  logUploadDone: "✓ Unggah selesai: {name}",
  logUploadFailed: "⚠ Unggah gagal: {error}",
  logUploadInvalid: "⚠ Respons unggah tidak valid",
  logUploadNetwork: "⚠ Unggah gagal (jaringan)",

  // --- halaman kebutuhan ---
  reqTitle: "Kebutuhan",
  reqSubtitle: "Yang diperlukan Clipper untuk bekerja, dan yang sudah ada di komputer ini.",
  reqGroupTools: "Program yang dijalankan Clipper",
  reqGroupModels: "Model pengenal suara",
  reqGroupApps: "Aplikasi terpisah",
  reqRequired: "wajib",
  reqInstall: "Pasang",
  reqInstalling: "Memasang…",
  reqRemove: "Hapus",
  reqRemoved: "Dihapus.",
  reqRefresh: "Periksa lagi",
  reqOpenDownload: "Buka halaman unduh",
  reqPathPick: "Cari sendiri",
  reqPathChange: "Ganti berkasnya",
  reqPathSaved: "Tersimpan — berkas ini yang dipakai mulai sekarang.",
  reqMissing: "Belum siap: {list}. Klip belum bisa dibuat sebelum ini terpasang.",
  reqAllReady: "Semua yang dibutuhkan untuk membuat klip sudah terpasang.",
  reqWhereTitle: "Tempat penyimpanan",
  reqFolderClips: "Klip hasil",
  reqFolderCards: "Kartu berita",
  reqFolderChange: "Ubah…",
  reqFolderReset: "Kembali ke bawaan",
  reqFolderDefault: "Bawaan — di dalam folder data aplikasi.",
  reqWhereModels: "Model",
  reqWhereTools: "Program",
  reqWhereData: "Klip & cache",
  reqDevNote: "Sedang jalan dari checkout sumber, jadi semuanya tetap di dalam folder proyek.",

  logFontInvalid: "⚠ Font \"{font}\" belum valid — perbaiki dulu namanya",
  logJobStart: "▶ Job — {resolution}/{quality}, durasi={duration}, font={font}",
  logJobCreated: "Job {id}",
  logCancelling: "✕ Membatalkan…",
  logPreviewFailed: "⚠ Preview gagal: {error}",
  logClip: "{id} (skor {score})",
  logFinished: "✓ Selesai",
  errCreateJob: "gagal membuat job",
  errReadVideo: "gagal membaca video",
  errOldEngine:
    "respons bukan JSON — engine versi lama? Hentikan lalu jalankan ulang ./bin/clipper serve",

  newsTitle: "Kartu berita",
  newsIntro:
    "Ubah artikel jadi gambar siap posting. Isi kartu dan caption diambil",
  newsIntroBold: "apa adanya dari artikel",
  newsIntroTail: "— AI hanya memilih bagian mana yang paling menarik, tidak menulis ulang.",
  browserMissing:
    "Browser tidak ditemukan. Kartu dirender memakai Chrome/Chromium — pasang salah satunya, atau set",
  browserMissingTail: "ke path chrome.exe.",
  tabPasteLink: "Tempel link",
  tabBrowse: "Jelajah berita",
  searchPlaceholder: "Cari berita — mis. banjir jakarta",
  search: "Cari",
  searching: "Mencari…",
  articleLink: "Tautan artikel",
  fetch: "Ambil",
  fetching: "Membaca…",
  searchResultsFor: "Hasil pencarian",
  backToSources: "✕ kembali ke sumber",
  newsSource: "Sumber berita",
  loadingNews: "Memuat berita…",
  copyLink: "salin",
  copied: "✓ tersalin",
  copyOpening: "… membuka",
  copyLinkTitle: "Salin tautan artikel untuk dicek sendiri",

  analyzeHeading: "Pilih bagian paling menarik (AI memilih, bukan menulis)",
  engine: "Mesin",
  engineOllama: "Ollama (lokal, gratis)",
  engineClaude: "Claude (API)",
  model: "Model",
  analyze: "Analisis artikel",
  analyzing: "Menganalisis…",
  analyzingLocal: "Model lokal membaca artikel — biasanya 20–60 detik.",
  rankingIntro:
    "{n} paragraf, diurutkan dari yang paling ber-hook. Klik untuk memakainya sebagai isi kartu atau caption.",
  scoredByEngine: "dinilai {engine}",
  scoredAuto: "dinilai otomatis oleh engine",
  auto: "auto",
  useOnCard: "Pakai di kartu",
  onCard: "✓ di kartu",
  useAsCaption: "Jadikan caption",
  asCaption: "✓ jadi caption",

  cardContent: "Isi kartu (boleh disunting)",
  articleTitle: "Judul",
  cardText: "Teks di kartu",
  fromParagraph: "(paragraf #{n} dari artikel)",
  sourceBadge: "Sumber (badge)",
  date: "Tanggal",
  imageURL: "URL gambar",
  imageUnusedInQuote: "(tidak dipakai di gaya kutipan)",
  articleLinkCheck: "— untuk dicek sendiri sebelum diposting",
  caption: "Caption",
  hashtags: "Hashtag",
  hashtagsPlaceholder: "#Contoh #Tagar",
  photoFrame: "Bingkai foto",
  photoFrameHint: "— seret untuk menggeser, gulir untuk zoom",
  photoZoom: "Zoom {n}×",
  photoZoomHint: "Perbesar dulu agar foto bisa digeser.",
  photoOffset: "geser: {x}, {y} px",
  photoReset: "Kembalikan",
  style: "Gaya",
  styleDark: "Gelap",
  styleLight: "Terang",
  styleQuote: "Kutipan (tanpa foto)",
  ratio: "Rasio",
  ratioStory: "(Story/Reels)",
  ratioFeed: "(feed IG)",
  ratioSquare: "(persegi)",
  textAlign: "Rata teks",
  alignLeft: "Kiri",
  alignCenter: "Tengah",
  alignRight: "Kanan",
  alignJustify: "Kiri-kanan (justify)",
  justifyNote:
    "Rata kiri-kanan merenggangkan jarak antar kata. Pada judul besar yang cuma beberapa kata per baris, celahnya bisa terlihat lebar.",
  photoFit: "Cara foto dipasang",
  photoFitCover: "Penuhi bingkai — sisinya terpotong",
  photoFitWhole: "Foto utuh — tidak ada yang terpotong",
  photoFill: "Ruang sisanya diisi",
  photoFillBlur: "Salinan buram fotonya",
  photoFillSolid: "Warna latar kartu",

  cardColour: "Warna kartu",
  cardColourHint: "— menyetel latar, kertas, dan teks pendukung sekaligus",
  colourFromPhoto: "Ambil dari fotonya",
  colourCustom: "Saya tentukan sendiri",
  colourSwatchNote:
    "Yang dipakai hanya ronanya — terangnya dikunci supaya teks selalu terbaca. Itu sebabnya pilihannya daftar tetap, bukan pemilih warna bebas. Dipilih: {hex}",
  colourFromPhotoNote:
    "Ronanya diambil dari foto artikel. Terangnya tetap dikunci supaya teks selalu terbaca.",
  cardBoxBackground: "Kotak paragraf",
  boxAuto: "Kertas, mengikuti warna kartu",
  boxNone: "Tanpa kotak — teks langsung di atas foto",
  boxCustom: "Kertas, warna saya sendiri",
  boxNoneNote:
    "Tanpa kotak, teks diberi bayangan sebagai gantinya. Di foto yang ramai tetap lebih sulit dibaca.",


  fontSizes: "Ukuran huruf",
  fontTitle: "Judul artikel",
  fontParagraph: "Paragraf",
  fontStandard: "standar",
  fontStepsNote:
    "Langkah dari template standar — 0 berarti standar. Paragraf panjang tetap mengecil sendiri supaya tidak keluar kartu.",
  fontReset: "Kembali ke standar",
  previewEmpty: "Belum ada pratinjau — tekan Pratinjau.",
  headerSpace: "Geser isi ke bawah",
  headerSpaceNote:
    "Menurunkan judul, kotak paragraf, dan kaki kartu bersama-sama sebagai satu blok — ukuran teksnya tidak berubah. Berhenti sendiri saat isinya menyentuh tepi bawah, jadi sejauh apa ia bisa turun tergantung panjang paragrafnya.",
  cardDown: "Turunkan seluruh kartu",
  cardDownNote:
    "Fotonya ikut turun, dan pita kosong di atas memakai warna latar kartu. Berguna saat aplikasi memotong bagian atas kartumu.",
  previewCard: "Pratinjau",
  previewResult: "Pratinjau",
  previewNotSaved:
    "Baru pratinjau — belum ada yang disimpan. Tekan “Buat kartu” untuk menyimpannya.",
  buildCard: "Buat kartu",
  rendering: "Merender…",
  result: "Hasil",
  downloadZip: "Unduh ZIP (gambar + caption + sumber)",
  downloadImage: "Unduh gambar saja",
  sourceLabel: "Sumber:",
  creditSource: "Cantumkan sumber saat memposting.",
  errLoadNews: "gagal memuat berita",
  errReadArticle: "gagal membaca artikel",
  errAnalyze: "gagal menganalisis artikel",
  errBuildCard: "gagal membuat kartu",
  errOpenLink: "gagal membuka tautan",
  errCopy: "Tidak bisa menyalin otomatis — salin manual dari tautan sumber.",
};

const dictionaries: Record<Language, Record<MessageKey, string>> = { en, id };

/** Nilai yang bisa diselipkan ke placeholder {nama} di dalam pesan. */
export type Vars = Record<string, string | number>;

function format(template: string, vars?: Vars): string {
  if (!vars) return template;
  return template.replace(/\{(\w+)\}/g, (whole, key: string) =>
    key in vars ? String(vars[key]) : whole,
  );
}

type I18nValue = {
  lang: Language;
  setLang: (l: Language) => void;
  t: (key: MessageKey, vars?: Vars) => string;
};

const I18nContext = createContext<I18nValue | null>(null);

export function I18nProvider({ children }: { children: React.ReactNode }) {
  // Selalu mulai dari "en" supaya render server dan render pertama di browser
  // sama persis; pilihan tersimpan baru diterapkan setelah hidrasi.
  const [lang, setLangState] = useState<Language>("en");

  useEffect(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored && (LANGUAGES as readonly string[]).includes(stored)) {
        setLangState(stored as Language);
      }
    } catch {}
  }, []);

  useEffect(() => {
    document.documentElement.lang = lang;
  }, [lang]);

  const setLang = useCallback((l: Language) => {
    setLangState(l);
    try {
      localStorage.setItem(STORAGE_KEY, l);
    } catch {}
  }, []);

  const t = useCallback(
    (key: MessageKey, vars?: Vars) => format(dictionaries[lang][key], vars),
    [lang],
  );

  const value = useMemo(() => ({ lang, setLang, t }), [lang, setLang, t]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used inside <I18nProvider>");
  return ctx;
}

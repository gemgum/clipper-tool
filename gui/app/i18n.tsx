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
  brandTagline: "Long videos → 9:16 HD clips with subtitles + a viral score",
  tabClips: "Video clips",
  tabNews: "News cards",
  tabRequirements: "Requirements",
  save: "Save",
  download: "download",
  downloading: "downloading…",
  loading: "Loading…",
  engineUnreachable: "Cannot reach the engine at {url}. If you started the app from a terminal, check that window for an error.",

  // --- source panel ---
  dropTitle: "Drag & drop a video here",
  dropOr: "or",
  dropPick: "pick a file",
  dropOrPaste: "· or paste a path manually",
  uploadingPct: "Uploading… {pct}%",
  browseButton: "📁 Choose from this computer",
  dropNoCopy: "A file that is already on this computer is used where it is — nothing is copied.",
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
  videoPath: "Video path",
  outputDir: "Output folder (optional — empty = data/<job>)",
  outputDirPlaceholder: "/home/user/clips",

  // --- render settings ---
  renderSettings: "Render settings",
  groupEngine: "Processing engine",
  mode: "Mode",
  modeOffline: "offline (free)",
  modeHybrid: "hybrid (Claude API)",
  whisperModel: "Whisper model",
  modelNotDownloaded: "not downloaded",

  groupQuality: "Video quality",
  resolution: "Resolution",
  quality: "Quality",
  qualityDraft: "draft (fast)",
  qualityHd: "HD (balanced)",
  qualityMax: "max (best, slow)",
  fps: "FPS",
  fpsTip: "Motion smoothness, not sharpness",
  fpsSource: "Match source",

  fitMode: "How the video fits",
  fitModeTip: "How the video is placed into the upright 9:16 frame",
  fitCenter: "Center of the Picture — crop to fill",
  fitWhole: "Whole Picture — starts with the whole video",
  fitFace: "Follow Face — not available yet",
  fitWholeNote:
    "The entire video resolution is used, so nothing is cropped at all. That is why the background exists: it fills the space the video cannot reach.",
  zoomLabel: "Zoom ({n}%)",
  zoomTip:
    "How far the video is zoomed in from the starting point of the mode above. Steps of 5%.",
  zoomEndWhole: "0% · whole video",
  zoomEndSmall: "5% · small",
  zoomEndFull: "100% · fills frame",
  fitWholeCropNote:
    "The video is enlarged from its fitting size, so its sides are cropped. That is what this slider is for.",
  zoomFullNote:
    "At 100% the picture fills the whole frame, so the background is not used.",
  background: "Background",
  backgroundTip: "Used when the video does not cover the whole frame",
  backgroundBlur: "Blurred from the video",
  backgroundBlack: "Solid black",

  groupClips: "Clip output",
  clipDuration: "Clip length",
  clipDurationTip: "How long each clip is",
  durationAuto: "auto (45s – 2 min)",
  durationAbout: "± {n}",
  maxClips: "Maximum count",
  maxClipsTip: "Upper bound on clips produced from one video",
  saveClips: "Save clips as",
  saveClipsTip:
    "A clean clip is useful if you want to redo the subtitles in another editor",
  saveBurn: "With subtitles (burned in)",
  saveClean: "Clip only — no subtitles",
  saveBoth: "Both (2 files per clip)",

  // --- AI engine panel ---
  aiEngine: "AI engine — selects and scores moments (mode: {mode})",
  apiKeyClaude: "Claude API key",
  keyStored: "✓ stored",
  keyPlaceholderStored: "•••• (type to replace)",
  claudeModel: "Claude model (stronger = better and more expensive)",
  claudeHaiku: "Haiku 4.5 — cheap and fast",
  claudeSonnet: "Sonnet 5 — balanced",
  claudeOpus: "Opus 4.8 — best",
  noKeyWarning:
    "No API key yet — hybrid mode will fail. Enter a key, or switch to offline mode.",
  offlineEngine: "Offline score engine",
  offlineOllama: "Local AI (Ollama) — smarter",
  offlineHeuristic: "Heuristic — no AI, fast",
  engineNoFallback:
    "Your chosen engine is used as-is — if it fails, the job stops with the reason (it is never silently swapped for another engine).",
  transcriptFix: "Fix the transcript with AI",
  transcriptFixTip:
    "Speech recognition leaves dialogue dashes, misplaced punctuation and misheard words behind",
  transcriptFixNote:
    "An LLM repairs punctuation, sentence structure and obviously misheard words before the clips are cut — so it improves the subtitles AND where each clip starts and ends. Corrections that rewrite rather than repair are rejected automatically and reported.",
  transcriptFixNeedsLLM:
    "Needs an LLM even with the heuristic engine: Claude in hybrid mode, Ollama otherwise. If it is unreachable the job stops — untick this to use the raw transcript.",
  terms: "Names and terms in this video",
  termsPlaceholder: "Londo Ireng, Mahfud MD, URI",
  termsNote:
    "Speech recognition does not know regional words, names or new acronyms, so it writes down the nearest word it does know — \"Londo Ireng\" comes out as \"Londo Irang\". List the correct spellings here, separated by commas, and the correction step puts them back. Only words that sound like one of these are touched.",
  localModel: "Local model (run through Ollama)",
  modelNeedsDownload: "needs download",
  modelReady: "✓ ready",
  modelNotCapable: "⚠ not capable enough",
  checkingOllama: "Checking Ollama…",
  ollamaNotDetected:
    "⚠ Ollama not detected. Install it from ollama.com, then run",
  recheck: "check again",
  ollamaModelMissing: "Ollama is running, but {model} is not installed.",
  downloadModel: "⬇ download model",
  ollamaReady: "✓ Ollama is running, model {model} is ready",
  ollamaModelWeak:
    "⚠ {model} is installed but {note}. The job will stop if the model fails to select moments — use another model or",
  downloadSelected: "⬇ download the selected model",
  noModelsInstalled: " — no models are installed at all.",

  // --- subtitle settings ---
  clipAppearance:
    "How the clip looks — drag the subtitle on the preview to position it",
  groupVideoInFrame: "Video inside the 9:16 frame",
  emptyFrame: "Empty frame",
  loadPreview: "👁 Load a preview frame",
  loadingPreview: "Loading preview…",
  previewHintVideo:
    "Load one frame from your video to see the subtitle over the real image.",
  previewHintNoVideo:
    "Pick a video above first. Meanwhile you can already position the subtitle in this frame.",
  reloadPreview: "🔄 reload preview",
  resetPreview: "✕ reset preview",
  guidesAlways: "keep centre guides",
  previewTime: "Preview frame time: {t}s",
  previewNote:
    "The preview uses the zoom you picked ({zoom}%), so the subtitle position here matches the render. The font is still approximate — the real font is used at render time.",
  zoneTop: "covered by top UI",
  zoneBottom: "caption & account name",
  zoneRight: "action buttons",

  font: "Font",
  fontOther: "✏️ Other font — type it in…",
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
  effect: "Effect",
  boxBackground: "Box background",
  size: "Size ({n})",
  outline: "Outline ({n})",
  subStyle: "Subtitle style",
  subStyleTip: "How words appear on screen",
  subNormal: "Normal — whole sentences",
  subKaraoke: "Per-word highlight — active word lit up",
  subWord: "One word per screen — viral style",
  highlightColor: "Highlight colour",
  subSpeed: "Subtitle pacing",
  subSpeedTip: "Slower = less text at once, held longer",
  speedSlow: "Slow — easiest to read",
  speedNormal: "Normal",
  speedDense: "Dense — more text",
  position: "Position",
  resetCentre: "reset to centre",
  platformGuide: "Social media guides",
  platformGuideTip: "Areas covered by the app's buttons and caption",
  platformNone: "No guides",
  platformGeneric: "Generic (safest)",
  placeSafe: "⤓ Place in the safe area",
  unsafeWarning: "⚠ The subtitle sits in the area covered by {platform}'s UI",
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
  cancel: "⏹ Cancel",
  statusDone: "✅ Done",
  statusStopped: "⛔ Stopped",
  log: "Log",
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
  logReconnect: "🔄 Reconnecting to a running job: {id}",
  logKeySaved: "✅ API key saved",
  logKeyEmpty: "⚠ API key is empty",
  logKeyFailed: "⚠ Could not save the API key",
  logPullStart: "⬇ Downloading model {model} via Ollama (may take minutes)…",
  logPullDone: "✅ Model {model} is ready",
  logPullFailed: "⚠ Model download failed: {error}",
  logLocating: "🔎 Looking for {name} on this computer…",
  logLocated: "✅ Found on this computer: {path} — used in place, nothing copied",
  logLocateMiss: "{name} was not found on this computer — uploading a copy instead",
  logUploadStart: "⬆ Uploading {name} ({size} MB)…",
  logUploadDone: "✅ Upload finished: {name}",
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
  logJobCreated: "📋 Job {id}",
  logCancelling: "⏹ Cancelling…",
  logPreviewFailed: "⚠ Preview failed: {error}",
  logClip: "🎬 {id} (score {score})",
  logFinished: "✅ Finished",
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
  copyLink: "🔗 copy",
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
  previewCard: "👁 Preview",
  previewResult: "Preview",
  previewNotSaved:
    "Preview only — nothing has been saved yet. Press “Create card” to keep this one.",
  buildCard: "Create card",
  rendering: "Rendering…",
  result: "Result",
  downloadZip: "⬇ Download ZIP (image + caption + source)",
  downloadImage: "⬇ Download the image only",
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
  brandTagline: "Video panjang → klip 9:16 HD bersubtitle + skor viral",
  tabClips: "Klip video",
  tabNews: "Kartu berita",
  tabRequirements: "Kebutuhan",
  save: "Simpan",
  download: "unduh",
  downloading: "mengunduh…",
  loading: "Memuat…",
  engineUnreachable: "Tidak bisa menghubungi engine di {url}. Kalau aplikasi dijalankan dari terminal, lihat jendela itu — pesannya ada di sana.",

  dropTitle: "Seret & lepas video ke sini",
  dropOr: "atau",
  dropPick: "pilih file",
  dropOrPaste: "· atau tempel path manual",
  uploadingPct: "Mengunggah… {pct}%",
  browseButton: "📁 Pilih dari komputer ini",
  dropNoCopy: "Berkas yang memang sudah ada di komputer ini dipakai di tempatnya — tidak ada yang disalin.",
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
  videoPath: "Path video",
  outputDir: "Folder output (opsional — kosong = data/<job>)",
  outputDirPlaceholder: "/home/user/hasil-klip",

  renderSettings: "Setelan render",
  groupEngine: "Mesin pemroses",
  mode: "Mode",
  modeOffline: "offline (gratis)",
  modeHybrid: "hybrid (Claude API)",
  whisperModel: "Model Whisper",
  modelNotDownloaded: "belum diunduh",

  groupQuality: "Kualitas video",
  resolution: "Resolusi",
  quality: "Kualitas",
  qualityDraft: "draft (cepat)",
  qualityHd: "HD (seimbang)",
  qualityMax: "max (terbaik, lambat)",
  fps: "FPS",
  fpsTip: "Kehalusan gerak, bukan ketajaman",
  fpsSource: "Ikut sumber",

  fitMode: "Cara video dipasang",
  fitModeTip: "Cara video dimasukkan ke bingkai tegak 9:16",
  fitCenter: "Center of the Picture — potong tengah",
  fitWhole: "Whole Picture — mulai dari video utuh",
  fitFace: "Follow Face — belum tersedia",
  fitWholeNote:
    "Seluruh resolusi video dipakai, jadi tidak ada yang terpotong sama sekali. Itulah alasan latar ada: ia mengisi ruang yang tidak terjangkau videonya.",
  zoomLabel: "Zoom ({n}%)",
  zoomTip:
    "Seberapa jauh video di-zoom dari titik awal mode di atas. Kelipatan 5%.",
  zoomEndWhole: "0% · video utuh",
  zoomEndSmall: "5% · kecil",
  zoomEndFull: "100% · isi penuh",
  fitWholeCropNote:
    "Video diperbesar dari ukuran pas-nya, jadi sisinya terpotong. Memang itu gunanya penggeser ini.",
  zoomFullNote:
    "Pada 100% gambar memenuhi seluruh bingkai, jadi latar tidak dipakai.",
  background: "Latar",
  backgroundTip: "Dipakai saat video tidak menutupi seluruh bingkai",
  backgroundBlur: "Blur dari videonya",
  backgroundBlack: "Hitam polos",

  groupClips: "Hasil klip",
  clipDuration: "Durasi klip",
  clipDurationTip: "Panjang tiap klip",
  durationAuto: "auto (45 dtk – 2 mnt)",
  durationAbout: "± {n}",
  maxClips: "Jumlah maksimum",
  maxClipsTip: "Batas atas jumlah klip dari 1 video",
  saveClips: "Simpan klip",
  saveClipsTip:
    "Klip polos berguna bila subtitle mau diatur ulang di editor lain",
  saveBurn: "Dengan subtitle (dibakar)",
  saveClean: "Klip saja — tanpa subtitle",
  saveBoth: "Keduanya (2 berkas per klip)",

  aiEngine: "Mesin AI — memilih & menilai momen (mode: {mode})",
  apiKeyClaude: "API Key Claude",
  keyStored: "✓ tersimpan",
  keyPlaceholderStored: "•••• (isi untuk mengganti)",
  claudeModel: "Model Claude (makin kuat = makin bagus & mahal)",
  claudeHaiku: "Haiku 4.5 — murah & cepat",
  claudeSonnet: "Sonnet 5 — seimbang",
  claudeOpus: "Opus 4.8 — terbaik",
  noKeyWarning:
    "Belum ada API key — mode hybrid akan gagal. Masukkan key, atau pindah ke mode offline.",
  offlineEngine: "Mesin skor offline",
  offlineOllama: "AI lokal (Ollama) — lebih pintar",
  offlineHeuristic: "Heuristik — tanpa AI, cepat",
  engineNoFallback:
    "Mesin pilihan Anda dipakai apa adanya — bila gagal, job berhenti dengan pesan sebabnya (tidak diam-diam diganti mesin lain).",
  transcriptFix: "Perbaiki transkrip dengan AI",
  transcriptFixTip:
    "Pengenalan suara menyisakan tanda hubung dialog, tanda baca salah tempat, dan kata salah dengar",
  transcriptFixNote:
    "LLM membenahi tanda baca, struktur kalimat, dan kata yang jelas salah dengar sebelum klip dipotong — jadi ia memperbaiki subtitle SEKALIGUS titik awal & akhir tiap klip. Koreksi yang sifatnya menulis ulang ditolak otomatis dan dilaporkan.",
  transcriptFixNeedsLLM:
    "Butuh LLM walau mesin skornya heuristik: Claude di mode hybrid, selain itu Ollama. Bila tak terjangkau job berhenti — hilangkan centang ini untuk memakai transkrip mentah.",
  terms: "Nama & istilah di video ini",
  termsPlaceholder: "Londo Ireng, Mahfud MD, URI",
  termsNote:
    "Pengenalan suara tidak mengenal kata daerah, nama orang, atau akronim baru, jadi ia menuliskan kata terdekat yang ia tahu — \"Londo Ireng\" keluar jadi \"Londo Irang\". Tulis ejaan yang benar di sini, dipisah koma, dan tahap koreksi akan mengembalikannya. Hanya kata yang bunyinya mirip yang disentuh.",
  localModel: "Model lokal (dijalankan via Ollama)",
  modelNeedsDownload: "perlu unduh",
  modelReady: "✓ siap",
  modelNotCapable: "⚠ kurang memadai",
  checkingOllama: "Mengecek Ollama…",
  ollamaNotDetected:
    "⚠ Ollama tak terdeteksi. Pasang dari ollama.com lalu jalankan",
  recheck: "cek ulang",
  ollamaModelMissing: "Ollama jalan, tapi {model} belum ada.",
  downloadModel: "⬇ unduh model",
  ollamaReady: "✓ Ollama jalan, model {model} siap",
  ollamaModelWeak:
    "⚠ {model} terpasang tapi {note}. Job akan berhenti bila model gagal memilih momen — pakai model lain atau",
  downloadSelected: "⬇ unduh model terpilih",
  noModelsInstalled: " — belum ada model terpasang sama sekali.",

  clipAppearance:
    "Tampilan klip — geser subtitle di preview untuk atur posisi",
  groupVideoInFrame: "Video di dalam bingkai 9:16",
  emptyFrame: "Bingkai kosong",
  loadPreview: "👁 Muat preview frame",
  loadingPreview: "Memuat preview…",
  previewHintVideo:
    "Muat satu frame dari videomu untuk melihat subtitle di atas gambar asli.",
  previewHintNoVideo:
    "Pilih video dulu di atas. Sementara itu posisi subtitle sudah bisa diatur di bingkai ini.",
  reloadPreview: "🔄 muat ulang preview",
  resetPreview: "✕ reset preview",
  guidesAlways: "garis tengah terus",
  previewTime: "Waktu frame preview: {t}s",
  previewNote:
    "Preview memakai zoom yang sedang dipilih ({zoom}%), jadi posisi subtitle di sini sama dengan hasil render. Font masih kira-kira — font asli dipakai saat render.",
  zoneTop: "ketutup UI atas",
  zoneBottom: "caption & nama akun",
  zoneRight: "tombol aksi",

  font: "Font",
  fontOther: "✏️ Font lain — ketik manual…",
  fontPlaceholder: "mis. Poppins",
  fontChecking: "Memeriksa font…",
  fontHint: "Ketik nama family font — huruf/angka, spasi, titik, ' & atau -",
  fontFound: "✓ {family} ditemukan ({source})",
  color: "Warna",
  colorWhite: "Putih",
  colorYellow: "Kuning",
  colorGreen: "Hijau",
  colorCyan: "Biru muda",
  effect: "Efek",
  boxBackground: "Latar kotak",
  size: "Ukuran ({n})",
  outline: "Garis tepi ({n})",
  subStyle: "Gaya subtitle",
  subStyleTip: "Cara kata ditampilkan di layar",
  subNormal: "Normal — kalimat utuh",
  subKaraoke: "Highlight per-kata — kata aktif disorot",
  subWord: "Satu kata per layar — gaya viral",
  highlightColor: "Warna sorot",
  subSpeed: "Kecepatan subtitle",
  subSpeedTip: "Makin lambat = makin sedikit teks sekaligus & tampil lebih lama",
  speedSlow: "Lambat — paling mudah dibaca",
  speedNormal: "Normal",
  speedDense: "Padat — teks lebih banyak",
  position: "Posisi",
  resetCentre: "reset tengah",
  platformGuide: "Pembatas sosmed",
  platformGuideTip: "Area yang tertutup tombol & caption aplikasi",
  platformNone: "Tanpa pembatas",
  platformGeneric: "Umum (paling aman)",
  placeSafe: "⤓ Taruh di area aman",
  unsafeWarning: "⚠ Subtitle masuk area yang tertutup UI {platform}",
  sampleWord: "Contoh",
  sampleLine1: "Beginilah tampilan",
  sampleLine2: "subtitle Anda nanti",
  sampleLine3: "pada kecepatan ini",

  modelMissingWarn: "Model {model} belum diunduh. Jalankan",
  modelMissingWarnTail: "lalu muat ulang.",
  start: "Mulai proses",
  processing: "Memproses…",
  cancel: "⏹ Batalkan",
  statusDone: "✅ Selesai",
  statusStopped: "⛔ Berhenti",
  log: "Log",
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

  logReconnect: "🔄 Menyambung ke job berjalan: {id}",
  logKeySaved: "✅ API key tersimpan",
  logKeyEmpty: "⚠ API key kosong",
  logKeyFailed: "⚠ Gagal menyimpan API key",
  logPullStart: "⬇ Mengunduh model {model} via Ollama (bisa beberapa menit)…",
  logPullDone: "✅ Model {model} siap",
  logPullFailed: "⚠ Unduh model gagal: {error}",
  logLocating: "🔎 Mencari {name} di komputer ini…",
  logLocated: "✅ Ketemu di komputer ini: {path} — dipakai di tempat, tanpa salinan",
  logLocateMiss: "{name} tidak ditemukan di komputer ini — diunggah sebagai salinan",
  logUploadStart: "⬆ Unggah {name} ({size} MB)…",
  logUploadDone: "✅ Unggah selesai: {name}",
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
  logJobCreated: "📋 Job {id}",
  logCancelling: "⏹ Membatalkan…",
  logPreviewFailed: "⚠ Preview gagal: {error}",
  logClip: "🎬 {id} (skor {score})",
  logFinished: "✅ Selesai",
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
  copyLink: "🔗 salin",
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
  previewCard: "👁 Pratinjau",
  previewResult: "Pratinjau",
  previewNotSaved:
    "Baru pratinjau — belum ada yang disimpan. Tekan “Buat kartu” untuk menyimpannya.",
  buildCard: "Buat kartu",
  rendering: "Merender…",
  result: "Hasil",
  downloadZip: "⬇ Unduh ZIP (gambar + caption + sumber)",
  downloadImage: "⬇ Unduh gambar saja",
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

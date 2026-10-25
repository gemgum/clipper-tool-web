"use client";

// Halaman klip. Isinya tinggal KERANGKA dan KEADAAN: dua kolom, tujuh panel
// bernama, dan state yang dipakai lebih dari satu panel (job, preset tersimpan,
// dan bahan permintaan /api/jobs).
//
// Dipecah begini bukan karena rapi lebih enak dilihat. Berkas ini pernah 1.300
// baris dengan belasan tingkat bersarang, dan memindahkan satu blok di dalamnya
// GAGAL TIGA KALI berturut-turut: panel mendarat tersarang di dalam panel lain,
// TypeScript tetap lolos (ia memeriksa tag seimbang, bukan tag berada di induk
// yang benar), dan salahnya baru terlihat di browser. Dengan panel jadi satu tag
// masing-masing, salah tempat langsung kelihatan di berkas sepanjang 20 baris
// (notes/29).
//
// Ikon: lucide-react (ISC). Emoji dibuang bukan karena selera — gunting dan
// folder tampil sebagai KOTAK KOSONG di jendela Tauri Linux yang tidak punya
// font emoji. Karena itu berkas ini juga tidak memuat satu emoji pun, termasuk
// di komentarnya: greplah, dan hasil kosong berarti aturannya masih dipegang.
//
// Lambang yang TIDAK diganti: ⚠ ✓ ✕ → ↗ ↓ ↑ ✗. Semuanya simbol teks biasa yang
// ada di font mana pun.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Eye, Grid3x3, Move, Play, Smartphone } from "lucide-react";
import Stage, { StageBar } from "./stage";
import PageHeader from "./page-header";
import { useAI, useAISettings } from "./ai";
import { useI18n } from "./i18n";
import Alerts from "./alerts";
import { eng, engineURL, isWeb, useWeb, post } from "./engine";
import Picker from "./picker";
import { recall, useKeep, useRestore, useSticky } from "./persist";

import SourceCard from "./source-card";
import MoreControls, { zoomBounds } from "./more-controls";
import PreviewPanel, { PLATFORMS, linesFor } from "./preview-panel";
import { CENTER_X, CENTER_Y, PLAY_H, PLAY_W } from "./drag";
import type { Font, FontCheck } from "./preview-panel";
import { watermarkToAPI } from "./watermark-model";
import { HL_FONT, MarkFont, useMark } from "./mark";
import { NO_FX } from "./caption-presets";
import type { SubFx } from "./caption-presets";
import type { WhisperModel } from "./more-controls";
import RunView from "./run-view";
import ResultView, { downloadAllURL } from "./result-view";
import type { JobClip } from "./result-view";
import { baseName } from "./file-list";

export default function Home() {
  const { lang, t } = useI18n();

  const [path, setPath] = useState("");
  // Detik pratinjau: dipilih penggeser di Preview atau filmstrip di kartu Source.
  const [previewTime, setPreviewTime] = useState(5);
  const [outputDir, setOutputDir] = useState("");
  const [model, setModel] = useState("base");
  const [resolution, setResolution] = useState("1080p");
  const [quality, setQuality] = useState("hd");
  const [reframe, setReframe] = useState("center");
  const [background, setBackground] = useState("blur");
  const [zoom, setZoom] = useState(zoomBounds("center").natural);
  const [fps, setFps] = useState(0);

  // Mesin AI (scoring). Yang di sini hanya yang ikut ke /api/jobs dan ke preset
  // tersimpan — kunci API dan status Ollama dipegang <AiEnginePanel> sendiri.
  // Mesin skor: id dari daftar mesin bersama, atau "heuristic" (tanpa LLM).
  const [engine, setEngine] = useState("ollama");
  const [llmModel, setLlmModel] = useState("");
  const [transcriptFix, setTranscriptFix] = useState(true);
  const [terms, setTerms] = useState("");
  const [durationPreset, setDurationPreset] = useState("auto");
  // Nilai bawaan DESIGN.md §13. Isian tersimpan (useRestore) tetap menang.
  const [maxClips, setMaxClips] = useState(15);
  const [models, setModels] = useState<WhisperModel[]>([]);

  // Subtitle
  const [fonts, setFonts] = useState<Font[]>([]);
  const [subFont, setSubFont] = useState("Montserrat");
  const [subSize, setSubSize] = useState(94);
  const [subColor, setSubColor] = useState("white");
  const [subX, setSubX] = useState(540);
  const [subY, setSubY] = useState(960);
  const [subOutline, setSubOutline] = useState(5);
  const [subBox, setSubBox] = useState(false);
  const [subUpper, setSubUpper] = useState(false);       // huruf kapital (template caption)
  const [subFx, setSubFx] = useState<SubFx>(NO_FX);       // gerak & bayangan preset caption
  const [subMode, setSubMode] = useState("highlight");    // normal | highlight | karaoke | word | build
  const [subHighlight, setSubHighlight] = useState("#FFD400");
  const [subSpeed, setSubSpeed] = useState("normal");     // slow | normal | dense
  const [platform, setPlatform] = useState("tiktok");     // "off" = tanpa pembatas
  const [saveMode, setSaveMode] = useState("both");       // burn | clean | both

  // Watermark = Mark yang SAMA dengan halaman Mark & News (useMark): banyak
  // logo, kotak judul/kartu, seret + kisi, kunci logo. llm = judul boleh dari
  // judul LLM tiap klip. Hook-nya dipanggil di bawah, setelah path & kisi ada.
  const [timed, setTimed] = useState(false);
  const [logoTarget, setLogoTarget] = useState(-1);

  // Font manual (di luar daftar bawaan) + hasil pengecekannya di engine.
  // Tinggal di sini, bukan di <PreviewPanel>: start() menolak job yang fontnya
  // belum lolos cek, jadi halaman ini harus tahu hasilnya.
  const fontFromPreset = useRef<string>("");
  const [fontManual, setFontManual] = useState(false);
  const [fontCheck, setFontCheck] = useState<FontCheck | null>(null);
  const [fontChecking, setFontChecking] = useState(false);

  // Upload
  const [uploading, setUploading] = useState(false);
  const [uploadPct, setUploadPct] = useState(0);
  // Pemilih berkas milik sendiri: "video" untuk sumber, "out" untuk folder
  // keluaran, null = tertutup.
  const [picker, setPicker] = useState<null | "video" | "out" | "banner">(null);

  // Job
  const [jobId, setJobId] = useState<string | null>(null);
  const [stage, setStage] = useState("");
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);

  // Tiga layar, satu alur (DESIGN.md §3): Atur klip → Sedang diproses → Hasil.
  const [screen, setScreen] = useState<"setup" | "run" | "result">("setup");
  const [jobClips, setJobClips] = useState<JobClip[]>([]);
  const [startedAt, setStartedAt] = useState(() => Date.now());
  const [failedStage, setFailedStage] = useState(-1);
  const [retrying, setRetrying] = useState(false);
  const [rerendering, setRerendering] = useState(false);
  // Dinaikkan untuk berlangganan SSE lagi pada job yang SAMA (render ulang).
  const [subKey, setSubKey] = useState(0);
  const stageRef = useRef("");
  stageRef.current = stage;
  // Kisi, garis bantu, dan area tertutup UI = setelan lihat, diingat browser.
  const [grid, setGrid] = useSticky<number>("clips.grid", 20);
  const [alwaysGuides, setAlwaysGuides] = useSticky("clips.gridAlways", false);
  const [showZones, setShowZones] = useSticky("clips.zones", true);
  const m = useMark({ format: "short", first: path, timed, grid, llm: true });
  const router = useRouter();
  // Mesin AI dari Pengaturan (global, atau pengecualian alat "clips"). Ia
  // MENANG atas isian lama yang tersimpan: pemilihnya sudah tidak ada di sini.
  const ai = useAI("clips");
  // Transkripsi juga dari Pengaturan (baris Video clips), tidak dipilih di
  // sini lagi: job tidak mengirim "transcriber", engine memakai setelan itu.
  const transcriber = useAISettings().data?.transcriber?.engine ?? "whisper";
  useEffect(() => {
    if (!ai) return;
    setEngine(ai.engine);
    setLlmModel(ai.model);
  }, [ai]);
  const web = useWeb();

  const locale = lang === "id" ? "id-ID" : "en-GB";
  const addLog = useCallback((text: string) => {
    setLogs((prev) => [...prev, `[${new Date().toLocaleTimeString(locale)}] ${text}`]);
  }, [locale]);

  // Daftar model ditarik ulang setiap jendela kembali fokus.
  //
  // Model diunduh di HALAMAN LAIN (Requirements), dan tanpa penarikan ulang
  // halaman ini terus mengira modelnya belum ada — peringatannya bertahan dan
  // tombol Mulai tetap mati sampai aplikasi ditutup. Itu yang membuat "harus
  // direstart dulu".
  useEffect(() => {
    const loadModels = () =>
      fetch(eng(`/api/models`)).then((r) => r.json()).then((m: WhisperModel[]) => {
        setModels(m);
        setModel((cur) => {
          // Pilihan pengguna dipertahankan selama modelnya benar-benar ada;
          // kalau tidak, ambil yang sudah terunduh.
          if (m.some((x) => x.name === cur && x.downloaded)) return cur;
          return m.find((x) => x.downloaded)?.name ?? cur;
        });
      }).catch(() => {});
    window.addEventListener("focus", loadModels);
    return () => window.removeEventListener("focus", loadModels);
  }, []);

  useEffect(() => {
    fetch(eng(`/api/models`)).then((r) => r.json()).then((m: WhisperModel[]) => {
      setModels(m);
      const downloaded = m.find((x) => x.downloaded);
      if (downloaded) setModel(downloaded.name);
    }).catch(() => addLog(`⚠ ${t("engineUnreachable", { url: engineURL() })}`));
    fetch(eng(`/api/fonts`)).then((r) => r.json()).then((f: Font[]) => {
      setFonts(f);
      // Jangan menimpa font dari preset — daftar font datang belakangan, dulu
      // pilihan tersimpan selalu tergantikan font pertama tiap halaman dimuat.
      if (f[0] && !fontFromPreset.current) setSubFont(f[0].name);
      // Font tersimpan yang bukan bawaan berarti font manual.
      else if (fontFromPreset.current && !f.some((x) => x.name === fontFromPreset.current)) setFontManual(true);
    }).catch(() => {});
  }, [addLog, t]);

  // Font manual divalidasi di engine (format + benar-benar terpasang), ditunda
  // 400 ms supaya tidak memanggil fc-match tiap huruf yang diketik.
  useEffect(() => {
    if (!fontManual) { setFontCheck(null); return; }
    const name = subFont.trim();
    if (!name) { setFontCheck(null); return; }
    setFontChecking(true);
    const timer = setTimeout(() => {
      fetch(eng(`/api/font-check?name=${encodeURIComponent(name)}`))
        .then((r) => r.json()).then(setFontCheck)
        .catch(() => setFontCheck({ valid: false, name, family: "", source: "", error: "engine unreachable", scale: 0 }))
        .finally(() => setFontChecking(false));
    }, 400);
    return () => { clearTimeout(timer); setFontChecking(false); };
  }, [fontManual, subFont]);

  // Muat preset tersimpan (localStorage) sekali di awal.
  useEffect(() => {
    try {
      const s = JSON.parse(localStorage.getItem("clipper.preset") || "{}");
      if (s.resolution) setResolution(s.resolution);
      if (s.quality) setQuality(s.quality);
      // Preset lama menyimpan reframe:"fit" — pilihan yang kini sudah tidak ada
      // di dropdown. Dibiarkan, select-nya tampil kosong DAN engine
      // menerjemahkannya jadi zoom 0, menimpa zoom tersimpan tanpa pemberitahuan.
      // Jadi dimigrasikan di sini, sama seperti yang dilakukan engine.
      if (s.reframe) setReframe(s.reframe);
      // Arti zoom sempat berubah dua kali di tengah pengembangan. Preset yang
      // menyimpan penandanya dibuang saja ke default — menebak-nebak arti angka
      // lama lebih berisiko daripada mulai dari setelan bawaan.
      if (typeof s.zoom === "number" && !s.zoomAxis) setZoom(s.zoom);
      if (s.background) setBackground(s.background);
      if (typeof s.fps === "number") setFps(s.fps);
      if (s.engine) setEngine(s.engine);
      if (s.llmModel) setLlmModel(s.llmModel);
      if (typeof s.transcriptFix === "boolean") setTranscriptFix(s.transcriptFix);
      if (typeof s.terms === "string") setTerms(s.terms);
      if (s.durationPreset) setDurationPreset(s.durationPreset);
      if (s.maxClips) setMaxClips(s.maxClips);
      if (s.subFont) { setSubFont(s.subFont); fontFromPreset.current = s.subFont; }
      if (s.subSize) setSubSize(s.subSize);
      if (s.subColor) setSubColor(s.subColor);
      if (typeof s.subOutline === "number") setSubOutline(s.subOutline);
      if (typeof s.subBox === "boolean") setSubBox(s.subBox);
      if (typeof s.subUpper === "boolean") setSubUpper(s.subUpper);
      if (s.subFx) setSubFx({ ...NO_FX, ...s.subFx });
      if (s.subMode) setSubMode(s.subMode);
      else if (typeof s.subKaraoke === "boolean" && s.subKaraoke) setSubMode("karaoke"); // preset lama
      if (s.subHighlight) setSubHighlight(s.subHighlight);
      if (s.subSpeed) setSubSpeed(s.subSpeed);
      if (s.platform) setPlatform(s.platform);
      if (s.saveMode) setSaveMode(s.saveMode);
    } catch {}
  }, []);

  // Simpan preset setiap kali setelan berubah.
  useEffect(() => {
    const preset = { resolution, quality, reframe, background, zoom, fps, engine, llmModel, transcriptFix, terms, durationPreset, maxClips, subFont, subSize, subColor, subOutline, subBox, subUpper, subFx, subMode, subHighlight, subSpeed, platform, saveMode };
    try { localStorage.setItem("clipper.preset", JSON.stringify(preset)); } catch {}
  }, [resolution, quality, reframe, background, zoom, fps, engine, llmModel, transcriptFix, terms, durationPreset, maxClips, subFont, subSize, subColor, subOutline, subBox, subUpper, subFx, subMode, subHighlight, subSpeed, platform, saveMode]);

  // Pulihkan log job TERAKHIR, lalu sambung ulang bila ia masih berjalan.
  //
  // Dua-duanya di sini, dan urutannya penting. Kotak log dulu cuma state React:
  // berpindah tab melepas komponen halaman ini dan seluruh isinya hilang, tanpa
  // satu pun tempat lain yang menyimpannya. Sekarang engine menulisnya ke
  // <DataDir>/jobs/<id>.log dan halaman tinggal membacanya kembali — jadi yang
  // dipulihkan bukan cuma job yang masih hidup, melainkan juga yang sudah
  // selesai atau gagal. Justru yang gagal yang paling ingin dibaca ulang.
  //
  // Log diisi lewat setLogs, bukan addLog: barisnya sudah bercap waktu dari
  // engine, dan menambahkan cap kedua di depannya cuma membuatnya tak terbaca.
  useEffect(() => {
    fetch(eng(`/api/jobs`)).then((r) => r.json()).then(async (jobs: any[]) => {
      if (!Array.isArray(jobs) || !jobs.length) return;
      // Job yang masih hidup didahulukan atas yang sekadar paling baru: pada
      // antrean berisi dua, yang terbaru justru yang belum mulai dan lognya
      // masih kosong, sedangkan yang sedang berjalan itulah yang ingin dilihat.
      const byNewest = [...jobs].sort((a, b) => (a.created_at < b.created_at ? 1 : -1));
      const newest = byNewest.find((j) => j.status === "running") || byNewest[0];
      try {
        const res = await fetch(eng(`/api/jobs/${newest.id}/log`));
        const data = await res.json();
        if (Array.isArray(data.lines) && data.lines.length) setLogs(data.lines);
      } catch {
        // Log yang tak terbaca tidak boleh menghalangi penyambungan ulang.
      }
      if (newest.status === "running" || newest.status === "queued") {
        setStage(newest.stage || "");
        setProgress(newest.progress || 0);
        setBusy(true);
        setJobId(newest.id); // memicu langganan SSE → progress lanjut terlihat
        setScreen("run");
        setJobClips(Array.isArray(newest.clips) ? newest.clips : []);
        if (newest.created_at) setStartedAt(Date.parse(newest.created_at));
        addLog(t("logReconnect", { id: newest.id }));
      } else {
        // Tidak ada yang berjalan: hasil yang terakhir dilihat dibuka lagi —
        // pindah halaman lalu kembali tidak boleh membuang layar hasil.
        const done = jobs.find((j) => j.id === recall<string>("clips.result") && Array.isArray(j.clips));
        if (done) { setJobClips(done.clips); setJobId(done.id); setScreen("result"); }
      }
    }).catch(() => {});
  }, [addLog, t]);

  const selectedModel = models.find((m) => m.name === model);
  const modelMissing = selectedModel && !selectedModel.downloaded;

  // Job yang hasilnya sedang dilihat diingat browser (dibuka lagi saat kembali
  // ke halaman ini); dilupakan hanya lewat "New batch".
  const [, setResultJob] = useSticky<string>("clips.result", "");
  useEffect(() => { if (screen === "result" && jobId) setResultJob(jobId); }, [screen, jobId, setResultJob]);

  // Isian tab klip ikut disimpan, dengan alasan yang sama seperti tab kartu:
  // halaman bisa termuat ulang tanpa diminta, dan menyusun ulang setelan render
  // dari awal jauh lebih menyakitkan daripada mengetik satu path.
  //
  // Yang TIDAK disimpan: daftar klip & status job — itu milik engine, dan
  // ditarik ulang saat halaman dibuka.
  useKeep("clips", {
    outputDir, model, resolution, quality, reframe, background, zoom, fps,
    durationPreset, maxClips, saveMode, transcriptFix, terms,
    engine, llmModel,
    subFont, subSize, subX, subY, subColor, subOutline, subBox, subUpper, subFx, subMode, subHighlight, subSpeed,
    timed, mark: { watermark: m.watermark, logos: m.logos, logoOn: m.logoOn },
  });
  useRestore<Record<string, unknown>>("clips", (v) => {
    const set = <T,>(fn: (x: T) => void, val: unknown) => {
      if (val !== undefined && val !== null) fn(val as T);
    };
    set(setOutputDir, v.outputDir);
    set(setModel, v.model);
    set(setResolution, v.resolution);
    set(setQuality, v.quality);
    set(setReframe, v.reframe);
    set(setBackground, v.background);
    set(setZoom, v.zoom);
    set(setFps, v.fps);
    set(setDurationPreset, v.durationPreset);
    set(setMaxClips, v.maxClips);
    set(setSaveMode, v.saveMode);
    set(setTranscriptFix, v.transcriptFix);
    set(setTerms, v.terms);
    set(setEngine, v.engine);
    set(setLlmModel, v.llmModel);
    set(setSubFont, v.subFont);
    set(setSubSize, v.subSize);
    set(setSubX, v.subX);
    set(setSubY, v.subY);
    set(setSubColor, v.subColor);
    set(setSubOutline, v.subOutline);
    set(setSubBox, v.subBox);
    set(setSubUpper, v.subUpper);
    if (v.subFx) setSubFx({ ...NO_FX, ...(v.subFx as SubFx) });
    set(setSubMode, v.subMode);
    set(setSubHighlight, v.subHighlight);
    set(setSubSpeed, v.subSpeed);
    set(setTimed, v.timed);
    // Mark tersimpan (logo + judul); simpanan lama = satu objek watermark
    // dengan satu gambar — applyMark mengubahnya jadi daftar logo.
    if (v.mark && typeof v.mark === "object") m.applyMark(v.mark as Record<string, unknown>);
    else if (v.watermark && typeof v.watermark === "object") m.applyMark({ watermark: v.watermark });
  });

  const uploadFile = useCallback((file: File) => {
    setUploading(true); setUploadPct(0);
    addLog(t("logUploadStart", { name: file.name, size: (file.size / 1e6).toFixed(0) }));
    const form = new FormData(); form.append("file", file);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", eng(`/api/upload`));
    xhr.upload.onprogress = (e) => { if (e.lengthComputable) setUploadPct(e.loaded / e.total); };
    xhr.onload = () => {
      setUploading(false);
      try {
        const data = JSON.parse(xhr.responseText);
        if (data.path) { setPath(data.path); addLog(t("logUploadDone", { name: data.name })); }
        else addLog(t("logUploadFailed", { error: data.error }));
      } catch { addLog(t("logUploadInvalid")); }
    };
    xhr.onerror = () => { setUploading(false); addLog(t("logUploadNetwork")); };
    xhr.send(form);
  }, [addLog, t]);

  // fetchUrl: CEK dulu apakah videonya bisa diambil (tanpa mengunduh). Tidak
  // bisa → peringatan di kartu Source. Bisa → langsung unduh ke folder unggahan
  // (yt-dlp, lalu Cobalt bila gagal); balasannya {path} seperti unggahan biasa.
  const [fetchPhase, setFetchPhase] = useState<"" | "checking" | "downloading">("");
  const [fetchTitle, setFetchTitle] = useState("");
  const [fetchWarn, setFetchWarn] = useState("");
  const fetchUrl = useCallback(async (url: string) => {
    setFetchWarn(""); setFetchTitle(""); setFetchPhase("checking");
    addLog(t("logFetchCheck", { url }));
    try {
      const info = await post(`/api/fetch/check`, { url });
      setFetchTitle(info.title || "");
      addLog(t("logFetchOk", { title: info.title, engine: info.engine }));
    } catch (e: any) {
      setFetchPhase(""); setFetchWarn(e.message);
      addLog(t("logFetchFailed", { error: e.message }));
      return;
    }
    setFetchPhase("downloading");
    try {
      const data = await post(`/api/fetch`, { url });
      setPath(data.path); addLog(t("logFetchDone", { name: data.name }));
    } catch (e: any) {
      setFetchWarn(e.message);
      addLog(t("logFetchFailed", { error: e.message }));
    } finally { setFetchPhase(""); }
  }, [addLog, t]);

  // Video aktif disimpan PER AKUN di server (/api/source), bukan di
  // localStorage: dengan akun yang sama, komputer mana pun membuka video dan
  // pratinjau yang sama. Dibaca sekali saat halaman dibuka; setelah itu tiap
  // pergantian (unggah, YouTube, Remove, job selesai) dikirim balik.
  const sourceReady = useRef(false);
  useEffect(() => {
    fetch(eng(`/api/source`)).then((r) => (r.ok ? r.json() : null))
      .then((d) => { if (d?.path) setPath(d.path); })
      .catch(() => {})
      .finally(() => { sourceReady.current = true; });
  }, []);
  useEffect(() => {
    if (!sourceReady.current) return;
    post(`/api/source`, { path }).catch(() => {});
  }, [path]);

  // removeSource: kosongkan sumber. Engine menghapus berkasnya di mode web bila
  // itu unggahan yang belum dipakai job; gagal pun sumber tetap dikosongkan.
  const removeSource = useCallback(() => {
    const p = path;
    setPath("");
    post(`/api/upload/remove`, { path: p }).catch(() => {});
  }, [path]);

  // useFile menerima berkas dari seret & lepas atau dari pemilih browser.
  //
  // Browser tidak memberi tahu di mana berkas itu berada — hanya nama dan
  // ukurannya. Padahal berkasnya ada di mesin yang sama dengan engine, jadi
  // engine ditanya dulu: kalau ia menemukannya, path itu langsung dipakai dan
  // tidak ada satu byte pun yang disalin. Unggahan hanya cadangan.
  const useFile = useCallback(async (file: File) => {
    // Mode web: berkasnya di komputer pengguna, bukan di server — langsung unggah.
    if (await isWeb()) { uploadFile(file); return; }
    addLog(t("logLocating", { name: file.name }));
    try {
      const data = await post(`/api/locate`, { name: file.name, size: file.size });
      if (data.path) {
        setPath(data.path);
        addLog(t("logLocated", { path: data.path }));
        return;
      }
    } catch {
      // Engine mati atau versinya lama: unggahan di bawah tetap jalan.
    }
    addLog(t("logLocateMiss", { name: file.name }));
    uploadFile(file);
  }, [addLog, t, uploadFile]);

  const start = useCallback(async () => {
    // Font manual yang belum lolos pengecekan ditolak di sini — kalau diteruskan,
    // libass diam-diam mengganti fontnya dan hasil render tidak sesuai preview.
    if (fontManual && !fontCheck?.valid) {
      addLog(t("logFontInvalid", { font: subFont }) + (fontCheck?.error ? ` (${fontCheck.error})` : ""));
      return;
    }
    setError(""); setProgress(0); setStage("");
    setBusy(true); setJobId(null);
    setJobClips([]); setFailedStage(-1); setStartedAt(Date.now()); setScreen("run");
    addLog(t("logJobStart", { resolution, quality, duration: durationPreset, font: subFont }));
    try {
      const data = await post(`/api/jobs`, {
          source: { type: "path", value: path },
          options: {
            whisper_model: model, resolution, quality, reframe, background,
            zoom: Number(zoom), fps: Number(fps),
            // Satu kotak model untuk semua mesin; pipeline membaca yang
            // sesuai dengan mesin yang dipilih.
            provider: engine,
            llm_model: llmModel, ollama_model: llmModel,
            transcript_fix: transcriptFix ? "on" : "off",
            terms: terms.split(/[,;\n]/).map((v) => v.trim()).filter(Boolean),
            duration_preset: durationPreset, max_clips: Number(maxClips), output_dir: outputDir,
            subtitle_output: saveMode,
            subtitle: {
              font: subFont, size: Number(subSize), x: subX, y: subY, color: subColor, bold: true,
              outline: Number(subOutline), box: subBox, uppercase: subUpper,
              mode: subMode, highlight_color: subHighlight, speed: subSpeed, ...subFx,
            },
            watermark: watermarkToAPI({ ...m.eff, format: "short" }, HL_FONT),
          },
      }, t("errCreateJob"));
      setJobId(data.id); addLog(t("logJobCreated", { id: data.id }));
    } catch (e: any) { setError(e.message); setBusy(false); addLog(`⚠ ${e.message}`); }
  }, [path, engine, llmModel, model, transcriber, resolution, quality, reframe, background, zoom, fps, engine, llmModel, transcriptFix, terms, durationPreset, maxClips, outputDir, subFont, subSize, subX, subY, subColor, subOutline, subBox, subUpper, subFx, subMode, subHighlight, subSpeed, saveMode, m.eff, addLog, fontManual, fontCheck, t]);

  const cancel = useCallback(async () => {
    if (!jobId) return;
    addLog(t("logCancelling"));
    // Tanpa badan, tapi jenisnya tetap disebut: engine menolak POST yang bukan
    // JSON, dan itulah yang memaksa halaman asing melewati preflight lebih dulu.
    await post(`/api/jobs/${jobId}/cancel`).catch(() => {});
  }, [jobId, addLog, t]);

  useEffect(() => {
    if (!jobId) return;
    const events = new EventSource(eng(`/api/jobs/${jobId}/events`));
    events.addEventListener("progress", (e: MessageEvent) => {
      const d = JSON.parse(e.data);
      if (d.stage) setStage(d.stage);
      if (typeof d.progress === "number") setProgress(d.progress);
      if (d.message) { addLog(`${d.stage || ""}: ${d.message}`); }
      // Ringkasan waktu per tahap, hanya ada di peristiwa terakhir. Dicatat
      // tanpa awalan tahap: isinya tabel monospace yang perataannya harus utuh
      // (.logbox sudah pre-wrap + monospace).
      if (d.summary) addLog(d.summary);
    });
    events.addEventListener("clip", (e: MessageEvent) => {
      const clip = JSON.parse(e.data) as JobClip;
      addLog(t("logClip", { id: clip.id, score: clip.score }));
      // Klip muncul di layar proses begitu jadi (DESIGN.md §5).
      setJobClips((prev) => {
        const i = prev.findIndex((c) => c.id === clip.id);
        if (i < 0) return [...prev, clip];
        const next = [...prev]; next[i] = clip; return next;
      });
    });
    events.addEventListener("done", () => {
      setProgress(1); setBusy(false); addLog(t("logFinished")); events.close();
      setRerendering(false);
      // Daftar akhir dari engine: urutan, status gagal, dan klip yang tidak
      // sempat terkirim lewat SSE.
      fetch(eng(`/api/jobs/${jobId}`)).then((r) => r.json())
        .then((j) => { if (Array.isArray(j.clips)) setJobClips(j.clips); }).catch(() => {});
      setScreen("result");
      // Mode web: engine sudah menghapus video unggahannya (notes/42).
      isWeb().then((w) => { if (w) { setPath(""); addLog(t("webSourceRemoved")); } });
    });
    events.addEventListener("error", (e: MessageEvent) => {
      let msg = "error";
      try { msg = JSON.parse((e as any).data).message || msg; } catch {}
      setError(msg); setBusy(false); addLog(`⚠ ${msg}`); events.close();
      setRerendering(false);
      // Dibatalkan → kembali ke layar 1 (DESIGN.md §9). Gagal → kartu tahap
      // tempat ia berhenti jadi merah, dengan tombol coba lagi.
      if (/canceled by the user/i.test(msg)) { setError(""); setScreen("setup"); return; }
      const of: Record<string, number> = { extracting: 0, transcribing: 0, correcting: 1, segmenting: 1, scoring: 1, rendering: 2 };
      setFailedStage(of[stageRef.current] ?? 0);
    });
    return () => events.close();
  }, [jobId, subKey, addLog, t]);

  // "Coba lagi dari tahap ini": job baru dengan setelan sama; tahap yang
  // sudah selesai diambil dari cache engine.
  const retry = useCallback(async () => {
    if (!jobId) return;
    setRetrying(true);
    try {
      const d = await post(`/api/jobs/${jobId}/retry`, {}, "retry failed");
      setError(""); setFailedStage(-1); setProgress(0); setStage(""); setBusy(true);
      setStartedAt(Date.now()); setJobId(d.id); addLog(t("logJobCreated", { id: d.id }));
    } catch (e: any) { setError(e.message); }
    finally { setRetrying(false); }
  }, [jobId, addLog, t]);

  // "Render ulang yang gagal": job yang SAMA, hanya klip berstatus failed.
  const rerender = useCallback(async () => {
    if (!jobId) return;
    setRerendering(true);
    try {
      await post(`/api/jobs/${jobId}/rerender`, {}, "rerender failed");
      setBusy(true); setProgress(0); setStage("rendering"); setFailedStage(-1);
      setStartedAt(Date.now()); setScreen("run"); setSubKey((k) => k + 1);
    } catch (e: any) { setError(e.message); setRerendering(false); }
  }, [jobId]);

  // Font yang di-@font-face untuk preview: bawaan + font manual yang lolos cek.
  const previewFonts = useMemo(() => {
    const names = fonts.map((f) => f.name);
    if (fontManual && fontCheck?.valid && !names.includes(fontCheck.family)) names.push(fontCheck.family);
    return names;
  }, [fonts, fontManual, fontCheck]);

  // Angka zoom artinya berbeda per mode, jadi membawanya menyeberang saat mode
  // berganti hanya menghasilkan nilai yang tak berarti. Disetel ke titik awal
  // mode barunya.
  const changeReframe = useCallback((next: string) => {
    setReframe(next);
    setZoom(zoomBounds(next).natural);
  }, []);

  // Geometri blok subtitle. Tinggal di halaman, bukan di <PreviewPanel>, sebab
  // More controls memakainya juga: peringatan "menabrak zona" dan tombol "taruh
  // di area aman" keduanya butuh tinggi blok yang sama.
  //
  // Jarak antarbaris libass PERSIS sebesar ukuran fontnya, berapa pun fontnya —
  // diukur: dua baris pada ukuran 72 berjarak 72 px. Faktor 1,17 yang dulu ada
  // di sini diukur saat font bawaan tidak pernah benar-benar dipakai libass
  // (notes/29), jadi ia mengukur font cadangan sistem, bukan Montserrat.
  const blockH = Math.round(subSize * linesFor(subMode, subSpeed, subFx.stack));
  // Jangkar ada di tepi atas blok, jadi supaya blok terlihat di tengah bidang
  // titiknya harus setengah tinggi blok di atas garis tengah.
  const centerAnchorY = Math.round(CENTER_Y - blockH / 2);

  // Zona yang tertutup UI aplikasi (undefined = pembatas dimatikan).
  const zone = platform !== "off" ? PLATFORMS[platform]?.zone : undefined;
  // Yang boleh menabrak zona adalah blok penuhnya. Kalau hanya titik jangkar
  // yang diperiksa, halaman 2-3 baris bisa menjulur ke zona bawah sementara
  // peringatannya diam saja.
  const inUnsafe = !!zone && (
    subY / PLAY_H < zone.top ||
    (subY + blockH) / PLAY_H > 1 - zone.bottom ||
    subX / PLAY_W > 1 - zone.right
  );
  // Taruh subtitle tepat di atas zona bawah — disisakan setinggi blok, karena
  // baris tambahan tumbuh ke bawah dari titik ini. X tetap di tengah bidang:
  // subtitle ditulis rata tengah terhadap titik ini, jadi menengahkannya ke
  // lebar sisa (di luar kolom tombol kanan) justru membuatnya miring ke kiri.
  const placeSafe = () => {
    setSubX(CENTER_X);
    setSubY(zone ? Math.round(PLAY_H * (1 - zone.bottom) - blockH - 40) : centerAnchorY);
  };
  // Operator tidak pernah mengetik koordinat (DESIGN.md §4.2): ganti platform,
  // ukuran, atau gaya → subtitle ditaruh lagi tepat di atas area caption.
  // Posisi manual (seret di pratinjau) berlaku sampai pilihan berikutnya.
  const placeRef = useRef(placeSafe);
  placeRef.current = placeSafe;
  useEffect(() => { placeRef.current(); }, [platform, subSize, subMode, subSpeed, subFx.stack]);

  // Ukuran huruf sebagai tiga pilihan; garis tepi mengikuti (≈ ukuran/18 —
  // setara text-stroke ukuran/9 di mockup, yang separuhnya jatuh di dalam huruf).
  const pickSize = (px: number) => { setSubSize(px); setSubOutline(Math.round(px / 18)); };

  const fileName = path ? baseName(path) : "";
  const platformName = PLATFORMS[platform]?.label || platform;
  const okClips = jobClips.filter((c) => c.status !== "failed");
  const summary = [
    { label: t("sumStyle"), value: t(subMode === "highlight" ? "styleHighlightName" : subMode === "karaoke" ? "styleKaraokeName" : subMode === "word" ? "subWord" : "styleNormalName") },
    { label: t("textSize"), value: subSize >= 120 ? t("sizeLarge") : subSize >= 94 ? t("sizeMedium") : subSize <= 72 ? t("sizeSmall") : `${subSize}` },
    { label: t("sumPlatform"), value: platformName },
    { label: t("clipCountLabel"), value: String(maxClips) },
    { label: t("clipLength"), value: durationPreset === "30" ? t("lenShort") : durationPreset === "60" ? t("lenMedium") : t("lenAuto") },
    { label: t("sumClean"), value: saveMode === "burn" ? t("no") : t("yes") },
  ];

  const canStart = !busy && !!path && !modelMissing;
  const preview = (
    <PreviewPanel
      alwaysGuides={alwaysGuides}
      showZones={showZones}
      path={path} reframe={reframe} background={background} zoom={zoom} zone={zone}
      fonts={fonts} subFont={subFont} setSubFont={setSubFont}
      fontManual={fontManual} setFontManual={setFontManual}
      fontCheck={fontCheck} fontChecking={fontChecking}
      subSize={subSize} setSubSize={setSubSize}
      subColor={subColor} setSubColor={setSubColor}
      subOutline={subOutline} setSubOutline={setSubOutline}
      subBox={subBox} setSubBox={setSubBox} subUpper={subUpper} subFx={subFx}
      subMode={subMode} setSubMode={setSubMode}
      subHighlight={subHighlight} setSubHighlight={setSubHighlight}
      subSpeed={subSpeed} setSubSpeed={setSubSpeed}
      subX={subX} setSubX={setSubX} subY={subY} setSubY={setSubY}
      blockH={blockH} centerAnchorY={centerAnchorY}
      m={m}
      addLog={addLog}
      previewTime={previewTime} setPreviewTime={setPreviewTime}
    />
  );

  // Aksi kepala halaman berganti per layar; judulnya tetap (DESIGN.md §3).
  const actions = screen === "setup" ? (
    <>
      <button type="button" className="primary big" onClick={start} disabled={!canStart}>
        <Play className="ico" aria-hidden="true" /> {t("makeClips", { n: maxClips })}
      </button>
    </>
  ) : screen === "run" ? (
    <>
      <button type="button" className="ghost" onClick={() => router.push("/history?f=clips")} disabled={failedStage >= 0}>{t("runInBackground")}</button>
      <button type="button" className="danger" onClick={cancel} disabled={!busy || !jobId}>{t("cancelRun")}</button>
    </>
  ) : (
    <>
      <button type="button" className="ghost" onClick={() => { setScreen("setup"); setJobClips([]); setResultJob(""); }}>{t("newBatch")}</button>
      {okClips.length > 0 && (
        <a className="btn-primary big" href={downloadAllURL(jobClips)} download>{t("downloadAll", { n: okClips.length })}</a>
      )}
    </>
  );
  const clipsLabel = (n: number) => (n === 1 ? t("nClipsOne") : t("nClipsLine", { n }));
  const headerSub = screen === "setup" ? t("subClips")
    : `${fileName} · ${clipsLabel(screen === "run" ? maxClips : okClips.length)}`;

  return (
    <div className="screen scroll clips-v2">
      <PageHeader title={t("tabClips")} subtitle={headerSub}>{actions}</PageHeader>
      {/* Font asli untuk pratinjau: subtitle (termasuk font manual yang lolos
          cek) + judul Mark — satu pemuat, sama dengan Mark & News. */}
      <MarkFont names={[...previewFonts, HL_FONT]} />

      {/* Galat & peringatan MELAYANG (alerts.tsx), tidak lagi disisipkan sebagai
          kepala halaman: kepala yang muncul-hilang menggeser seluruh isi di
          bawahnya, dan itu terjadi persis saat pengguna sedang menekan sesuatu.
          Kemajuan job pindah ke panel Start, yang tempatnya selalu ada. */}
      <Alerts items={[
        // Gagal di tengah job: sebabnya sudah tertulis di kartu tahap yang
        // merah, jadi tidak diulang sebagai notifikasi melayang.
        error && !(screen === "run" && failedStage >= 0) && { kind: "error" as const, text: error },
        modelMissing && { kind: "warn" as const, key: `model-${model}`,
          // Tautan ke halaman yang bisa MEMASANGNYA, bukan perintah terminal:
          // `./setup.sh` adalah skrip pengembang yang bahkan tidak ada di
          // aplikasi Windows, dan menyuruh orang menjalankannya sama dengan
          // menyuruhnya berhenti.
          text: <>{t("modelMissingWarn", { model })} {t("modelMissingWarnTail")}{" "}
            <a href="/requirements">{t("openRequirements")}</a></> },
      ]} />

      {/* Pemilih berkas: modal, jadi ia berdiri di luar grid kolom. */}
      {picker && (
        <Picker
          mode={picker === "out" ? "folder" : "file"}
          start={picker === "out" ? outputDir : picker === "banner" ? m.logos[logoTarget]?.image || "" : path}
          onPick={(p) => {
            if (picker === "out") setOutputDir(p);
            else if (picker === "banner") m.putLogo(p, logoTarget);
            else setPath(p);
            setPicker(null);
          }}
          onClose={() => setPicker(null)}
        />
      )}

      {screen === "setup" && (
        <div className="setup-grid">
          <div className="setup-main">
            <SourceCard path={path} previewTime={previewTime} onSeek={setPreviewTime} onPick={() => setPicker("video")} onRemove={removeSource} onFile={useFile} onUrl={fetchUrl} fetchPhase={fetchPhase} fetchTitle={fetchTitle} fetchWarn={fetchWarn}
              uploading={uploading} uploadPct={uploadPct} />

            <MoreControls disabled={!path} addLog={addLog} web={web}
              model={model} setModel={setModel} models={models} transcriber={transcriber}
              engine={engine} setEngine={setEngine} llmModel={llmModel} setLlmModel={setLlmModel}
              transcriptFix={transcriptFix} setTranscriptFix={setTranscriptFix} terms={terms} setTerms={setTerms}
              resolution={resolution} setResolution={setResolution} quality={quality} setQuality={setQuality}
              fps={fps} setFps={setFps} outputDir={outputDir} setOutputDir={setOutputDir} onPickOut={() => setPicker("out")}
              fonts={fonts} subFont={subFont} setSubFont={setSubFont} fontManual={fontManual} setFontManual={setFontManual}
              fontCheck={fontCheck} fontChecking={fontChecking}
              subColor={subColor} setSubColor={setSubColor} subSize={subSize} setSubSize={setSubSize}
              subOutline={subOutline} setSubOutline={setSubOutline} subSpeed={subSpeed} setSubSpeed={setSubSpeed}
              subBox={subBox} setSubBox={setSubBox} subMode={subMode} setSubMode={setSubMode}
              subUpper={subUpper} setSubUpper={setSubUpper} subFx={subFx} setSubFx={setSubFx}
              platform={platform} setPlatform={setPlatform}
              reframe={reframe} setReframe={changeReframe} background={background} setBackground={setBackground}
              zoom={zoom} setZoom={setZoom}
              m={m} timed={timed} setTimed={setTimed} setError={setError}
              onPickLogo={(i) => { setLogoTarget(i); setPicker("banner"); }}
              subHighlight={subHighlight} setSubHighlight={setSubHighlight} pickSize={pickSize}
              maxClips={maxClips} setMaxClips={setMaxClips}
              durationPreset={durationPreset} setDurationPreset={setDurationPreset}
              saveMode={saveMode} setSaveMode={setSaveMode} />

          </div>

          {/* Pratinjau: kolom baku yang menempel di kanan (stage.tsx). Alat
              posisi & bantu-lihat di bawahnya, tempat subtitlenya diseret. */}
          <Stage title={t("previewTitle")} badge="9:16" bad={inUnsafe} help={inUnsafe ? t("unsafeWarning") : t("dragHint")}>
            {/* Satu baris alat di atas pratinjau (StageBar, sama dengan Mark & News):
                area aman · area tertutup UI, lalu kisi + garis bantu (tombol "tengah"
                dibuang: kembar dengan garis bantu, subtitle tetap menempel di tengah saat digeser). */}
            <StageBar grid={{ grid, setGrid, always: alwaysGuides, setAlways: setAlwaysGuides }}
              lead={
                <fieldset className="pv-tools" disabled={!path}>
                  <button type="button" className="ghost tiny icon-only" title={t("mcSafe")} aria-label={t("mcSafe")} disabled={!zone} onClick={placeSafe}>
                    <Smartphone className="ico" aria-hidden="true" /></button>
                  <button type="button" className={"ghost tiny icon-only" + (showZones ? " active" : "")} aria-pressed={showZones}
                    title={t("toolAreas")} aria-label={t("toolAreas")} onClick={() => setShowZones(!showZones)}><Eye className="ico" aria-hidden="true" /></button>
                </fieldset>
              } />
            {preview}
          </Stage>
        </div>
      )}

      {screen === "run" && (
        <RunView
          stage={stage} progress={progress} startedAt={startedAt} total={maxClips}
          clips={jobClips} error={error} logs={logs} summary={summary}
          failedStage={failedStage} onRetry={retry} retrying={retrying}
          onChangeSettings={async () => { await cancel(); setScreen("setup"); }}
        />
      )}

      {screen === "result" && (
        <ResultView clips={jobClips} platformName={platformName} onRerender={rerender} rerendering={rerendering} />
      )}
    </div>
  );
}

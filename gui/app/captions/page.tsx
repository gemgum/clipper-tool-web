"use client";

// Tab keempat: pembuat caption.
//
// Bentuknya menyalin halaman "/" apa adanya — dua kolom, panel bernama, tanpa
// bilah atas: kiri yang DILIHAT (caption hasil + log), kanan yang DIISI lalu
// dijalankan (daftar video, setelan, mesin AI, tombol Mulai).
//
// Satu video atau tiga puluh adalah alur yang SAMA: bulk cuma daftar yang lebih
// panjang. Tidak ada mode kedua, tidak ada tombol kedua.

import { useCallback, useEffect, useRef, useState } from "react";
import EmptyState from "../empty-state";
import PageHeader from "../page-header";
import { X, Copy, Folder, Film, Captions, Check, Plus } from "lucide-react";
import { Segmented } from "../clip-steps";
import { eng, isWeb, upload, useWeb } from "../engine";
import { useI18n } from "../i18n";
import Alerts from "../alerts";
import { useAI } from "../ai";
import LogPanel from "../log-panel";
import Picker from "../picker";
import Stepper from "../stepper";

type WhisperModel = { name: string; downloaded: boolean; size: string };
type Variant = { style?: string; hook: string; body: string; tags?: string[]; violations?: string[] };
type FileResult = {
  video: string;
  name: string;
  txt?: string;
  transcript?: string;
  variants?: Variant[];
  video_seconds?: number;
  used_seconds?: number;
  error?: string;
};
type CaptionJob = {
  id: string;
  status: string;
  stage: string;
  progress: number;
  log?: string[];
  error?: string;
  result?: { files: FileResult[]; engine: string };
};

// Nama berkas dari sebuah path, tanpa peduli pemisah mana yang dipakai OS-nya.
const baseName = (p: string) => p.split(/[\\/]/).pop() || p;

// Urutan mutu model whisper, terbaik dulu. Nama yang tidak ada di daftar ini
// (model baru) dianggap paling belakang — ia tidak boleh diam-diam terpilih.
const WHISPER_RANK = ["large-v3-turbo", "large-v3", "large", "medium", "small", "base", "tiny"];

const best = (models: WhisperModel[]) =>
  models
    .filter((m) => m.downloaded)
    .sort((a, b) =>
      (WHISPER_RANK.indexOf(a.name) + 1 || 99) - (WHISPER_RANK.indexOf(b.name) + 1 || 99))[0]?.name;

const fmtDur = (sec: number) => {
  const s = Math.round(sec);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
};

export default function CaptionsPage() {
  const { t } = useI18n();

  // --- daftar video ---
  const [videos, setVideos] = useState<string[]>([]);
  // Mode web (notes/42): video DIUNGGAH lewat input berkas ini, dan kabar
  // unggahannya tampil di bawah tombolnya.
  const web = useWeb();
  const fileInput = useRef<HTMLInputElement>(null);
  const [upNote, setUpNote] = useState("");
  const [paste, setPaste] = useState("");
  const [picking, setPicking] = useState<"" | "file" | "folder" | "out">("");
  // Folder tujuan. KOSONG = di sebelah tiap videonya, dan itu bawaannya: di
  // situlah orang mencarinya saat hendak memposting.
  const [outDir, setOutDir] = useState("");
  const [dragOver, setDragOver] = useState(false);

  // --- setelan ---
  //
  // Model whisper ADA DI SINI, bukan dianggap urusan pemasangan: caption hanya
  // sebaik kata-kata yang dipakai menulisnya, dan itu yang paling menentukan
  // hasilnya. Tanpa pilihan ini halaman selalu memakai model bawaan, dan
  // percakapan sehari-hari bahasa Indonesia adalah yang paling dirugikan.
  const [models, setModels] = useState<WhisperModel[]>([]);
  const [whisper, setWhisper] = useState("");
  const [minutes, setMinutes] = useState(5);
  const [variants, setVariants] = useState(3);
  const [terms, setTerms] = useState("");

  // --- mesin ---
  // Dari Pengaturan (global atau pengecualian alat "captions"), DESIGN-Clipper-
  // Lanjutan §5: mesin AI & model Whisper tidak lagi dipilih di alat ini.
  const ai = useAI("captions");
  const engine = ai?.engine || "ollama";
  const model = ai?.model || "";

  // --- job ---
  const [jobId, setJobId] = useState("");
  const [job, setJob] = useState<CaptionJob | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState("");

  const busy = job?.status === "running" || (!!jobId && !job);

  // Daftar model ditarik ulang tiap jendela kembali fokus: model diunduh di
  // halaman Requirements, dan tanpa ini halaman terus mengira ia belum ada.
  //
  // Yang dipilih otomatis adalah model TERBAIK yang sudah ada, bukan yang
  // pertama di daftar. Bedanya nyata: "yang pertama" jatuh ke base — model uji
  // cepat — dan caption yang ditulis dari transkrip base tidak bisa diselamatkan
  // tahap mana pun sesudahnya. Klip pendek, jadi model besar pun terjangkau.
  useEffect(() => {
    const load = () =>
      fetch(eng("/api/models")).then((r) => r.json()).then((m: WhisperModel[]) => {
        setModels(m);
        setWhisper((cur) => {
          if (m.some((x) => x.name === cur && x.downloaded)) return cur;
          return best(m) ?? cur;
        });
      }).catch(() => {});
    load();
    window.addEventListener("focus", load);
    return () => window.removeEventListener("focus", load);
  }, []);

  // Satu langganan SSE untuk seluruh halaman, dibuka sekali. Job berjalan
  // menit-menitan per video, jadi kabarnya datang dari sini — bukan polling.
  useEffect(() => {
    const es = new EventSource(eng("/api/captions/events"));
    es.addEventListener("caption", (ev) => {
      const j: CaptionJob = JSON.parse((ev as MessageEvent).data);
      setJobId((cur) => {
        if (cur && j.id !== cur) return cur;
        setJob(j);
        if (j.log) setLogs(j.log);
        if (j.error) setError(j.error);
        // Mode web: engine menghapus video unggahan begitu job sukses, jadi
        // daftarnya ikut dibersihkan — menjalankannya lagi butuh unggah ulang.
        if (j.status === "done") {
          const done = (j.result?.files ?? []).map((f) => f.video);
          isWeb().then((w) => w && setVideos((v) => v.filter((x) => !done.includes(x))));
        }
        return cur || j.id;
      });
    });
    return () => es.close();
  }, []);

  const add = useCallback((paths: string[]) => {
    setVideos((cur) => {
      const out = [...cur];
      for (const p of paths) {
        // Kutip pembungkus dibuang di sini juga, bukan cuma di engine: "Copy as
        // path" di Explorer selalu memasangnya, dan daftar di layar harus
        // menampilkan nama berkasnya — bukan `"C:\…mp4"` lengkap dengan kutip.
        const path = p.trim().replace(/^["']|["']$/g, "").trim();
        if (path && !out.includes(path)) out.push(path);
      }
      return out;
    });
  }, []);

  // Seluruh isi folder sekaligus — jalan bulk yang sebenarnya. Engine sudah
  // menandai mana yang video di /api/browse, jadi penyaringannya tidak perlu
  // ditebak dari akhiran nama berkas di sini.
  const addFolder = useCallback(async (dir: string) => {
    try {
      const r = await fetch(eng(`/api/browse?dir=${encodeURIComponent(dir)}`));
      const data = await r.json();
      if (!r.ok) throw new Error(data.error || "browse failed");
      const found = (data.entries || []).filter((e: any) => e.video).map((e: any) => e.path);
      if (!found.length) { setError(t("capFolderEmpty")); return; }
      add(found);
    } catch (e) {
      setError(String(e));
    }
  }, [add, t]);

  // Berkas yang dilepas TIDAK diunggah: engine jalan di mesin yang sama, jadi
  // ia ditanya di mana berkasnya (notes/24). Beberapa sekaligus boleh.
  const dropFiles = useCallback(async (files: FileList | File[]) => {
    // Mode web: engine di server, mencarinya di sana pasti gagal — unggah.
    const web = await isWeb();
    for (const f of Array.from(files)) {
      if (web) {
        try {
          add([await upload(f, (x) => setUpNote(`${f.name}: ${t("uploadingPct", { pct: Math.round(x * 100) })}`))]);
        } catch (e) { setError(`${f.name}: ${String(e)}`); }
        setUpNote("");
        continue;
      }
      try {
        const r = await fetch(eng("/api/locate"), {
          method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ name: f.name, size: f.size }),
        });
        const data = await r.json();
        if (r.ok && data.path) { add([data.path]); continue; }
      } catch { /* engine mati: laporkan di bawah */ }
      setError(`${f.name}: ${t("capFailed")}`);
    }
  }, [add, t]);

  const start = async () => {
    setError("");
    setLogs([]);
    setJob(null);
    setJobId("");
    try {
      const r = await fetch(eng("/api/captions"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          videos, engine, model, minutes, variants, terms,
          out_dir: outDir, whisper_model: whisper,
        }),
      });
      const data = await r.json();
      if (!r.ok) throw new Error(data?.error || data?.message || "failed to start");
      setJobId(data.id);
    } catch (e) {
      setError(String(e));
    }
  };

  const cancel = async () => {
    if (!jobId) return;
    try {
      const r = await fetch(eng(`/api/captions/${jobId}/cancel`), {
        method: "POST", headers: { "Content-Type": "application/json" }, body: "{}",
      });
      if (!r.ok) throw new Error(await r.text());
    } catch (e) {
      setError(String(e));
    }
  };

  // Yang disalin = caption LENGKAP dengan tagarnya: itulah yang ditempel ke
  // aplikasi sebelah, dan tagar yang harus disusun ulang sendiri adalah tagar
  // yang cepat atau lambat lupa ikut.
  const copy = async (key: string, v: Variant) => {
    const text = [v.hook, v.body, v.tags?.length ? v.tags.map((x) => "#" + x).join(" ") : ""]
      .filter(Boolean).join("\n\n");
    try {
      await navigator.clipboard.writeText(text);
      setCopied(key);
      setTimeout(() => setCopied(""), 1500);
    } catch {
      setError(t("errCopy"));
    }
  };

  const files = job?.result?.files ?? [];

  // "Semua" = batas yang jauh melebihi durasi video mana pun (0 berarti
  // bawaan 5 menit di engine).
  const ALL_MIN = 24 * 60;
  const styleLabel = (s?: string) => t(s === "question" ? "capStyleQuestion" : s === "short" ? "capStyleShort" : "capStyleDirect");
  const capText = (v: Variant) => [v.hook, v.body, v.tags?.length ? v.tags.map((x) => "#" + x).join(" ") : ""].filter(Boolean).join("\n\n");
  const doneFiles = files.filter((f) => !f.error && (f.variants ?? []).length);
  const copyAll = async () => {
    const text = doneFiles.map((f) => [f.name, ...(f.variants ?? []).map((v) => `[${styleLabel(v.style)}]\n${capText(v)}`)].join("\n\n")).join("\n\n---\n\n");
    try { await navigator.clipboard.writeText(text); setCopied("all"); setTimeout(() => setCopied(""), 1500); } catch { setError(t("errCopy")); }
  };
  // Transkrip ikut di hasil tiap video (engine caption.FileResult.Transcript).
  const [shownTxt, setShownTxt] = useState<Set<number>>(new Set());
  const toggleTxt = (i: number) => setShownTxt((cur) => { const n = new Set(cur); if (n.has(i)) n.delete(i); else n.add(i); return n; });
  const pct = Math.round((job?.progress ?? 0) * 100);

  return (
    <div className="screen scroll clips-v2 captions-v2">
      <PageHeader title={t("tabCaptions")} subtitle={t("subCaptions")}>
        {doneFiles.length > 0 && !busy && (
          <button type="button" className="ghost" onClick={copyAll}><Copy className="ico" aria-hidden="true" /> {copied === "all" ? t("copied") : t("capCopyAll")}</button>
        )}
        {busy ? (
          <button type="button" className="danger" onClick={cancel}>{t("cancelRun")}</button>
        ) : (
          <button type="button" className="primary big" onClick={start} disabled={videos.length === 0}>
            {files.length ? t("capRedo") : videos.length === 1 ? t("capStartOne") : t("capStart", { n: videos.length })}
          </button>
        )}
      </PageHeader>
      <Alerts items={[error && { kind: "error" as const, text: error }]} />

      <div className="cap-body">
        <section className="cap-results">
          {busy && (
            <div className="card" aria-live="polite">
              <p className="hi-title">{t("capRunning")}</p>
              <div className="bar wr-gap"><div style={{ width: `${pct}%` }} /></div>
              <p className="meta">{pct}%</p>
            </div>
          )}
          {!busy && !files.length && (
            <section className="card"><EmptyState icon={Captions} title={t("capEmptyTitle")} description={t("capEmpty")} /></section>
          )}
          {doneFiles.length > 0 && (
            <div className="banner good" role="status">
              <span className="banner-ico" aria-hidden="true"><Check className="ico" /></span>
              <p><b>{doneFiles.length === 1 ? t("capDoneOne") : t("capDone", { n: doneFiles.length })}</b> {t("capDoneMore", { n: variants })}</p>
            </div>
          )}
          {files.map((f, idx) => (
            <article key={f.video} className="card cap-card">
              <div className="cap-card-head">
                <span className="cap-thumb" aria-hidden="true"><Film className="ico" /></span>
                <div className="grow">
                  <p className="hi-title">{f.name}</p>
                  <p className="meta">{f.error ? `${t("capFailed")}: ${f.error}`
                    : [f.video_seconds ? fmtDur(f.video_seconds) : "",
                       f.used_seconds ? t("capListened", { used: fmtDur(f.used_seconds) }) : ""].filter(Boolean).join(" · ")}</p>
                </div>
                {!f.error && job && (
                  <>
                    <button type="button" className="ghost" onClick={() => toggleTxt(idx)}>{shownTxt.has(idx) ? t("capHideTranscript") : t("capShowTranscript")}</button>
                    {web && f.txt && <a className="btn-ghost" href={eng(`/api/captions/${job.id}/file?i=${idx}`)} download>.txt</a>}
                  </>
                )}
              </div>
              {shownTxt.has(idx) && <pre className="cap-transcript">{f.transcript || t("capNoTranscript")}</pre>}
              {(f.variants ?? []).map((v, i) => (
                <div key={i} className="cap-option">
                  <div className="cap-option-head">
                    <span className="hi-badge">{styleLabel(v.style)}</span>
                    <span className="grow" />
                    <span className="meta">{t("capChars", { n: capText(v).length })}</span>
                    <button type="button" className="ghost" onClick={() => copy(f.video + i, v)}>{copied === f.video + i ? t("copied") : t("capCopy")}</button>
                  </div>
                  <p className="cap-hook">{v.hook}</p>
                  {v.body && <p className="cap-body-text">{v.body}</p>}
                  {!!v.tags?.length && <p className="meta">{v.tags.map((x) => "#" + x).join(" ")}</p>}
                  {(v.violations ?? []).map((w, k) => <p key={k} className="cap-warn">{t("capCheck")}: {w}</p>)}
                </div>
              ))}
              {!f.error && !(f.variants ?? []).length && <p className="meta">{t("capNoSpeech")}</p>}
            </article>
          ))}
        </section>

        <section className="cap-side">
          <div className={"card cap-drop" + (dragOver ? " over" : "")}
            onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
            onDragLeave={() => setDragOver(false)}
            onDrop={(e) => { e.preventDefault(); setDragOver(false); if (e.dataTransfer.files?.length) dropFiles(e.dataTransfer.files); }}>
            <div className="card-head">
              <h2>{t("capVideosTitle")}</h2>
              <span className="meta">{videos.length === 1 ? t("capFilesOne") : t("capFiles", { n: videos.length })}</span>
            </div>
            <ul className="cap-videos">
              {videos.map((v) => (
                <li key={v}>
                  <Film className="ico" aria-hidden="true" />
                  <span className="grow" title={v}>{baseName(v)}</span>
                  <button type="button" className="ghost tiny icon-only" aria-label={t("capRemove")} title={t("capRemove")} disabled={busy}
                    onClick={() => setVideos((cur) => cur.filter((x) => x !== v))}><X className="ico" aria-hidden="true" /></button>
                </li>
              ))}
            </ul>
            {web ? (
              <>
                <input ref={fileInput} type="file" accept="video/*" multiple hidden
                  onChange={(e) => { const fs = Array.from(e.target.files ?? []); e.target.value = ""; if (fs.length) dropFiles(fs); }} />
                <button type="button" className="ghost cap-add" disabled={!!upNote} onClick={() => fileInput.current?.click()}>
                  <Plus className="ico" aria-hidden="true" /> {t("capAddVideo")}
                </button>
                {upNote && <p className="meta">{upNote}</p>}
              </>
            ) : (
              <>
                <div className="cap-add-row">
                  <button type="button" className="ghost cap-add" onClick={() => setPicking("file")}><Plus className="ico" aria-hidden="true" /> {t("capAddVideo")}</button>
                  <button type="button" className="ghost" onClick={() => setPicking("folder")}><Folder className="ico" aria-hidden="true" /> {t("capPickFolder")}</button>
                </div>
                <div className="path-row wr-gap">
                  <input value={paste} onChange={(e) => setPaste(e.target.value)} aria-label={t("capPastePlaceholder")}
                    onKeyDown={(e) => { if (e.key === "Enter") { add(paste.split("\n")); setPaste(""); } }} placeholder={t("capPastePlaceholder")} />
                  <button type="button" className="ghost" disabled={!paste.trim()} onClick={() => { add(paste.split("\n")); setPaste(""); }}>{t("capAdd")}</button>
                </div>
              </>
            )}
          </div>

          <div className="card">
            <h2>{t("capHowTitle")}</h2>
            <p className="step-label wr-gap">{t("capListenLabel")}</p>
            <Segmented label={t("capListenLabel")} value={minutes >= ALL_MIN ? ALL_MIN : minutes} onChange={setMinutes}
              options={[{ value: 2, name: t("capMin2") }, { value: 5, name: t("capMin5") }, { value: ALL_MIN, name: t("capMinAll") }]} />
            <p className="meta cap-hint">{t("capListenHint")}</p>
            <p className="step-label wr-gap">{t("capVariantsLabel")}</p>
            <Stepper value={variants} onChange={setVariants} min={1} max={5} />
            <label className="step-label wr-gap" htmlFor="cap-terms">{t("terms")}</label>
            <input id="cap-terms" value={terms} onChange={(e) => setTerms(e.target.value)} placeholder={t("termsPlaceholder")} disabled={busy} />
            <p className="meta cap-hint">{t("capTermsHint")}</p>
            {!web && (
              <>
                <label className="step-label wr-gap" htmlFor="cap-out">{t("outputDir")}</label>
                <div className="path-row">
                  <input id="cap-out" value={outDir} onChange={(e) => setOutDir(e.target.value)} placeholder={t("capOutPlaceholder")} disabled={busy} />
                  <button type="button" className="ghost" onClick={() => setPicking("out")}>{t("pickerGo")}…</button>
                </div>
              </>
            )}
          </div>

          <details className="card adv">
            <summary>{t("advancedTitle")}</summary>
            <p className="meta adv-hint">{t("capAdvancedHint")}</p>
            <p className="ai-line">{t("whisperModel")}: <b>{whisper || "–"}</b> <a href="/requirements">{t("aiChangeInSettings")}</a></p>
            <p className="ai-line">{t("aiInUse")} <b>{engine}</b>{model && <code>{model}</code>} <a href="/requirements">{t("aiChangeInSettings")}</a></p>
          </details>
        </section>
      </div>

      <div className="wr-tech">
        <details className="card tech">
          <summary>{t("techNotes")}</summary>
          <LogPanel logs={logs} />
        </details>
      </div>

      {picking && (
        <Picker
          mode={picking === "file" ? "file" : "folder"}
          onPick={(p) => {
            if (picking === "folder") addFolder(p);
            else if (picking === "out") setOutDir(p);
            else add([p]);
            setPicking("");
          }}
          onClose={() => setPicking("")}
        />
      )}
    </div>
  );
}

"use client";

// Tab keempat: pembuat caption.
//
// Bentuknya menyalin halaman "/" apa adanya — dua kolom, panel bernama, tanpa
// bilah atas: kiri yang DILIHAT (caption hasil + log), kanan yang DIISI lalu
// dijalankan (daftar video, setelan, mesin AI, tombol Mulai).
//
// Satu video atau tiga puluh adalah alur yang SAMA: bulk cuma daftar yang lebih
// panjang. Tidak ada mode kedua, tidak ada tombol kedua.

import { copyText } from "../copy-link";
import { useEffect, useState } from "react";
import EmptyState from "../empty-state";
import PageHeader from "../page-header";
import { X, Copy, Film, Captions, Check } from "lucide-react";
import { Segmented } from "../clip-steps";
import { Group, Row } from "../tab-card";
import { eng, isWeb, useWeb } from "../engine";
import { useI18n } from "../i18n";
import Alerts from "../alerts";
import { useAI } from "../ai";
import { TechNotes } from "../log-panel";
import { useJob } from "../use-job";
import Picker from "../picker";
import FileCard, { baseName, useFiles } from "../file-list";
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

// Nama berkas dari sebuah path, tanpa peduli pemisah mana yang dipakai OS-nya.

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

// Isi folder yang ditambahkan: video saja.
const KEEP = (e: { video?: boolean; path: string }) => !!e.video;

export default function CaptionsPage() {
  const { t } = useI18n();

  // --- daftar video ---
  // Mode web (notes/42): video DIUNGGAH lewat input berkas ini, dan kabar
  // unggahannya tampil di bawah tombolnya.
  const web = useWeb();
  const [picking, setPicking] = useState<"" | "file" | "folder" | "out">("");
  // Folder tujuan. KOSONG = di sebelah tiap videonya, dan itu bawaannya: di
  // situlah orang mencarinya saat hendak memposting.
  const [outDir, setOutDir] = useState("");

  // --- setelan ---
  //
  // Model whisper ADA DI SINI, bukan dianggap urusan pemasangan: caption hanya
  // sebaik kata-kata yang dipakai menulisnya, dan itu yang paling menentukan
  // hasilnya. Tanpa pilihan ini halaman selalu memakai model bawaan, dan
  // percakapan sehari-hari bahasa Indonesia adalah yang paling dirugikan.
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
  const [error, setError] = useState("");
  // Daftar berkas + cara menambahnya: satu untuk semua halaman (file-list.tsx).
  const fl = useFiles(KEEP, setError);
  const { files: videos, setFiles: setVideos, add, addFolder } = fl;
  // Siklus job latar (use-job.ts): langganan SSE, start, cancel.
  const { job, logs, busy, start: startJob, cancel } = useJob<{ files: FileResult[]; engine: string }>("/api/captions", "caption", setError, (j) => {
    // Mode web: engine menghapus video unggahan begitu job sukses, jadi daftarnya
    // ikut dibersihkan — menjalankannya lagi butuh unggah ulang.
    const gone = (j.result?.files ?? []).map((f) => f.video);
    isWeb().then((w) => w && setVideos((v) => v.filter((x) => !gone.includes(x))));
  });
  const [copied, setCopied] = useState("");


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
        setWhisper((cur) => {
          if (m.some((x) => x.name === cur && x.downloaded)) return cur;
          return best(m) ?? cur;
        });
      }).catch(() => {});
    load();
    window.addEventListener("focus", load);
    return () => window.removeEventListener("focus", load);
  }, []);



  const start = () => startJob({ videos, engine, model, minutes, variants, terms, out_dir: outDir, whisper_model: whisper });

  // Yang disalin = caption LENGKAP dengan tagarnya: itulah yang ditempel ke
  // aplikasi sebelah, dan tagar yang harus disusun ulang sendiri adalah tagar
  // yang cepat atau lambat lupa ikut.
  const copy = async (key: string, v: Variant) => {
    const text = [v.hook, v.body, v.tags?.length ? v.tags.map((x) => "#" + x).join(" ") : ""]
      .filter(Boolean).join("\n\n");
    try {
      await copyText(text);
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
    try { await copyText(text); setCopied("all"); setTimeout(() => setCopied(""), 1500); } catch { setError(t("errCopy")); }
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

      {/* Kiri = yang diisi, kanan = hasilnya (aturan kolom CLAUDE.md). */}
      <div className="io-grid">
        <section className="io-in">
          <FileCard title={t("capVideosTitle")} fl={fl} web={web} accept="video/*" addLabel={t("capAddVideo")}
            onPick={(m) => setPicking(m)}>
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
          </FileCard>

          {/* Setelan dalam bentuk baris kartu klip (tab-card.tsx). Tanpa tab:
              isinya sedikit, tab cuma menambah klik. */}
          <div className="card cap-set">
            <h2>{t("capHowTitle")}</h2>
            <Group title={t("capGroupCaption")}>
              <Row label={t("capListenLabel")} note={t("capListenHint")}>
                <Segmented label={t("capListenLabel")} value={minutes >= ALL_MIN ? ALL_MIN : minutes} onChange={setMinutes}
                  options={[{ value: 2, name: t("capMin2") }, { value: 5, name: t("capMin5") }, { value: ALL_MIN, name: t("capMinAll") }]} />
              </Row>
              <Row label={t("capVariantsLabel")}>
                <Stepper value={variants} onChange={setVariants} min={1} max={5} />
              </Row>
              <Row label={t("terms")} note={t("capTermsHint")}>
                <input id="cap-terms" value={terms} onChange={(e) => setTerms(e.target.value)} placeholder={t("termsPlaceholder")} disabled={busy} aria-label={t("terms")} />
              </Row>
              {!web && (
                <Row label={t("outputDir")}>
                  <div className="path-row">
                    <input id="cap-out" value={outDir} onChange={(e) => setOutDir(e.target.value)} placeholder={t("capOutPlaceholder")} disabled={busy} aria-label={t("outputDir")} />
                    <button type="button" className="ghost" onClick={() => setPicking("out")}>{t("pickerGo")}…</button>
                  </div>
                </Row>
              )}
              <Row label={t("whisperModel")}><span className="mc-val">{whisper || "–"}</span></Row>
              <Row label={t("mcEngineRow")}>
                <span className="mc-val">{engine}{model && <> · {model}</>}</span>
                <a className="mc-link" href="/requirements">{t("aiChangeInSettings")} →</a>
              </Row>
            </Group>
          </div>
          {/* Log di dasar kolom kiri: halaman ini tidak bergulir, kolomnya yang bergulir. */}
          <TechNotes logs={logs} />
        </section>
        <section className="io-out">
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

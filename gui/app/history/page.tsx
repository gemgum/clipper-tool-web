"use client";

// Riwayat hasil — DESIGN-Clipper-Lanjutan.md §6 (9 Oktober 2026). SATU daftar
// untuk kelima alat, disaring per alat lewat chip. Pekerjaan yang berjalan di
// atas (bisa dibatalkan), yang gagal menyebut tahap & sebabnya dengan jalan
// keluar, dan skor klip diberi keterangan arti — bukan singkatan terpotong.
//
// Sumbernya endpoint yang sudah ada: /api/jobs (klip), /api/cards (kartu),
// /api/posts (artikel), /api/captions, /api/watermark. Ketiga yang terakhir
// kini tersimpan di disk (<DataDir>/runs), jadi bertahan setelah aplikasi
// ditutup.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { Download, FileText, History, Trash2 } from "lucide-react";
import EmptyState from "../empty-state";
import { useConfirm } from "../confirm";
import PageHeader from "../page-header";
import Alerts from "../alerts";
import { eng, engineURL } from "../engine";
import { useI18n } from "../i18n";
import type { MessageKey } from "../i18n";

type Clip = { id: string; job_id: string; start: number; end: number; duration: number; score: number; title: string; status: string; error?: string };
type Job = { id: string; status: string; stage: string; progress: number; error?: string; created_at: string; updated_at?: string; input?: string; source?: string; clips?: Clip[] };
type Card = { id: string; made: string; bytes: number; file: string; zip: string; title?: string; source?: string; ratio?: string };
type BG<T> = { id: string; status: string; stage: string; progress: number; error?: string; created_at: string; result?: T };
type PostRes = { draft?: { title?: string; words?: number }; basket?: unknown; sources?: unknown[] };
type CapRes = { files: { name: string; txt?: string; error?: string }[] };
type WmRes = { files: { name: string; output?: string; error?: string }[] };

type Kind = "clips" | "cards" | "writer" | "captions" | "watermark";
type Entry = {
  key: string; kind: Kind; status: string; created: string; title: string;
  job?: Job; card?: Card; post?: BG<PostRes>; cap?: BG<CapRes>; wm?: BG<WmRes>;
};

const KIND_LABEL: Record<Kind, MessageKey> = {
  clips: "hiKindClips", cards: "hiKindCards", writer: "hiKindWriter", captions: "hiKindCaptions", watermark: "hiKindWatermark",
};
// Tahap engine → tahap yang dikenali operator (sama dengan layar proses klip).
const STAGE_LABEL: Record<string, MessageKey> = {
  extracting: "stageTranscribe", transcribing: "stageTranscribe", correcting: "stagePick",
  segmenting: "stagePick", scoring: "stagePick", rendering: "stageRender",
};
const base = (p = "") => p.split(/[\\/]/).pop() || p;
const fmtMin = (sec: number) => `${Math.floor(sec / 60)}:${String(Math.floor(sec % 60)).padStart(2, "0")}`;
const band = (s: number) => (s >= 75 ? "strong" : s >= 60 ? "medium" : "plain");

export default function HistoryPage() {
  const { t, lang } = useI18n();
  const [jobs, setJobs] = useState<Job[] | null>(null);
  const [cards, setCards] = useState<Card[]>([]);
  const [posts, setPosts] = useState<BG<PostRes>[]>([]);
  const [caps, setCaps] = useState<BG<CapRes>[]>([]);
  const [wms, setWms] = useState<BG<WmRes>[]>([]);
  const [filter, setFilter] = useState<Kind | "all" | "failed">("all");
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [ask, confirmEl] = useConfirm();
  // Pratinjau kartu ukuran penuh: <dialog> bawaan (Esc & fokus diurus browser).
  const zoomRef = useRef<HTMLDialogElement>(null);
  const [zoom, setZoom] = useState("");
  const openZoom = (src: string) => { setZoom(src); zoomRef.current?.showModal(); };

  const load = useCallback(() => {
    const get = (p: string) => fetch(eng(p)).then((r) => r.json());
    get("/api/jobs").then((d) => setJobs(Array.isArray(d) ? d : [])).catch(() => { setError(t("engineUnreachable", { url: engineURL() })); setJobs([]); });
    get("/api/cards").then((d) => setCards(Array.isArray(d) ? d : [])).catch(() => {});
    get("/api/posts").then((d) => setPosts(d.posts || [])).catch(() => {});
    get("/api/captions").then((d) => setCaps(d.captions || [])).catch(() => {});
    get("/api/watermark").then((d) => setWms(d.jobs || [])).catch(() => {});
  }, [t]);
  useEffect(load, [load]);
  // ?f=cards dsb.: tombol "My cards" & "Run in background" membuka saringannya
  // sendiri, bukan "All".
  useEffect(() => {
    const f = new URLSearchParams(window.location.search).get("f");
    if (f && (["clips", "cards", "writer", "captions", "watermark", "failed"] as string[]).includes(f)) setFilter(f as Kind | "failed");
  }, []);
  // Yang berjalan diperbarui tiap 4 detik selama masih ada.
  const anyRunning = (jobs || []).some((j) => j.status === "running" || j.status === "queued")
    || [...posts, ...caps, ...wms].some((j) => j.status === "running");
  useEffect(() => {
    if (!anyRunning) return;
    const id = setInterval(load, 4000);
    return () => clearInterval(id);
  }, [anyRunning, load]);

  const entries = useMemo<Entry[]>(() => {
    const out: Entry[] = [];
    (jobs || []).forEach((j) => out.push({ key: `job/${j.id}`, kind: "clips", status: j.status, created: j.created_at, title: base(j.input || j.source || j.id), job: j }));
    cards.forEach((c) => out.push({ key: `card/${c.id}`, kind: "cards", status: "done", created: c.made, title: c.title || c.id, card: c }));
    posts.forEach((p) => out.push({ key: `post/${p.id}`, kind: "writer", status: p.status, created: p.created_at, title: p.result?.draft?.title || p.id, post: p }));
    caps.forEach((c) => out.push({ key: `cap/${c.id}`, kind: "captions", status: c.status, created: c.created_at, title: (c.result?.files || []).map((f) => f.name).join(", ") || c.id, cap: c }));
    wms.forEach((w) => out.push({ key: `wm/${w.id}`, kind: "watermark", status: w.status, created: w.created_at, title: (w.result?.files || []).map((f) => f.name).join(", ") || w.id, wm: w }));
    return out.sort((a, b) => (a.created < b.created ? 1 : -1));
  }, [jobs, cards, posts, caps, wms]);

  const isRunning = (e: Entry) => e.status === "running" || e.status === "queued";
  const isFailed = (e: Entry) => e.status === "error";
  const count = (f: typeof filter) => entries.filter((e) => f === "all" ? true : f === "failed" ? isFailed(e) : e.kind === f && !isFailed(e)).length;
  const shown = entries.filter((e) => filter === "all" ? true : filter === "failed" ? isFailed(e) : e.kind === filter && !isFailed(e));
  const running = shown.filter(isRunning);
  const rest = shown.filter((e) => !isRunning(e) && e.status !== "canceled");

  const when = (iso: string) => {
    try { return new Date(iso).toLocaleString(lang === "id" ? "id-ID" : "en-GB", { day: "numeric", month: "long", hour: "2-digit", minute: "2-digit" }); } catch { return iso; }
  };
  const took = (a: string, b?: string) => {
    const s = (Date.parse(b || "") - Date.parse(a)) / 1000;
    return s > 0 ? t("hiTook", { m: Math.max(1, Math.round(s / 60)) }) : "";
  };
  const eta = (e: Entry) => {
    const p = e.job?.progress ?? e.post?.progress ?? e.cap?.progress ?? e.wm?.progress ?? 0;
    const el = (Date.now() - Date.parse(e.created)) / 1000;
    const rem = p > 0.05 ? (el * (1 - p)) / p : -1;
    return { pct: Math.round(p * 100), text: rem < 0 ? t("etaUnknown") : rem < 60 ? t("etaUnderMin") : t("etaAbout", { n: Math.round(rem / 60) }) };
  };

  const cancel = async (e: Entry) => {
    const url = e.kind === "clips" ? `/api/jobs/${e.job!.id}/cancel` : e.kind === "writer" ? `/api/posts/${e.post!.id}/cancel`
      : e.kind === "captions" ? `/api/captions/${e.cap!.id}/cancel` : `/api/watermark/${e.wm!.id}/cancel`;
    await fetch(eng(url), { method: "POST", headers: { "content-type": "application/json" } }).catch(() => {});
    load();
  };
  const retry = async (e: Entry) => {
    try {
      const res = await fetch(eng(`/api/jobs/${e.job!.id}/retry`), { method: "POST", headers: { "content-type": "application/json" } });
      const d = await res.json();
      if (!res.ok) throw new Error(d.error);
      load();
    } catch (err: any) { setError(err.message); }
  };

  // Pesan galat dari balasan engine. Tidak selalu JSON: engine yang lebih lama
  // dari GUI-nya membalas "404 page not found" untuk rute yang belum ia kenal.
  const failText = async (res: Response) => {
    const raw = await res.text().catch(() => "");
    try { return JSON.parse(raw).error || t("historyDeleteFailed"); }
    catch { return `${t("historyDeleteFailed")} (HTTP ${res.status}${raw ? ": " + raw.trim() : ""})`; }
  };

  // Hapus satu entri riwayat (yang sudah berhenti). Berkas hasilnya tidak ikut.
  const removeEntry = async (e: Entry) => {
    if (!(await ask(t("hiConfirmDelete")))) return;
    const url = e.kind === "clips" ? `/api/jobs/${e.job!.id}` : e.kind === "cards" ? `/api/cards/${e.card!.id}`
      : e.kind === "writer" ? `/api/posts/${e.post!.id}` : e.kind === "captions" ? `/api/captions/${e.cap!.id}` : `/api/watermark/${e.wm!.id}`;
    try { const res = await fetch(eng(url), { method: "DELETE" }); if (!res.ok) setError(await failText(res)); }
    catch (err: any) { setError(err.message); }
    load();
  };

  const toggle = (k: string) => setPicked((p) => { const n = new Set(p); if (n.has(k)) n.delete(k); else n.add(k); return n; });
  const toggleAll = (keys: string[]) => setPicked((p) => {
    const n = new Set(p); const on = keys.every((k) => n.has(k));
    keys.forEach((k) => (on ? n.delete(k) : n.add(k))); return n;
  });
  const zipURL = (keys: string[]) => {
    const q = new URLSearchParams();
    keys.forEach((k) => (k.startsWith("card/") ? q.append("card", k.slice(5)) : q.append("clip", k)));
    return eng(`/api/download?${q.toString()}`);
  };
  const removePicked = async () => {
    if (picked.size === 0 || !(await ask(t("historyConfirm", { n: picked.size })))) return;
    setBusy(true); setError("");
    for (const k of picked) {
      const url = k.startsWith("card/") ? `/api/cards/${k.slice(5)}` : `/api/jobs/${k.split("/")[0]}/clips/${k.split("/")[1]}`;
      try { const res = await fetch(eng(url), { method: "DELETE" }); if (!res.ok) setError(await failText(res)); }
      catch (err: any) { setError(err.message); }
    }
    setPicked(new Set()); setBusy(false); load();
  };

  const chips: (Kind | "all" | "failed")[] = ["all", "clips", "cards", "writer", "captions", "watermark", "failed"];

  return (
    <div className="screen scroll clips-v2 history-v2">
      <PageHeader title={t("tabHistory")} subtitle={t("hiSub")}>
        {picked.size > 0 && (
          <>
            <button type="button" className="danger" onClick={removePicked} disabled={busy}><Trash2 className="ico" aria-hidden="true" /> {t("hiDeletePicked", { n: picked.size })}</button>
            <a className="btn-primary big" href={zipURL([...picked])}><Download className="ico" aria-hidden="true" /> {t("hiDownloadPicked", { n: picked.size })}</a>
          </>
        )}
      </PageHeader>
      <Alerts items={[error && { kind: "error" as const, text: error }]} />

      {confirmEl}
      <dialog ref={zoomRef} className="hi-zoom" onClick={(e) => { if (e.target === e.currentTarget) zoomRef.current?.close(); }}>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        {zoom && <img src={zoom} alt={t("hiPreview")} />}
        <button type="button" className="ghost" onClick={() => zoomRef.current?.close()}>{t("close")}</button>
      </dialog>

      <div className="hi-body">
        <div className="nc-chips" role="group" aria-label={t("tabHistory")}>
          {chips.map((c) => (
            <button key={c} type="button" className={"chip-btn" + (filter === c ? " on" : "")} aria-pressed={filter === c}
              onClick={() => setFilter(c)}>
              {c === "all" ? t("ncAll") : c === "failed" ? t("hiFailed") : t(KIND_LABEL[c])} {count(c)}
            </button>
          ))}
        </div>

        {jobs !== null && shown.length === 0 && (
          <section className="card">
            <EmptyState icon={History} title={t("historyEmpty")} description={t("historyEmptyHint")}
              action={<div className="hi-links">
                <Link className="btn-link" href="/">{t("tabClips")}</Link><Link className="btn-link" href="/news">{t("tabNews")}</Link>
                <Link className="btn-link" href="/writer">{t("tabWriter")}</Link><Link className="btn-link" href="/captions">{t("tabCaptions")}</Link>
                <Link className="btn-link" href="/watermark">{t("tabWatermark")}</Link>
              </div>} />
          </section>
        )}

        {running.map((e) => {
          const s = eta(e);
          const stage = e.job?.stage || e.post?.stage || e.cap?.stage || e.wm?.stage || "";
          return (
            <article key={e.key} className="card hi-job running">
              <div className="hi-head">
                <span className={"hi-badge " + e.kind}>{t(KIND_LABEL[e.kind])}</span>
                <div className="grow">
                  <p className="hi-title">{e.title}</p>
                  <p className="meta">{t("hiStarted", { t: when(e.created) })}{STAGE_LABEL[stage] ? ` · ${t(STAGE_LABEL[stage]).toLowerCase()}` : ""}</p>
                </div>
                <div className="hi-progress">
                  <div className="bar slim" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={s.pct}><div style={{ width: `${s.pct}%` }} /></div>
                  <p className="meta">{s.pct}% · {s.text}</p>
                </div>
                <button type="button" className="danger" onClick={() => cancel(e)}>{t("cancelRun")}</button>
              </div>
            </article>
          );
        })}

        {rest.map((e) => {
          if (isFailed(e)) {
            const stage = e.job?.stage ? STAGE_LABEL[e.job.stage] : undefined;
            const why = e.job?.error || e.post?.error || e.cap?.error || e.wm?.error || "";
            return (
              <article key={e.key} className="card hi-job failed">
                <div className="hi-head">
                  <span className="hi-badge failed">{t("hiFailedBadge")}</span>
                  <div className="grow">
                    <p className="hi-title">{t(KIND_LABEL[e.kind])} · {when(e.created)}</p>
                    <p className="hi-why">{stage ? t("hiStoppedAt", { stage: t(stage).toLowerCase(), why }) : why}</p>
                  </div>
                  <button type="button" className="ghost" onClick={() => removeEntry(e)} title={t("historyDelete")} aria-label={t("historyDelete")}><Trash2 className="ico" aria-hidden="true" /></button>
                  <Link className="btn-ghost" href="/requirements">{t("hiOpenSettings")}</Link>
                  {e.kind === "clips" && <button type="button" className="dark" onClick={() => retry(e)}>{t("hiRetry")}</button>}
                </div>
              </article>
            );
          }
          if (e.kind === "clips") {
            const j = e.job!;
            const clips = (j.clips || []).slice().sort((a, b) => b.score - a.score);
            const keys = clips.filter((c) => c.status !== "failed").map((c) => `${j.id}/${c.id}`);
            const allOn = keys.length > 0 && keys.every((k) => picked.has(k));
            return (
              <article key={e.key} className="card hi-job">
                <div className="hi-head">
                  <span className="hi-badge clips">{t("hiKindClips")}</span>
                  <div className="grow">
                    <p className="hi-title">{e.title}</p>
                    <p className="meta">{[when(e.created), took(j.created_at, j.updated_at), (clips.length === 1 ? t("nClipsOne") : t("nClipsLine", { n: clips.length }))].filter(Boolean).join(" · ")}</p>
                  </div>
                  {keys.length > 0 && (
                    <>
                      <label className="chk"><input type="checkbox" checked={allOn} onChange={() => toggleAll(keys)} /> {t("hiPickAll")}</label>
                      <a className="btn-ghost" href={zipURL(keys)}>{t("hiDownloadAll")}</a>
                    </>
                  )}
                </div>
                {clips.length > 0 && (
                  <>
                    <div className="hi-clips">
                      {clips.map((c) => {
                        const k = `${j.id}/${c.id}`;
                        const file = eng(`/api/jobs/${j.id}/clips/${c.id}/file`);
                        return (
                          <div key={c.id} className={"hi-clip" + (picked.has(k) ? " on" : "") + (c.status === "failed" ? " failed" : "")}>
                            <div className="hi-thumb">
                              {c.status !== "failed" && <video src={file + "#t=1"} preload="metadata" muted />}
                              {c.status !== "failed" && (
                                <input type="checkbox" className="hi-pick" checked={picked.has(k)} onChange={() => toggle(k)} aria-label={t("hiPickClip", { n: c.id })} />
                              )}
                              <span className={"g-score " + band(c.score)}>{c.score}</span>
                              <span className="g-dur">{Math.round(c.duration)}s</span>
                            </div>
                            <div className="hi-clip-body">
                              <p className="hi-clip-title" title={c.title}>{c.status === "failed" ? `${t("clipFailed")}: ${c.error || ""}` : (c.title || t("noTitle"))}</p>
                              <p className="meta">{t("hiMinute", { a: fmtMin(c.start), b: fmtMin(c.end) })}</p>
                              {c.status !== "failed" && (
                                <div className="g-actions">
                                  <a className="btn-ghost" href={file} download={`${c.id}.mp4`}>{t("hiVideo")}</a>
                                  <a className="btn-ghost" href={file + "?variant=txt"} download={`${c.id}.txt`}>{t("hiText")}</a>
                                </div>
                              )}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                    <div className="hi-legend">
                      <span><b>{t("hiLegendScore")}</b> {t("hiLegendScoreText")}</span>
                      <span><b>{t("hiLegendHook")}</b> {t("hiLegendHookText")}</span>
                      <span><b>{t("hiLegendEmotion")}</b> {t("hiLegendEmotionText")}</span>
                      <span><b>{t("hiLegendClear")}</b> {t("hiLegendClearText")}</span>
                    </div>
                  </>
                )}
              </article>
            );
          }
          if (e.kind === "cards") {
            const c = e.card!;
            const k = `card/${c.id}`;
            return (
              <article key={e.key} className="card hi-job">
                <div className="hi-head">
                  <input type="checkbox" checked={picked.has(k)} onChange={() => toggle(k)} aria-label={t("hiPickClip", { n: c.id })} />
                  <button type="button" className="hi-card-zoom" onClick={() => openZoom(eng(c.file))} aria-label={t("hiPreview")} title={t("hiPreview")}>
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img className="hi-card-thumb" src={eng(c.file)} alt="" loading="lazy" />
                  </button>
                  <span className="hi-badge cards">{t("hiKindCards")}</span>
                  <div className="grow">
                    <p className="hi-title">{e.title}</p>
                    <p className="meta">{[when(e.created), c.source, t("hiCardMeta", { ratio: c.ratio || "9:16" })].filter(Boolean).join(" · ")}</p>
                  </div>
                  <a className="btn-ghost" href={eng(c.file)} download={`${c.id}.png`}><Download className="ico" aria-hidden="true" /> {t("downloadClip")}</a>
                </div>
              </article>
            );
          }
          if (e.kind === "writer") {
            const p = e.post!;
            return (
              <article key={e.key} className="card hi-job">
                <div className="hi-head">
                  <span className="hi-badge writer">{t("hiKindWriter")}</span>
                  <div className="grow">
                    <p className="hi-title">{e.title}</p>
                    <p className="meta">{[when(e.created), p.result?.draft?.words ? t("hiWords", { n: p.result.draft.words }) : ""].filter(Boolean).join(" · ")}</p>
                  </div>
                  <a className="btn-ghost" href={eng(`/api/posts/${p.id}/file?name=article`)} download><FileText className="ico" aria-hidden="true" /> {t("downloadClip")}</a>
                </div>
              </article>
            );
          }
          const files = (e.cap?.result?.files || e.wm?.result?.files || []) as { name: string; error?: string }[];
          const id = e.cap?.id || e.wm?.id || "";
          const path = e.kind === "captions" ? "captions" : "watermark";
          return (
            <article key={e.key} className="card hi-job">
              <div className="hi-head">
                <span className={"hi-badge " + e.kind}>{t(KIND_LABEL[e.kind])}</span>
                <div className="grow">
                  <p className="hi-title">{t("hiVideos", { n: files.length })}</p>
                  <p className="meta">{when(e.created)}</p>
                </div>
              </div>
              <ul className="hi-files">
                {files.map((f, i) => (
                  <li key={i}>
                    <span className="grow">{f.name}</span>
                    {f.error ? <span className="hi-why">{f.error}</span>
                      : <a className="btn-ghost" href={eng(`/api/${path}/${id}/file?i=${i}`)} download>{t("downloadClip")}</a>}
                  </li>
                ))}
              </ul>
            </article>
          );
        })}
      </div>
    </div>
  );
}

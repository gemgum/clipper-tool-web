"use client";

// Tab ketiga: penulis artikel (notes/38) — susunan DESIGN-Clipper-Lanjutan §3
// (9 Oktober 2026): kiri sumber terpilih bernomor 1–5 + tempel link + berita
// hari ini; kanan artikel hasil sebagai artikel sungguhan dengan penanda sumber
// di akhir tiap kalimat. Mesin AI dari Pengaturan; log dilipat.
//
// Tab sendiri, bukan menumpang /news, karena aturannya berlawanan: di sana LLM
// tidak boleh menulis satu kata pun, di sini ia memang menulis. Mencampurnya
// cepat atau lambat membuat teks karangan keluar sebagai kutipan verbatim.

import { useCallback, useEffect, useRef, useState } from "react";
import EmptyState from "../empty-state";
import PageHeader from "../page-header";
import { X, Copy, FileText, Info, PenLine } from "lucide-react";
import { eng } from "../engine";
import { useI18n } from "../i18n";
import Alerts from "../alerts";
import LogPanel from "../log-panel";
import { useAI } from "../ai";

// Batas artikel sumber. Angkanya BUKAN tetap: tahap 2 mengirim seluruh fakta
// dari semua sumber dalam satu panggilan, jadi model berjendela kecil menabrak
// dinding konteks di lima sumber. Engine yang tahu jendelanya (writer.
// MaxSourcesFor), jadi angkanya ditanyakan — dan ditanyakan SEBELUM tombol
// ditekan, bukan dilaporkan sebagai galat sesudah lima menit menunggu.
// Yang di sini hanya nilai awal sampai jawabannya datang.
const MAX_SOURCES = 5;

type Article = { title: string; url: string; source: string; image?: string; date?: string; domain?: string };
type Violation = { kind: string; text: string; detail: string };
type Claim = { text: string; source: number; paragraph: number };
type Draft = { title: string; lead: string; body: string[]; words: number; tags?: string[]; violations?: Violation[]; claims?: Claim[] };
// SourceRef = sumber yang BENAR-BENAR dipakai job itu, dari engine. Bukan
// keranjang di layar: keranjang masih bisa diubah setelah job jalan, dan kaki
// artikel harus menyebut yang sama persis dengan yang ditulis ke artikel.md.
type SourceRef = { title: string; url: string; media: string };
type PostJob = {
  id: string;
  status: string;
  stage: string;
  progress: number;
  log?: string[];
  error?: string;
  result?: { post: { dir: string; image?: string }; draft: Draft; sources?: SourceRef[]; basket?: { skipped?: { url: string; reason: string }[] } };
};

export default function WriterPage() {
  const { t, lang } = useI18n();

  // --- keranjang sumber ---
  const [basket, setBasket] = useState<Article[]>([]);
  const [paste, setPaste] = useState("");
  const [typed, setTyped] = useState("");
  const [items, setItems] = useState<Article[]>([]);
  const [listBusy, setListBusy] = useState(false);

  // --- mesin ---
  // Dari Pengaturan: mesin global, atau pengecualian alat "writer" — itulah
  // pengganti "pakai mesin lain untuk menulis" (DESIGN-Clipper-Lanjutan §11).
  const ai = useAI("writer");
  const engine = ai?.engine || "ollama";
  const model = ai?.model || "";

  // --- job ---
  const [jobId, setJobId] = useState("");
  const [job, setJob] = useState<PostJob | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState("");
  const esRef = useRef<EventSource | null>(null);

  const busy = job?.status === "running" || (!!jobId && !job);

  const loadList = useCallback(async (q: string) => {
    setListBusy(true);
    setError("");
    try {
      const url = q
        ? eng(`/api/news/list?max=30&q=${encodeURIComponent(q)}`)
        : eng("/api/news/list?max=30&feed=all");
      const r = await fetch(url);
      if (!r.ok) throw new Error(await r.text());
      setItems((await r.json()) ?? []);
    } catch (e) {
      setError(String(e));
    } finally {
      setListBusy(false);
    }
  }, []);

  useEffect(() => { loadList(""); }, [loadList]);


  // Satu langganan SSE untuk seluruh halaman, dibuka sekali. Kabar job yang
  // sedang berjalan datang dari sini, bukan dari polling: satu job memanggil
  // LLM berkali-kali dan berjalan menit-menitan.
  useEffect(() => {
    const es = new EventSource(eng("/api/posts/events"));
    esRef.current = es;
    es.addEventListener("post", (ev) => {
      const j: PostJob = JSON.parse((ev as MessageEvent).data);
      setJobId((cur) => {
        if (cur && j.id !== cur) return cur;
        setJob(j);
        if (j.log) setLogs(j.log);
        if (j.error) setError(j.error);
        return cur || j.id;
      });
    });
    return () => { es.close(); esRef.current = null; };
  }, []);

  const inBasket = (url: string) => basket.some((a) => a.url === url);

  // Mengklik berita = memasukkannya ke keranjang, dan mengkliknya lagi
  // mengeluarkannya. Tombol "+ Add" tersendiri cuma menyempitkan sasaran klik
  // jadi seukuran tulisan padahal SELURUH baris tidak punya arti lain di tab
  // ini. Tombol kecil di baris itu dipakai untuk yang memang tidak bisa
  // ditebak: menyalin tautannya.
  const toggle = (a: Article) => {
    setBasket((cur) => {
      if (cur.some((x) => x.url === a.url)) return cur.filter((x) => x.url !== a.url);
      return cur.length >= maxSources ? cur : [...cur, a];
    });
  };

  // Batas sumber untuk mesin yang MENULIS — tahap itu yang memuat semua fakta
  // sekaligus. Tanpa mesin tulis terpisah, mesin bacanya yang dipakai.
  const [maxSources, setMaxSources] = useState(MAX_SOURCES);
  useEffect(() => {
    const id = engine;
    const m = model;
    if (!id) return;
    let alive = true;
    fetch(eng(`/api/posts/limits?engine=${encodeURIComponent(id)}&model=${encodeURIComponent(m)}`))
      .then((r) => r.json())
      .then((d) => { if (alive && d?.max_sources > 0) setMaxSources(d.max_sources); })
      .catch(() => { /* batas bawaan tetap berlaku */ });
    return () => { alive = false; };
  }, [engine, model]);



  // addPasted menerima beberapa alamat sekaligus, satu per baris. Alamat yang
  // ditempel belum punya judul — engine yang membacanya nanti; di sini cukup
  // domainnya supaya barisnya tidak kosong.
  const addPasted = () => {
    const urls = paste.split(/[\s,]+/).map((s) => s.trim()).filter((s) => /^https?:\/\//.test(s));
    setBasket((cur) => {
      const out = [...cur];
      for (const u of urls) {
        if (out.length >= maxSources) break;
        if (out.some((x) => x.url === u)) continue;
        let host = u;
        try { host = new URL(u).hostname.replace(/^www\./, ""); } catch { /* biarkan apa adanya */ }
        out.push({ title: u, url: u, source: host });
      }
      return out;
    });
    setPaste("");
  };

  const start = async () => {
    setError("");
    setLogs([]);
    setJob(null);
    setJobId("");
    try {
      const r = await fetch(eng("/api/posts"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          urls: basket.map((a) => a.url),
          engine,
          model,
          write_engine: "",
          write_model: "",
          lang,
        }),
      });
      const data = await r.json();
      if (!r.ok) throw new Error(data?.error || data?.message || "failed to start");
      setJobId(data.id);
    } catch (e) {
      setError(String(e));
    }
  };

  // Membatalkan job. Satu job memanggil LLM berkali-kali dan berjalan
  // menit-menitan; tanpa ini satu-satunya cara menghentikan pilihan yang salah
  // adalah menutup aplikasinya.
  const cancel = async () => {
    if (!jobId) return;
    try {
      const r = await fetch(eng(`/api/posts/${jobId}/cancel`), {
        method: "POST", headers: { "Content-Type": "application/json" }, body: "{}",
      });
      if (!r.ok) throw new Error(await r.text());
    } catch (e) {
      setError(String(e));
    }
  };

  const draft = job?.result?.draft;
  const violations = draft?.violations ?? [];
  const used = job?.result?.sources ?? [];

  const copy = async (what: "title" | "body") => {
    if (!draft) return;
    // Yang disalin = artikel LENGKAP: tagar dan kaki sumber ikut, sebab inilah
    // yang ditempel ke media pemilik proyek. Atribusi yang harus diingat sendiri
    // adalah atribusi yang cepat atau lambat lupa ditempel.
    const foot = used.map((s) => `${s.title}: ${s.url}`);
    const text = what === "title"
      ? draft.title
      : [draft.title, "", draft.lead, "", ...draft.body,
         ...(draft.tags?.length ? ["", draft.tags.join(" ")] : []),
         ...(foot.length ? ["", t("writerSources") + ":", ...foot] : [])].join("\n\n");
    try {
      await navigator.clipboard.writeText(text);
      setCopied(what);
      setTimeout(() => setCopied(""), 1500);
    } catch {
      setError(t("errCopy"));
    }
  };

  const skipped = job?.result?.basket?.skipped ?? [];
  const skipReason = (url: string) => skipped.find((s) => s.url === url)?.reason;
  const full = basket.length >= maxSources;
  const docxURL = jobId && draft ? eng(`/api/posts/${jobId}/file?name=docx`) : "";

  return (
    <div className="screen scroll clips-v2 writer-v2">
      <PageHeader title={t("tabWriter")} subtitle={t("wrSub")}>
        {busy ? (
          <button type="button" className="danger" onClick={cancel}>{t("cancelRun")}</button>
        ) : (
          <button type="button" className="primary big" onClick={start} disabled={basket.length === 0}>
            <PenLine className="ico" aria-hidden="true" /> {basket.length === 1 ? t("wrStartOne") : t("wrStart", { n: basket.length })}
          </button>
        )}
      </PageHeader>
      <Alerts items={[error && { kind: "error" as const, text: error }]} />

      <div className="wr-body">
        <section className="wr-side">
          <div className="card">
            <div className="card-head">
              <h2>{t("wrPicked")}</h2>
              <span className="meta">{t("wrCount", { n: basket.length, m: maxSources })}</span>
            </div>
            {basket.length === 0 ? (
              <p className="wr-empty">{t("wrEmptyBasket")}</p>
            ) : (
              <ol className="wr-sources">
                {basket.map((a, i) => {
                  const why = skipReason(a.url);
                  return (
                    <li key={a.url} className={why ? "skipped" : ""}>
                      <span className="wr-no">{i + 1}</span>
                      <div className="grow">
                        <p className="wr-title">{a.title}</p>
                        <p className="meta">{why ? t("wrSkipped", { why }) : (a.source || a.domain)}</p>
                      </div>
                      <button type="button" className="icon-only ghost tiny" aria-label={t("wrRemoveSource", { n: i + 1 })}
                        onClick={() => toggle(a)} disabled={busy}><X className="ico" aria-hidden="true" /></button>
                    </li>
                  );
                })}
              </ol>
            )}
            <label className="step-label wr-gap" htmlFor="wr-link">{t("wrPasteLabel")}</label>
            <div className="path-row">
              <input id="wr-link" type="url" value={paste} placeholder="https://..." onChange={(e) => setPaste(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") addPasted(); }} disabled={full} />
              <button type="button" className="ghost" onClick={addPasted} disabled={!paste.trim() || full}>{t("wrAdd")}</button>
            </div>
            {full && <p className="meta wr-full">{t("wrFull", { m: maxSources })}</p>}
          </div>

          <div className="card">
            <div className="card-head">
              <h2>{t("wrToday")}</h2>
            </div>
            <div className="path-row wr-search">
              <input type="search" value={typed} placeholder={t("searchPlaceholder")} aria-label={t("ncSearchLabel")}
                onChange={(e) => setTyped(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter") loadList(typed.trim()); }} />
              <button type="button" className="ghost" onClick={() => loadList(typed.trim())} disabled={listBusy}>{t("ncSearchBtn")}</button>
            </div>
            {listBusy && items.length === 0 && <p className="meta">{t("loadingNews")}</p>}
            <ul className="wr-suggest">
              {items.filter((a) => !inBasket(a.url)).slice(0, 12).map((a) => (
                <li key={a.url}>
                  <div className="wr-thumb">{a.image && /* eslint-disable-next-line @next/next/no-img-element */ <img src={a.image} alt="" loading="lazy" />}</div>
                  <div className="grow">
                    <p className="wr-title">{a.title}</p>
                    <p className="meta">{a.source || a.domain}</p>
                  </div>
                  <button type="button" className="ghost" onClick={() => toggle(a)} disabled={full || busy}
                    title={full ? t("wrFull", { m: maxSources }) : undefined}>{t("wrAdd")}</button>
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section className="card wr-article">
          <div className="card-head">
            <h2>{t("wrResult")}</h2>
            {draft && (
              <>
                <button type="button" className="ghost" onClick={() => copy("body")}><Copy className="ico" aria-hidden="true" /> {copied === "body" ? t("copied") : t("wrCopyText")}</button>
                <a className="btn-ghost" href={docxURL} download="article.docx"><FileText className="ico" aria-hidden="true" /> {t("wrDocx")}</a>
              </>
            )}
          </div>

          {!draft && !busy && (
            <EmptyState icon={FileText} title={t("writerEmptyTitle")} description={t("writerEmpty", { max: maxSources })} />
          )}
          {busy && (
            <div className="wr-running" aria-live="polite">
              <p className="hi-title">{t("writerRunning")}</p>
              <div className="bar"><div style={{ width: `${Math.round((job?.progress || 0) * 100)}%` }} /></div>
              <p className="meta">{Math.round((job?.progress || 0) * 100)}%</p>
            </div>
          )}
          {draft && (
            <>
              <p className="note wr-note"><Info className="ico" aria-hidden="true" /> {t("wrMarkersNote")}</p>
              {violations.length > 0 && (
                <div className="nc-checks">
                  <b>{t("writerUnverified", { n: violations.length })}</b>
                  <ul>{violations.map((v, i) => <li key={i}><code>{v.text}</code> {v.detail}</li>)}</ul>
                </div>
              )}
              <h3 className="wr-h">{draft.title}</h3>
              {[draft.lead, ...draft.body].filter(Boolean).map((p, i) => (
                <p key={i} className={"wr-p" + (i === 0 ? " lead" : "")}>
                  {splitSentences(p).map((sen, k) => {
                    const n = sourceOf(sen, draft.claims || []);
                    const src = n > 0 ? used[n - 1] : undefined;
                    return (
                      <span key={k}>
                        {k > 0 ? " " : ""}{sen}
                        {n > 0 && (src?.url
                          ? <a className="src-chip" href={src.url} target="_blank" rel="noreferrer" title={src.title}>{n}</a>
                          : <sup className="src-chip">{n}</sup>)}
                      </span>
                    );
                  })}
                </p>
              ))}
              {used.length > 0 && (
                <div className="wr-foot">
                  <p className="wr-foot-h">{t("writerSources")}</p>
                  {used.map((s, i) => (
                    <p key={i}><b>{i + 1}.</b> {s.title}{s.media ? `, ${s.media}` : ""}</p>
                  ))}
                </div>
              )}
            </>
          )}
        </section>
      </div>

      <div className="wr-tech">
        <details className="card tech">
          <summary>{t("techNotes")}</summary>
          <LogPanel logs={logs} />
        </details>
      </div>
    </div>
  );
}

// Sama dengan writer.SplitSentences / SourceOf di engine (docx.go): kalimat
// dipecah di tanda akhir, dan kalimat diberi nomor sumber bila teks klaimnya
// sama atau saling memuat. Nomor 1-based; klaim menyimpan sumber 0-based.
function splitSentences(p: string): string[] {
  return (p.match(/[^.!?]+[.!?]+["”’)]*\s*|[^.!?]+$/g) || []).map((s) => s.trim()).filter(Boolean);
}
function sourceOf(sentence: string, claims: Claim[]): number {
  const norm = (s: string) => s.trim().replace(/^[.!?"”’ ]+|[.!?"”’ ]+$/g, "").toLowerCase();
  const s = norm(sentence);
  if (!s) return 0;
  const c = claims.find((c) => { const ct = norm(c.text); return ct && (ct === s || s.includes(ct) || ct.includes(s)); });
  return c ? c.source + 1 : 0;
}

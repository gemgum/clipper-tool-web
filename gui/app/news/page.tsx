"use client";

// Halaman News cards — DESIGN-NEWSCARD.md (9 Oktober 2026). Tiga layar, satu
// alur: Pilih artikel → Susun kartu → Simpan & bagikan. Logika yang sudah
// terbukti (daftar berita, mengambil artikel, pratinjau otomatis, simpan,
// isian tersimpan) dipakai apa adanya; yang berubah susunan layarnya.
//
// Keputusan pemilik di dokumen itu: AI BOLEH menulis ringkasan kartu dan
// caption, dijaga pagar fakta engine (writer.CheckText — angka, kutipan, nama
// yang tidak ada di artikel ditandai). notes/13 diperbarui.
//
// Ikon: lucide-react (ISC) — alasannya di gui/app/page.tsx.
import { ChevronLeft, ChevronRight, Copy, Download, ImagePlus, Plus, Sparkles } from "lucide-react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import PageHeader from "../page-header";
import { useI18n } from "../i18n";
import { eng, engineURL } from "../engine";
import { useKeep, useRestore } from "../persist";
import Stepper from "../stepper";
import Select from "../select";
import Alerts from "../alerts";
import EmptyState from "../empty-state";
import { useAI } from "../ai";
import { Segmented } from "../clip-steps";
import { Newspaper, LayoutGrid } from "lucide-react";

// Harus sama dengan card.FontSteps / HeaderMax / CardTopMax di engine.
const FONT_STEPS = 10;
const HEADER_MAX = 400;
const CARD_TOP_MAX = 400;
// Berapa artikel diminta tiap "Muat lebih banyak".
// Kelipatan 5: satu baris kartu berisi lima artikel.
const PAGE = 25;
// Ambang penghitung karakter (DESIGN-NEWSCARD §5.2).
const IDEAL = 140;
const LIMIT = 180;

type Article = {
  title: string; summary: string; url: string; image: string; images?: string[];
  source: string; domain: string; date: string; published: string;
};
type Feed = { id: string; name: string; url: string; topic: string };
type Config = { feeds: Feed[]; has_browser: boolean; browser: string };
type Violation = { kind: string; text: string; detail: string };
type Saved = { id: string; file: string; zip: string; width: number; height: number; bytes: number };
type CardItem = { id: string; made: string; bytes: number; file: string; zip: string; title?: string; source?: string };

const EMPTY: Article = { title: "", summary: "", url: "", image: "", images: [], source: "", domain: "", date: "", published: "" };

// Preset tampilan (§5.2 + empat tambahan pemilik 9 Oktober 2026) — warnanya ditetapkan engine (card.Request.theme);
// di sini hanya contoh warnanya untuk kotak pilihan.
const THEMES = [
  { id: "dark", key: "ncThemeDark", swatch: "#15181D", style: "dark" },
  { id: "light", key: "ncThemeLight", swatch: "#FFFFFF", style: "light" },
  { id: "photo", key: "ncThemePhoto", swatch: "#3C2A1E", style: "dark" },
  { id: "paper", key: "ncThemePaper", swatch: "#F2EBDD", style: "light" },
  { id: "navy", key: "ncThemeNavy", swatch: "#0F2747", style: "dark" },
  { id: "red", key: "ncThemeRed", swatch: "#7F1714", style: "dark" },
  { id: "forest", key: "ncThemeForest", swatch: "#163A2D", style: "dark" },
  { id: "plum", key: "ncThemePlum", swatch: "#2E1F4A", style: "dark" },
] as const;
const RATIOS = [
  { id: "9:16", key: "ncRatioStory", w: 1080, h: 1920 },
  { id: "4:5", key: "ncRatioFeed", w: 1080, h: 1350 },
  { id: "1:1", key: "ncRatioSquare", w: 1080, h: 1080 },
] as const;

export default function News() {
  const { lang, t } = useI18n();
  const router = useRouter();
  const [screen, setScreen] = useState<"pick" | "compose" | "done">("pick");
  const [config, setConfig] = useState<Config | null>(null);
  const [error, setError] = useState("");

  // Layar 1: dua jalan masuk.
  const [link, setLink] = useState("");
  const [linkError, setLinkError] = useState("");
  const [typed, setTyped] = useState("");
  const [query, setQuery] = useState("");
  const [limit, setLimit] = useState(PAGE);
  const [more, setMore] = useState(true);
  const [items, setItems] = useState<Article[]>([]);
  // true: daftar pertama langsung dimuat; tanpa ini sempat tampil "Nothing found" sebelum kerangka.
  const [listBusy, setListBusy] = useState(true);
  const [sourceFilter, setSourceFilter] = useState("");
  const [picked, setPicked] = useState("");      // url artikel yang sedang dipilih
  const [fetching, setFetching] = useState(false);

  // Layar 2: bahan kartu.
  const [article, setArticle] = useState<Article>(EMPTY);
  const [theme, setTheme] = useState("dark");
  const [ratio, setRatio] = useState("9:16");
  const [align, setAlign] = useState("left");
  const [caption, setCaption] = useState("");
  const [hashtags, setHashtags] = useState("");
  // Mesin AI dari Pengaturan (global atau pengecualian alat "news").
  const ai = useAI("news");
  const engine = ai?.engine || "ollama";
  const model = ai?.model || "";
  const [writing, setWriting] = useState<"" | "summary" | "caption">("");
  const [checks, setChecks] = useState<{ summary: Violation[]; caption: Violation[] }>({ summary: [], caption: [] });
  const [writeError, setWriteError] = useState("");
  const [uploading, setUploading] = useState(false);
  // Laci "atur sendiri".
  const [zoom, setZoom] = useState(1);
  const [photoFit, setPhotoFit] = useState("cover");
  const [photoFill, setPhotoFill] = useState("blur");
  const [titleStep, setTitleStep] = useState(0);
  const [paragraphStep, setParagraphStep] = useState(0);
  const [header, setHeader] = useState(0);
  const [cardTop, setCardTop] = useState(0);

  const [preview, setPreview] = useState("");
  const [previewBusy, setPreviewBusy] = useState(false);
  // Layar 3.
  const [saved, setSaved] = useState<Saved | null>(null);
  const [saving, setSaving] = useState(false);
  const [renderError, setRenderError] = useState("");
  const [copied, setCopied] = useState(false);
  const [cards, setCards] = useState<CardItem[] | null>(null);

  useEffect(() => {
    fetch(eng(`/api/news/feeds`)).then((r) => r.json()).then(setConfig)
      .catch(() => setError(t("engineUnreachable", { url: engineURL() })));
  }, [t]);

  // ---------- Layar 1 ----------
  const loadList = useCallback(async (search: string, max: number) => {
    setListBusy(true);
    try {
      const param = search ? `q=${encodeURIComponent(search)}` : `feed=all`;
      const res = await fetch(eng(`/api/news/list?${param}&max=${max}&lang=${lang}`));
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || t("errLoadNews"));
      setItems(data);
      setMore(Array.isArray(data) && data.length >= max);
    } catch (e: any) {
      setItems([]); setMore(false); setError(e.message);
    } finally { setListBusy(false); }
  }, [lang, t]);
  useEffect(() => { loadList(query, limit); }, [query, limit, loadList]);

  const runSearch = () => { setLimit(PAGE); setMore(true); setSourceFilter(""); setQuery(typed.trim()); };

  // Artikel baru = semua yang ditulis untuk artikel lama dibuang.
  const useArticle = useCallback((a: Article) => {
    setArticle({ ...EMPTY, ...a, images: a.images?.length ? a.images : (a.image ? [a.image] : []) });
    setCaption(""); setHashtags(""); setChecks({ summary: [], caption: [] }); setWriteError("");
    setPreview(""); setSaved(null); setZoom(1);
  }, []);

  const readArticle = useCallback(async (url: string, fallback?: Article) => {
    const res = await fetch(eng(`/api/news/article`), {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url, lang }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || t("errReadArticle"));
    // Yang sudah diketahui dari daftar tidak dibuang bila halamannya lebih miskin.
    return { ...data, image: data.image || fallback?.image || "", source: data.source || fallback?.source || "",
      date: data.date || fallback?.date || "", published: data.published || fallback?.published || "" } as Article;
  }, [lang, t]);

  // Klik pertama MEMILIH (artikel dibaca di belakang), klik kedua lanjut.
  const pickItem = useCallback(async (a: Article) => {
    if (picked === a.url && article.url) { setScreen("compose"); return; }
    setPicked(a.url); useArticle(a); setFetching(true); setError("");
    try {
      const full = await readArticle(a.url, a);
      useArticle(full);
      if (full.url && full.url !== a.url) {
        setItems((list) => list.map((x) => (x.url === a.url ? { ...x, url: full.url } : x)));
        setPicked(full.url);
      }
    } catch (e: any) { setError(e.message); }
    finally { setFetching(false); }
  }, [picked, article.url, useArticle, readArticle]);

  const fetchLink = useCallback(async () => {
    if (!link.trim()) return;
    setFetching(true); setLinkError("");
    try {
      const full = await readArticle(link.trim());
      useArticle(full); setPicked(full.url); setScreen("compose");
    } catch (e: any) { setLinkError(e.message); }
    finally { setFetching(false); }
  }, [link, readArticle, useArticle]);

  // ---------- Layar 2 ----------
  const write = useCallback(async (kind: "summary" | "caption") => {
    if (!article.url) return;
    setWriting(kind); setWriteError("");
    try {
      const res = await fetch(eng(`/api/news/write`), {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: article.url, engine, model, lang, kind }),
      });
      const d = await res.json();
      if (!res.ok) throw new Error(d.error || "write failed");
      if (kind === "summary") setArticle((a) => ({ ...a, summary: d.text }));
      else { setCaption(d.text); setHashtags((d.hashtags || []).map((h: string) => "#" + h.replace(/^#/, "")).join(" ")); }
      setChecks((c) => ({ ...c, [kind]: d.violations || [] }));
    } catch (e: any) {
      // Ringkasan gagal: textarea tetap berisi teks artikel (§9), plus catatan kecil.
      setWriteError(t("ncAiFailed", { error: e.message }));
    } finally { setWriting(""); }
  }, [article.url, engine, model, lang, t]);

  // "Otomatis dulu" (§2): begitu masuk layar 2, caption ditulis bila masih kosong.
  useEffect(() => {
    if (screen === "compose" && article.url && !caption && !writing) write("caption");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen, article.url]);

  const uploadImage = useCallback(async (f: File) => {
    setUploading(true); setError("");
    try {
      const form = new FormData(); form.append("file", f);
      const res = await fetch(eng(`/api/news/image`), { method: "POST", body: form });
      const d = await res.json();
      if (!res.ok) throw new Error(d.error || "upload failed");
      setArticle((a) => ({ ...a, image: d.path, images: [...(a.images || []), d.path] }));
    } catch (e: any) { setError(e.message); }
    finally { setUploading(false); }
  }, []);

  const themeDef = THEMES.find((x) => x.id === theme) || THEMES[0];
  const cardBody = useCallback((r: string, previewOnly: boolean) => JSON.stringify({
    article, theme, style: themeDef.style, ratio: r, align, lang, preview: previewOnly, caption,
    hashtags: hashtags.split(/\s+/).map((h) => h.replace(/^#/, "")).filter(Boolean),
    photo: { offset_x: 0, offset_y: 0, zoom, fit: photoFit, fill: photoFill },
    fonts: { title: titleStep, paragraph: paragraphStep }, header, card_top: cardTop,
  }), [article, theme, themeDef.style, align, lang, caption, hashtags, zoom, photoFit, photoFill, titleStep, paragraphStep, header, cardTop]);

  const fingerprint = cardBody(ratio, true);
  useEffect(() => {
    if (screen !== "compose" || !article.title || !config?.has_browser) return;
    const id = setTimeout(async () => {
      setPreviewBusy(true);
      try {
        const res = await fetch(eng(`/api/card`), { method: "POST", headers: { "Content-Type": "application/json" }, body: fingerprint });
        const d = await res.json();
        if (!res.ok) throw new Error(d.error || t("errBuildCard"));
        setPreview(eng(`${d.file}?v=${Date.now()}`));
      } catch (e: any) { setError(e.message); }
      finally { setPreviewBusy(false); }
    }, 700);
    return () => clearTimeout(id);
  }, [fingerprint, screen, config?.has_browser]); // eslint-disable-line react-hooks/exhaustive-deps

  const save = useCallback(async () => {
    setSaving(true); setRenderError("");
    try {
      const res = await fetch(eng(`/api/card`), { method: "POST", headers: { "Content-Type": "application/json" }, body: cardBody(ratio, false) });
      const d = await res.json();
      if (!res.ok) throw new Error(d.error || t("errBuildCard"));
      let bytes = 0;
      try { bytes = Number((await fetch(eng(d.file), { method: "HEAD" })).headers.get("content-length")) || 0; } catch {}
      setSaved({ id: d.id, file: eng(d.file), zip: eng(d.zip), width: d.width, height: d.height, bytes });
      setCards(null); setScreen("done");
    } catch (e: any) { setRenderError(e.message); setScreen("done"); }
    finally { setSaving(false); }
  }, [cardBody, ratio, t]);

  // ---------- Layar 3 ----------
  useEffect(() => {
    if (screen !== "done" || cards) return;
    fetch(eng(`/api/cards`)).then((r) => r.json()).then((d) => setCards(Array.isArray(d) ? d : [])).catch(() => setCards([]));
  }, [screen, cards]);

  const otherRatio = ratio === "4:5" ? "9:16" : "4:5";
  const renderAlt = useCallback(async () => {
    try {
      const res = await fetch(eng(`/api/card`), { method: "POST", headers: { "Content-Type": "application/json" }, body: cardBody(otherRatio, false) });
      const d = await res.json();
      if (!res.ok) throw new Error(d.error || t("errBuildCard"));
      setCards(null);
      const a = document.createElement("a"); a.href = eng(d.file); a.download = `card-${otherRatio.replace(":", "x")}.png`; a.click();
    } catch (e: any) { setError(e.message); }
  }, [cardBody, otherRatio, t]);

  // Caption yang tersalin PERSIS seperti kotak pratinjaunya (§6): teks, hashtag,
  // lalu baris kredit — tanpa em dash.
  const fullCaption = [caption.trim(), hashtags.trim(),
    t("ncSourceLine", { source: article.source || article.domain, date: article.date ? `, ${article.date}` : "" })]
    .filter(Boolean).join("\n\n");
  const copyCaption = async () => {
    try { await navigator.clipboard.writeText(fullCaption); setCopied(true); setTimeout(() => setCopied(false), 1800); } catch {}
  };

  const newCard = () => { setScreen("pick"); setPicked(""); useArticle(EMPTY); setRenderError(""); };

  // ---------- isian tersimpan ----------
  useKeep("news", { article, caption, hashtags, theme, ratio, align, zoom, photoFit, photoFill, titleStep, paragraphStep, header, cardTop });
  useRestore<Record<string, unknown>>("news", (v) => {
    const set = <T,>(fn: (x: T) => void, val: unknown) => { if (val !== undefined && val !== null) fn(val as T); };
    set(setArticle, v.article); set(setCaption, v.caption); set(setHashtags, v.hashtags);
    if (typeof v.theme === "string" && THEMES.some((x) => x.id === v.theme)) setTheme(v.theme);
    set(setRatio, v.ratio); set(setAlign, v.align); set(setZoom, v.zoom);
    set(setPhotoFit, v.photoFit); set(setPhotoFill, v.photoFill);
    set(setTitleStep, v.titleStep); set(setParagraphStep, v.paragraphStep); set(setHeader, v.header); set(setCardTop, v.cardTop);
  });

  // ---------- turunan ----------
  const ago = useMemo(() => {
    const rtf = new Intl.RelativeTimeFormat(lang === "id" ? "id" : "en", { numeric: "auto" });
    return (iso: string) => {
      const ms = Date.parse(iso);
      if (!iso || Number.isNaN(ms)) return "";
      const min = Math.round((ms - Date.now()) / 60000);
      if (Math.abs(min) < 60) return rtf.format(min, "minute");
      const h = Math.round(min / 60);
      if (Math.abs(h) < 24) return rtf.format(h, "hour");
      return rtf.format(Math.round(h / 24), "day");
    };
  }, [lang]);
  const sources = useMemo(() => [...new Set(items.map((a) => a.source).filter(Boolean))].slice(0, 6), [items]);
  const shown = sourceFilter ? items.filter((a) => a.source === sourceFilter) : items;
  const n = article.summary.length;
  const band = n <= IDEAL ? "ok" : n <= LIMIT ? "dense" : "long";
  const ratioDef = RATIOS.find((r) => r.id === ratio) || RATIOS[0];
  const sizeText = saved?.bytes ? `, ${saved.bytes > 1e6 ? (saved.bytes / 1e6).toFixed(1) + " MB" : Math.round(saved.bytes / 1e3) + " KB"}` : "";

  const headerProps = screen === "pick"
    ? { title: t("tabNews"), subtitle: t("ncSub"), actions: (
        <>
          <button type="button" className="ghost" onClick={() => router.push("/history?f=cards")}><LayoutGrid className="ico" aria-hidden="true" /> {t("ncMyCards")}</button>
        </>) }
    : screen === "compose"
    ? { title: article.title || t("tabNews"), subtitle: [article.source || article.domain, article.date].filter(Boolean).join(" · "), actions: (
        <>
          <button type="button" className="ghost" onClick={() => setScreen("pick")}><ChevronLeft className="ico" aria-hidden="true" /> {t("ncChangeArticle")}</button>
          <button type="button" className="primary big" onClick={save} disabled={saving || !article.title || !config?.has_browser}>
            {saving ? t("rendering") : t("ncSaveContinue")} <ChevronRight className="ico" aria-hidden="true" />
          </button>
        </>) }
    : { title: t("tabNews"), subtitle: t("ncReady"), actions: (
        <>
          <button type="button" className="ghost" onClick={() => setScreen("compose")}>{t("ncEditCard")}</button>
          <button type="button" className="primary big" onClick={newCard}><Plus className="ico" aria-hidden="true" /> {t("ncNewCard")}</button>
        </>) };

  return (
    <div className="screen scroll clips-v2 news-v2">
      <PageHeader title={headerProps.title} subtitle={headerProps.subtitle}>{headerProps.actions}</PageHeader>
      <Alerts items={[
        error && { kind: "error" as const, text: error },
        config && !config.has_browser && { kind: "warn" as const, key: "no-browser", text: `${t("browserMissing")} CLIPPER_CHROME ${t("browserMissingTail")}` },
      ]} />

      {screen === "pick" && (
        <div className="nc-pick">
          <div>
            <h2 className="nc-h">{t("ncPickTitle")}</h2>
            <p className="nc-lead">{t("ncPickLead")}</p>
          </div>

          <section className="card nc-search">
            <div className="nc-field grow">
              <label htmlFor="nc-q">{t("ncSearchLabel")}</label>
              <input id="nc-q" type="search" value={typed} onChange={(e) => setTyped(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") runSearch(); }} placeholder={t("searchPlaceholder")} />
            </div>
            <button type="button" className="dark" onClick={runSearch} disabled={listBusy}>{t("ncSearchBtn")}</button>
            <span className="nc-or">{t("ncOr")}</span>
            <div className="nc-field">
              <label htmlFor="nc-link">{t("ncPasteLabel")}</label>
              <input id="nc-link" type="url" value={link} placeholder="https://…" onChange={(e) => setLink(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") fetchLink(); }} />
            </div>
            <button type="button" className="ghost" onClick={fetchLink} disabled={!link.trim() || fetching}>{fetching ? t("fetching") : t("ncFetchBtn")}</button>
            {linkError && (
              <div className="nc-link-error" role="alert">
                <p>{linkError}</p>
                <button type="button" className="ghost" onClick={() => { useArticle({ ...EMPTY, url: link.trim() }); setScreen("compose"); }}>{t("ncFillManually")}</button>
              </div>
            )}
          </section>

          <div className="nc-chips" role="group" aria-label={t("ncSources")}>
            <span className="meta">{t("ncSources")}</span>
            {["", ...sources].map((s) => (
              <button key={s || "all"} type="button" className={"chip-btn" + (sourceFilter === s ? " on" : "")}
                aria-pressed={sourceFilter === s} onClick={() => setSourceFilter(s)}>{s || t("ncAll")}</button>
            ))}
            <span className="grow" />
            <span className="meta">{t("ncCount", { n: shown.length })}</span>
          </div>

          {listBusy && items.length === 0 ? (
            <div className="nc-grid">{Array.from({ length: 10 }, (_, i) => <div key={i} className="nc-article skel" aria-hidden="true"><div className="nc-thumb" /><div className="nc-body"><span /><span /></div></div>)}</div>
          ) : shown.length === 0 ? (
            <section className="card"><EmptyState icon={Newspaper} title={t("ncNoResults", { q: query || "…" })} description={t("ncNoResultsHint")} /></section>
          ) : (
            <div className="nc-grid">
              {shown.map((a) => {
                const on = picked === a.url;
                return (
                  <article key={a.url} className={"nc-article" + (on ? " on" : "")}>
                    <div className="nc-thumb">{a.image && /* eslint-disable-next-line @next/next/no-img-element */ <img src={a.image} alt="" loading="lazy" />}</div>
                    <div className="nc-body">
                      <div className="nc-meta">
                        {a.source && <span className="nc-badge">{a.source}</span>}
                        <span className="meta">{ago(a.published) || a.date}</span>
                      </div>
                      <h3>{a.title}</h3>
                      <button type="button" className={on ? "primary" : "ghost"} onClick={() => pickItem(a)} disabled={on && fetching}>
                        {on ? (fetching ? t("ncReading") : t("ncContinue")) : t("ncPickArticle")}
                      </button>
                    </div>
                  </article>
                );
              })}
            </div>
          )}
          {more && items.length > 0 && !sourceFilter && (
            <button type="button" className="ghost nc-more" onClick={() => setLimit((v) => v + PAGE)} disabled={listBusy}>{listBusy ? t("loadingNews") : t("ncMore")}</button>
          )}
          <p className="meta nc-foot">{t("ncCredit")}</p>
        </div>
      )}

      {screen === "compose" && (
        <div className="nc-compose">
          <div className="nc-preview-col">
            <Segmented label={t("ratio")} value={ratio} onChange={setRatio}
              options={RATIOS.map((r) => ({ value: r.id, name: t(r.key) }))} />
            <div className="nc-card" style={{ aspectRatio: ratio.replace(":", " / ") }}>
              {preview ? /* eslint-disable-next-line @next/next/no-img-element */ <img src={preview} alt="" /> : <span className="meta">{t("ncPreviewWait")}</span>}
              {previewBusy && <span className="nc-busy">{t("rendering")}</span>}
              {band === "long" && <span className="nc-warn">{t("ncTooLongOnCard")}</span>}
            </div>
            <p className="meta">{t("ncActualSize", { w: ratioDef.w, h: ratioDef.h })}</p>
          </div>

          <div className="nc-panels">
            <section className="card">
              <div className="card-head">
                <h2>{t("ncCardText")}</h2>
                <button type="button" className="ai-btn" onClick={() => write("summary")} disabled={!!writing || !article.url}>
                  <Sparkles className="ico" aria-hidden="true" /> {writing === "summary" ? t("analyzing") : t("ncSummarise")}
                </button>
              </div>
              <label className="step-label" htmlFor="nc-sum">{t("ncSummaryLabel")}</label>
              <textarea id="nc-sum" rows={3} value={article.summary} onChange={(e) => { setArticle((a) => ({ ...a, summary: e.target.value })); setChecks((c) => ({ ...c, summary: [] })); }} />
              <div className="nc-count">
                <span className={"count-" + band}>{t(band === "ok" ? "ncCharsOk" : band === "dense" ? "ncCharsDense" : "ncCharsLong", { n })}</span>
                <span className="meta">{t("ncSafeLimit")}</span>
              </div>
              <Checks list={checks.summary} />
              {writeError && <p className="nc-note-err">{writeError}</p>}
            </section>

            <section className="card">
              <h2>{t("ncPhoto")}</h2>
              <p className="step-hint flush">{(article.images?.length ? t("ncPhotoHint") : t("ncPhotoNone"))}</p>
              <div className="nc-photos">
                {(article.images || []).map((src, i) => (
                  <button key={src} type="button" className={"photo-btn" + (article.image === src ? " on" : "")}
                    aria-pressed={article.image === src} aria-label={t("ncPhotoN", { n: i + 1 })}
                    onClick={() => setArticle((a) => ({ ...a, image: src }))}>
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img alt="" src={src.startsWith("http") ? src : eng(`/api/image?path=${encodeURIComponent(src)}`)} />
                  </button>
                ))}
                <label className="photo-btn upload">
                  <input type="file" accept="image/png,image/jpeg,image/webp" hidden disabled={uploading}
                    onChange={(e) => { const f = e.target.files?.[0]; if (f) uploadImage(f); e.target.value = ""; }} />
                  <ImagePlus className="ico" aria-hidden="true" />
                  <span>{uploading ? "…" : t("ncUpload")}</span>
                </label>
              </div>
            </section>

            <section className="card">
              <h2>{t("ncLook")}</h2>
              <div className="theme-grid" role="group" aria-label={t("ncLook")}>
                {THEMES.map((x) => (
                  <button key={x.id} type="button" className={"theme-tile" + (theme === x.id ? " on" : "")}
                    aria-pressed={theme === x.id} onClick={() => setTheme(x.id)}>
                    <span className="theme-swatch" style={{ background: x.swatch }} />
                    <span>{t(x.key)}</span>
                  </button>
                ))}
              </div>
              <p className="step-label nc-gap">{t("ncAlign")}</p>
              <div className="nc-align">
                <Segmented label={t("ncAlign")} value={align} onChange={setAlign}
                  options={[{ value: "left", name: t("ncAlignLeft") }, { value: "center", name: t("ncAlignCenter") },
                    { value: "right", name: t("ncAlignRight") }, { value: "justify", name: t("ncAlignJustify") }]} />
              </div>
            </section>

            <section className="card">
              <div className="card-head">
                <h2>{t("ncCaptionTitle")}</h2>
                <button type="button" className="ai-btn" onClick={() => write("caption")} disabled={!!writing || !article.url}>
                  <Sparkles className="ico" aria-hidden="true" /> {writing === "caption" ? t("analyzing") : t("ncWriteForMe")}
                </button>
              </div>
              <label className="step-label" htmlFor="nc-cap">{t("ncCaptionLabel")}</label>
              <textarea id="nc-cap" rows={3} value={caption} onChange={(e) => { setCaption(e.target.value); setChecks((c) => ({ ...c, caption: [] })); }} />
              <Checks list={checks.caption} />
              <label className="step-label nc-gap" htmlFor="nc-tags">{t("ncHashtags")}</label>
              <input id="nc-tags" value={hashtags} onChange={(e) => setHashtags(e.target.value)} placeholder="#..." />
              <p className="note">{t("ncCreditNote", { source: article.source || article.domain || "…" })}</p>
            </section>

            <details className="card adv">
              <summary>{t("ncAdvanced")}</summary>
              <p className="meta adv-hint">{t("ncAdvancedHint")}</p>
              <div className="grid3">
                <div className="field"><label>{t("articleTitle")}</label><input value={article.title} onChange={(e) => setArticle((a) => ({ ...a, title: e.target.value }))} /></div>
                <div className="field"><label>{t("sourceBadge")}</label><input value={article.source} onChange={(e) => setArticle((a) => ({ ...a, source: e.target.value }))} /></div>
                <div className="field"><label>{t("date")}</label><input value={article.date} onChange={(e) => setArticle((a) => ({ ...a, date: e.target.value }))} /></div>
              </div>
              <div className="field"><label>{t("imageURL")}</label><input value={article.image} onChange={(e) => setArticle((a) => ({ ...a, image: e.target.value }))} /></div>
              <div className="grid4">
                <div className="field"><label>{t("fontTitle")}</label><Stepper value={titleStep} onChange={setTitleStep} min={-FONT_STEPS} max={FONT_STEPS} /></div>
                <div className="field"><label>{t("fontParagraph")}</label><Stepper value={paragraphStep} onChange={setParagraphStep} min={-FONT_STEPS} max={FONT_STEPS} /></div>
                <div className="field"><label>{t("headerSpace")}</label><Stepper value={header} onChange={setHeader} min={0} max={HEADER_MAX} step={10} suffix="px" /></div>
                <div className="field"><label>{t("cardDown")}</label><Stepper value={cardTop} onChange={setCardTop} min={0} max={CARD_TOP_MAX} step={10} suffix="px" /></div>
              </div>
              <div className="grid3">
                <div className="field"><label>{t("photoFitLabel")}</label>
                  <Select value={photoFit} onChange={(v) => { setPhotoFit(v); setZoom(1); }} options={[
                    { value: "cover", label: t("photoFitCover") }, { value: "whole", label: t("photoFitWhole") }]} /></div>
                <div className="field"><label>{t("photoFill")}</label>
                  <Select value={photoFill} onChange={setPhotoFill} disabled={photoFit !== "whole"} options={[
                    { value: "blur", label: t("photoFillBlur") }, { value: "solid", label: t("photoFillSolid") }]} /></div>
                <div className="field"><label>{t("photoZoom")}</label>
                  <Stepper value={Math.round(zoom * 100)} onChange={(v) => setZoom(v / 100)} min={100} max={400} step={5} suffix="%" /></div>
              </div>
              <button type="button" className="ghost" onClick={() => { setTitleStep(0); setParagraphStep(0); setHeader(0); setCardTop(0); setZoom(1); }}
                disabled={titleStep === 0 && paragraphStep === 0 && header === 0 && cardTop === 0 && zoom === 1}>{t("fontReset")}</button>
            </details>
          </div>
        </div>
      )}

      {screen === "done" && (
        <div className="nc-done">
          <section className="nc-final">
            <div className="nc-final-card" style={{ aspectRatio: ratio.replace(":", " / ") }}>
              {/* eslint-disable-next-line @next/next/no-img-element */}
              {saved && <img src={saved.file} alt="" />}
            </div>
            <div className="nc-final-side">
              {renderError ? (
                <div className="banner warn" role="alert">
                  <p>{t("ncRenderFailed", { error: renderError })}</p>
                  <button type="button" className="primary" onClick={save} disabled={saving}>{t("ncRetryRender")}</button>
                </div>
              ) : saved && (
                <div className="banner good" role="status">
                  <span className="banner-ico" aria-hidden="true"><Download className="ico" /></span>
                  <p><b>{t("ncSaved")}</b> {t("ncSavedMeta", { w: saved.width, h: saved.height, size: sizeText })}</p>
                </div>
              )}
              {saved && (
                <div className="nc-actions">
                  <a className="btn-dark" href={saved.file} download={`card-${ratio.replace(":", "x")}.png`}><Download className="ico" aria-hidden="true" /> {t("ncDownloadPng")}</a>
                  <button type="button" className="ghost" onClick={copyCaption}><Copy className="ico" aria-hidden="true" /> {copied ? t("ncCopied") : t("ncCopyCaption")}</button>
                  <button type="button" className="ghost" onClick={renderAlt}>{t(otherRatio === "4:5" ? "ncDownload45" : "ncDownload916")}</button>
                </div>
              )}
              <section className="card">
                <h2>{t("ncCaptionPreview")}</h2>
                <p className="nc-caption">{fullCaption}</p>
                <p className="meta">{t("ncCaptionPreviewNote")}</p>
              </section>
            </div>
          </section>

          <section>
            <div className="result-bar">
              <h2>{t("ncGalleryTitle")}</h2>
              {cards && cards.length > 0 && <span className="meta">{t("ncGalleryCount", { n: cards.length })}</span>}
            </div>
            {cards && cards.length === 0 ? (
              <section className="card"><EmptyState icon={Newspaper} title={t("ncGalleryEmpty")} description={t("ncGalleryEmptyHint")}
                action={<button type="button" className="primary" onClick={newCard}>{t("ncFirstCard")}</button>} /></section>
            ) : (
              <div className="nc-gallery">
                {(cards || []).map((c) => (
                  <article key={c.id} className="nc-mini">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img src={eng(c.file)} alt="" loading="lazy" />
                    <div className="nc-mini-body">
                      <span className="meta">{[c.source, ago(c.made)].filter(Boolean).join(" · ")}</span>
                      <a className="btn-ghost" href={eng(c.file)} download={`${c.id}.png`}>{t("downloadClip")}</a>
                    </div>
                  </article>
                ))}
              </div>
            )}
          </section>
        </div>
      )}
    </div>
  );
}

// Hasil pagar fakta untuk teks tulisan AI: yang tidak ditemukan di artikel.
function Checks({ list }: { list: Violation[] }) {
  const { t } = useI18n();
  if (!list.length) return null;
  return (
    <div className="nc-checks" role="status">
      <b>{t("ncAiCheck")}</b>
      <ul>{list.map((v, i) => <li key={i}><code>{v.text}</code> {v.detail}</li>)}</ul>
    </div>
  );
}

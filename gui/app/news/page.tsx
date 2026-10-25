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
import { copyText } from "../copy-link";
import { Check, ChevronLeft, Crop as CropIcon, ChevronRight, Copy, Download, Eye, ImageIcon, ImagePlus, Plus, X, RectangleVertical, Sparkles, Stamp, Type, Bookmark } from "lucide-react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import PageHeader from "../page-header";
import { useI18n } from "../i18n";
import { eng, engineURL, post, useWeb } from "../engine";
import { useKeep, useRestore, useSticky } from "../persist";
import Stepper from "../stepper";
import Alerts from "../alerts";
import EmptyState from "../empty-state";
import { useAI } from "../ai";
import { AlignPicker, Segmented, Toggle, type Align } from "../clip-steps";
import TabCard, { Group, Pad, Row, Split } from "../tab-card";
import Stage, { Shot, StageBar, StageFrame } from "../stage";
import Picker from "../picker";
import Guides from "../guides";
import CropDialog, { type Crop } from "../crop";
import Select from "../select";
import { CENTER_X } from "../drag";
import ColorPanel, { isDark, toHex } from "../color-panel";
import { outlineColor, watermarkToAPI } from "../watermark-model";
import type { Format } from "../watermark-model";
import { HL_FONT, MarkFont, MarkLayers, MarkLogoGroup, MarkTemplates, SIZES, useMark, useTemplates, watermarkColors } from "../mark";
import { Newspaper, LayoutGrid } from "lucide-react";
import ThemePicker, { THEMES } from "../themes";

// Harus sama dengan card.FontSteps / HeaderMax / CardTopMax di engine.
const FONT_STEPS = 10;
const CARD_TOP_MAX = 400;
// Berapa artikel diminta tiap "Muat lebih banyak".
// Kelipatan 5: satu baris kartu berisi lima artikel.
const PAGE = 25;
// Ambang penghitung karakter (DESIGN-NEWSCARD §5.2).
// tidy = satu paragraf per blok, dipisah SATU baris kosong; spasi berlebih dibuang.
const tidy = (s: string) => s.split(/\n+/).map((p) => p.replace(/\s+/g, " ").trim()).filter(Boolean).join("\n\n");
const IDEAL = 140;
const LIMIT = 180;

type Article = {
  title: string; summary: string; url: string; image: string; images?: string[];
  source: string; domain: string; date: string; published: string;
  // body = paragraf asli badan artikel untuk carousel (/api/news/body), diambil sekali.
  body?: string[];
};
type Feed = { id: string; name: string; url: string; topic: string };
type Config = { feeds: Feed[]; has_browser: boolean; browser: string };
type Violation = { kind: string; text: string; detail: string };
// Geometry = card.Geometry: letak area foto & batas geser, piksel kanvas.
type Geometry = { width: number; height: number; photo_top: number; photo_h: number; photo_base: number; scale: number; offset_w: number; offset_h: number; pages: number;
  ill?: { index: number; x: number; y: number; w: number; h: number; base_x: number; base_y: number } };
// Ill = foto lanjutan carousel: gambar di zona atas kartu ke-`page` (2 = kartu
// kedua), digeser dx/dy dari tempat bawaannya (piksel kanvas).
type Ill = { image: string; page: number; size: "s" | "m" | "l"; dx: number; dy: number; w?: number; h?: number; crop?: Crop | null };
type Saved = { id: string; file: string; zip: string; width: number; height: number; bytes: number; pages?: number };
type CardItem = { id: string; made: string; bytes: number; file: string; zip: string; title?: string; source?: string };

const EMPTY: Article = { title: "", summary: "", url: "", image: "", images: [], source: "", domain: "", date: "", published: "" };

// Rasio kartu ↔ format bingkai Mark (ukurannya sama; card.FormatOf di engine).
const FORMAT_OF: Record<string, Format> = { "9:16": "short", "4:5": "feed45", "1:1": "feed11" };
const RATIOS = [
  { id: "9:16", key: "ncRatioStory", w: 1080, h: 1920 },
  { id: "4:5", key: "ncRatioFeed", w: 1080, h: 1350 },
  { id: "1:1", key: "ncRatioSquare", w: 1080, h: 1080 },
] as const;

export default function News() {
  const { lang, t } = useI18n();
  const router = useRouter();
  // Layar & tab diingat browser: kembali ke halaman ini = posisi yang sama.
  const [screen, setScreen] = useSticky<"pick" | "compose" | "done">("news.screen", "pick");
  const [tab, setTab] = useSticky<"format" | "photo" | "headline" | "mark" | "template">("news.tab", "headline");
  // Tab tersimpan dari versi lama ("caption") jatuh ke Text, tempat caption sekarang.
  const openTab = (["format", "photo", "headline", "mark", "template"] as const).includes(tab) ? tab : "headline";
  // Kendali Format yang tampil di tab Headline (pemilih seperti Watermark).
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
  const [align, setAlign] = useState<Align>("left");
  const [titleAlign, setTitleAlign] = useState<Align>("left");
  const [caption, setCaption] = useState("");
  const [hashtags, setHashtags] = useState("");
  // Mesin AI dari Pengaturan (global atau pengecualian alat "news").
  const ai = useAI("news");
  const engine = ai?.engine || "ollama";
  const model = ai?.model || "";
  const [writing, setWriting] = useState<"" | "summary" | "caption" | "story">("");
  const [checks, setChecks] = useState<{ summary: Violation[]; caption: Violation[]; story: Violation[] }>({ summary: [], caption: [], story: [] });
  // story = isi halaman lanjutan carousel yang ditulis AI (bisa disunting);
  // kosong = paragraf asli artikel.
  const [story, setStory] = useState("");
  // useSummary = ringkasan dipakai di kartu pertama (centang di tab Text).
  const [useSummary, setUseSummary] = useState(true);
  // ills = foto lanjutan carousel; selIll = yang sedang dipilih (daftar & pratinjau).
  const [ills, setIlls] = useState<Ill[]>([]);
  const [selIll, setSelIll] = useState(-1);
  // cropIll = foto lanjutan yang sedang di-crop (jendela CropDialog), -1 = tidak ada.
  const [cropIll, setCropIll] = useState(-1);
  const illSrc = (image: string) => (image.startsWith("http") ? image : eng(`/api/image?path=${encodeURIComponent(image)}`));
  // boxPhoto = gaya box: foto artikel di belakang kartu lanjutan; mati = tampilan kartu berwarna kotak.
  const [boxPhoto, setBoxPhoto] = useState(true);
  const setIll = (i: number, patch: Partial<Ill>) => setIlls((l) => l.map((x, j) => (j === i ? { ...x, ...patch } : x)));
  // Foto baru = halaman berikutnya setelah yang terakhir dipakai (berurutan).
  const addIll = (image: string) => {
    setIlls((l) => [...l, { image, page: Math.max(1, ...l.map((x) => x.page)) + 1, size: "m", dx: 0, dy: 0 }]);
    setSelIll(ills.length);
  };
  const [writeError, setWriteError] = useState("");
  const [uploading, setUploading] = useState(false);

  // ---------- Mark (logo, template, judul Mark) — bersama halaman Mark ----------
  // Gaya kartu: "card" = kartu News (Chrome) + logo & headline tambahan Mark;
  // "box" = foto artikel penuh + kotak judul Mark berisi judul & kredit artikel.
  const [layout, setLayout] = useState<"card" | "box">("card");
  // Kisi pratinjau (sama dengan halaman Mark): seretan logo, kotak judul, dan
  // batas foto menempel ke kisi ini; diingat browser.
  const [grid, setGrid] = useSticky("news.grid", 40);
  const [gridAlways, setGridAlways] = useSticky("news.gridAlways", true);
  const m = useMark({ format: FORMAT_OF[ratio] || "short", first: article.image, grid });
  const tpl = useTemplates((e) => setError(e));
  const [activeTpl, setActiveTpl] = useSticky("news.template", "");
  // Tab Text dibagi dua (kiri teks kartu, kanan caption) supaya tidak memanjang.
  const [textTab, setTextTab] = useSticky<"card" | "caption">("news.textTab", "card");
  // Pemilih berkas lokal untuk logo ke-i (-1 = tambah); null = tertutup.
  const [logoPicker, setLogoPicker] = useState<number | null>(null);
  const web = useWeb();
  const credit = [article.source || article.domain, article.date].filter(Boolean).join(" · ");
  // Gaya box: kotak judul Mark diisi judul & kredit artikel — saat gaya dipilih
  // dan saat artikelnya berganti. Selebihnya diatur bebas di panel Mark (tab
  // Mark), sama persis dengan halaman Mark.
  // Gaya card: Mark hanya membawa logo — judulnya milik kartu News, jadi teks
  // Mark dikosongkan (termasuk sisa simpanan lama).
  useEffect(() => {
    if (layout === "box") m.setWatermark({ hlText: article.title, hlCredit: credit });
    else if (m.watermark.hlText || m.watermark.hlBox) m.setWatermark({ hlText: "", hlBox: false });
  }, [layout, article.title, credit, m.watermark.hlText, m.watermark.hlBox]); // eslint-disable-line react-hooks/exhaustive-deps
  const changeLayout = (v: "card" | "box") => {
    if (v === "box") m.setWatermark({ hlBox: true, hlStyle: "box" });
    setLayout(v);
  };
  // Template: bentuknya sama dengan template halaman Mark; format ↔ rasio kartu.
  const applyTemplate = (v: Record<string, unknown>) => {
    const r = Object.keys(FORMAT_OF).find((k) => FORMAT_OF[k] === v.format);
    if (r) setRatio(r);
    m.applyMark(v, m.watermark.hlText);
  };
  // Template bawaan terpasang di setiap artikel baru (dibaca lewat ref supaya
  // useArticle tetap stabil).
  const applyDefault = useRef(() => {});
  applyDefault.current = () => {
    if (tpl.defaultTemplate) { applyTemplate(tpl.defaultTemplate.data); setActiveTpl(tpl.defaultTemplate.name); }
  };
  // Laci "atur sendiri".
  const [photoFit, setPhotoFit] = useState("cover");
  const [photoFill, setPhotoFill] = useState("blur");
  const [titleStep, setTitleStep] = useState(0);
  const [paragraphStep, setParagraphStep] = useState(0);
  const [cardTop, setCardTop] = useState(0);

  // Pratinjau seketika (HTML kartu + letak bagian yang bisa diseret) dan hasil
  // render sungguhan dari Chrome (tombol Preview result).
  const [live, setLive] = useState<{ html: string; geometry: Geometry } | null>(null);
  // Carousel seluruh artikel: kartu pertama + halaman lanjutan berisi badan
  // artikel. page = halaman yang sedang dipratinjau (0 = kartu pertama).
  const [whole, setWhole] = useState(false);
  const [page, setPage] = useState(0);
  const [shot, setShot] = useState("");
  const [shooting, setShooting] = useState(false);
  // Geseran batas foto–isi (card.Request.CardY), diseret di pratinjau.
  const [cardY, setCardY] = useState(0);
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
    setCaption(""); setHashtags(""); setStory(""); setIlls([]); setSelIll(-1); setChecks({ summary: [], caption: [], story: [] }); setWriteError("");
    setShot(""); setSaved(null);
    if (a.title) applyDefault.current();
  }, []);

  const readArticle = useCallback(async (url: string, fallback?: Article) => {
    const data = await post(`/api/news/article`, { url, lang }, t("errReadArticle"));
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
  const write = useCallback(async (kind: "summary" | "caption" | "story") => {
    if (!article.url) return;
    setWriting(kind); setWriteError("");
    try {
      // Isi carousel SELALU ditulis ulang dari teks ASLI artikel yang tersimpan
      // (article.body) — bukan dari isi kotak, yang sesudah sekali tulis sudah
      // berupa hasil AI. Artikelnya tidak diunduh lagi.
      const text = kind === "story" ? (article.body?.length ? article.body.join("\n") : story) : "";
      const d = await post(`/api/news/write`, { url: article.url, engine, model, lang, kind, text, title: article.title, source: article.source || article.domain }, "write failed");
      if (kind === "summary") setArticle((a) => ({ ...a, summary: d.text }));
      else if (kind === "story") setStory(tidy(d.text));
      else { setCaption(d.text); setHashtags((d.hashtags || []).map((h: string) => "#" + h.replace(/^#/, "")).join(" ")); }
      setChecks((c) => ({ ...c, [kind]: d.violations || [] }));
    } catch (e: any) {
      // Ringkasan gagal: textarea tetap berisi teks artikel (§9), plus catatan kecil.
      setWriteError(t("ncAiFailed", { error: e.message }));
    } finally { setWriting(""); }
  }, [article.url, article.title, article.source, article.domain, article.body, story, engine, model, lang, t]);

  // "Otomatis dulu" (§2): begitu masuk layar 2, caption ditulis bila masih kosong.
  useEffect(() => {
    if (screen === "compose" && article.url && !caption && !writing) write("caption");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [screen, article.url]);

  const uploadImage = useCallback(async (f: File, asIll = false) => {
    setUploading(true); setError("");
    try {
      const form = new FormData(); form.append("file", f);
      const res = await fetch(eng(`/api/news/image`), { method: "POST", body: form });
      const d = await res.json();
      if (!res.ok) throw new Error(d.error || "upload failed");
      if (asIll) addIll(d.path);
      else setArticle((a) => ({ ...a, image: d.path, images: [...(a.images || []), d.path] }));
    } catch (e: any) { setError(e.message); }
    finally { setUploading(false); }
  }, [ills.length]); // eslint-disable-line react-hooks/exhaustive-deps

  const themeDef = THEMES.find((x) => x.id === theme) || THEMES[0];
  const cardBody = useCallback((r: string, previewOnly: boolean) => JSON.stringify({
    article: useSummary ? article : { ...article, summary: "" }, theme, style: /^#/.test(theme) ? (isDark(theme) ? "dark" : "light") : themeDef.style, ratio: r, align, title_align: titleAlign, lang, preview: previewOnly, caption,
    hashtags: hashtags.split(/\s+/).map((h) => h.replace(/^#/, "")).filter(Boolean),
    photo: { fit: photoFit, fill: photoFill },
    fonts: { title: titleStep, paragraph: paragraphStep }, header: 0, card_top: cardTop, card_y: cardY,
    whole, body: !whole ? [] : story.trim() ? story.split(/\n+/).map((p) => p.trim()).filter(Boolean) : article.body || [],
    // Gaya box: halaman lanjutan memakai rupa kotak judul Mark (warna lewat toHex,
    // sama dengan pratinjau Mark). Gaya card: panel tema seperti kartu pertama.
    illustrations: whole ? ills.filter((x) => x.image) : [],
    box: whole && layout === "box" ? {
      no_photo: !boxPhoto,
      fill: toHex(m.watermark.hlBoxColor || "white"), text: toHex(m.watermark.hlColor),
      outline: m.watermark.hlOutline, outline_color: toHex(outlineColor(m.watermark)),
      sharp: !!m.watermark.hlSharp,
    } : null,
  }), [article, theme, themeDef.style, align, titleAlign, lang, caption, hashtags, photoFit, photoFill, titleStep, paragraphStep, cardTop, cardY, whole, story, useSummary, ills, boxPhoto,
    layout, m.watermark.hlBoxColor, m.watermark.hlColor, m.watermark.hlOutline, m.watermark.hlOutlineColor, m.watermark.hlSharp]);
  // Yang benar-benar dirender (simpan & Preview result): kartu + gaya + Mark,
  // dibakar engine Mark yang sama dengan halaman Mark.
  const renderBody = (r: string, previewOnly: boolean) => JSON.stringify({
    ...JSON.parse(cardBody(r, previewOnly)), layout, font: HL_FONT,
    mark: watermarkToAPI({ ...m.eff, format: FORMAT_OF[r] || "short" }, HL_FONT),
  });

  // Pratinjau seketika: tanpa Chrome, jadi cukup jeda pendek (juga saat diseret).
  const fingerprint = cardBody(ratio, true);
  useEffect(() => {
    if (screen !== "compose" || !article.title) return;
    const id = setTimeout(() => {
      post<{ html: string; geometry: Geometry }>(`/api/card/html`, { ...JSON.parse(fingerprint), page }, t("errBuildCard")).then((d) => {
        setLive(d);
        // Halaman yang tidak ada lagi (badan dipecah ulang) → kembali ke halaman terakhir.
        if (page >= d.geometry.pages) setPage(Math.max(0, d.geometry.pages - 1));
      }).catch((e) => setError(e.message));
    }, 80);
    return () => clearTimeout(id);
  }, [fingerprint, screen, page]); // eslint-disable-line react-hooks/exhaustive-deps
  // Badan artikel diambil SEKALI saat carousel dinyalakan, lalu ikut tersimpan.
  useEffect(() => {
    if (!whole || !article.url || article.body) return;
    post<{ paragraphs: string[] }>(`/api/news/body`, { url: article.url, lang }, t("errReadArticle"))
      .then((d) => setArticle((a) => (a.url === article.url ? { ...a, body: d.paragraphs } : a))).catch((e) => setError(e.message));
  }, [whole, article.url, article.body]); // eslint-disable-line react-hooks/exhaustive-deps
  // Begitu badan artikel ada, kotak Carousel text berisi paragraf ASLI-nya;
  // tombol AI lalu menceritakan ulang teks itu.
  useEffect(() => {
    if (article.body?.length && !story) setStory(tidy(article.body.join("\n")));
  }, [article.body]); // eslint-disable-line react-hooks/exhaustive-deps
  const pages = live?.geometry.pages || 1;
  const previewResult = async () => {
    setShooting(true); setError("");
    try {
      const d = await post(`/api/card`, renderBody(ratio, true), t("errBuildCard"));
      setShot(eng(`${d.file}?page=${page + 1}&v=${Date.now()}`));
    } catch (e: any) { setError(e.message); }
    setShooting(false);
  };

  // Seret di pratinjau (delta, piksel layar → piksel kanvas): foto = geser
  // OffsetX/Y dalam batas engine (offset_w/h); isi = Header (ruang 1920).
  // Skala dari lebar bingkai SEBENARNYA (.nc-card dibatasi tinggi layar juga).
  const [cardW, setCardW] = useState(0);
  // Bingkai yang sama juga induk lapisan Mark (m.boxRef: seret logo & judul).
  const cardRef = useCallback((el: HTMLDivElement | null) => {
    m.boxRef.current = el;
    if (!el) return;
    new ResizeObserver(() => setCardW(el.clientWidth)).observe(el);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const frameH = Math.round((cardW * m.fh) / m.fw);
  const photoSrc = !article.image ? "" : article.image.startsWith("http") ? article.image : eng(`/api/image?path=${encodeURIComponent(article.image)}`);
  const geo = live?.geometry;
  const k = geo && cardW ? cardW / geo.width : 1;
  // Seret isi = batas foto–isi (CardY) lewat fungsi seret yang SAMA dengan Mark
  // (m.dragProps: kisi, magnet di posisi bawaan); jepit 20–85% = card.render.
  const edgeDrag = geo ? m.dragProps(m.fw / 2, geo.photo_top + geo.photo_h, (_x, y) => {
    const photo = Math.min(Math.max(Math.round(y) - geo.photo_top, Math.trunc(geo.height / 5)), Math.trunc((geo.height * 85) / 100));
    setCardY(photo - geo.photo_base);
  }, geo.photo_top + geo.photo_base, m.fh) : {};
  const standard = titleStep === 0 && paragraphStep === 0 && cardTop === 0 && cardY === 0;
  const resetLook = () => { setTitleStep(0); setParagraphStep(0); setCardTop(0); setCardY(0); };

  const save = useCallback(async () => {
    setSaving(true); setRenderError("");
    try {
      const d = await post(`/api/card`, renderBody(ratio, false), t("errBuildCard"));
      let bytes = 0;
      try { bytes = Number((await fetch(eng(d.file), { method: "HEAD" })).headers.get("content-length")) || 0; } catch {}
      setSaved({ id: d.id, file: eng(d.file), zip: eng(d.zip), width: d.width, height: d.height, bytes, pages: d.pages });
      setCards(null); setScreen("done");
    } catch (e: any) { setRenderError(e.message); setScreen("done"); }
    finally { setSaving(false); }
  }, [renderBody, ratio, t]); // eslint-disable-line react-hooks/exhaustive-deps

  // ---------- Layar 3 ----------
  useEffect(() => {
    if (screen !== "done" || cards) return;
    fetch(eng(`/api/cards`)).then((r) => r.json()).then((d) => setCards(Array.isArray(d) ? d : [])).catch(() => setCards([]));
  }, [screen, cards]);

  const otherRatio = ratio === "4:5" ? "9:16" : "4:5";
  const renderAlt = useCallback(async () => {
    try {
      const d = await post(`/api/card`, renderBody(otherRatio, false), t("errBuildCard"));
      setCards(null);
      const a = document.createElement("a"); a.href = eng(d.file); a.download = `card-${otherRatio.replace(":", "x")}.png`; a.click();
    } catch (e: any) { setError(e.message); }
  }, [renderBody, otherRatio, t]); // eslint-disable-line react-hooks/exhaustive-deps

  // Caption yang tersalin PERSIS seperti kotak pratinjaunya (§6): teks, hashtag,
  // lalu baris kredit — tanpa em dash.
  const fullCaption = [caption.trim(), hashtags.trim(),
    t("ncSourceLine", { source: article.source || article.domain, date: article.date ? `, ${article.date}` : "" })]
    .filter(Boolean).join("\n\n");
  const copyCaption = async () => {
    try { await copyText(fullCaption); setCopied(true); setTimeout(() => setCopied(false), 1800); } catch {}
  };

  const newCard = () => { setScreen("pick"); setPicked(""); useArticle(EMPTY); setRenderError(""); };

  // ---------- isian tersimpan ----------
  useKeep("news", { article, caption, hashtags, story, useSummary, ills, boxPhoto, theme, ratio, align, titleAlign, photoFit, photoFill, titleStep, paragraphStep, cardTop, cardY, saved, whole,
    layout, mark: { watermark: m.watermark, logos: m.logos, logoOn: m.logoOn } });
  useRestore<Record<string, unknown>>("news", (v) => {
    const set = <T,>(fn: (x: T) => void, val: unknown) => { if (val !== undefined && val !== null) fn(val as T); };
    set(setArticle, v.article); set(setCaption, v.caption); set(setHashtags, v.hashtags); set(setStory, v.story); set(setUseSummary, v.useSummary); set(setIlls, v.ills); set(setBoxPhoto, v.boxPhoto);
    if (typeof v.theme === "string" && (v.theme === "auto" || /^#[0-9a-f]{6}$/i.test(v.theme) || THEMES.some((x) => x.id === v.theme))) setTheme(v.theme);
    set(setRatio, v.ratio); set(setAlign, v.align); set(setTitleAlign, v.titleAlign); set(setCardY, v.cardY); set(setSaved, v.saved); set(setWhole, v.whole);
    if (v.layout === "card" || v.layout === "box") setLayout(v.layout);
    if (v.mark && typeof v.mark === "object") m.applyMark(v.mark as Record<string, unknown>);
    set(setPhotoFit, v.photoFit); set(setPhotoFill, v.photoFill);
    set(setTitleStep, v.titleStep); set(setParagraphStep, v.paragraphStep); set(setCardTop, v.cardTop);
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
          <button type="button" className="primary big" onClick={save} disabled={saving || !article.title || (layout === "card" && !config?.has_browser)}>
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
      <MarkFont />
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
        // Tata letak baku (halaman klip): kiri = kartu setelan bertab, kanan =
        // kolom pratinjau yang menempel (stage.tsx).
        <div className="setup-grid">
          <div className="setup-main">
            {/* Bentuk acuan Watermark: Format | Photo | Headline | Caption. */}
            {/* Satu kendali untuk satu fungsi: Format (bingkai) | Photo | Text (teks
                + SATU Format + SATU Theme) | Mark (logo, headline tambahan, template) | Caption. */}
            <TabCard label={t("tabNews")} open={openTab} setOpen={setTab} tabs={[
              { key: "format", icon: RectangleVertical, label: t("wmFormatTitle") },
              { key: "photo", icon: ImageIcon, label: t("ncPhoto") },
              { key: "headline", icon: Type, label: t("ncTabText") },
              { key: "mark", icon: Stamp, label: t("ncTabMark") },
              { key: "template", icon: Bookmark, label: t("wmTplTitle") },
            ]}>
              {openTab === "format" && (
                // Sejajar: kiri = bingkai, gaya & cara foto dipasang; kanan = posisi
                // kartu (milik kartu News: mati di gaya box). Batas foto–isi diseret
                // langsung di pratinjau.
                <Group title={t("wmFormatTitle")}>
                  <Split sideOff={layout === "box"} side={<>
                    <span className="mc-r-l">{t("cardDown")}</span>
                    <Stepper value={cardTop} onChange={setCardTop} min={0} max={CARD_TOP_MAX} step={10} suffix="px" />
                    <span className="mc-note">{t("ncEdgeDrag")}</span>
                    <button type="button" className="ghost" onClick={resetLook} disabled={standard}>{t("fontReset")}</button>
                  </>}>
                    <Row label={t("wmStyle")} note={t(layout === "box" ? "ncStyleBoxNote" : "ncStyleCardNote")}>
                      <Segmented label={t("wmStyle")} value={layout} onChange={changeLayout}
                        options={[{ value: "card" as const, name: t("wmStyleLook") }, { value: "box" as const, name: t("wmStyleBox") }]} />
                    </Row>
                    {/* Carousel: kartu pertama + badan artikel di halaman lanjutan. */}
                    <Row label={t("ncWhole")} note={t("ncWholeNote")} off={!article.url}>
                      <Toggle checked={whole} onChange={(on) => { setWhole(on); setPage(0); }} label={t("ncWholeOn")} />
                    </Row>
                    {/* Gaya box di kartu lanjutan: foto artikel di belakang, atau
                        tampilan kartu (panel penuh) dengan warna kotak. */}
                    <Row label={t("ncBg")} off={layout !== "box" || !whole}>
                      <Segmented label={t("ncBg")} value={boxPhoto ? "photo" : "color"} onChange={(v) => setBoxPhoto(v === "photo")}
                        options={[{ value: "photo", name: t("ncBgPhoto") }, { value: "color", name: t("ncBgColor") }]} />
                    </Row>
                    {/* Sudut kotak judul: hanya ada di gaya box. */}
                    <Row label={t("wmCorner")} off={layout !== "box"}>
                      <Segmented label={t("wmCorner")} value={m.watermark.hlSharp ? "sharp" : "round"} onChange={(v) => m.setWatermark({ hlSharp: v === "sharp" })}
                        options={[{ value: "round", name: t("wmCornerRound") }, { value: "sharp", name: t("wmCornerSharp") }]} />
                    </Row>
                    <Row label={t("photoFitLabel")} off={layout === "box"}>
                      <Segmented label={t("photoFitLabel")} value={photoFit} onChange={setPhotoFit}
                        options={[{ value: "cover", name: t("photoFitCover") }, { value: "whole", name: t("photoFitWhole") }]} />
                    </Row>
                    {/* Selalu ada; hanya berarti saat seluruh foto dimuat. */}
                    <Row label={t("photoFill")} off={photoFit !== "whole" || layout === "box"}>
                      <Segmented label={t("photoFill")} value={photoFill} onChange={setPhotoFill}
                        options={[{ value: "blur", name: t("photoFillBlur") }, { value: "solid", name: t("photoFillSolid") }]} />
                    </Row>
                  </Split>
                </Group>
              )}

              {openTab === "photo" && (
                // Pilihan foto (pilih, hapus, unggah) lalu alamatnya.
                <Group title={t("ncPhoto")}>
                      <Pad>
                        <span className="mc-note">{article.images?.length ? t("ncPhotoHint") : t("ncPhotoNone")}</span>
                        <div className="nc-photos">
                          {(article.images || []).map((src, i) => (
                            <div key={src} className="photo-item">
                              <button type="button" className={"photo-btn" + (article.image === src ? " on" : "")}
                                aria-pressed={article.image === src} aria-label={t("ncPhotoN", { n: i + 1 })}
                                onClick={() => setArticle((a) => ({ ...a, image: src }))}>
                                {/* eslint-disable-next-line @next/next/no-img-element */}
                                <img alt="" src={src.startsWith("http") ? src : eng(`/api/image?path=${encodeURIComponent(src)}`)} />
                              </button>
                              {/* Pakai juga sebagai foto lanjutan carousel (kartu 2 dst.). */}
                              <button type="button" className="ghost tiny icon-only photo-add" aria-label={t("ncIllUse")} title={t("ncIllUse")}
                                disabled={!whole} onClick={() => addIll(src)}><Plus className="ico" aria-hidden="true" /></button>
                              {/* Hapus dari pilihan; foto terpilih yang dihapus diganti foto pertama yang tersisa. */}
                              <button type="button" className="ghost tiny icon-only photo-del" aria-label={t("ncPhotoRemove")} title={t("ncPhotoRemove")}
                                onClick={() => setArticle((a) => {
                                  const images = (a.images || []).filter((x) => x !== src);
                                  return { ...a, images, image: a.image === src ? images[0] || "" : a.image };
                                })}><X className="ico" aria-hidden="true" /></button>
                            </div>
                          ))}
                          <label className="photo-btn upload">
                            <input type="file" accept="image/png,image/jpeg,image/webp" hidden disabled={uploading}
                              onChange={(e) => { const f = e.target.files?.[0]; if (f) uploadImage(f); e.target.value = ""; }} />
                            <ImagePlus className="ico" aria-hidden="true" />
                            <span>{uploading ? "…" : t("ncUpload")}</span>
                          </label>
                        </div>
                      </Pad>
                      <Row label={t("imageURL")}><input value={article.image} aria-label={t("imageURL")} onChange={(e) => setArticle((a) => ({ ...a, image: e.target.value }))} /></Row>
                </Group>
              )}
              {openTab === "photo" && (
                // Foto lanjutan carousel: di zona atas kartu 2 dst., teks di bawahnya.
                // Berurutan (foto ke-1 = kartu 2), halamannya bisa diganti per foto;
                // posisinya diseret di pratinjau (menempel ke kisi).
                <Group title={t("ncIllTitle")}>
                  <Pad>
                    <fieldset className="cp-plain nc-use-body" disabled={!whole}>
                      <span className="mc-note">{whole ? t("ncIllNote") : t("ncIllOff")}</span>
                      {ills.length > 0 ? (
                        <ul className="item-list nc-ills">
                          {ills.map((x, i) => (
                            <li key={i} className={i === selIll ? "on" : ""} onClick={() => setSelIll(i)}>
                              {/* eslint-disable-next-line @next/next/no-img-element */}
                              <img className="nc-ill-thumb" alt="" src={illSrc(x.image)} />
                              <div className="nc-ill-page">
                                <Select value={String(x.page)} ariaLabel={t("ncIllPage")} onChange={(v) => { setIll(i, { page: Number(v) }); setPage(Number(v) - 1); }}
                                  options={Array.from({ length: Math.max(pages, x.page) - 1 }, (_, k) => ({ value: String(k + 2), label: t("ncIllPageN", { n: k + 2 }) }))} />
                              </div>
                              <Segmented label={t("wmTextSize")} value={x.w ? ("" as "s") : x.size} onChange={(v) => setIll(i, { size: v, w: 0, h: 0 })}
                                options={[{ value: "s" as const, name: "S" }, { value: "m" as const, name: "M" }, { value: "l" as const, name: "L" }]} />
                              <button type="button" className={"ghost tiny icon-only" + (x.crop ? " active" : "")} aria-label={t("cropTitle")} title={t("cropTitle")}
                                onClick={(e) => { e.stopPropagation(); setCropIll(i); }}><CropIcon className="ico" aria-hidden="true" /></button>
                              <button type="button" className="ghost tiny icon-only" aria-label={t("ncPhotoRemove")} title={t("ncPhotoRemove")}
                                onClick={(e) => { e.stopPropagation(); setIlls((l) => l.filter((_, j) => j !== i)); setSelIll(-1); }}><X className="ico" aria-hidden="true" /></button>
                            </li>
                          ))}
                        </ul>
                      ) : <span className="meta">{t("ncIllNone")}</span>}
                      <label className="ghost nc-ill-upload">
                        <input type="file" accept="image/png,image/jpeg,image/webp" hidden disabled={uploading || !whole}
                          onChange={(e) => { const f = e.target.files?.[0]; if (f) uploadImage(f, true); e.target.value = ""; }} />
                        <ImagePlus className="ico" aria-hidden="true" /> {uploading ? "…" : t("ncIllUpload")}
                      </label>
                    </fieldset>
                  </Pad>
                </Group>
              )}

              {openTab === "headline" && <>
                {/* Dua bagian: teks kartu | caption postingan. */}
                <div className="nc-subtabs">
                  <Segmented label={t("ncTabText")} value={textTab} onChange={setTextTab}
                    options={[{ value: "card", name: t("ncTextCard") }, { value: "caption", name: t("ncTextCaption") }]} />
                </div>
                {textTab === "card" && <Group title={t("ncCardText")}>
                  <Row label={t("articleTitle")}>
                    <input value={article.title} aria-label={t("articleTitle")} onChange={(e) => setArticle((a) => ({ ...a, title: e.target.value }))} />
                  </Row>
                  <Row label={t("sourceBadge")}>
                    <input value={article.source} aria-label={t("sourceBadge")} onChange={(e) => setArticle((a) => ({ ...a, source: e.target.value }))} />
                  </Row>
                  <Row label={t("date")}>
                    <input value={article.date} aria-label={t("date")} onChange={(e) => setArticle((a) => ({ ...a, date: e.target.value }))} />
                  </Row>
                  <Pad>
                    {/* Ringkasan hanya ada di kartu News: mati di gaya box. */}
                    <fieldset className="cp-plain" disabled={layout === "box"}>
                    <div className="nc-pad-head">
                      {/* Centang = ringkasan dipakai di kartu. Mati: teksnya tetap
                          disimpan, kartu tampil tanpa ringkasan (judul membesar). */}
                      <label className={"nc-use" + (useSummary ? " on" : "")}>
                        <input type="checkbox" checked={useSummary} onChange={(e) => setUseSummary(e.target.checked)} />
                        <span className="nc-use-box" aria-hidden="true"><Check className="ico" /></span>
                        <span className="nc-use-text">{t("ncSummaryLabel")}</span>
                        <span className="nc-use-state">{t(useSummary ? "ncUseOn" : "ncUseOff")}</span>
                      </label>
                      <button type="button" className="ai-btn" onClick={() => write("summary")} disabled={!!writing || !article.url || !useSummary}>
                        <Sparkles className="ico" aria-hidden="true" /> {writing === "summary" ? t("analyzing") : t("ncSummarise")}
                      </button>
                    </div>
                    <fieldset className="cp-plain nc-use-body" disabled={!useSummary}>
                    <textarea id="nc-sum" rows={3} value={article.summary} aria-label={t("ncSummaryLabel")} onChange={(e) => { setArticle((a) => ({ ...a, summary: e.target.value })); setChecks((c) => ({ ...c, summary: [] })); }} />
                    <div className="nc-count">
                      <span className={"count-" + band}>{t(band === "ok" ? "ncCharsOk" : band === "dense" ? "ncCharsDense" : "ncCharsLong", { n })}</span>
                      <span className="meta">{t("ncSafeLimit")}</span>
                    </div>
                    <Checks list={checks.summary} />
                    </fieldset>
                    {writeError && <p className="nc-note-err">{writeError}</p>}
                    </fieldset>
                  </Pad>
                  {/* Isi carousel (kartu 2 dst.): mula-mula paragraf asli artikel, lalu
                      AI menceritakan ulang teks itu bergaya kreator. Mati selama
                      carousel (tab Format) belum dinyalakan. */}
                  <Pad>
                    <fieldset className="cp-plain" disabled={!whole}>
                    <div className="nc-pad-head">
                      <label className="mc-r-l" htmlFor="nc-story">{t("ncStoryLabel")}</label>
                      <button type="button" className="ai-btn" onClick={() => write("story")} disabled={!!writing || !story.trim()}>
                        <Sparkles className="ico" aria-hidden="true" /> {writing === "story" ? t("analyzing") : t("ncWriteForMe")}
                      </button>
                    </div>
                    {/* Tingginya mengikuti isi (field-sizing), dan dirapikan per
                        paragraf saat ditinggalkan. */}
                    <textarea id="nc-story" className="nc-story" rows={6} value={story} placeholder={t("ncStoryEmpty")}
                      onChange={(e) => { setStory(e.target.value); setChecks((c) => ({ ...c, story: [] })); }}
                      onBlur={() => setStory((v) => tidy(v))} />
                    <Checks list={checks.story} />
                    </fieldset>
                  </Pad>
                </Group>}
                {textTab === "caption" && <Group title={t("ncCaptionTitle")}>
                  <Pad>
                    <div className="nc-pad-head">
                      <label className="mc-r-l" htmlFor="nc-cap">{t("ncCaptionLabel")}</label>
                      <button type="button" className="ai-btn" onClick={() => write("caption")} disabled={!!writing || !article.url}>
                        <Sparkles className="ico" aria-hidden="true" /> {writing === "caption" ? t("analyzing") : t("ncWriteForMe")}
                      </button>
                    </div>
                    <textarea id="nc-cap" rows={4} value={caption} onChange={(e) => { setCaption(e.target.value); setChecks((c) => ({ ...c, caption: [] })); }} />
                    <Checks list={checks.caption} />
                  </Pad>
                  <Row label={t("ncHashtags")} note={t("ncCreditNote", { source: article.source || article.domain || "…" })}>
                    <input id="nc-tags" value={hashtags} aria-label={t("ncHashtags")} onChange={(e) => setHashtags(e.target.value)} placeholder="#..." />
                  </Row>
                </Group>}
              </>}

              {openTab === "mark" && <>
                <MarkLogoGroup m={m} web={web} setError={setError} openPicker={setLogoPicker} />
                {/* Panel judul. Kiri = teks di kartu, dibedakan Headline & Summary
                    (gaya box: hanya Headline, kotak judulnya); kanan = kotak judul
                    Mark (garis tepi, warna) — hanya berlaku di gaya box; sudutnya di tab Format.
                    Teks diisi di tab Text, gaya dipilih di tab Format. */}
                {(() => {
                  const box = layout === "box";
                  const steps = { s: -3, m: 0, l: 3 } as const;
                  const sizeOf = (step: number) => (step <= -2 ? "s" : step >= 2 ? "l" : "m");
                  const sml = <T extends string | number>(value: T, onChange: (v: T) => void, vals: [T, T, T]) => (
                    <Segmented label={t("wmTextSize")} value={value} onChange={onChange}
                      options={[{ value: vals[0], name: t("wmSmall") }, { value: vals[1], name: t("wmMedium") }, { value: vals[2], name: t("wmLarge") }]} />
                  );
                  return (
                    <Group title={t("wmTabHeadline")}>
                      <Split sideOff={!box} side={<>
                        {/* Tebal garis tepi ada di sasaran Outline panel warna; tongkat
                            ajaib = warna otomatis dari foto artikel (Text & Box). */}
                        <ColorPanel targets={watermarkColors(m.watermark, m.setWatermark, t, true, article.image ? m.autoColor : undefined)} />
                      </>}>
                        <Row label={t("wmTheme")} off={box}>
                          {/* Auto: engine mencari warna yang menyambung dasar foto artikel
                              (card.AutoLook — sama dengan Auto di halaman Mark). */}
                          <ThemePicker label={t("wmTheme")} value={theme} onChange={setTheme}
                            auto={{ onClick: () => setTheme("auto"), disabled: !article.image }} />
                        </Row>
                        {/* Satu ukuran & satu perataan untuk SEMUA teks kartu (keputusan
                            pemilik 25 Oktober 2026): judul, ringkasan, dan teks halaman
                            lanjutan carousel; gaya box ikut kotak judul Mark. */}
                        <Row label={t("wmTextSize")}>
                          {sml(box ? (["s", "m", "l"] as const)[SIZES.indexOf(m.watermark.hlSize)] ?? "m" : sizeOf(titleStep), (v) => {
                            setTitleStep(steps[v]); setParagraphStep(steps[v]);
                            if (box) m.setWatermark({ hlSize: SIZES[["s", "m", "l"].indexOf(v)] });
                          }, ["s", "m", "l"])}
                        </Row>
                        <Row label={t("wmAlign")}>
                          <AlignPicker label={t("wmAlign")} value={box ? m.watermark.hlAlign || "left" : titleAlign} onChange={(v) => {
                            setTitleAlign(v); setAlign(v);
                            if (box) m.setWatermark({ hlAlign: v });
                          }} />
                        </Row>
                      </Split>
                    </Group>
                  );
                })()}
              </>}

              {openTab === "template" && (
                <MarkTemplates tpl={tpl} active={activeTpl} onApply={(v, name) => { applyTemplate(v); setActiveTpl(name); }}
                  data={() => ({ format: FORMAT_OF[ratio] || "short", logoOn: m.logoOn, logos: m.logos, watermark: { ...m.watermark, hlText: "" } })} />
              )}
            </TabCard>
          </div>

          {/* Pratinjau seketika seperti Watermark: halaman kartu dari engine
              (/api/card/html, template yang SAMA dengan PNG) di dalam iframe,
              ditambah lapis seret batas foto–isi. "Preview result" = PNG dari Chrome. */}
          <Stage title={t("previewTitle")} badge={`${ratioDef.w} × ${ratioDef.h}`} help={t("ncDragHelp")}
            action={
              <button type="button" className="wm-preview-btn" disabled={!article.title || shooting || (layout === "card" && !config?.has_browser)} onClick={previewResult}>
                <Eye className="ico" aria-hidden="true" /> {shooting ? t("wmPreviewing") : t("wmPreviewResult")}
              </button>
            }>
              <StageBar grid={{ grid, setGrid, always: gridAlways, setAlways: setGridAlways }}
                ratio={{ label: t("ratio"), value: ratio, onChange: setRatio, options: RATIOS.map((r) => ({ value: r.id, label: r.id })) }}
                lead={pages > 1 && (
                  <div className="nc-pager" role="group" aria-label={t("ncPages")}>
                    <button type="button" className="ghost tiny icon-only" aria-label={t("ncPagePrev")} disabled={page === 0} onClick={() => setPage(page - 1)}>
                      <ChevronLeft className="ico" aria-hidden="true" /></button>
                    <span>{page + 1} / {pages}</span>
                    <button type="button" className="ghost tiny icon-only" aria-label={t("ncPageNext")} disabled={page >= pages - 1} onClick={() => setPage(page + 1)}>
                      <ChevronRight className="ico" aria-hidden="true" /></button>
                  </div>
                )} />
            <StageFrame>
              <div className="nc-card nc-live" ref={cardRef} style={{ aspectRatio: ratio.replace(":", " / "), "--pvh": `${frameH}px` } as React.CSSProperties}>
                {layout === "box" && page === 0 ? (
                  // Gaya box: foto artikel memenuhi bingkai (potong tengah, sama dengan engine).
                  photoSrc ? /* eslint-disable-next-line @next/next/no-img-element */ <img className="nc-box-photo" src={photoSrc} alt="" draggable={false} />
                    : <span className="meta">{t("ncPhotoNone")}</span>
                ) : live && geo ? <>
                  <iframe title={t("previewTitle")} srcDoc={live.html} sandbox="" tabIndex={-1}
                    style={{ width: geo.width, height: geo.height, transform: `scale(${k})` }} />
                  {page === 0 && <div className="nc-drag can" {...edgeDrag} style={{ top: (geo.photo_top + geo.photo_h * 0.75) * k, bottom: 0 }} />}
                  {/* Foto lanjutan halaman ini: seret (menempel ke kisi), klik = opsi halaman & ukuran. */}
                  {page > 0 && geo.ill && ills[geo.ill.index] && (() => {
                    const g = geo.ill, x = ills[g.index];
                    // Ukuran & tempat bawaan dihitung di sini dari tarikan (w/h),
                    // supaya kotaknya ikut tetikus tanpa menunggu engine.
                    const w = x.w || g.w, h = x.h || g.h, baseX = x.w ? Math.round((geo.width - w) / 2) : g.base_x;
                    const left = baseX + x.dx, top = g.base_y + x.dy;
                    const drag = m.dragProps(left + w / 2, top + h / 2, (cx, cy) =>
                      setIll(g.index, { dx: Math.round(cx - w / 2 - baseX), dy: Math.round(cy - h / 2 - g.base_y) }),
                      g.base_y + h / 2, m.fh, baseX + w / 2);
                    // Sudut kanan bawah: tarik = ukuran kotak bebas (piksel kanvas).
                    const resize = (e: React.PointerEvent) => {
                      e.stopPropagation();
                      const sx = e.clientX, sy = e.clientY;
                      const el = e.currentTarget as HTMLElement;
                      el.setPointerCapture(e.pointerId);
                      const mv = (ev: PointerEvent) => setIll(g.index, {
                        w: Math.max(120, Math.round(w + ((ev.clientX - sx) * 2) / k)), h: Math.max(80, Math.round(h + (ev.clientY - sy) / k)),
                      });
                      const stop = () => { el.removeEventListener("pointermove", mv); el.removeEventListener("pointerup", stop); };
                      el.addEventListener("pointermove", mv); el.addEventListener("pointerup", stop);
                    };
                    return <>
                      <div className={"nc-ill-drag" + (selIll === g.index ? " on" : "")} {...drag}
                        onPointerDown={(e) => { setSelIll(g.index); drag.onPointerDown(e); }}
                        style={{ left: left * k, top: top * k, width: w * k, height: h * k }}>
                        {selIll === g.index && <span className="nc-ill-resize" onPointerDown={resize} title={t("ncIllResize")} />}
                      </div>
                      {selIll === g.index && (
                        <div className="nc-ill-bar">
                          <Select value={String(x.page)} ariaLabel={t("ncIllPage")} onChange={(v) => { setIll(g.index, { page: Number(v) }); setPage(Number(v) - 1); }}
                            options={Array.from({ length: Math.max(pages, x.page) - 1 }, (_, n) => ({ value: String(n + 2), label: t("ncIllPageN", { n: n + 2 }) }))} />
                          <Segmented label={t("wmTextSize")} value={x.w ? ("" as "s") : x.size} onChange={(v) => setIll(g.index, { size: v, w: 0, h: 0 })}
                            options={[{ value: "s" as const, name: "S" }, { value: "m" as const, name: "M" }, { value: "l" as const, name: "L" }]} />
                          <button type="button" className={"ghost tiny icon-only" + (x.crop ? " active" : "")} aria-label={t("cropTitle")} title={t("cropTitle")}
                            onClick={() => setCropIll(g.index)}><CropIcon className="ico" aria-hidden="true" /></button>
                        </div>
                      )}
                    </>;
                  })()}
                </> : <span className="meta">{t("ncPreviewWait")}</span>}
                {cardW > 0 && <Guides grid={grid} visible={gridAlways || m.dragAt !== null} playH={m.fh}
                  atX={(m.dragAt?.x ?? m.placed[m.sel]?.x) === CENTER_X} atY={(m.dragAt?.y ?? m.placed[m.sel]?.y) === m.fh / 2} dragAt={m.dragAt} />}
                {/* Logo & judul Mark di atas kartu — lapisan yang sama dengan halaman Mark. */}
                {/* Halaman lanjutan carousel: hanya logo — judul/kotak judul milik kartu pertama. */}
                {cardW > 0 && <MarkLayers m={page === 0 ? m : { ...m, nb: null, hlLines: [] }} frameH={frameH} />}
                {layout === "card" && useSummary && band === "long" && <span className="nc-warn">{t("ncTooLongOnCard")}</span>}
              </div>
            </StageFrame>
          </Stage>
        </div>
      )}

      {screen === "done" && (
        // Bentuk acuan (Watermark): kiri = status, caption, galeri; kanan =
        // kolom pratinjau dengan kartu jadi dan tombol unduhnya.
        <div className="setup-grid">
          <div className="setup-main">
            {renderError ? (
              <div className="banner warn" role="alert">
                <p>{t("ncRenderFailed", { error: renderError })}</p>
                <button type="button" className="primary" onClick={save} disabled={saving}>{t("ncRetryRender")}</button>
              </div>
            ) : saved && (
              <div className="banner good" role="status">
                <span className="banner-ico" aria-hidden="true"><Download className="ico" /></span>
                <p><b>{t("ncSaved")}</b> {t("ncSavedMeta", { w: saved.width, h: saved.height, size: sizeText })}
                  {(saved.pages || 1) > 1 && <> {t("ncSavedPages", { n: saved.pages! })}</>}</p>
              </div>
            )}
            <section className="card">
              <div className="card-head">
                <h2>{t("ncCaptionPreview")}</h2>
                <button type="button" className="ghost" onClick={copyCaption}><Copy className="ico" aria-hidden="true" /> {copied ? t("ncCopied") : t("ncCopyCaption")}</button>
              </div>
              <p className="nc-caption">{fullCaption}</p>
              <p className="meta">{t("ncCaptionPreviewNote")}</p>
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

          <Stage title={t("previewTitle")} badge={ratio} help={t("ncActualSize", { w: ratioDef.w, h: ratioDef.h })}
            tools={<>
              <a className={"btn-dark" + (saved ? "" : " off")} href={saved?.file} download={`card-${ratio.replace(":", "x")}.png`} aria-disabled={!saved}>
                <Download className="ico" aria-hidden="true" /> {t("ncDownloadPng")}</a>
              <button type="button" className="ghost" onClick={renderAlt} disabled={!saved}>{t(otherRatio === "4:5" ? "ncDownload45" : "ncDownload916")}</button>
            </>}>
            <StageFrame>
              <div className="nc-card" style={{ aspectRatio: ratio.replace(":", " / ") }}>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                {saved && <img src={saved.file} alt="" />}
              </div>
            </StageFrame>
          </Stage>
        </div>
      )}
      {cropIll >= 0 && ills[cropIll] && (
        <CropDialog src={illSrc(ills[cropIll].image)} crop={ills[cropIll].crop}
          onApply={(c) => { setIll(cropIll, { crop: c }); setCropIll(-1); }} onClose={() => setCropIll(-1)} />
      )}
      {shot && <Shot src={shot} label={t("wmPreviewResult")} onClose={() => setShot("")} />}
      {tpl.confirmEl}
      {logoPicker !== null && (
        <Picker mode="file" start={m.logos[logoPicker]?.image || m.logos[0]?.image}
          onPick={(p) => { m.putLogo(p, logoPicker); setLogoPicker(null); }} onClose={() => setLogoPicker(null)} />
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

"use client";

// Tab watermark: logo + judul untuk video yang SUDAH jadi (DESIGN-Clipper-Lanjutan §4).
//
// Halaman klip memotong lalu membakar identitas; halaman ini hanya membakar.
// Koordinat mentah diganti tarikan langsung: logo dan judul diseret di
// pratinjau, sembilan titik jangkar jadi jalan cepat, dan ukuran logo satu
// angka yang menjaga proporsinya.
//
// notes/44: format feed 4:5 & 1:1 (foto juga), beberapa logo sekaligus, kisi
// yang selalu terlihat, dan kotak judul gaya kartu berita yang ukurannya
// mengikuti teks — logo pertama bisa ditempel otomatis di tepi atasnya.
// Logo bisa DIKUNCI ke kotak itu (ikut bergeser bersamanya) dan menempel ke
// tepinya saat diseret. Setelan bisa disimpan sebagai template di server.
//
// Ikon: lucide-react (ISC). Tanpa satu emoji pun — alasannya di gui/app/page.tsx.

import { useEffect, useState } from "react";
import PageHeader from "../page-header";
import { Bookmark, Eye, Film, RectangleVertical, Type, X, ImageIcon } from "lucide-react";

import { eng, isWeb, useWeb } from "../engine";
import { useI18n } from "../i18n";
import Alerts from "../alerts";
import { watermarkToAPI, watermarkOn, frameSize } from "../watermark-model";
import type { Format } from "../watermark-model";
import { CENTER_X } from "../drag";
import Guides from "../guides";
import { Segmented } from "../clip-steps";
import TabCard, { Group, Row, Split } from "../tab-card";
import Stage, { Shot, StageBar, StageFrame, useStageWidth } from "../stage";
import { TechNotes } from "../log-panel";
import { useJob } from "../use-job";
import Picker from "../picker";
import FileCard, { baseName, useFiles } from "../file-list";
import Select from "../select";
import { useKeep, useRestore, useSticky } from "../persist";
import { HL_FONT, MarkFont, MarkTiming, MarkHeadline, MarkLayers, MarkLogoGroup, MarkTemplates, useMark, useTemplates } from "../mark";

type FileResult = { video: string; name: string; output?: string; seconds?: number; error?: string };
type Probe = { w: number; h: number; dur: number } | "error";

const isImg = (p: string) => /\.(jpe?g|png|webp)$/i.test(p);
const ok = (p: Probe | undefined) => !!p && p !== "error";
const mmss = (s: number) => `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, "0")}`;
// Berkas yang sudah seukuran format (selisih rasio <= 2%) dipakai apa adanya.
const fits = (p: Probe | undefined, f: Format) => {
  const { w, h } = frameSize(f);
  return !!p && p !== "error" && Math.abs(p.w / p.h - w / h) <= 0.02;
};

// Isi folder yang ditambahkan: video DAN foto.
const KEEP = (e: { video?: boolean; path: string }) => !!e.video || isImg(e.path);

export default function WatermarkPage() {
  const { t } = useI18n();

  const [probes, setProbes] = useState<Record<string, Probe>>({});
  // Mode web (notes/42): video & logo DIUNGGAH lewat input berkas.
  const web = useWeb();
  const [picking, setPicking] = useState<"" | "file" | "folder" | "out" | "logo">("");
  const [logoTarget, setLogoTarget] = useState(-1);
  const [outDir, setOutDir] = useState("");
  const [quality, setQuality] = useState("hd");
  const [format, setFormat] = useState<Format>("short");
  // Berkas berasio lain: dipotong memenuhi bingkai, atau utuh di atas latar blur.
  const [fit, setFit] = useState<"crop" | "whole">("crop");
  const [timed, setTimed] = useState(false);
  // Tab diingat browser: kembali ke halaman ini = tab yang sama.
  const [tab, setTab] = useSticky<"format" | "logo" | "headline" | "template">("mark.tab", "format");
  // Lebar bingkai = kolom pratinjau baku (stage.tsx), tidak pernah ikut isi.
  const stageW = useStageWidth();
  const [grid, setGrid] = useState(40);
  const [gridAlways, setGridAlways] = useState(true);

  const [error, setError] = useState("");
  // Daftar berkas + cara menambahnya: satu untuk semua halaman (file-list.tsx).
  const fl = useFiles(KEEP, setError);
  const { files: videos, setFiles: setVideos, add, addFolder } = fl;
  // Siklus job latar (use-job.ts): langganan SSE, start, cancel.
  const { jobId, job, logs, busy, start: startJob, cancel } = useJob<{ files: FileResult[] }>("/api/watermark", "watermark", setError, (j) => {
    // Mode web: engine menghapus video unggahan begitu job sukses, jadi daftarnya
    // ikut dibersihkan — menjalankannya lagi butuh unggah ulang.
    const gone = (j.result?.files ?? []).filter((f) => !f.error).map((f) => f.video);
    isWeb().then((w) => w && setVideos((v) => v.filter((x) => !gone.includes(x))));
  });

  // Pratinjau memakai berkas PERTAMA yang cocok formatnya: watermark-nya sama
  // untuk semuanya. Foto tidak punya detik ke-1.
  const first = videos.find((v) => ok(probes[v])) || "";
  // Setelan Mark (logo, judul, seret, tata letak) — bersama halaman News (mark.tsx).
  const m = useMark({ format, first, timed, grid });
  const { fw, fh, watermark, setWatermark, eff, cardOn, frameCardY, dragAt, placed, sel } = m;
  const reframe = fit === "whole" ? "reframe=fit&background=blur&zoom=0" : "reframe=center&background=black&zoom=100";
  const frame = first ? eng(`/api/frame?path=${encodeURIComponent(first)}&t=${isImg(first) ? 0 : 1}&${reframe}&format=${format}${cardOn ? `&card=1&card_y=${frameCardY}` : ""}`) : "";

  // Rasio dibaca dari berkasnya (bukan namanya) begitu video masuk daftar.
  useEffect(() => {
    for (const v of videos) {
      if (probes[v]) continue;
      fetch(eng(`/api/probe?path=${encodeURIComponent(v)}`)).then((r) => r.json().then((d) => ({ ok: r.ok, d })))
        .then(({ ok, d }) => setProbes((p) => ({ ...p, [v]: ok && d.width ? { w: d.width, h: d.height, dur: d.duration } : "error" })))
        .catch(() => setProbes((p) => ({ ...p, [v]: "error" })));
    }
  }, [videos, probes]);

  // Setelan watermark LENGKET: identitas akun dipilih sekali, bukan disusun
  // ulang tiap kali hendak memposting.
  useKeep("watermark", { videos, outDir, quality, watermark, logoOn: m.logoOn, logos: m.logos, timed, format, fit, grid, gridAlways });
  // Satu penerap untuk dua sumber: simpanan browser (useRestore) dan template.
  // keepText = teks judul yang sudah diketik TIDAK ditimpa template.
  const applySettings = (v: Record<string, unknown>, keepText?: string) => {
    if (typeof v.timed === "boolean") setTimed(v.timed);
    if (v.format === "short" || v.format === "feed45" || v.format === "feed11") setFormat(v.format);
    if (v.fit === "crop" || v.fit === "whole") setFit(v.fit);
    if (typeof v.grid === "number") setGrid(v.grid);
    if (typeof v.gridAlways === "boolean") setGridAlways(v.gridAlways);
    m.applyMark(v, keepText);
  };
  useRestore<Record<string, unknown>>("watermark", (v) => {
    if (Array.isArray(v.videos)) setVideos(v.videos as string[]);
    if (typeof v.outDir === "string") setOutDir(v.outDir);
    if (typeof v.quality === "string") setQuality(v.quality);
    applySettings(v);
  });

  // Template di server: satu daftar untuk Mark & News (notes/44).
  const tpl = useTemplates(setError);
  const [activeTpl, setActiveTpl] = useSticky("mark.template", "");

  // Semua berkas yang terbaca ikut; rasio lain disesuaikan engine.
  const ready = videos.filter((v) => ok(probes[v]));
  const nothing = !watermarkOn(eff);

  // Pratinjau hasil: engine membakar satu frame berkas pertama persis seperti
  // hasil akhirnya (watermark.Preview).
  const [shot, setShot] = useState("");
  const [shooting, setShooting] = useState(false);
  const previewResult = async () => {
    if (!first) return;
    setShooting(true); setError("");
    try {
      const r = await fetch(eng("/api/watermark/preview"), {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ file: first, fit, font: HL_FONT, watermark: watermarkToAPI(eff, HL_FONT) }),
      });
      if (!r.ok) throw new Error((await r.json().catch(() => null))?.error || "preview failed");
      const url = URL.createObjectURL(await r.blob());
      setShot((old) => { if (old) URL.revokeObjectURL(old); return url; });
    } catch (e) { setError(String(e)); }
    setShooting(false);
  };

  const start = () => startJob({ videos: ready, quality, out_dir: outDir, fit, font: HL_FONT, watermark: watermarkToAPI(eff, HL_FONT) });

  const files = job?.result?.files ?? [];
  const resultOf = (v: string) => files.findIndex((f) => f.video === v);
  const pct = Math.round((job?.progress ?? 0) * 100);
  // Bingkai pratinjau: lebar paling 340 px, tinggi paling 604 px.
  const frameH = Math.round((stageW * fh) / fw);
  return (
    <div className="screen scroll clips-v2 wm-v2">
      <PageHeader title={t("tabWatermark")} subtitle={t("subWatermark")}>
        {busy ? (
          <button type="button" className="danger" onClick={cancel}>{t("cancelRun")}</button>
        ) : (
          <button type="button" className="primary big" onClick={start} disabled={ready.length === 0 || nothing}
            title={nothing ? t("wmNothing") : undefined}>
            {files.length ? t("wmRedo") : ready.length === 1 ? t("wmStartOne") : t("wmStart", { n: ready.length })}
          </button>
        )}
      </PageHeader>
      <MarkFont />

      <Alerts items={[error && { kind: "error" as const, text: error }]} />

      {/* Tata letak baku (halaman klip): kiri = berkas + kartu setelan bertab,
          kanan = kolom pratinjau yang menempel (stage.tsx). */}
      <div className="setup-grid">
        <div className="setup-main">
          <FileCard title={t("wmVideosTitle")} fl={fl} web={web} accept="video/*,image/*" addLabel={t("wmAddFile")} note={t("wmVideosNote")}
            onPick={(m) => setPicking(m)}>
            {busy && (
              <div aria-live="polite">
                <p className="meta">{t("wmRunning")} {pct}%</p>
                <div className="bar wr-gap"><div style={{ width: `${pct}%` }} /></div>
              </div>
            )}
            <ul className="cap-videos wm-videos">
              {videos.map((v) => {
                const p = probes[v];
                const ri = resultOf(v);
                const r = ri >= 0 ? files[ri] : undefined;
                const info = p && p !== "error" ? (isImg(v) ? `${p.w}×${p.h}` : `${mmss(p.dur)} · ${p.w}×${p.h}`) : "";
                const [tag, cls] = r ? (r.error ? [t("wmFailedTag"), "bad"] : [t("wmDone"), "ok"])
                  : !p ? [t("wmChecking"), ""] : p === "error" ? [t("wmFailedTag"), "bad"]
                  : fits(p, format) ? [t("wmReady"), "ok"] : [t("wmFitTag"), "ok"];
                return (
                  <li key={v}>
                    {isImg(v) ? <ImageIcon className="ico" aria-hidden="true" /> : <Film className="ico" aria-hidden="true" />}
                    <div className="grow">
                      <p className="wm-name" title={v}>{baseName(v)}</p>
                      <p className="meta">{r?.error || (r?.output ? t("wmSaved", { name: baseName(r.output) }) : info)}</p>
                    </div>
                    {web && r?.output && <a className="btn-ghost" href={eng(`/api/watermark/${jobId}/file?i=${ri}`)} download>{t("download")}</a>}
                    <span className={"wm-tag " + cls}>{tag}</span>
                    <button type="button" className="ghost tiny icon-only" aria-label={t("capRemove")} title={t("capRemove")} disabled={busy}
                      onClick={() => setVideos((cur) => cur.filter((x) => x !== v))}><X className="ico" aria-hidden="true" /></button>
                  </li>
                );
              })}
            </ul>
          </FileCard>

          <TabCard label={t("tabWatermark")} open={tab} setOpen={setTab} tabs={[
            { key: "format", icon: RectangleVertical, label: t("wmFormatTitle") },
            { key: "logo", icon: ImageIcon, label: t("wmLogoTitle") },
            { key: "headline", icon: Type, label: t("wmTabHeadline") },
            { key: "template", icon: Bookmark, label: t("wmTplTitle") },
          ]}>
            {tab === "format" && (
              // Sejajar: kiri = format, kanan = keluaran.
              <Group title={t("wmFormatTitle")}>
                <Split side={<>
                  <span className="mc-r-l" title={t("wmQualityTip")}>{t("quality")}</span>
                  <Select value={quality} onChange={setQuality} disabled={busy} ariaLabel={t("quality")} options={[
                    { value: "draft", label: t("qualityDraft") },
                    { value: "hd", label: t("qualityHd") },
                    { value: "max", label: t("qualityMax") },
                  ]} />
                  {!web && <>
                    <span className="mc-r-l">{t("outputDir")}</span>
                    <div className="path-row">
                      <input id="wm-out" value={outDir} onChange={(e) => setOutDir(e.target.value)} placeholder={t("wmOutPlaceholder")} disabled={busy} aria-label={t("outputDir")} />
                      <button type="button" className="ghost" onClick={() => setPicking("out")}>{t("pickerGo")}…</button>
                    </div>
                  </>}
                </>}>
                    {/* Format (rasio) ada di baris alat di atas pratinjau (StageBar), sama dengan News. */}
                    <Row label={t("wmFitTitle")}>
                      <Segmented label={t("wmFitTitle")} value={fit} onChange={setFit}
                        options={[{ value: "crop" as const, name: t("wmFitCrop") }, { value: "whole" as const, name: t("wmFitWhole") }]} />
                    </Row>
                    <MarkTiming timed={timed} setTimed={setTimed} m={m} />
                </Split>
              </Group>
            )}

            {tab === "logo" && <MarkLogoGroup m={m} web={web} setError={setError} openPicker={(i) => { setLogoTarget(i); setPicking("logo"); }} />}
            {tab === "headline" && <MarkHeadline m={m} />}
            {tab === "template" && (
              <MarkTemplates tpl={tpl} active={activeTpl} onApply={(v, name) => { applySettings(v, watermark.hlText); setActiveTpl(name); }}
                data={() => ({ format, fit, logoOn: m.logoOn, logos: m.logos, timed, grid, gridAlways, watermark: { ...watermark, hlText: "" } })} />
            )}
          </TabCard>
          {/* Log di dasar kolom kiri: halaman ini tidak bergulir, kolomnya yang bergulir. */}
          <TechNotes logs={logs} />
        </div>

        <Stage title={t("previewTitle")} badge={`${fw} × ${fh}`} help={t("wmDragHint")}
          action={
            <button type="button" className="wm-preview-btn" disabled={!first || shooting || nothing} title={t("wmPreviewTip")} onClick={previewResult}>
              <Eye className="ico" aria-hidden="true" /> {shooting ? t("wmPreviewing") : t("wmPreviewResult")}
            </button>
          }>
          <StageBar grid={{ grid, setGrid, always: gridAlways, setAlways: setGridAlways }}
            ratio={{ label: t("wmFormatTitle"), value: format, onChange: (v) => setFormat(v as Format),
              options: [{ value: "short", label: "9:16" }, { value: "feed45", label: "4:5" }, { value: "feed11", label: "1:1" }] }} />
          <StageFrame>
            <div className="preview9x16 wm-frame" ref={m.boxRef}
              style={{ width: stageW, height: frameH, "--pvh": `${frameH}px` } as React.CSSProperties}>
              {frame ? (
                /* eslint-disable-next-line @next/next/no-img-element */
                <img src={frame} alt="" draggable={false} />
              ) : (
                <div className="preview-empty">
                  <div className="pe-icon" aria-hidden="true" />
                  <div className="pe-title">{t("wmEmptyFrame")}</div>
                </div>
              )}
              <Guides grid={grid} visible={gridAlways || dragAt !== null} playH={fh}
                atX={(dragAt?.x ?? placed[sel]?.x) === CENTER_X} atY={(dragAt?.y ?? placed[sel]?.y) === fh / 2} dragAt={dragAt} />
              <MarkLayers m={m} frameH={frameH} />
            </div>
          </StageFrame>
        </Stage>
      </div>


      {tpl.confirmEl}
      {shot && <Shot src={shot} label={t("wmPreviewResult")} onClose={() => setShot("")} />}
      {picking && (
        <Picker
          mode={picking === "folder" || picking === "out" ? "folder" : "file"}
          start={picking === "logo" ? m.logos[logoTarget]?.image || m.logos[0]?.image : undefined}
          onPick={(p) => {
            if (picking === "folder") addFolder(p);
            else if (picking === "out") setOutDir(p);
            else if (picking === "logo") m.putLogo(p, logoTarget);
            else add([p]);
            setPicking("");
          }}
          onClose={() => setPicking("")}
        />
      )}
    </div>
  );
}

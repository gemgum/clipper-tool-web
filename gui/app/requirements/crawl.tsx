"use client";

// Tab "News search" di halaman setelan (notes/45): kunci Serper & SociaVault,
// saldo, crawl terakhir, dan lencana kecil bila kredit habis / kunci ditolak.
// Di sinilah SATU-SATUNYA tempat notifikasi crawl (keputusan pemilik).

import { useCallback, useEffect, useState } from "react";
import { RefreshCw } from "lucide-react";
import { eng, post } from "../engine";
import { useI18n, type MessageKey } from "../i18n";
import { Group, Row } from "../tab-card";

type API = { configured: boolean; ok: boolean; balance?: number; checked_at?: string; problem?: string; error?: string };
type State = { last_crawl: string; next_after: string; running: boolean; items: number; queries: string[]; apis: Record<string, API> };

const APIS = [
  { id: "serper", name: "Serper", role: "crawlSerperRole", keysUrl: "https://serper.dev/api-keys" },
  { id: "sociavault", name: "SociaVault", role: "crawlSociaVaultRole", keysUrl: "https://sociavault.com/dashboard" },
] as const;

const PROBLEM: Record<string, MessageKey> = {
  no_credits: "crawlNoCredits", key_rejected: "crawlKeyRejected", rate_limited: "crawlRateLimited", unreachable: "crawlUnreachable",
};

// Tanggal Go kosong = "0001-01-01T00:00:00Z".
const when = (s?: string) => (s && !s.startsWith("0001") ? new Date(s).toLocaleString() : "–");

export default function CrawlSettings() {
  const { t } = useI18n();
  const [st, setSt] = useState<State | null>(null);
  const [keys, setKeys] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(() => {
    fetch(eng("/api/news/crawl")).then((r) => r.json()).then(setSt).catch(() => {});
  }, []);
  useEffect(() => { load(); }, [load]);
  // Selama crawl berjalan, status diperbarui tiap 3 detik.
  useEffect(() => {
    if (!st?.running) return;
    const id = setTimeout(load, 3000);
    return () => clearTimeout(id);
  }, [st, load]);

  const save = async (id: string, value: string) => {
    setBusy(id); setError("");
    try { setSt(await post<State>("/api/news/crawl/keys", { [id]: value })); setKeys((k) => ({ ...k, [id]: "" })); }
    catch (e: any) { setError(e.message); } // eslint-disable-line @typescript-eslint/no-explicit-any
    finally { setBusy(""); }
  };
  const crawl = async () => {
    setBusy("crawl"); setError("");
    try { await post("/api/news/crawl", {}); setTimeout(load, 500); }
    catch (e: any) { setError(e.message); } // eslint-disable-line @typescript-eslint/no-explicit-any
    finally { setBusy(""); }
  };

  return (
    <>
      <section className="card">
        <h2>{t("crawlTitle")}</h2>
        <p className="step-hint flush">{t("crawlHint", { queries: st?.queries.join(", ") || "…" })}</p>
        {APIS.map((a) => {
          const s = st?.apis[a.id];
          const level = !s?.configured ? "" : s.problem ? "bad" : s.ok ? "ok" : "";
          const label = !s?.configured ? t("crawlNotSet") : s.problem ? t(PROBLEM[s.problem] || "crawlUnreachable") : s.ok ? t("crawlOK") : t("crawlNotChecked");
          return (
            <Group key={a.id} title={a.name}>
              <Row label={t("crawlStatus")} note={s?.error ? s.error : t(a.role)}>
                <span className="crawl-status">
                  <span className={"set-label " + level}>{label}</span>
                  {s?.balance !== undefined && <span className="meta">{t("crawlBalance", { n: s.balance.toLocaleString() })}</span>}
                  {when(s?.checked_at) !== "–" && <span className="meta">{t("crawlChecked", { at: when(s?.checked_at) })}</span>}
                </span>
              </Row>
              <Row label={t("engineKeyLabel")}>
                <span className="crawl-key">
                  <input type="password" autoComplete="off" value={keys[a.id] || ""} aria-label={`${a.name} ${t("engineKeyLabel")}`}
                    placeholder={s?.configured ? t("crawlKeySaved") : t("crawlKeyEmpty")}
                    onChange={(e) => setKeys((k) => ({ ...k, [a.id]: e.target.value }))} />
                  <button type="button" className="primary" disabled={!!busy || !(keys[a.id] || "").trim()} onClick={() => save(a.id, keys[a.id])}>{t("save")}</button>
                  {s?.configured && <button type="button" className="ghost" disabled={!!busy} onClick={() => save(a.id, "")}>{t("engineRemoveKey")}</button>}
                  <a className="meta" href={a.keysUrl} target="_blank" rel="noreferrer">{t("engineGetKey")}</a>
                </span>
              </Row>
            </Group>
          );
        })}
      </section>
      <section className="card">
        <div className="card-head">
          <h2>{t("crawlLastTitle")}</h2>
          <button type="button" className="ghost" disabled={!!busy || !!st?.running || !Object.values(st?.apis || {}).some((a) => a.configured)} onClick={crawl}>
            <RefreshCw className="ico" aria-hidden="true" /> {st?.running ? t("crawlRunning") : t("crawlNow")}
          </button>
        </div>
        <p className="meta">{t("crawlLast", { at: when(st?.last_crawl), next: when(st?.next_after), n: st?.items ?? 0 })}</p>
        <p className="meta">{t("crawlNowNote")}</p>
        {error && <p className="nc-note-err">{error}</p>}
      </section>
    </>
  );
}

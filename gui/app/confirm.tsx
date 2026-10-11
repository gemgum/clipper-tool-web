"use client";

import { useCallback, useRef, useState } from "react";
import { useI18n } from "./i18n";

// Konfirmasi milik aplikasi, pengganti window.confirm() — kotak bawaan browser
// menulis "localhost:8787 says" dan tidak ikut tema (permintaan pemilik
// 9 Oktober 2026). <dialog> bawaan: Esc, fokus, dan latar diurus browser.
//
//   const [ask, confirmEl] = useConfirm();
//   if (!(await ask(t("..."), t("delete")))) return;
//   ... {confirmEl} di dalam JSX halaman.
export function useConfirm(): [(text: string, okLabel?: string) => Promise<boolean>, React.ReactNode] {
  const { t } = useI18n();
  const ref = useRef<HTMLDialogElement>(null);
  const resolver = useRef<((ok: boolean) => void) | null>(null);
  const [text, setText] = useState("");
  const [ok, setOk] = useState("");

  const ask = useCallback((msg: string, okLabel?: string) => {
    setText(msg); setOk(okLabel || "");
    ref.current?.showModal();
    return new Promise<boolean>((res) => { resolver.current = res; });
  }, []);
  const done = (v: boolean) => {
    resolver.current?.(v); resolver.current = null;
    ref.current?.close();
  };

  const el = (
    <dialog ref={ref} className="app-confirm" onCancel={() => done(false)}
      onClick={(e) => { if (e.target === e.currentTarget) done(false); }}>
      <p>{text}</p>
      <div className="app-confirm-actions">
        <button type="button" className="ghost" onClick={() => done(false)}>{t("cancel")}</button>
        <button type="button" className="danger" autoFocus onClick={() => done(true)}>{ok || t("historyDelete")}</button>
      </div>
    </dialog>
  );
  return [ask, el];
}

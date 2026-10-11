"use client";

import { User } from "lucide-react";
import { useEffect, useState } from "react";
import { LANGUAGES, useI18n } from "./i18n";
import Popover from "./popover";

const KEY = "clipper.code";

// Kode akses. UNTUK SEKARANG hanya disimpan — belum menggerbangi apa pun.
//
// Ditulis begitu dengan sengaja: gerbang yang benar harus diperiksa ENGINE,
// bukan halaman. Kode yang hanya dicek di JavaScript bisa dilewati siapa pun
// yang membuka devtools, jadi memasangnya sekarang cuma memberi rasa aman yang
// palsu. Yang dikerjakan sekarang adalah tempat memasukkannya; pemeriksaannya
// menyusul di engine bersama keputusan bagaimana kodenya diterbitkan.
export default function AccountButton() {
  const { t, lang, setLang } = useI18n();
  const [code, setCode] = useState("");
  const [saved, setSaved] = useState("");

  useEffect(() => {
    try {
      const v = localStorage.getItem(KEY) || "";
      setSaved(v); setCode(v);
    } catch {}
  }, []);

  const save = (close: () => void) => {
    const v = code.trim();
    try { v ? localStorage.setItem(KEY, v) : localStorage.removeItem(KEY); } catch {}
    setSaved(v);
    close();
  };

  return (
    <Popover width={280} buttonClass="rail-account-btn" side="beside" ariaLabel={t("navAccount")} label={
      <>
        {/* Avatar = huruf pertama kode akses, kalau belum ada: ikon kosong. */}
        <span className="rail-avatar" aria-hidden="true">{saved ? saved[0].toUpperCase() : <User className="ico" />}</span>
        <span className="rail-label">{t("navAccount")}</span>
      </>
    }>
      {(close) => (
        <div className="pop-form">
          {/* Bahasa = preferensi pribadi, sederajat dengan akun & tema
              (DESIGN-Navigasi §4), jadi pindah ke sini dari panel gerigi. */}
          <div className="settings-head">{t("settingsLanguage")}</div>
          <div className="lang-switch">
            {LANGUAGES.map((l) => (
              <button key={l} type="button" className={l === lang ? "active" : ""} aria-pressed={l === lang} onClick={() => setLang(l)}>
                {l === "en" ? "English" : "Indonesia"}
              </button>
            ))}
          </div>
          <div className="settings-title">{t("accountTitle")}</div>
          <p className="meta">{t("accountHint")}</p>
          <input
            value={code}
            spellCheck={false}
            placeholder={t("accountPlaceholder")}
            onChange={(e) => setCode(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && save(close)}
          />
          <div className="pop-actions">
            <button onClick={() => save(close)}>{t("save")}</button>
            {saved && <button className="ghost" onClick={() => { setCode(""); try { localStorage.removeItem(KEY); } catch {} setSaved(""); }}>
              {t("accountClear")}
            </button>}
          </div>
        </div>
      )}
    </Popover>
  );
}

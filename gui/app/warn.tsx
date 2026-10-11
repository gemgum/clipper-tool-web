"use client";

import { TriangleAlert } from "lucide-react";
import Popover from "./popover";
import { useI18n } from "./i18n";

// Peringatan yang menempel pada kendali yang bersalah, dan TIDAK menggeser
// apa pun.
//
// Bentuk lamanya satu baris teks di bawah kendali (`<div className="warn">`),
// dan tiap baris itu muncul atau hilang seluruh panel di bawahnya bergerak —
// tepat saat pengguna mengubah setelan yang memicunya. Sekarang yang muncul
// hanya LAMBANG di dalam label: label sudah setinggi satu baris teks, jadi
// lambang 14 px di dalamnya tidak menambah satu piksel pun.
//
// Isinya lengkap — kalimat DAN tombol tindakannya — di dalam popup melayang.
export default function Warn({ children, width = 300 }: { children: React.ReactNode; width?: number }) {
  const { t } = useI18n();
  return (
    <Popover width={width} buttonClass="warn-chip" label={
      <TriangleAlert className="ico" aria-hidden="true" aria-label={t("warning")} />
    }>
      {() => <div className="warn-pop">{children}</div>}
    </Popover>
  );
}

"use client";
import type { Align } from "./clip-steps";

// Watermark: banner PNG milik pengguna + teks di atasnya, dibakar ke setiap klip.
//
// Bentuknya satu objek, bukan belasan state terpisah, karena ia melintasi tiga
// berkas — halaman yang memegangnya, panel setelan yang mengubahnya, dan
// pratinjau yang menggambarnya. Tiga belas prop yang diteruskan satu per satu
// adalah tiga belas kesempatan untuk lupa satu.
//
// Koordinatnya memakai ruang 1080x1920 yang sama dengan subtitle, jadi angka di
// pratinjau berarti hal yang sama dengan angka yang dikirim ke engine.

export type Watermark = {
  image: string;   // path lokal; kosong = watermark mati
  x: number;       // titik TENGAH banner
  y: number;
  // KOTAK tempat gambar diletakkan, persen lebar & tinggi bingkai. Gambarnya
  // dimuat utuh ke dalamnya — tidak digepengkan, tidak dipotong.
  width: number;
  height: number;
  at: number;      // detik muncul, relatif awal klip
  dur: number;     // berapa detik tampil; 0 = sampai klip habis
  // Sumber teks. "text" = satu teks untuk semua klip, bisa dipratinjau apa
  // adanya. "llm" = judul yang dipilihkan LLM untuk tiap klip — beda tiap klip,
  // jadi pratinjau hanya bisa menampilkan contoh.
  hlSource: "text" | "llm";
  hlText: string;
  hlSize: number;
  hlColor: string;
  hlOutline: number;
  // Geseran dari TITIK TENGAH kotak watermark, bukan koordinat mutlak. 0,0
  // berarti tepat di tengah gambarnya — dan itu bawaannya.
  hlDX: number;
  hlDY: number;
  // Judul berdiri sendiri di bingkai, tidak dikurung kotak gambar. Hanya
  // halaman watermark yang menyalakannya (config.Headline.Free di engine).
  hlFree?: boolean;
  // Kotak kartu berita + baris kredit di bawah judul (config.Headline.Box).
  hlBox?: boolean;
  hlCredit?: string;
  // Sudut kotak judul lancip (true) atau melingkar.
  hlSharp?: boolean;
  // Warna isi kotak judul (nama atau #rrggbb); kosong = putih.
  hlBoxColor?: string;
  // Perataan teks di dalam kotak judul.
  hlAlign?: Align;
  // Gaya kotak judul di GUI: "box" = warna diatur sendiri + pilihan sudut;
  // "look" = tampilan jadi kartu berita. Engine tidak membacanya — Look hanya
  // MENGISI hlBoxColor & hlColor.
  hlStyle?: "box" | "look";
  // Look terpilih: id tema (themes.ts) atau "#rrggbb" isian sendiri.
  hlLook?: string;
  // Warna garis aksen & lencana kredit tampilan Look, dan teks di lencana
  // (diisi /api/watermark/look).
  hlBadge?: string;
  hlOnBadge?: string;
  // Geseran tegak blok judul kartu dari bawaannya (config.Headline.CardY).
  hlCardY?: number;
  // Warna garis tepi: huruf (judul biasa) atau KOTAK (kotak judul). Kosong =
  // otomatis (hitam, putih untuk teks hitam).
  hlOutlineColor?: string;
  // Logo TAMBAHAN di samping image (config.Watermark.Logos).
  logos?: Logo[];
  // Bingkai halaman watermark. Halaman klip selalu "short".
  format?: Format;
};

export type Logo = { image: string; x: number; y: number; width: number; height: number };
export type Format = "short" | "feed45" | "feed11";

// Ruang koordinat per format. Harus sama dengan config.FrameSize di engine.
export function frameSize(f: Format | undefined) {
  return { w: 1080, h: f === "feed45" ? 1350 : f === "feed11" ? 1080 : 1920 };
}

// Harus sama dengan config.DefaultWatermark() di engine.
export const DEFAULT_WATERMARK: Watermark = {
  image: "", x: 540, y: 700, width: 25, height: 25, at: 0, dur: 0,
  hlSource: "text", hlText: "", hlSize: 64, hlColor: "white", hlOutline: 3,
  hlDX: 0, hlDY: 0,
};

// watermarkOn = ada yang akan tergambar. Headline tanpa banner sah: teks tetap di
// atas video adalah bentuk watermark yang paling murah.
export const watermarkOn = (b: Watermark) =>
  !!b.image || !!b.logos?.length || !!b.hlText.trim() || b.hlSource === "llm";

// watermarkToAPI menerjemahkan ke bentuk yang dibaca config.Watermark di engine.
//
// font datang dari pilihan font SUBTITLE, dan itu disengaja: headline memakai
// keluarga font yang sama supaya pratinjau bisa memakai metrik yang sudah
// dihitung engine untuk font itu (lihat fontScale di preview-panel). Pemilih
// font kedua berarti pengukuran kedua, dan pratinjau yang meleset dari hasil
// render adalah persis kegagalan yang sudah pernah terjadi (notes/29).
export const watermarkToAPI = (b: Watermark, font: string) => ({
  image: b.image,
  x: Math.round(b.x), y: Math.round(b.y),
  width: Math.round(b.width),
  height: Math.round(b.height),
  at: Number(b.at) || 0,
  for: Number(b.dur) || 0,
  format: b.format || "short",
  logos: (b.logos || []).map((l) => ({
    image: l.image, x: Math.round(l.x), y: Math.round(l.y),
    width: Math.round(l.width), height: Math.round(l.height),
  })),
  headline: {
    source: b.hlSource,
    font,
    text: b.hlSource === "llm" ? "" : b.hlText,
    size: Math.round(b.hlSize),
    color: b.hlColor,
    bold: true,
    outline: Math.round(b.hlOutline),
    // Teks hitam butuh tepi terang; warna lain tepi hitam.
    outline_color: b.hlOutlineColor || (b.hlColor === "black" ? "white" : "black"),
    free: !!b.hlFree,
    box: !!b.hlBox,
    credit: b.hlBox ? (b.hlCredit || "") : "",
    sharp: !!b.hlSharp,
    box_color: b.hlBoxColor || "",
    align: b.hlAlign || "left",
    // Style Look = tampilan kartu berita (config.Headline.Card).
    card: !!b.hlBox && b.hlStyle === "look",
    badge: b.hlBadge || "",
    on_badge: b.hlOnBadge || "",
    card_y: Math.round(b.hlCardY || 0),
    dx: Math.round(b.hlDX), dy: Math.round(b.hlDY),
  },
});

// wrapHeadline meniru headlineLines() di engine (internal/subtitle).
//
// Ada di dua tempat, dan itu disengaja: engine yang menulis .ass, tapi
// pratinjau harus menunjukkan PEMENGGALAN YANG SAMA. Pratinjau yang menampilkan
// satu baris sementara hasil rendernya tiga adalah bentuk kebohongan yang
// paling mahal — ketahuannya setelah klipnya jadi.
//
// Angka-angkanya harus sama dengan sisi Go: margin 60 di ruang 1080, dan faktor
// lebar huruf 0,6.
const HL_PAD = 16;

// headlineBox = kotak yang MENGURUNG teks: kotak gambar bila ada gambarnya,
// seluruh bingkai bila tidak. Harus sama dengan config.Watermark.HeadlineBox.
export function headlineBox(b: Watermark) {
  const f = frameSize(b.format);
  if (!b.image || b.hlFree || b.hlBox) return { cx: f.w / 2, cy: f.h / 2, w: f.w, h: f.h };
  return { cx: b.x, cy: b.y, w: (f.w * b.width) / 100, h: (f.h * b.height) / 100 };
}

// Tata letak kotak kartu berita DARI ENGINE (POST /api/watermark/layout →
// subtitle.NewsBox). Tidak dihitung ulang di sini: engine mengukur dengan
// berkas font asli, dan tebakan kedua di GUI adalah cara pratinjau berbeda
// dari hasil render.
export type NewsLayout = {
  left: number; top: number; w: number; h: number;
  lines: string[]; credit: number; pad: number; radius: number;
  stretch?: boolean[]; // baris yang direntangkan kiri-kanan (Align justify)
  photo?: number; // > 0 = tampilan kartu berita: tinggi area gambar (subtitle.CardBox)
};

export function wrapHeadline(text: string, size: number, boxW: number): string[] {
  const usable = boxW - 2 * HL_PAD;
  const maxChars = Math.max(4, Math.floor(usable / ((size || 64) * 0.6)));
  const out: string[] = [];
  for (const para of text.split("\n")) {
    let line = "";
    for (const word of para.split(/\s+/).filter(Boolean)) {
      if (!line) line = word;
      else if (line.length + 1 + word.length <= maxChars) line += " " + word;
      else { out.push(line); line = word; }
    }
    if (line) out.push(line);
  }
  return out;
}

// headlineAnchor menaruh blok teks di dalam kotaknya: tengah kotak, digeser
// sebanyak dx/dy, lalu DIJEPIT supaya seluruh bloknya tetap di dalam.
//
// Meniru headlineAnchor() di engine (internal/subtitle), termasuk perkiraannya
// (0,6 x ukuran per huruf, satu baris setinggi ukurannya). Digandakan dengan
// alasan yang sama seperti wrapHeadline: engine yang menulis .ass, tapi
// pratinjau harus mengurung di tempat yang SAMA.
export function headlineAnchor(lines: string[], size: number, dx: number, dy: number,
                               box: { cx: number; cy: number; w: number; h: number }) {
  const longest = lines.reduce((n, l) => Math.max(n, l.length), 0);
  const blockW = longest * (size || 64) * 0.6;
  const blockH = lines.length * (size || 64);
  const limitX = Math.max(0, (box.w - blockW) / 2);
  const limitY = Math.max(0, (box.h - blockH) / 2);
  return {
    x: box.cx + Math.max(-limitX, Math.min(limitX, dx)),
    y: box.cy + Math.max(-limitY, Math.min(limitY, dy)),
  };
}

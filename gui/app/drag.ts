"use client";

// Menggeser sesuatu di atas bingkai pratinjau.
//
// SATU mesin untuk semua lapis — subtitle, gambar watermark, headline — dan
// sekarang untuk dua halaman sekaligus (klip & watermark). Dulu logikanya
// menempel pada state subtitle di dalam <PreviewPanel>; setiap lapis baru
// berarti salinan keempat dari magnet, grid, dan batas bidang yang sama, dan
// yang terlewat baru ketahuan sebagai "lapis yang satu ini terasa beda".

import { useCallback, useRef, useState } from "react";

// Ruang koordinat penempatan. Sama dengan config.PlayResX/Y di engine — semua
// yang digambar di atas video dinyatakan di sini, apa pun resolusi rendernya.
export const PLAY_W = 1080, PLAY_H = 1920;
// Titik tengah bidang + toleransi magnet. 20 ≈ 5 piksel layar pada pratinjau
// 270 px: cukup terasa tanpa bikin susah menaruh sesuatu sedikit di luar tengah.
export const CENTER_X = 540, CENTER_Y = 960;
const MAGNET = 20;

// snap membulatkan ke kelipatan terdekat. g = 0 berarti tidak menempel.
export const snap = (v: number, g: number) => (g > 0 ? Math.round(v / g) * g : v);

export type Point = { x: number; y: number };

/**
 * useLayerDrag mengembalikan pembuat penangan untuk tiap lapis yang bisa
 * digeser, plus titik yang sedang dipegang (untuk garis sumbu & angkanya).
 *
 * Selisih titik pegang disimpan saat tombol ditekan, lalu ikut dijumlahkan tiap
 * gerakan. Dulu posisi langsung disamakan dengan kursor, jadi yang digeser
 * melompat ke bawah kursor begitu disentuh — itu yang terasa "kurang stabil".
 */
export function useLayerDrag(boxRef: React.RefObject<HTMLDivElement | null>, grid: number) {
  const [dragAt, setDragAt] = useState<Point | null>(null);
  const grab = useRef({ dx: 0, dy: 0 });
  const dragging = useRef(false);

  const point = useCallback((e: React.PointerEvent): Point => {
    const rect = boxRef.current!.getBoundingClientRect();
    return {
      x: ((e.clientX - rect.left) / rect.width) * PLAY_W,
      y: ((e.clientY - rect.top) / rect.height) * PLAY_H,
    };
  }, [boxRef]);

  const end = useCallback(() => {
    dragging.current = false;
    setDragAt(null);
  }, []);

  /**
   * dragProps membuat penangan untuk satu lapis.
   *
   * magnetY & maxY berbeda per lapis: subtitle menempel ke titik yang membuat
   * BLOKNYA di tengah (jangkarnya di tepi atas), sedangkan banner menempel ke
   * tengah bidang karena jangkarnya memang titik tengahnya sendiri.
   */
  const dragProps = useCallback((
    x: number, y: number,
    onMove: (x: number, y: number) => void,
    magnetY: number, maxY: number,
    // Titik magnet mendatar. Bawaannya tengah BINGKAI, tapi headline dikurung
    // kotak watermark yang bisa berdiri di mana saja — magnet ke tengah bingkai
    // di situ justru menarik teks keluar dari kotaknya.
    magnetX: number = CENTER_X,
  ) => {
    const clamp = (nx: number, ny: number) =>
      onMove(Math.round(Math.max(0, Math.min(PLAY_W, nx))),
             Math.round(Math.max(0, Math.min(maxY, ny))));
    return {
      tabIndex: 0,
      onPointerDown: (e: React.PointerEvent) => {
        if (!boxRef.current) return;
        e.preventDefault();
        const p = point(e);
        grab.current = { dx: x - p.x, dy: y - p.y };
        dragging.current = true;
        setDragAt({ x, y });
        e.currentTarget.setPointerCapture?.(e.pointerId);
      },
      onPointerMove: (e: React.PointerEvent) => {
        if (!dragging.current || !boxRef.current) return;
        const p = point(e);
        let nx = p.x + grab.current.dx;
        let ny = p.y + grab.current.dy;
        // Alt = abaikan grid. Grid adalah bawaan, bukan pagar: harus selalu ada
        // jalan menaruh sesuatu satu piksel di luar kotaknya.
        const g = e.altKey ? 0 : grid;
        // Magnet MENANG atas grid, dan itu bukan selera: titik tengahnya sering
        // bukan kelipatan grid (pada grid 24, X tengah 540 tidak terjangkau sama
        // sekali), jadi menempelkan ke grid lebih dulu membuat "tepat di tengah"
        // mustahil dicapai — persis kemampuan yang tidak boleh hilang.
        nx = Math.abs(nx - magnetX) < MAGNET ? magnetX : snap(nx, g);
        ny = Math.abs(ny - magnetY) < MAGNET ? magnetY : snap(ny, g);
        setDragAt({ x: Math.round(nx), y: Math.round(ny) });
        clamp(nx, ny);
      },
      onPointerUp: end,
      onPointerCancel: end,
      // Tombol panah menggeser 1 piksel (Shift = 10) — penempatan halus tanpa
      // harus mematikan grid, dan satu-satunya cara menaruh sesuatu dengan angka
      // yang benar-benar bisa diulang.
      onKeyDown: (e: React.KeyboardEvent) => {
        const step = e.shiftKey ? 10 : 1;
        const move: Record<string, [number, number]> = {
          ArrowLeft: [-step, 0], ArrowRight: [step, 0],
          ArrowUp: [0, -step], ArrowDown: [0, step],
        };
        const d = move[e.key];
        if (!d) return;
        e.preventDefault();
        clamp(x + d[0], y + d[1]);
      },
    };
  }, [boxRef, point, grid, end]);

  return { dragAt, dragProps };
}

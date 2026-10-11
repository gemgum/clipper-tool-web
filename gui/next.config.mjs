/** @type {import('next').NextConfig} */

// Ekspor statis: `next build` menghasilkan HTML+JS biasa di gui/out, yang
// disajikan engine sendiri. Aplikasi desktop tidak boleh menuntut Node.js
// terpasang di komputer pengguna, dan seluruh GUI ini memang berjalan di
// browser — tidak ada satu pun bagian yang butuh server Next.
//
// trailingSlash supaya tiap halaman jadi <nama>/index.html; itu bentuk yang
// bisa disajikan penyaji berkas statis apa pun tanpa aturan penulisan ulang.
const nextConfig = {
  output: "export",
  trailingSlash: true,
  images: { unoptimized: true },
};

export default nextConfig;

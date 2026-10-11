// Tailwind v4 dipasang lewat plugin PostCSS — tidak ada tailwind.config.js lagi,
// seluruh setelannya ada di CSS (blok @theme di app/globals.css).
const config = {
  plugins: { "@tailwindcss/postcss": {} },
};

export default config;

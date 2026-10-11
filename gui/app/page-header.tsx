// Kepala halaman: judul, satu baris subjudul, dan (bila ada) aksi utama.
//
// Keputusan pemilik 9 Oktober 2026 (DESIGN.md §5.2): tiap halaman punya <h1>
// dan aksi utamanya duduk DI SINI, bukan di dasar kolom kanan. Tingginya
// tetap — subjudul satu baris dipotong elipsis — supaya isi di bawahnya tidak
// bergeser saat bahasa atau status berganti.
export default function PageHeader({
  title, subtitle, children,
}: {
  title: string;
  subtitle?: string;
  /** Aksi utama + status (mis. <RunPanel>). */
  children?: React.ReactNode;
}) {
  return (
    <header className="page-header">
      <div className="ph-text">
        <h1>{title}</h1>
        {subtitle && <p title={subtitle}>{subtitle}</p>}
      </div>
      {children && <div className="ph-actions">{children}</div>}
    </header>
  );
}

// Keadaan kosong yang seragam (DESIGN.md §5.10): ikon, judul, satu kalimat
// langkah berikutnya, dan — bila ada — satu aksi. Kalimatnya menyebut apa
// yang harus dilakukan, bukan "tidak ada data".
export default function EmptyState({
  icon: Icon, title, description, action,
}: {
  icon: React.ComponentType<{ className?: string; "aria-hidden"?: boolean | "true" }>;
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="empty">
      <Icon className="empty-ico" aria-hidden="true" />
      <h3>{title}</h3>
      {description && <p>{description}</p>}
      {action}
    </div>
  );
}

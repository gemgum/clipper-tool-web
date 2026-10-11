import { useI18n } from "./i18n";

// Kerangka daftar berita selama pemuatan pertama (DESIGN.md §5.8). Diam, tanpa
// denyut (dial gerak 1), dan disertai kalimat yang menyebut APA yang dimuat —
// kerangka saja tidak mengatakan apa-apa.
export default function NewsSkeleton() {
  const { t } = useI18n();
  return (
    <div className="news-list" aria-busy="true">
      <p className="meta feed-more">{t("loadingNews")}</p>
      {Array.from({ length: 6 }, (_, i) => (
        <div key={i} className="news-item skel" aria-hidden="true">
          <span className="skel-img" />
          <span className="skel-lines"><span /><span /><span /></span>
        </div>
      ))}
    </div>
  );
}

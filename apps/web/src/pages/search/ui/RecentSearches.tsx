import { useRecentSearches } from '@/features/search-history/recentSearches';
import { Tag } from '@/shared/ui';
import { SectionHeader } from '@/widgets/section-header/SectionHeader';

import styles from './SearchPage.module.css';

/** "Recent searches" (canvas SearchEmpty / SearchError): removable tags; a click repeats the search. */
export function RecentSearches({ onPick }: { onPick: (query: string) => void }) {
  const items = useRecentSearches((s) => s.items);
  const remove = useRecentSearches((s) => s.remove);
  const clear = useRecentSearches((s) => s.clear);
  if (items.length === 0) return null;

  return (
    <section aria-labelledby="recent-searches-title" className={styles.sectionTight}>
      <SectionHeader titleId="recent-searches-title" title="Recent searches" action="Clear all" onAction={clear} />
      <ul className={styles.tagList}>
        {items.map((q) => (
          <li key={q}>
            <Tag onRemove={() => remove(q)} removeLabel={`Remove ${q} from recent searches`}>
              <button type="button" className={styles.tagButton} onClick={() => onPick(q)} aria-label={`Search for ${q}`}>
                {q}
              </button>
            </Tag>
          </li>
        ))}
      </ul>
    </section>
  );
}

import { useNavigate } from 'react-router';

import { routes } from '@/shared/config/routes';
import { Chip, MediaCard, Skeleton } from '@/shared/ui';
import { InlineAlert } from '@/shared/ui/patterns/InlineAlert';
import { CollectionCard } from '@/widgets/collection-card/CollectionCard';
import { GenreTile } from '@/widgets/genre-tile/GenreTile';
import { SectionHeader } from '@/widgets/section-header/SectionHeader';

import { useSearchBrowse } from '../api/search';

import { RecentSearches } from './RecentSearches';
import styles from './SearchPage.module.css';

const count = new Intl.NumberFormat('en-US');

/** Search before a query — canvas "Search — before query" (SearchEmpty). */
export function SearchBrowse({ onPick }: { onPick: (query: string) => void }) {
  const browse = useSearchBrowse();
  const navigate = useNavigate();

  return (
    <>
      <RecentSearches onPick={onPick} />

      {browse.isError && (
        <InlineAlert title="Browse isn't responding" description="Genres and collections couldn't load. Search still works." onRetry={() => void browse.refetch()} />
      )}

      <section aria-labelledby="genres-title" className={styles.section}>
        <SectionHeader titleId="genres-title" title="Browse genres" action="All genres" onAction={() => navigate(routes.shelf('genres'))} />
        <div className={styles.genreGrid}>
          {browse.data
            ? browse.data.genres.map((g) => (
                <GenreTile key={g.slug} label={g.label} count={`${count.format(g.releaseCount)} releases`} art={g.art} to={routes.genre(g.slug)} />
              ))
            : Array.from({ length: 12 }, (_, i) => <Skeleton key={i} shape="rect" height={104} radius="card" />)}
        </div>
      </section>

      {browse.data && browse.data.moods.length > 0 && (
        <section aria-labelledby="moods-title" className={styles.sectionTight}>
          <SectionHeader titleId="moods-title" title="Moods" />
          <div role="group" aria-labelledby="moods-title" className={styles.chips}>
            {browse.data.moods.map((m) => (
              <Chip key={m.id} selected={false} onChange={() => navigate(routes.mood(m.id))}>
                {m.label}
              </Chip>
            ))}
          </div>
        </section>
      )}

      <section aria-labelledby="collections-title" className={styles.section}>
        <SectionHeader
          titleId="collections-title"
          title="Collections from ICYRE"
          kicker={browse.data?.collections.kicker}
          action="See all"
          onAction={() => navigate(routes.shelf('collections'))}
        />
        <div className={styles.cardGrid}>
          {browse.data
            ? browse.data.collections.items.map((p) => <CollectionCard key={p.id} item={p} />)
            : Array.from({ length: 6 }, (_, i) => <MediaCard key={i} title="" state="loading" />)}
        </div>
      </section>
    </>
  );
}

import { useState } from 'react';
import { useNavigate } from 'react-router';

import { useCurrentUser } from '@/entities/user/api';
import { useCollectionPlayState, usePlayCollection } from '@/features/player';
import { routes } from '@/shared/config/routes';
import { formatDuration } from '@/shared/lib/formatTime';
import { Chip, FeaturedCard, MediaCard, Skeleton, TrackRow } from '@/shared/ui';
import { InlineAlert } from '@/shared/ui/patterns/InlineAlert';
import { PageToolbar } from '@/widgets/page-toolbar/PageToolbar';
import { QuickTile } from '@/widgets/quick-tile/QuickTile';
import { SectionHeader } from '@/widgets/section-header/SectionHeader';

import { useHomeFeed, type Collection, type DailyMix, type HomeFeed } from '../api/homeFeed';
import { greeting } from '../lib/greeting';

import { AlbumOfTheWeek } from './AlbumOfTheWeek';
import { CollectionCard } from './CollectionCard';
import styles from './HomePage.module.css';
import { TrendingSection } from './TrendingSection';

type Filter = 'all' | 'album' | 'playlist' | 'artist';

const FILTERS: Array<{ value: Filter; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'album', label: 'Albums' },
  { value: 'playlist', label: 'Playlists' },
  { value: 'artist', label: 'Artists' },
];

/** Home — the entry screen of the product canvas (Main.dc.html). */
export function HomePage() {
  const feed = useHomeFeed();
  const user = useCurrentUser();
  const [filter, setFilter] = useState<Filter>('all');
  const { kicker, title } = greeting(new Date(), user.data?.displayName);

  return (
    <>
      <PageToolbar />
      <div className={styles.page}>
        <section className={styles.intro}>
          <div className={styles.introText}>
            <span className="ic-overline">{kicker}</span>
            <h1 className="t-h1">{title}</h1>
            <p className="t-body t-secondary">Quiet records for a late start, picked from what you played this week.</p>
          </div>
          <div role="group" aria-label="Filter home" className={styles.filters}>
            {FILTERS.map((f) => (
              <Chip key={f.value} selected={filter === f.value} onChange={() => setFilter(f.value)}>
                {f.label}
              </Chip>
            ))}
          </div>
        </section>

        {feed.isPending && <HomeSkeleton />}
        {feed.isError && (
          <InlineAlert
            title="Home isn't responding"
            description="ICYRE couldn't load your recommendations. Your library still works."
            onRetry={() => void feed.refetch()}
          />
        )}
        {feed.data && <HomeSections feed={feed.data} filter={filter} />}
      </div>
    </>
  );
}

function HomeSections({ feed, filter }: { feed: HomeFeed; filter: Filter }) {
  const navigate = useNavigate();
  const play = usePlayCollection();
  const show = (kind: Exclude<Filter, 'all'>) => filter === 'all' || filter === kind;
  const pick = (items: Collection[]) => (filter === 'all' ? items : items.filter((i) => i.kind === filter));

  const recent = pick(feed.recentlyPlayed);
  const recommended = pick(feed.recommended.items);

  return (
    <>
      {recent.length > 0 && (
        <section aria-labelledby="recent-title" className={styles.section}>
          <SectionHeader titleId="recent-title" title="Recently played" action="History" onAction={() => navigate(routes.history)} />
          <div className={styles.quickGrid}>
            {recent.map((item) => (
              <RecentTile key={`${item.kind}-${item.id}`} item={item} onPlay={() => void play(item.kind, item.id)} />
            ))}
          </div>
        </section>
      )}

      {show('album') && feed.albumOfTheWeek && <AlbumOfTheWeek data={feed.albumOfTheWeek} />}

      {recommended.length > 0 && (
        <section aria-labelledby="rec-title" className={styles.section}>
          <SectionHeader titleId="rec-title" title="Recommended for you" kicker={feed.recommended.kicker} action="See all" onAction={() => navigate(routes.shelf('recommended'))} />
          <div className={styles.cardGrid}>
            {recommended.map((item) => (
              <CollectionCard key={`${item.kind}-${item.id}`} item={item} />
            ))}
          </div>
        </section>
      )}

      {show('album') && feed.newReleases.items.length > 0 && (
        <section aria-labelledby="new-title" className={styles.section}>
          <SectionHeader titleId="new-title" title="New releases" kicker={feed.newReleases.kicker} action="See all" onAction={() => navigate(routes.shelf('new-releases'))} />
          <div className={styles.cardGrid}>
            {feed.newReleases.items.map((item) => (
              <CollectionCard key={item.id} item={item} />
            ))}
          </div>
        </section>
      )}

      {filter === 'all' && <TrendingSection today={feed.trending.today} week={feed.trending.week} />}

      {show('playlist') && (feed.madeForYou.featured || feed.madeForYou.playlists.length > 0) && (
        <section aria-labelledby="mfy-title" className={styles.section}>
          <SectionHeader titleId="mfy-title" title="Made for you" kicker="Updated every morning" action="See all" onAction={() => navigate(routes.shelf('made-for-you'))} />
          <div className={styles.madeForYou}>
            {feed.madeForYou.featured && <DailyMixCard mix={feed.madeForYou.featured} onOpen={() => navigate(routes.playlist(feed.madeForYou.featured!.id))} />}
            <div className={styles.mixGrid}>
              {feed.madeForYou.playlists.map((item) => (
                <CollectionCard key={item.id} item={item} />
              ))}
            </div>
          </div>
        </section>
      )}

      {show('artist') && feed.followedArtists.length > 0 && (
        <section aria-labelledby="artists-title" className={styles.section}>
          <SectionHeader titleId="artists-title" title="Artists you follow" action="Manage" onAction={() => navigate(routes.library)} />
          <div className={styles.cardGrid}>
            {feed.followedArtists.map((item) => (
              <CollectionCard key={item.id} item={item} />
            ))}
          </div>
        </section>
      )}
    </>
  );
}

function RecentTile({ item, onPlay }: { item: Collection; onPlay: () => void }) {
  const playState = useCollectionPlayState(item.kind, item.id);
  const common = { art: item.art, coverUrl: item.coverUrl, playState, onPlay };
  switch (item.kind) {
    case 'album':
      return <QuickTile {...common} title={item.title} subtitle={`Album · ${item.artistName}`} to={routes.album(item.id)} />;
    case 'playlist':
      return <QuickTile {...common} title={item.title} subtitle={`Playlist · ${item.trackCount ?? 0} tracks`} to={routes.playlist(item.id)} />;
    case 'artist':
      return <QuickTile {...common} title={item.name} subtitle="Artist" to={routes.artist(item.id)} />;
  }
}

function DailyMixCard({ mix, onOpen }: { mix: DailyMix; onOpen: () => void }) {
  const play = usePlayCollection();
  const playState = useCollectionPlayState('playlist', mix.id);
  return (
    <FeaturedCard
      title={mix.title}
      artist={mix.artistsLine}
      category="Made for you"
      serial={mix.serial}
      art={mix.art}
      cover={mix.coverUrl}
      meta={[`${mix.trackCount ?? 0} tracks`, formatDuration(mix.durationSec)]}
      badges={['featured']}
      state={playState === 'playing' ? 'playing' : 'default'}
      width="100%"
      onPlay={() => void play('playlist', mix.id)}
      onOpen={onOpen}
    />
  );
}

function HomeSkeleton() {
  return (
    <div className={styles.skeleton} aria-busy="true" aria-label="Loading home">
      <div className={styles.quickGrid}>
        {Array.from({ length: 6 }, (_, i) => (
          <Skeleton key={i} shape="rect" height={64} radius="md" />
        ))}
      </div>
      <div className={styles.cardGrid}>
        {Array.from({ length: 6 }, (_, i) => (
          <MediaCard key={i} title="" state="loading" />
        ))}
      </div>
      <div className={styles.trackList}>
        {Array.from({ length: 4 }, (_, i) => (
          <TrackRow key={i} index={i + 1} title="" artist="" duration={0} state="loading" />
        ))}
      </div>
    </div>
  );
}

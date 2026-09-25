import { Link } from 'react-router';

import { albumTypeLabel } from '@/entities/album/model';
import type { Track } from '@/entities/track/model';
import { useCollectionPlayState, usePlayCollection, usePlayerStore, usePlayTrackList } from '@/features/player';
import { routes } from '@/shared/config/routes';
import { cx } from '@/shared/lib/cx';
import { Artwork, Badge, IconButton, MediaCard, Skeleton, TrackRow, type TrackRowState } from '@/shared/ui';
import { CollectionCard } from '@/widgets/collection-card/CollectionCard';
import { SectionHeader } from '@/widgets/section-header/SectionHeader';

import type { SearchResult, SearchType } from '../api/search';

import styles from './SearchPage.module.css';

interface SearchResultsProps {
  data: SearchResult;
  type: SearchType;
  onTab: (type: SearchType) => void;
  stale?: boolean;
}

/** Search results — canvas "Search — results (All)" and the per-type tabs. */
export function SearchResults({ data, type, onTab, stale }: SearchResultsProps) {
  const cls = cx(styles.results, stale && styles.stale);

  if (type === 'tracks') {
    return (
      <div className={cls}>
        <TrackTable tracks={data.tracks} label="Matching tracks" showAlbum />
      </div>
    );
  }
  if (type !== 'all') {
    const items = type === 'artists' ? data.artists : type === 'albums' ? data.albums : data.playlists;
    return (
      <div className={cls}>
        <div className={styles.cardGrid}>
          {items.map((item) => (
            <CollectionCard key={item.id} item={item} />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className={cls}>
      {(data.topResult || data.tracks.length > 0) && (
        <div className={styles.topRow}>
          {data.topResult && (
            <section aria-labelledby="top-result-title" className={styles.sectionTight}>
              <SectionHeader titleId="top-result-title" title="Top result" />
              <TopResultCard result={data.topResult} />
            </section>
          )}
          {data.tracks.length > 0 && (
            <section aria-labelledby="tracks-title" className={styles.sectionTight}>
              <SectionHeader
                titleId="tracks-title"
                title="Tracks"
                action={data.counts.tracks > data.tracks.length ? `All ${data.counts.tracks} tracks` : undefined}
                onAction={() => onTab('tracks')}
              />
              <TrackTable tracks={data.tracks} label="Matching tracks" />
            </section>
          )}
        </div>
      )}

      <Shelf id="artists" title="Artists" items={data.artists} more={data.counts.artists > data.artists.length} onMore={() => onTab('artists')} />
      <Shelf id="albums" title="Albums" items={data.albums} more={data.counts.albums > data.albums.length} onMore={() => onTab('albums')} />
      <Shelf id="playlists" title="Playlists" items={data.playlists} more={data.counts.playlists > data.playlists.length} onMore={() => onTab('playlists')} />
    </div>
  );
}

function Shelf({ id, title, items, more, onMore }: { id: string; title: string; items: SearchResult['artists' | 'albums' | 'playlists']; more: boolean; onMore: () => void }) {
  if (items.length === 0) return null;
  return (
    <section aria-labelledby={`${id}-title`} className={styles.sectionTight}>
      <SectionHeader titleId={`${id}-title`} title={title} action={more ? 'See all' : undefined} onAction={onMore} />
      <div className={styles.cardGrid}>
        {items.map((item) => (
          <CollectionCard key={item.id} item={item} />
        ))}
      </div>
    </section>
  );
}

function TrackTable({ tracks, label, showAlbum }: { tracks: Track[]; label: string; showAlbum?: boolean }) {
  const playList = usePlayTrackList();
  const currentId = usePlayerStore((s) => s.currentTrack?.id);
  const isPlaying = usePlayerStore((s) => s.isPlaying);
  const state = (t: Track): TrackRowState => (!t.available ? 'unavailable' : t.id === currentId ? (isPlaying ? 'playing' : 'paused') : 'default');

  return (
    <div role="table" aria-label={label} className={styles.trackTable}>
      {tracks.map((t, i) => (
        <TrackRow
          key={t.id}
          index={i + 1}
          title={t.title}
          artist={t.artistName}
          album={showAlbum ? t.albumTitle : undefined}
          duration={t.durationSec}
          art={t.art}
          cover={t.coverUrl}
          explicit={t.explicit}
          liked={t.liked}
          state={state(t)}
          onPlay={() => playList(tracks, t)}
        />
      ))}
    </div>
  );
}

function TopResultCard({ result }: { result: NonNullable<SearchResult['topResult']> }) {
  const { item } = result;
  const play = usePlayCollection();
  const playState = useCollectionPlayState(item.kind, item.id);
  const name = item.kind === 'artist' ? item.name : item.title;
  const to = item.kind === 'artist' ? routes.artist(item.id) : item.kind === 'album' ? routes.album(item.id) : routes.playlist(item.id);
  const typeLabel = item.kind === 'artist' ? 'Artist' : item.kind === 'album' ? `${item.albumType ? albumTypeLabel[item.albumType] : 'Album'} · ${item.artistName}` : 'Playlist';
  const playing = playState === 'playing';

  return (
    <div className={styles.topCard}>
      <div className={cx(styles.topArt, item.kind === 'artist' && styles.topArtCircle)}>
        <Artwork art={item.art} src={item.coverUrl} shape={item.kind === 'artist' ? 'circle' : 'square'} alt={name} />
      </div>
      <div className={styles.topFoot}>
        <div className={styles.topText}>
          <Link to={to} className={`t-h2 ${styles.topName}`}>
            {name}
          </Link>
          <div className={styles.topMeta}>
            {result.verified && (
              <Badge tone="info" icon="check-circle">
                Verified
              </Badge>
            )}
            <span className="t-body-sm t-secondary">{typeLabel}</span>
            {result.monthlyListeners && <span className="ic-meta">{result.monthlyListeners} monthly</span>}
          </div>
        </div>
        <IconButton icon={playing ? 'pause' : 'play'} label={`${playing ? 'Pause' : 'Play'} ${name}`} variant="play" size="lg" onClick={() => void play(item.kind, item.id)} />
      </div>
    </div>
  );
}

/** Loading layout — canvas "Search results — loading". */
export function SearchSkeleton({ type }: { type: SearchType }) {
  if (type === 'tracks') {
    return (
      <div aria-busy="true" aria-label="Loading results" className={styles.trackTable}>
        {Array.from({ length: 8 }, (_, i) => (
          <TrackRow key={i} index={i + 1} title="" artist="" duration={0} state="loading" />
        ))}
      </div>
    );
  }
  const cards = (
    <div className={styles.cardGrid}>
      {Array.from({ length: 6 }, (_, i) => (
        <MediaCard key={i} title="" state="loading" />
      ))}
    </div>
  );
  if (type !== 'all') return <div aria-busy="true" aria-label="Loading results">{cards}</div>;

  return (
    <div aria-busy="true" aria-label="Loading results" className={styles.results}>
      <div className={styles.topRow}>
        <section className={styles.sectionTight}>
          <div className={styles.skeletonHead}>
            <Skeleton width={120} height={20} />
          </div>
          <div className={styles.topCard}>
            <Skeleton shape="circle" width={112} height={112} />
            <div className={styles.skeletonLines}>
              <Skeleton width="55%" height={24} />
              <Skeleton width="35%" />
            </div>
          </div>
        </section>
        <section className={styles.sectionTight}>
          <div className={styles.skeletonHead}>
            <Skeleton width={80} height={20} />
          </div>
          <div className={styles.trackTable}>
            {Array.from({ length: 4 }, (_, i) => (
              <TrackRow key={i} index={i + 1} title="" artist="" duration={0} state="loading" />
            ))}
          </div>
        </section>
      </div>
      <section className={styles.sectionTight}>
        <div className={styles.skeletonHead}>
          <Skeleton width={96} height={20} />
        </div>
        {cards}
      </section>
    </div>
  );
}

import { useMemo, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router';

import { albumTypeLabel } from '@/entities/album/model';
import type { Track } from '@/entities/track/model';
import { usePlayerStore, useSourcePlayback, type PlaybackSource } from '@/features/player';
import { ApiError } from '@/shared/api/errors';
import { routes } from '@/shared/config/routes';
import { cx } from '@/shared/lib/cx';
import { formatDuration } from '@/shared/lib/formatTime';
import { Artwork, Avatar, Badge, Button, IconButton, MediaCard, SegmentedControl, Skeleton, Tag, TrackListHeader, TrackRow, type TrackRowState } from '@/shared/ui';
import { EmptyState } from '@/shared/ui/patterns/EmptyState';
import { InlineAlert } from '@/shared/ui/patterns/InlineAlert';
import { CollectionCard } from '@/widgets/collection-card/CollectionCard';
import { PageToolbar } from '@/widgets/page-toolbar/PageToolbar';
import { SectionHeader } from '@/widgets/section-header/SectionHeader';

import { useAlbumPage, type AlbumPage as AlbumPageData } from '../api/albumPage';

import styles from './AlbumPage.module.css';

const LATER = 'Available in a later iteration';

type ListView = 'list' | 'compact';
const VIEW_OPTIONS = [
  { value: 'list' as const, icon: 'menu' as const, label: 'List' },
  { value: 'compact' as const, icon: 'queue' as const, label: 'Compact' },
];

const plays = new Intl.NumberFormat('en-US');
const releaseLong = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' });

/** Album — canvas Album.dc.html ("Album — Prism Hours"). */
export function AlbumPage() {
  const { albumId = '' } = useParams();
  const page = useAlbumPage(albumId);
  const navigate = useNavigate();

  const title = page.data?.album.title;
  const crumbs = title ? [{ label: 'Home', to: routes.home }, { label: title }] : undefined;

  return (
    <div className={styles.page}>
      <div aria-hidden="true" className={styles.glow} />
      <PageToolbar crumbs={crumbs} />
      <div className={styles.content}>
        {page.isPending && <AlbumSkeleton />}
        {page.isError &&
          (page.error instanceof ApiError && page.error.isNotFound ? (
            <EmptyState
              icon="alert"
              title="Album not found"
              description="It may have been removed from the catalogue, or the link is incorrect."
              action={
                <Button variant="secondary" iconStart="home" onClick={() => navigate(routes.home)}>
                  Back to Home
                </Button>
              }
            />
          ) : (
            <InlineAlert
              title="Album isn't responding"
              description="ICYRE couldn't load this album. Your library still works."
              onRetry={() => void page.refetch()}
            />
          ))}
        {page.data && <AlbumContent data={page.data} />}
      </div>
    </div>
  );
}

function AlbumContent({ data }: { data: AlbumPageData }) {
  const { album, artist, tracks } = data;
  const source = useMemo<PlaybackSource>(() => ({ kind: 'album', id: album.id }), [album.id]);
  const { state, playAll, shuffleAll, playTrack } = useSourcePlayback(source, tracks);
  const [view, setView] = useState<ListView>('list');
  const playing = state === 'playing';

  return (
    <>
      <section aria-labelledby="album-title" className={styles.hero}>
        <div className={styles.cover}>
          <div className={styles.coverInner}>
            <Artwork art={album.art} src={album.coverUrl} alt={`${album.title} cover`} />
          </div>
        </div>
        <div className={styles.heroText}>
          <div className={styles.kicker}>
            <span className="ic-overline">{albumTypeLabel[album.albumType]}</span>
            {album.serial && <span className="ic-meta">{album.serial}</span>}
          </div>
          <h1 id="album-title" className={`t-display ${styles.title}`}>
            {album.title}
          </h1>
          <div className={styles.byline}>
            <Avatar name={artist.name} src={artist.avatarUrl} size="xs" art={artist.art} />
            <Link to={routes.artist(artist.id)} className={styles.artistLink}>
              {artist.name}
            </Link>
            <span className="ic-meta">
              · {album.year} · {album.trackCount} tracks · {formatDuration(album.durationSec)}
            </span>
          </div>
          {(album.tags.length > 0 || album.hiRes) && (
            <div className={styles.tags}>
              {album.tags.map((t) => (
                <Tag key={t}>{t}</Tag>
              ))}
              {album.hiRes && <Badge tone="info">Hi-Res</Badge>}
            </div>
          )}
        </div>
      </section>

      <div className={styles.actions}>
        <Button variant="iridescent" size="lg" iconStart={playing ? 'pause' : 'play'} onClick={playAll}>
          {playing ? 'Pause' : 'Play'}
        </Button>
        <Button variant="secondary" size="lg" iconStart="shuffle" onClick={shuffleAll}>
          Shuffle
        </Button>
        <IconButton icon="plus" label={`Save ${album.title} to library`} size="lg" disabled title={LATER} />
        <IconButton icon="more" label={`More options for ${album.title}`} size="lg" disabled title={LATER} />
        <div className={styles.spacer} />
        <SegmentedControl<ListView> size="sm" label="Track list view" options={VIEW_OPTIONS} value={view} onChange={setView} />
      </div>

      <section aria-label="Tracks" role="table" className={cx(styles.tracks, view === 'compact' && styles.compact)}>
        <TrackListHeader showCover={false} albumLabel="Plays" />
        {tracks.map((t, i) => (
          <AlbumTrackRow key={t.id} track={t} index={i + 1} onPlay={() => playTrack(t)} />
        ))}
        <div className={styles.release}>
          <span>Released {releaseLong.format(new Date(album.releaseDate))}</span>
          {album.copyright && <span>{album.copyright}</span>}
        </div>
      </section>

      {data.moreByArtist.length > 0 && (
        <section aria-labelledby="more-title" className={styles.more}>
          <MoreHeader artistId={artist.id} artistName={artist.name} />
          <div className={styles.cardGrid}>
            {data.moreByArtist.map((a) => (
              <CollectionCard key={a.id} item={a} albumSubtitle="type" />
            ))}
          </div>
        </section>
      )}
    </>
  );
}

function MoreHeader({ artistId, artistName }: { artistId: string; artistName: string }) {
  const navigate = useNavigate();
  return <SectionHeader titleId="more-title" title={`More by ${artistName}`} action="Discography" onAction={() => navigate(routes.artist(artistId))} />;
}

function AlbumTrackRow({ track, index, onPlay }: { track: Track; index: number; onPlay: () => void }) {
  const isCurrent = usePlayerStore((s) => s.currentTrack?.id === track.id);
  const isPlaying = usePlayerStore((s) => s.isPlaying);
  const state: TrackRowState = !track.available ? 'unavailable' : isCurrent ? (isPlaying ? 'playing' : 'paused') : 'default';
  return (
    <TrackRow
      showCover={false}
      index={index}
      title={track.title}
      artist={track.artistName}
      album={track.available ? (track.plays != null ? plays.format(track.plays) : '') : 'Not available in your region'}
      duration={track.durationSec}
      explicit={track.explicit}
      liked={track.liked}
      state={state}
      onPlay={onPlay}
    />
  );
}

function AlbumSkeleton() {
  return (
    <div aria-busy="true" aria-label="Loading album" className={styles.skeleton}>
      <div className={styles.hero}>
        <div className={styles.cover}>
          <Skeleton shape="rect" width="100%" height={230} radius="md" />
        </div>
        <div className={styles.heroText}>
          <Skeleton width={80} />
          <Skeleton width="50%" height={48} />
          <Skeleton width="35%" />
        </div>
      </div>
      <div className={styles.tracks}>
        {Array.from({ length: 6 }, (_, i) => (
          <TrackRow key={i} index={i + 1} title="" artist="" duration={0} state="loading" showCover={false} />
        ))}
      </div>
      <div className={styles.cardGrid}>
        {Array.from({ length: 6 }, (_, i) => (
          <MediaCard key={i} title="" state="loading" />
        ))}
      </div>
    </div>
  );
}

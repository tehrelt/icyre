import { useMemo, useState } from 'react';
import { useLocation, useNavigate, useSearchParams } from 'react-router';

import {
  useFollowedArtists,
  useLibrarySummary,
  useLikedTracks,
  useSavedAlbums,
  type FollowedArtistItem,
  type LikedTrackItem,
  type SavedAlbumItem,
} from '@/entities/library/api';
import { useMyPlaylists, type MyPlaylist } from '@/entities/playlist/api';
import { useCurrentUser } from '@/entities/user/api';
import { useArtistFollow, useFollowStore } from '@/features/follow-artist';
import { LikeableTrackRow, useLikeStore } from '@/features/like-track';
import { usePlayerStore, useSourcePlayback, type PlaybackSource } from '@/features/player';
import { useAlbumSave, useAlbumSaveStore } from '@/features/save-album';
import { ApiError } from '@/shared/api/errors';
import { routes } from '@/shared/config/routes';
import { cx } from '@/shared/lib/cx';
import { formatDuration } from '@/shared/lib/formatTime';
import {
  Avatar,
  Button,
  ContextMenu,
  Dropdown,
  Icon,
  IconButton,
  MediaCard,
  Menu,
  MenuItem,
  SearchInput,
  SegmentedControl,
  Skeleton,
  Surface,
  Tabs,
  TrackListHeader,
  TrackRow,
  type TabItem,
  type TrackRowState,
} from '@/shared/ui';
import { EmptyState } from '@/shared/ui/patterns/EmptyState';
import { InlineAlert } from '@/shared/ui/patterns/InlineAlert';
import { CollectionCard } from '@/widgets/collection-card/CollectionCard';
import { PageToolbar } from '@/widgets/page-toolbar/PageToolbar';
import { SectionHeader } from '@/widgets/section-header/SectionHeader';

import styles from './LibraryPage.module.css';

export type LibraryTab = 'all' | 'playlists' | 'albums' | 'artists' | 'tracks';
type SortOrder = 'recent' | 'alpha';
type ListView = 'list' | 'compact';

const LIBRARY_TABS: readonly LibraryTab[] = ['all', 'playlists', 'albums', 'artists', 'tracks'];
const SORT_OPTIONS = [
  { value: 'recent' as const, label: 'Recent' },
  { value: 'alpha' as const, label: 'A–Z' },
];
const VIEW_OPTIONS = [
  { value: 'list' as const, icon: 'menu' as const, label: 'List' },
  { value: 'compact' as const, icon: 'queue' as const, label: 'Compact' },
];

const OVERVIEW_LIMIT = 6;
const SPOTLIGHT_TRACKS = 3;

function parseTab(raw: string | null): LibraryTab {
  return (LIBRARY_TABS as readonly string[]).includes(raw ?? '') ? (raw as LibraryTab) : 'all';
}

function matchesQuery(query: string, ...fields: Array<string | undefined>): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return fields.some((f) => f?.toLowerCase().includes(q));
}

function sortByTitle<T>(items: T[], getTitle: (item: T) => string, sort: SortOrder): T[] {
  if (sort === 'recent') return items;
  return [...items].sort((a, b) => getTitle(a).localeCompare(getTitle(b)));
}

export interface LibraryPageProps {
  mode?: 'library' | 'liked';
}

/** Library (/library) and Liked tracks (/liked) screens. */
export function LibraryPage({ mode = 'library' }: LibraryPageProps) {
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const location = useLocation();
  const user = useCurrentUser();
  const signedIn = Boolean(user.data);

  const [filter, setFilter] = useState('');
  const [sort, setSort] = useState<SortOrder>('recent');
  const [view, setView] = useState<ListView>('list');

  const activeTab: LibraryTab = mode === 'liked' ? 'tracks' : parseTab(params.get('tab'));
  const setTab = (next: LibraryTab) => {
    const updated = new URLSearchParams(params);
    if (next === 'all') updated.delete('tab');
    else updated.set('tab', next);
    setParams(updated, { replace: true });
  };

  const summary = useLibrarySummary();
  const playlistsQuery = useMyPlaylists();
  const albumsQuery = useSavedAlbums(signedIn);
  const artistsQuery = useFollowedArtists(user.data?.id);
  const tracksQuery = useLikedTracks(signedIn);

  const likeOverrides = useLikeStore((s) => s.overrides);
  const albumOverrides = useAlbumSaveStore((s) => s.overrides);
  const followOverrides = useFollowStore((s) => s.overrides);

  const playlists = useMemo(() => {
    const raw = playlistsQuery.data ?? [];
    const filtered = raw.filter((p) => matchesQuery(filter, p.title, p.owner));
    return sortByTitle(filtered, (p) => p.title, sort);
  }, [playlistsQuery.data, filter, sort]);

  const albums = useMemo(() => {
    const raw = (albumsQuery.data?.pages.flatMap((p) => p.items) ?? []).filter((a) => albumOverrides[a.id] !== false);
    const filtered = raw.filter((a) => matchesQuery(filter, a.title, a.artistName));
    return sortByTitle(filtered, (a) => a.title, sort);
  }, [albumsQuery.data, albumOverrides, filter, sort]);

  const artists = useMemo(() => {
    const raw = (artistsQuery.data?.pages.flatMap((p) => p.items) ?? []).filter((a) => followOverrides[a.id] !== false);
    const filtered = raw.filter((a) => matchesQuery(filter, a.name));
    return sortByTitle(filtered, (a) => a.name, sort);
  }, [artistsQuery.data, followOverrides, filter, sort]);

  const tracks = useMemo(() => {
    const raw = (tracksQuery.data?.pages.flatMap((p) => p.items) ?? []).filter((t) => likeOverrides[t.id] !== false);
    const filtered = raw.filter((t) => matchesQuery(filter, t.title, t.artistName, t.albumTitle));
    return sortByTitle(filtered, (t) => t.title, sort);
  }, [tracksQuery.data, likeOverrides, filter, sort]);

  const likedSource = useMemo<PlaybackSource>(() => ({ kind: 'library', id: 'liked-tracks' }), []);
  const { state: likedPlayState, playAll, shuffleAll, playTrack } = useSourcePlayback(likedSource, tracks);

  const isUnauthorized = user.isError && user.error instanceof ApiError && user.error.status === 401;
  const isSignedOut = !user.isPending && (!user.data || isUnauthorized);

  const isLoading =
    user.isPending ||
    (signedIn &&
      (mode === 'liked'
        ? tracksQuery.isPending
        : playlistsQuery.isPending || albumsQuery.isPending || artistsQuery.isPending || tracksQuery.isPending));

  const hasError =
    signedIn &&
    (mode === 'liked'
      ? tracksQuery.isError
      : playlistsQuery.isError || albumsQuery.isError || artistsQuery.isError || tracksQuery.isError);

  const retryAll = () => {
    if (playlistsQuery.isError) void playlistsQuery.refetch();
    if (albumsQuery.isError) void albumsQuery.refetch();
    if (artistsQuery.isError) void artistsQuery.refetch();
    if (tracksQuery.isError) void tracksQuery.refetch();
  };

  const albumsCount = albumsQuery.hasNextPage ? (summary.data?.savedCount ?? albums.length) : albums.length;
  const tracksCount = tracksQuery.hasNextPage ? (summary.data?.likedTracksCount ?? tracks.length) : tracks.length;
  const totalLikedDuration = tracks.reduce((sum, t) => sum + t.durationSec, 0);

  const crumbs =
    mode === 'liked'
      ? [{ label: 'Library', to: routes.library }, { label: 'Liked tracks' }]
      : [{ label: 'Home', to: routes.home }, { label: 'Library' }];

  const tabs: TabItem<LibraryTab>[] = [
    { id: 'all', label: 'All' },
    { id: 'playlists', label: 'Playlists', count: playlists.length },
    { id: 'albums', label: 'Albums', count: albumsCount },
    { id: 'artists', label: 'Artists', count: artists.length },
    { id: 'tracks', label: 'Liked tracks', count: tracksCount },
  ];

  return (
    <div className={styles.page}>
      {mode === 'liked' && <div aria-hidden="true" className={styles.glow} />}
      <PageToolbar crumbs={crumbs} />
      <div className={styles.content}>
        {isSignedOut ? (
          <SignedOutLibrary
            mode={mode}
            onSignIn={() => navigate(routes.login, { state: { from: `${location.pathname}${location.search}` } })}
          />
        ) : isLoading ? (
          <LibrarySkeleton mode={mode} />
        ) : hasError ? (
          <InlineAlert
            title="Library isn't responding"
            description="ICYRE couldn't load your saved collection right now."
            onRetry={retryAll}
          />
        ) : mode === 'liked' ? (
          <LikedTracksView
            ownerName={user.data?.displayName ?? 'You'}
            ownerAvatar={user.data?.avatarUrl}
            ownerArt={user.data?.art ?? 0}
            tracks={tracks}
            tracksCount={tracksCount}
            totalDuration={totalLikedDuration}
            filter={filter}
            onFilterChange={setFilter}
            sort={sort}
            onSortChange={setSort}
            view={view}
            onViewChange={setView}
            playing={likedPlayState === 'playing'}
            onPlayAll={playAll}
            onShuffleAll={shuffleAll}
            onPlayTrack={playTrack}
            hasMore={Boolean(tracksQuery.hasNextPage)}
            loadingMore={tracksQuery.isFetchingNextPage}
            onLoadMore={() => void tracksQuery.fetchNextPage()}
            onExplore={() => navigate(routes.home)}
          />
        ) : (
          <>
            <section className={styles.head}>
              <div className={styles.headText}>
                <span className="ic-overline">Personal archive</span>
                <h1 className="t-h1">Your library</h1>
                <span className="ic-meta">
                  {playlists.length} playlists · {albumsCount} albums · {artists.length} artists · {tracksCount} liked tracks
                </span>
              </div>
              <div className={styles.filterField}>
                <SearchInput
                  size="md"
                  label="Filter library"
                  placeholder="Filter playlists, albums, artists or tracks"
                  shortcut=""
                  value={filter}
                  onChange={setFilter}
                />
              </div>
            </section>

            <div className={styles.controls}>
              <Tabs<LibraryTab> size="lg" label="Library sections" items={tabs} value={activeTab} onChange={setTab} />
              <div className={styles.controlsRight}>
                <SegmentedControl<SortOrder> size="sm" label="Sort library" options={SORT_OPTIONS} value={sort} onChange={setSort} />
              </div>
            </div>

            {activeTab === 'all' && (
              <LibraryOverview
                playlists={playlists}
                albums={albums}
                artists={artists}
                tracks={tracks}
                tracksCount={tracksCount}
                filter={filter}
                playing={likedPlayState === 'playing'}
                onPlayLiked={playAll}
                onPlayTrack={playTrack}
                onTab={setTab}
                onOpenLiked={() => navigate(routes.liked)}
                onClearFilter={() => setFilter('')}
                onExplore={() => navigate(routes.home)}
              />
            )}

            {activeTab === 'playlists' && (
              <PlaylistsTab playlists={playlists} filter={filter} onClearFilter={() => setFilter('')} onExplore={() => navigate(routes.home)} />
            )}

            {activeTab === 'albums' && (
              <AlbumsTab
                albums={albums}
                filter={filter}
                hasMore={Boolean(albumsQuery.hasNextPage)}
                loadingMore={albumsQuery.isFetchingNextPage}
                onLoadMore={() => void albumsQuery.fetchNextPage()}
                onClearFilter={() => setFilter('')}
                onExplore={() => navigate(routes.home)}
              />
            )}

            {activeTab === 'artists' && (
              <ArtistsTab
                artists={artists}
                filter={filter}
                hasMore={Boolean(artistsQuery.hasNextPage)}
                loadingMore={artistsQuery.isFetchingNextPage}
                onLoadMore={() => void artistsQuery.fetchNextPage()}
                onClearFilter={() => setFilter('')}
                onExplore={() => navigate(routes.home)}
              />
            )}

            {activeTab === 'tracks' && (
              <TracksTab
                tracks={tracks}
                totalDuration={totalLikedDuration}
                filter={filter}
                view={view}
                onViewChange={setView}
                playing={likedPlayState === 'playing'}
                onPlayAll={playAll}
                onShuffleAll={shuffleAll}
                onPlayTrack={playTrack}
                hasMore={Boolean(tracksQuery.hasNextPage)}
                loadingMore={tracksQuery.isFetchingNextPage}
                onLoadMore={() => void tracksQuery.fetchNextPage()}
                onClearFilter={() => setFilter('')}
                onExplore={() => navigate(routes.home)}
              />
            )}
          </>
        )}
      </div>
    </div>
  );
}

function SignedOutLibrary({ mode, onSignIn }: { mode: 'library' | 'liked'; onSignIn: () => void }) {
  return (
    <EmptyState
      icon={mode === 'liked' ? 'heart' : 'library'}
      title={mode === 'liked' ? 'Sign in to see your liked tracks' : 'Sign in to view your library'}
      description="Your saved albums, playlists, followed artists and liked tracks live here."
      action={
        <Button variant="iridescent" size="lg" iconStart="user" onClick={onSignIn}>
          Sign in
        </Button>
      }
    />
  );
}

interface OverviewProps {
  playlists: MyPlaylist[];
  albums: SavedAlbumItem[];
  artists: FollowedArtistItem[];
  tracks: LikedTrackItem[];
  tracksCount: number;
  filter: string;
  playing: boolean;
  onPlayLiked: () => void;
  onPlayTrack: (track: LikedTrackItem) => void;
  onTab: (tab: LibraryTab) => void;
  onOpenLiked: () => void;
  onClearFilter: () => void;
  onExplore: () => void;
}

function LibraryOverview({
  playlists,
  albums,
  artists,
  tracks,
  tracksCount,
  filter,
  playing,
  onPlayLiked,
  onPlayTrack,
  onTab,
  onOpenLiked,
  onClearFilter,
  onExplore,
}: OverviewProps) {
  const hasAny = playlists.length > 0 || albums.length > 0 || artists.length > 0 || tracks.length > 0;
  if (!hasAny) {
    return filter.trim() ? (
      <EmptyState
        title={`No matches for “${filter.trim()}”`}
        description="Try another title, artist or playlist name."
        action={
          <Button variant="secondary" onClick={onClearFilter}>
            Clear filter
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon="library"
        title="Your library is empty"
        description="Save albums, follow artists or like tracks to build your collection."
        action={
          <Button variant="secondary" iconStart="home" onClick={onExplore}>
            Explore Home
          </Button>
        }
      />
    );
  }

  const previewTracks = tracks.slice(0, SPOTLIGHT_TRACKS);
  const visiblePlaylists = filter.trim() ? playlists : playlists.slice(0, OVERVIEW_LIMIT);
  const visibleAlbums = filter.trim() ? albums : albums.slice(0, OVERVIEW_LIMIT);
  const visibleArtists = filter.trim() ? artists : artists.slice(0, OVERVIEW_LIMIT);

  return (
    <>
      {previewTracks.length > 0 && (
        <Surface material="collectible" padding="lg">
          <div className={styles.spotlight}>
            <div className={styles.spotlightCopy}>
              <span className="ic-overline">Liked tracks · {tracksCount} saved</span>
              <h2 className="t-h2">Your saved songs</h2>
              <span className="ic-meta">{previewTracks.map((t) => t.title).join(' · ')}</span>
              <div className={styles.spotlightActions}>
                <Button variant="iridescent" size="md" iconStart={playing ? 'pause' : 'play'} onClick={onPlayLiked}>
                  {playing ? 'Pause' : 'Play liked'}
                </Button>
                <Button variant="secondary" size="md" onClick={onOpenLiked}>
                  Open all
                </Button>
              </div>
            </div>
            <div role="table" aria-label="Liked tracks preview" className={styles.spotlightTracks}>
              {previewTracks.map((t, i) => (
                <LibraryTrackRow key={t.id} track={t} index={i + 1} onPlay={() => onPlayTrack(t)} />
              ))}
            </div>
          </div>
        </Surface>
      )}

      {visiblePlaylists.length > 0 && (
        <section aria-labelledby="lib-playlists-title" className={styles.section}>
          <SectionHeader
            titleId="lib-playlists-title"
            title="Playlists"
            action={playlists.length > OVERVIEW_LIMIT ? 'See all' : undefined}
            onAction={() => onTab('playlists')}
          />
          <div className={styles.cardGrid}>
            {visiblePlaylists.map((p) => (
              <CollectionCard key={p.id} item={p} />
            ))}
          </div>
        </section>
      )}

      {visibleAlbums.length > 0 && (
        <section aria-labelledby="lib-albums-title" className={styles.section}>
          <SectionHeader
            titleId="lib-albums-title"
            title="Saved albums"
            action={albums.length > OVERVIEW_LIMIT ? 'See all' : undefined}
            onAction={() => onTab('albums')}
          />
          <div className={styles.cardGrid}>
            {visibleAlbums.map((a) => (
              <LibraryAlbumCard key={a.id} album={a} />
            ))}
          </div>
        </section>
      )}

      {visibleArtists.length > 0 && (
        <section aria-labelledby="lib-artists-title" className={styles.section}>
          <SectionHeader
            titleId="lib-artists-title"
            title="Followed artists"
            action={artists.length > OVERVIEW_LIMIT ? 'See all' : undefined}
            onAction={() => onTab('artists')}
          />
          <div className={styles.cardGrid}>
            {visibleArtists.map((a) => (
              <LibraryArtistCard key={a.id} artist={a} />
            ))}
          </div>
        </section>
      )}
    </>
  );
}

function PlaylistsTab({
  playlists,
  filter,
  onClearFilter,
  onExplore,
}: {
  playlists: MyPlaylist[];
  filter: string;
  onClearFilter: () => void;
  onExplore: () => void;
}) {
  if (playlists.length === 0) {
    return filter.trim() ? (
      <EmptyState
        title={`No playlists matching “${filter.trim()}”`}
        action={
          <Button variant="secondary" onClick={onClearFilter}>
            Clear filter
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon="library"
        title="No playlists yet"
        description="Playlists you create or save will appear here."
        action={
          <Button variant="secondary" iconStart="home" onClick={onExplore}>
            Explore Home
          </Button>
        }
      />
    );
  }
  return (
    <div className={styles.cardGrid}>
      {playlists.map((p) => (
        <CollectionCard key={p.id} item={p} />
      ))}
    </div>
  );
}

function AlbumsTab({
  albums,
  filter,
  hasMore,
  loadingMore,
  onLoadMore,
  onClearFilter,
  onExplore,
}: {
  albums: SavedAlbumItem[];
  filter: string;
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
  onClearFilter: () => void;
  onExplore: () => void;
}) {
  if (albums.length === 0) {
    return filter.trim() ? (
      <EmptyState
        title={`No albums matching “${filter.trim()}”`}
        action={
          <Button variant="secondary" onClick={onClearFilter}>
            Clear filter
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon="library"
        title="No saved albums yet"
        description="Save an album from its page to keep it in your library."
        action={
          <Button variant="secondary" iconStart="home" onClick={onExplore}>
            Explore Home
          </Button>
        }
      />
    );
  }
  return (
    <>
      <div className={styles.cardGrid}>
        {albums.map((a) => (
          <LibraryAlbumCard key={a.id} album={a} />
        ))}
      </div>
      {hasMore && (
        <div className={styles.loadMore}>
          <Button variant="secondary" loading={loadingMore} onClick={onLoadMore}>
            Load more albums
          </Button>
        </div>
      )}
    </>
  );
}

function ArtistsTab({
  artists,
  filter,
  hasMore,
  loadingMore,
  onLoadMore,
  onClearFilter,
  onExplore,
}: {
  artists: FollowedArtistItem[];
  filter: string;
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
  onClearFilter: () => void;
  onExplore: () => void;
}) {
  if (artists.length === 0) {
    return filter.trim() ? (
      <EmptyState
        title={`No artists matching “${filter.trim()}”`}
        action={
          <Button variant="secondary" onClick={onClearFilter}>
            Clear filter
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon="user"
        title="No followed artists yet"
        description="Follow artists you listen to and manage them here."
        action={
          <Button variant="secondary" iconStart="home" onClick={onExplore}>
            Explore Home
          </Button>
        }
      />
    );
  }
  return (
    <>
      <div className={styles.cardGrid}>
        {artists.map((a) => (
          <LibraryArtistCard key={a.id} artist={a} />
        ))}
      </div>
      {hasMore && (
        <div className={styles.loadMore}>
          <Button variant="secondary" loading={loadingMore} onClick={onLoadMore}>
            Load more artists
          </Button>
        </div>
      )}
    </>
  );
}

function TracksTab({
  tracks,
  totalDuration,
  filter,
  view,
  onViewChange,
  playing,
  onPlayAll,
  onShuffleAll,
  onPlayTrack,
  hasMore,
  loadingMore,
  onLoadMore,
  onClearFilter,
  onExplore,
}: {
  tracks: LikedTrackItem[];
  totalDuration: number;
  filter: string;
  view: ListView;
  onViewChange: (v: ListView) => void;
  playing: boolean;
  onPlayAll: () => void;
  onShuffleAll: () => void;
  onPlayTrack: (track: LikedTrackItem) => void;
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
  onClearFilter: () => void;
  onExplore: () => void;
}) {
  if (tracks.length === 0) {
    return filter.trim() ? (
      <EmptyState
        title={`No liked tracks matching “${filter.trim()}”`}
        action={
          <Button variant="secondary" onClick={onClearFilter}>
            Clear filter
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon="heart"
        title="No liked tracks yet"
        description="Tap the heart on any track to save it here."
        action={
          <Button variant="secondary" iconStart="home" onClick={onExplore}>
            Explore Home
          </Button>
        }
      />
    );
  }

  return (
    <>
      <div className={styles.actions}>
        <Button variant="iridescent" size="lg" iconStart={playing ? 'pause' : 'play'} onClick={onPlayAll}>
          {playing ? 'Pause' : 'Play liked'}
        </Button>
        <Button variant="secondary" size="lg" iconStart="shuffle" onClick={onShuffleAll}>
          Shuffle
        </Button>
        <span className="ic-meta">
          {tracks.length} tracks · {formatDuration(totalDuration)}
        </span>
        <div className={styles.spacer} />
        <SegmentedControl<ListView> size="sm" label="Track list view" options={VIEW_OPTIONS} value={view} onChange={onViewChange} />
      </div>

      <section aria-label="Liked tracks" role="table" className={cx(styles.tracks, view === 'compact' && styles.compact)}>
        <TrackListHeader showCover albumLabel="Album" />
        {tracks.map((t, i) => (
          <LibraryTrackRow key={t.id} track={t} index={i + 1} onPlay={() => onPlayTrack(t)} />
        ))}
      </section>

      {hasMore && (
        <div className={styles.loadMore}>
          <Button variant="secondary" loading={loadingMore} onClick={onLoadMore}>
            Load more tracks
          </Button>
        </div>
      )}
    </>
  );
}

function LikedTracksView({
  ownerName,
  ownerAvatar,
  ownerArt,
  tracks,
  tracksCount,
  totalDuration,
  filter,
  onFilterChange,
  sort,
  onSortChange,
  view,
  onViewChange,
  playing,
  onPlayAll,
  onShuffleAll,
  onPlayTrack,
  hasMore,
  loadingMore,
  onLoadMore,
  onExplore,
}: {
  ownerName: string;
  ownerAvatar?: string | null;
  ownerArt: number;
  tracks: LikedTrackItem[];
  tracksCount: number;
  totalDuration: number;
  filter: string;
  onFilterChange: (v: string) => void;
  sort: SortOrder;
  onSortChange: (v: SortOrder) => void;
  view: ListView;
  onViewChange: (v: ListView) => void;
  playing: boolean;
  onPlayAll: () => void;
  onShuffleAll: () => void;
  onPlayTrack: (track: LikedTrackItem) => void;
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
  onExplore: () => void;
}) {
  return (
    <>
      <section aria-labelledby="liked-title" className={styles.hero}>
        <div className={styles.likedCover} aria-hidden="true">
          <div className={styles.likedCoverInner}>
            <Icon name="heart" size={56} filled />
          </div>
        </div>
        <div className={styles.heroText}>
          <span className="ic-overline">Collection</span>
          <h1 id="liked-title" className="t-display">
            Liked tracks
          </h1>
          <div className={styles.byline}>
            <Avatar name={ownerName} src={ownerAvatar} size="xs" art={ownerArt} />
            <span className={styles.ownerName}>{ownerName}</span>
            <span className="ic-meta">
              · {tracksCount} tracks · {formatDuration(totalDuration)}
            </span>
          </div>
        </div>
      </section>

      <div className={styles.actions}>
        <Button variant="iridescent" size="lg" iconStart={playing ? 'pause' : 'play'} disabled={tracks.length === 0} onClick={onPlayAll}>
          {playing ? 'Pause' : 'Play'}
        </Button>
        <Button variant="secondary" size="lg" iconStart="shuffle" disabled={tracks.length === 0} onClick={onShuffleAll}>
          Shuffle
        </Button>
        <div className={styles.spacer} />
        <div className={styles.filterField}>
          <SearchInput size="md" label="Filter liked tracks" placeholder="Filter liked tracks" shortcut="" value={filter} onChange={onFilterChange} />
        </div>
        <SegmentedControl<SortOrder> size="sm" label="Sort liked tracks" options={SORT_OPTIONS} value={sort} onChange={onSortChange} />
        <SegmentedControl<ListView> size="sm" label="Track list view" options={VIEW_OPTIONS} value={view} onChange={onViewChange} />
      </div>

      {tracks.length === 0 ? (
        filter.trim() ? (
          <EmptyState
            title={`No liked tracks matching “${filter.trim()}”`}
            action={
              <Button variant="secondary" onClick={() => onFilterChange('')}>
                Clear filter
              </Button>
            }
          />
        ) : (
          <EmptyState
            icon="heart"
            title="No liked tracks yet"
            description="Tap the heart on any track to save it here."
            action={
              <Button variant="secondary" iconStart="home" onClick={onExplore}>
                Explore Home
              </Button>
            }
          />
        )
      ) : (
        <>
          <section aria-label="Liked tracks" role="table" className={cx(styles.tracks, view === 'compact' && styles.compact)}>
            <TrackListHeader showCover albumLabel="Album" />
            {tracks.map((t, i) => (
              <LibraryTrackRow key={t.id} track={t} index={i + 1} onPlay={() => onPlayTrack(t)} />
            ))}
          </section>
          {hasMore && (
            <div className={styles.loadMore}>
              <Button variant="secondary" loading={loadingMore} onClick={onLoadMore}>
                Load more tracks
              </Button>
            </div>
          )}
        </>
      )}
    </>
  );
}

function LibraryAlbumCard({ album }: { album: SavedAlbumItem }) {
  const navigate = useNavigate();
  const { toggle, pending } = useAlbumSave(album.id, true);
  const menu = (
    <Menu label={`Options for ${album.title}`} autoFocus>
      <MenuItem label="Open album" icon="library" onSelect={() => navigate(routes.album(album.id))} />
      <MenuItem label="Remove from library" icon="close" danger disabled={pending} onSelect={() => void toggle()} />
    </Menu>
  );

  return (
    <ContextMenu menu={menu}>
      <div className={styles.cardWrap}>
        <CollectionCard item={album} />
        <div className={styles.cardMenu}>
          <Dropdown align="end" trigger={<IconButton icon="more" size="sm" label={`Options for ${album.title}`} />}>
            {menu}
          </Dropdown>
        </div>
      </div>
    </ContextMenu>
  );
}

function LibraryArtistCard({ artist }: { artist: FollowedArtistItem }) {
  const navigate = useNavigate();
  const { toggle, pending } = useArtistFollow(artist.id, true);
  const menu = (
    <Menu label={`Options for ${artist.name}`} autoFocus>
      <MenuItem label="Open artist" icon="user" onSelect={() => navigate(routes.artist(artist.id))} />
      <MenuItem label="Unfollow" icon="close" danger disabled={pending} onSelect={() => void toggle()} />
    </Menu>
  );

  return (
    <ContextMenu menu={menu}>
      <div className={styles.cardWrap}>
        <CollectionCard item={artist} />
        <div className={styles.cardMenu}>
          <Dropdown align="end" trigger={<IconButton icon="more" size="sm" label={`Options for ${artist.name}`} />}>
            {menu}
          </Dropdown>
        </div>
      </div>
    </ContextMenu>
  );
}

function LibraryTrackRow({ track, index, onPlay }: { track: LikedTrackItem; index: number; onPlay: () => void }) {
  const isCurrent = usePlayerStore((s) => s.currentTrack?.id === track.id);
  const isPlaying = usePlayerStore((s) => s.isPlaying);
  const state: TrackRowState = !track.available ? 'unavailable' : isCurrent ? (isPlaying ? 'playing' : 'paused') : 'default';
  return (
    <LikeableTrackRow
      track={track}
      known
      index={index}
      title={track.title}
      artist={track.artistName}
      album={track.albumTitle}
      duration={track.durationSec}
      art={track.art}
      cover={track.coverUrl}
      explicit={track.explicit}
      state={state}
      onPlay={onPlay}
    />
  );
}

function LibrarySkeleton({ mode }: { mode: 'library' | 'liked' }) {
  return (
    <div aria-busy="true" aria-label={mode === 'liked' ? 'Loading liked tracks' : 'Loading library'} className={styles.skeleton}>
      <div className={styles.head}>
        <div className={styles.headText}>
          <Skeleton width={110} />
          <Skeleton width={240} height={40} />
          <Skeleton width={280} />
        </div>
        <Skeleton shape="rect" width={320} height={40} radius="md" />
      </div>
      {mode === 'library' && (
        <div className={styles.cardGrid}>
          {Array.from({ length: 6 }, (_, i) => (
            <MediaCard key={i} title="" state="loading" />
          ))}
        </div>
      )}
      <div className={styles.tracks}>
        {Array.from({ length: 5 }, (_, i) => (
          <TrackRow key={i} index={i + 1} title="" artist="" duration={0} state="loading" />
        ))}
      </div>
    </div>
  );
}

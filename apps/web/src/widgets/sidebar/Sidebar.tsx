import { Link, useLocation } from 'react-router';

import { useLibrarySummary } from '@/entities/library/api';
import { routes } from '@/shared/config/routes';
import { Artwork, Logo, SidebarItem, Skeleton } from '@/shared/ui';

import styles from './Sidebar.module.css';

/** Shell / Sidebar from the product canvas (CSidebar). */
export function Sidebar() {
  const { pathname } = useLocation();
  const library = useLibrarySummary();
  const isActive = (to: string) => (to === routes.home ? pathname === to : pathname.startsWith(to));

  return (
    <div className={styles.sidebar}>
      <Link to={routes.home} aria-label="ICYRE home" className={styles.logo}>
        <span className={styles.logoFull}>
          <Logo height={26} />
        </span>
        <span className={styles.logoSymbol}>
          <Logo variant="symbol" height={28} />
        </span>
      </Link>

      <nav aria-label="Main" className={styles.group}>
        <SidebarItem icon="home" label="Home" to={routes.home} active={isActive(routes.home)} />
        <SidebarItem icon="search" label="Search" to={routes.search} active={isActive(routes.search)} />
        <SidebarItem icon="library" label="Library" to={routes.library} count={library.data?.savedCount} active={isActive(routes.library)} />
        <SidebarItem icon="heart" label="Liked tracks" to={routes.liked} count={library.data?.likedTracksCount} active={isActive(routes.liked)} />
      </nav>

      <div className={styles.playlistsHead}>
        <span className="ic-overline">Playlists</span>
        {library.data && <span className="ic-meta">{library.data.playlists.length}</span>}
      </div>
      <nav aria-label="Playlists" className={`${styles.group} ${styles.playlists}`}>
        <SidebarItem icon="plus" label="Create playlist" disabled />
        {library.isPending
          ? Array.from({ length: 4 }, (_, i) => (
              <div key={i} className={styles.skeletonRow}>
                <Skeleton shape="rect" width={28} height={28} radius="xs" />
                <Skeleton width="60%" />
              </div>
            ))
          : library.data?.playlists.map((p) => {
              const to = routes.playlist(p.id);
              return <SidebarItem key={p.id} label={p.title} to={to} art={<Artwork art={p.art} src={p.coverUrl} />} active={pathname === to} />;
            })}
      </nav>

      <div className={styles.footer}>
        <SidebarItem icon="user" label="Profile" to={routes.profile} active={isActive(routes.profile)} />
        <SidebarItem icon="settings" label="Settings" to={routes.settings} active={isActive(routes.settings)} />
      </div>
    </div>
  );
}

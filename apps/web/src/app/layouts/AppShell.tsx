import { Outlet } from 'react-router';

import { MobileNav } from '@/widgets/mobile-nav/MobileNav';
import { PlayerBar } from '@/widgets/player-bar/PlayerBar';
import { Sidebar } from '@/widgets/sidebar/Sidebar';

import styles from './AppShell.module.css';

/**
 * Application shell (canvas: Home / Album layout): sidebar, scrolling page,
 * global player. Pages render into the outlet; the player stays mounted.
 */
export function AppShell() {
  return (
    <div className={styles.shell}>
      <aside className={styles.sidebar}>
        <Sidebar />
      </aside>
      <main className={styles.main} id="main">
        <Outlet />
      </main>
      <div className={styles.player}>
        <PlayerBar />
      </div>
      <div className={styles.mobileNav}>
        <MobileNav />
      </div>
    </div>
  );
}

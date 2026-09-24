import { useLocation } from 'react-router';

import { routes } from '@/shared/config/routes';
import { MobileNavItem } from '@/shared/ui';

import styles from './MobileNav.module.css';

const ITEMS = [
  { icon: 'home', label: 'Home', to: routes.home },
  { icon: 'search', label: 'Search', to: routes.search },
  { icon: 'library', label: 'Library', to: routes.library },
  { icon: 'user', label: 'Profile', to: routes.profile },
] as const;

/** Mobile bottom navigation (Design System MobileNavItem; replaces the sidebar < 768px). */
export function MobileNav() {
  const { pathname } = useLocation();
  return (
    <nav aria-label="Main" className={styles.nav}>
      {ITEMS.map((it) => (
        <MobileNavItem
          key={it.to}
          icon={it.icon}
          label={it.label}
          to={it.to}
          active={it.to === routes.home ? pathname === it.to : pathname.startsWith(it.to)}
        />
      ))}
    </nav>
  );
}

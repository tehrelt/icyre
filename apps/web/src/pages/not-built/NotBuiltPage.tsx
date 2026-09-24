import { Link, useLocation } from 'react-router';

import { routes } from '@/shared/config/routes';
import { PageToolbar } from '@/widgets/page-toolbar/PageToolbar';

import styles from './NotBuiltPage.module.css';

/**
 * Placeholder for routes whose screens are scheduled in later frontend epics
 * (Album, Search, Library…). Keeps navigation inside the shell working.
 */
export function NotBuiltPage() {
  const { pathname } = useLocation();
  return (
    <>
      <PageToolbar />
      <section className={styles.page}>
        <span className="ic-overline">Coming next</span>
        <h1 className="t-h2">This screen isn&apos;t built yet</h1>
        <p className="t-body t-secondary">
          <code className="ic-meta">{pathname}</code> arrives in a later iteration. <Link to={routes.home}>Back to Home</Link>
        </p>
      </section>
    </>
  );
}

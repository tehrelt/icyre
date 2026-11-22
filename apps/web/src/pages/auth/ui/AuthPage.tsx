import { useEffect, useState } from 'react';
import { Link, Navigate, useLocation, useNavigate } from 'react-router';

import { useCurrentUser } from '@/entities/user/api';
import { AuthForm } from '@/features/auth';
import { routes } from '@/shared/config/routes';
import { cx } from '@/shared/lib/cx';
import { formatTime } from '@/shared/lib/formatTime';
import { Artwork, FeaturedCard, Logo, PlaybackState, SegmentedControl, Surface } from '@/shared/ui';

import styles from './AuthPage.module.css';

export interface AuthPageProps {
  mode: 'login' | 'register';
}

const AUTH_MODES = [
  { value: 'login' as const, label: 'Sign in' },
  { value: 'register' as const, label: 'Create account' },
];

const PREVIEW_DURATION_SEC = 227;

export function AuthPage({ mode }: AuthPageProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const user = useCurrentUser();
  const [previewPlaying, setPreviewPlaying] = useState(false);
  const [elapsed, setElapsed] = useState(0);

  useEffect(() => {
    if (!previewPlaying) return;
    const id = window.setInterval(() => {
      setElapsed((prev) => (prev + 1) % PREVIEW_DURATION_SEC);
    }, 1000);
    return () => window.clearInterval(id);
  }, [previewPlaying]);

  const rawFrom = (location.state as { from?: unknown } | null)?.from;
  const from = typeof rawFrom === 'string' && rawFrom.startsWith('/') && !rawFrom.startsWith('//') ? rawFrom : routes.home;

  if (user.data) {
    return <Navigate to={from} replace />;
  }

  const isLogin = mode === 'login';

  return (
    <div className={styles.shell}>
      <header className={styles.topbar}>
        <Link to={routes.home} aria-label="ICYRE home" className={styles.brandLink}>
          <Logo height={28} />
        </Link>
        <div className={styles.statusPill} title="Lossless streaming status">
          <span className={styles.statusDot} aria-hidden="true" />
          <span>24-bit · 96 kHz</span>
        </div>
      </header>

      <main className={styles.stage}>
        <Surface material="elevated" padding="none" className={styles.card}>
          <div className={styles.cardHead}>
            <div className={styles.segmentedWrap}>
              <SegmentedControl
                label="Authentication mode"
                value={mode}
                options={AUTH_MODES}
                onChange={(next) => navigate(next === 'login' ? routes.login : routes.register, { state: location.state })}
              />
            </div>

            <div className={styles.headingGroup}>
              <span className="ic-overline">{isLogin ? 'Listener access' : 'New membership'}</span>
              <h1 className={styles.title}>{isLogin ? 'Welcome back' : 'Create your account'}</h1>
              <p className={styles.subtitle}>
                {isLogin
                  ? 'Sign in to pick up your queue, saved albums and lossless library.'
                  : 'Start a lossless library with synced queues and collectible editions.'}
              </p>
            </div>
          </div>

          <AuthForm mode={mode} onSuccess={() => navigate(from, { replace: true })} />
        </Surface>

        <aside className={styles.showcase} aria-label="Featured release preview">
          <div className={styles.showcaseCopy}>
            <span className="ic-overline">Featured edition · Lossless</span>
            <h2 className={styles.showcaseTitle}>Music, framed like something worth keeping.</h2>
            <p className={styles.showcaseLede}>
              Every album sits in its own quiet sleeve with liner notes, edition numbers and gapless 24-bit playback.
            </p>
          </div>

          <div className={styles.showcaseCenter}>
            <FeaturedCard
              title="Prism Hours"
              artist="Nova Hale"
              category="Featured · Album"
              serial="No. 014"
              art={0}
              meta={['2026 · 11 tracks · 42 min']}
              badges={['featured']}
              state={previewPlaying ? 'playing' : 'default'}
              width={300}
              onPlay={() => setPreviewPlaying((p) => !p)}
              onOpen={() => navigate(routes.album('alb-prism-hours'))}
            />
          </div>

          <div className={cx(styles.showcaseTrack, previewPlaying && styles.showcaseTrackCurrent)}>
            <Artwork art={0} className={styles.showcaseThumb} />
            <div className={styles.showcaseTrackInfo}>
              <div className={styles.showcaseTrackTitle}>Glass Tides</div>
              <div className={styles.showcaseTrackSub}>Nova Hale · Prism Hours</div>
            </div>
            <PlaybackState state={previewPlaying ? 'playing' : 'paused'} size="sm" />
            <span className="ic-meta">
              {formatTime(elapsed)} / {formatTime(PREVIEW_DURATION_SEC)}
            </span>
          </div>
        </aside>
      </main>

      <footer className={styles.footer}>
        <span>ICYRE Audio · Edition 2026</span>
        <Link to={routes.home} className={styles.backLink}>
          Back to ICYRE
        </Link>
      </footer>
    </div>
  );
}

import { useState } from 'react';

import type { Track } from '@/entities/track/model';
import { LikeableTrackRow } from '@/features/like-track';
import { usePlayerStore, usePlayTrackList } from '@/features/player';
import { SegmentedControl, type TrackRowState } from '@/shared/ui';
import { SectionHeader } from '@/widgets/section-header/SectionHeader';

import styles from './HomePage.module.css';

type Period = 'today' | 'week';

const PERIODS = [
  { value: 'today' as const, label: 'Today' },
  { value: 'week' as const, label: 'This week' },
];

/** "Trending on ICYRE": dense track list with a period switch. */
export function TrendingSection({ today, week }: { today: Track[]; week: Track[] }) {
  const [period, setPeriod] = useState<Period>('today');
  const tracks = period === 'today' ? today : week;
  const playList = usePlayTrackList();
  const currentId = usePlayerStore((s) => s.currentTrack?.id);
  const isPlaying = usePlayerStore((s) => s.isPlaying);

  const rowState = (t: Track): TrackRowState => {
    if (!t.available) return 'unavailable';
    if (t.id === currentId) return isPlaying ? 'playing' : 'paused';
    return 'default';
  };

  return (
    <section aria-labelledby="trending-title" className={styles.section}>
      <SectionHeader
        titleId="trending-title"
        title="Trending on ICYRE"
        kicker={period === 'today' ? 'Most played today' : 'Most played this week'}
        aside={<SegmentedControl<Period> size="sm" label="Trending period" options={PERIODS} value={period} onChange={setPeriod} />}
      />
      <div role="table" aria-label="Trending tracks" className={styles.trackList}>
        {tracks.map((t, i) => (
          <LikeableTrackRow
            key={t.id}
            track={t}
            index={i + 1}
            title={t.title}
            artist={t.artistName}
            album={t.albumTitle}
            duration={t.durationSec}
            art={t.art}
            cover={t.coverUrl}
            explicit={t.explicit}
            badge={t.badge}
            state={rowState(t)}
            onPlay={() => playList(tracks, t)}
          />
        ))}
      </div>
    </section>
  );
}

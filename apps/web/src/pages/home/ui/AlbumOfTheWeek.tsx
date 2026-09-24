import { useNavigate } from 'react-router';

import { useCollectionPlayState, usePlayCollection } from '@/features/player';
import { routes } from '@/shared/config/routes';
import { formatDuration } from '@/shared/lib/formatTime';
import { Badge, Button, FeaturedCard, IconButton, Tag } from '@/shared/ui';

import type { AlbumOfTheWeek as AlbumOfTheWeekData } from '../api/homeFeed';

import styles from './HomePage.module.css';

const LATER = 'Available in a later iteration';

const releaseFormat = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' });

/** "Album of the week" editorial block: the one collectible FeaturedCard in view. */
export function AlbumOfTheWeek({ data }: { data: AlbumOfTheWeekData }) {
  const navigate = useNavigate();
  const play = usePlayCollection();
  const playState = useCollectionPlayState('album', data.album.id);
  const playing = playState === 'playing';
  const onPlay = () => void play('album', data.album.id);

  return (
    <section aria-labelledby="aotw-title" className={styles.aotw}>
      <div className={styles.aotwCard}>
        <FeaturedCard
          title={data.album.title}
          artist={data.album.artistName}
          category="Album"
          serial={data.serial}
          art={data.album.art}
          cover={data.album.coverUrl}
          meta={[String(data.album.year ?? ''), `${data.trackCount} tracks`, formatDuration(data.durationSec)].filter(Boolean)}
          badges={['featured', 'new']}
          state={playing ? 'playing' : 'default'}
          width="100%"
          onPlay={onPlay}
          onOpen={() => navigate(routes.album(data.album.id))}
        />
      </div>
      <div className={styles.aotwBody}>
        <span className="ic-overline">Album of the week</span>
        <h2 id="aotw-title" className="t-h2">
          {data.album.title}
        </h2>
        <p className="t-body-lg t-secondary">{data.description}</p>
        <div className={styles.aotwActions}>
          <Button variant="iridescent" size="lg" iconStart={playing ? 'pause' : 'play'} onClick={onPlay}>
            {playing ? 'Pause' : 'Play album'}
          </Button>
          <Button variant="secondary" size="lg" iconStart="plus" disabled title={LATER}>
            Add to library
          </Button>
          <IconButton icon="more" label={`More options for ${data.album.title}`} size="lg" disabled title={LATER} />
        </div>
        <div className={styles.tags}>
          {data.tags.map((t) => (
            <Tag key={t}>{t}</Tag>
          ))}
          {data.hiRes && <Badge tone="info">Hi-Res</Badge>}
        </div>
      </div>
      <dl className={styles.facts}>
        <div>
          <dt className="ic-overline">Label</dt>
          <dd className="t-body-sm">{data.label}</dd>
        </div>
        <div>
          <dt className="ic-overline">Released</dt>
          <dd className={`ic-meta ${styles.factMono}`}>{releaseFormat.format(new Date(data.releaseDate))}</dd>
        </div>
        <div>
          <dt className="ic-overline">Format</dt>
          <dd className={`ic-meta ${styles.factMono}`}>{data.format}</dd>
        </div>
        <div>
          <dt className="ic-overline">Catalog</dt>
          <dd className={`ic-meta ${styles.factMono}`}>{data.catalogNumber}</dd>
        </div>
      </dl>
    </section>
  );
}

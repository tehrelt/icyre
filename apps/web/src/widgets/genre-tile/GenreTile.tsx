import { Link } from 'react-router';

import { Artwork } from '@/shared/ui';

import styles from './GenreTile.module.css';

export interface GenreTileProps {
  label: string;
  /** Pre-formatted, e.g. "2,140 releases". */
  count: string;
  art?: number;
  to: string;
}

/** Media / Genre tile from the product canvas (CGenreTile). */
export function GenreTile({ label, count, art = 0, to }: GenreTileProps) {
  return (
    <Link to={to} className={styles.tile}>
      <span className={styles.text}>
        <span className={styles.label}>{label}</span>
        <span className="ic-meta">{count}</span>
      </span>
      <span aria-hidden="true" className={styles.art}>
        <Artwork art={art} />
      </span>
    </Link>
  );
}

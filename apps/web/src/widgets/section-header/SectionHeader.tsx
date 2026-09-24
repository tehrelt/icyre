import type { ReactNode } from 'react';

import { Button } from '@/shared/ui';

import styles from './SectionHeader.module.css';

export interface SectionHeaderProps {
  title: string;
  kicker?: string;
  action?: string;
  onAction?: () => void;
  /** Extra controls aligned right (e.g. a SegmentedControl). */
  aside?: ReactNode;
  titleId?: string;
}

/** Pattern / Section header from the product canvas (CSectionHeader). */
export function SectionHeader({ title, kicker, action, onAction, aside, titleId }: SectionHeaderProps) {
  return (
    <div className={styles.header}>
      <div className={styles.titles}>
        {kicker && <span className="ic-overline">{kicker}</span>}
        <h2 id={titleId} className={`t-h3 ${styles.title}`}>
          {title}
        </h2>
      </div>
      {aside}
      {action && (
        <Button variant="ghost" size="sm" iconEnd="chevron" onClick={onAction}>
          {action}
        </Button>
      )}
    </div>
  );
}

import type { ReactNode } from 'react';

import { Icon } from '../core';
import type { IconName } from '../icons';

import styles from './EmptyState.module.css';

export interface EmptyStateProps {
  icon?: IconName;
  title: string;
  description?: ReactNode;
  action?: ReactNode;
}

/** Centered empty state from the product canvas (Search results — no results). */
export function EmptyState({ icon = 'search', title, description, action }: EmptyStateProps) {
  return (
    <section aria-live="polite" className={styles.empty}>
      <div className={styles.icon}>
        <Icon name={icon} size={24} />
      </div>
      <h2 className={`t-h3 ${styles.title}`}>{title}</h2>
      {description && <p className={styles.description}>{description}</p>}
      {action && <div className={styles.action}>{action}</div>}
    </section>
  );
}

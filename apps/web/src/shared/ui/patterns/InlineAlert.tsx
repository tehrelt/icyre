import type { ReactNode } from 'react';

import { Button, Icon } from '../core';

import styles from './InlineAlert.module.css';

export interface InlineAlertProps {
  title: string;
  description?: ReactNode;
  onRetry?: () => void;
  retryLabel?: string;
}

/** Inline error block from the product canvas (Search results — error). */
export function InlineAlert({ title, description, onRetry, retryLabel = 'Try again' }: InlineAlertProps) {
  return (
    <section role="alert" className={styles.alert}>
      <div className={styles.icon}>
        <Icon name="alert" size={20} />
      </div>
      <div className={styles.text}>
        <span className={styles.title}>{title}</span>
        {description && <span className={styles.description}>{description}</span>}
      </div>
      {onRetry && (
        <Button variant="secondary" size="sm" iconStart="repeat" onClick={onRetry}>
          {retryLabel}
        </Button>
      )}
    </section>
  );
}

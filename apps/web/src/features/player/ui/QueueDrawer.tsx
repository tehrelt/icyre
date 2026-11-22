import { useEffect, useState } from 'react';
import { useShallow } from 'zustand/react/shallow';

import { formatDuration } from '@/shared/lib/formatTime';
import { Button, Drawer } from '@/shared/ui';

import { usePlayerStore } from '../model/playerStore';

import { QueueList } from './QueueList';

function useIsMobile(): boolean {
  const [mobile, setMobile] = useState(() =>
    typeof window !== 'undefined' && typeof window.matchMedia === 'function' ? window.matchMedia('(max-width: 767px)').matches : false,
  );

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
    const mq = window.matchMedia('(max-width: 767px)');
    const update = () => setMobile(mq.matches);
    update();
    mq.addEventListener('change', update);
    return () => mq.removeEventListener('change', update);
  }, []);

  return mobile;
}

export interface QueueDrawerProps {
  open: boolean;
  onClose: () => void;
}

/** Side (desktop/tablet) or bottom (mobile) sheet for the playback queue. */
export function QueueDrawer({ open, onClose }: QueueDrawerProps) {
  const isMobile = useIsMobile();
  const { queue, clearQueue } = usePlayerStore(
    useShallow((s) => ({
      queue: s.queue,
      clearQueue: s.clearQueue,
    })),
  );

  const totalSec = queue.reduce((sum, t) => sum + t.durationSec, 0);

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title="Queue"
      placement={isMobile ? 'bottom' : 'right'}
      footer={
        queue.length > 0 ? (
          <div style={{ width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 'var(--space-3)' }}>
            <span className="ic-meta">
              {queue.length} {queue.length === 1 ? 'track' : 'tracks'} · {formatDuration(totalSec)}
            </span>
            <Button variant="ghost" size="sm" onClick={clearQueue}>
              Clear queue
            </Button>
          </div>
        ) : undefined
      }
    >
      <QueueList />
    </Drawer>
  );
}

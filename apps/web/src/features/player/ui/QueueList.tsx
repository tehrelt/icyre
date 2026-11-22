import { useState } from 'react';
import { useShallow } from 'zustand/react/shallow';

import { cx } from '@/shared/lib/cx';
import { Button, QueueItem } from '@/shared/ui';
import { EmptyState } from '@/shared/ui/patterns/EmptyState';

import { usePlayerStore } from '../model/playerStore';

import styles from './QueueList.module.css';

/** Shared queue view used by both QueueDrawer and ExpandedPlayer. */
export function QueueList() {
  const { queue, queueIndex, currentTrack, isPlaying, buffering } = usePlayerStore(
    useShallow((s) => ({
      queue: s.queue,
      queueIndex: s.queueIndex,
      currentTrack: s.currentTrack,
      isPlaying: s.isPlaying,
      buffering: s.buffering,
    })),
  );
  const { playAt, removeFromQueue, moveInQueue, clearUpcoming } = usePlayerStore(
    useShallow((s) => ({
      playAt: s.playAt,
      removeFromQueue: s.removeFromQueue,
      moveInQueue: s.moveInQueue,
      clearUpcoming: s.clearUpcoming,
    })),
  );

  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null);

  if (queue.length === 0) {
    return (
      <EmptyState
        icon="queue"
        title="Your queue is empty"
        description="Pick an album, playlist or track to start listening."
      />
    );
  }

  const played = queueIndex > 0 ? queue.slice(0, queueIndex) : [];
  const upcoming = queueIndex >= 0 ? queue.slice(queueIndex + 1) : queue;
  const upcomingOffset = queueIndex >= 0 ? queueIndex + 1 : 0;
  const playbackState = buffering && isPlaying ? 'buffering' : isPlaying ? 'playing' : 'paused';

  return (
    <div className={styles.root}>
      {played.length > 0 && (
        <section className={styles.group} aria-label="Played tracks">
          <div className={styles.groupHead}>
            <span className="ic-overline">Played · {played.length}</span>
          </div>
          <div className={styles.list} role="list">
            {played.map((track, idx) => (
              <QueueItem
                key={`${track.id}-${idx}`}
                title={track.title}
                artist={track.artistName}
                duration={track.durationSec}
                cover={track.coverUrl}
                art={track.art}
                explicit={track.explicit}
                state="played"
                onPlay={() => playAt(idx)}
                onRemove={() => removeFromQueue(idx)}
              />
            ))}
          </div>
        </section>
      )}

      {currentTrack && queueIndex >= 0 && (
        <section className={styles.group} aria-label="Now playing">
          <div className={styles.groupHead}>
            <span className="ic-overline">Now playing</span>
          </div>
          <div className={styles.list} role="list">
            <QueueItem
              title={currentTrack.title}
              artist={currentTrack.artistName}
              duration={currentTrack.durationSec}
              cover={currentTrack.coverUrl}
              art={currentTrack.art}
              explicit={currentTrack.explicit}
              state="current"
              playbackState={playbackState}
              onPlay={() => playAt(queueIndex)}
              onRemove={() => removeFromQueue(queueIndex)}
            />
          </div>
        </section>
      )}

      {upcoming.length > 0 && (
        <section className={styles.group} aria-label="Next up">
          <div className={styles.groupHead}>
            <span className="ic-overline">Next up · {upcoming.length}</span>
            <Button variant="ghost" size="sm" onClick={clearUpcoming}>
              Clear
            </Button>
          </div>
          <div className={styles.list} role="list">
            {upcoming.map((track, offset) => {
              const actualIndex = upcomingOffset + offset;
              return (
                <div
                  key={`${track.id}-${actualIndex}`}
                  className={cx(styles.dragRow, dragOverIndex === actualIndex && styles.dragOver)}
                  draggable
                  onDragStart={(event) => {
                    setDragIndex(actualIndex);
                    event.dataTransfer.effectAllowed = 'move';
                    event.dataTransfer.setData('text/plain', String(actualIndex));
                  }}
                  onDragOver={(event) => {
                    if (dragIndex === null || dragIndex === actualIndex) return;
                    event.preventDefault();
                    event.dataTransfer.dropEffect = 'move';
                    setDragOverIndex(actualIndex);
                  }}
                  onDragLeave={() => {
                    if (dragOverIndex === actualIndex) setDragOverIndex(null);
                  }}
                  onDrop={(event) => {
                    event.preventDefault();
                    const from = dragIndex ?? Number(event.dataTransfer.getData('text/plain'));
                    if (Number.isInteger(from) && from !== actualIndex) {
                      moveInQueue(from, actualIndex);
                    }
                    setDragIndex(null);
                    setDragOverIndex(null);
                  }}
                  onDragEnd={() => {
                    setDragIndex(null);
                    setDragOverIndex(null);
                  }}
                >
                  <QueueItem
                    title={track.title}
                    artist={track.artistName}
                    duration={track.durationSec}
                    cover={track.coverUrl}
                    art={track.art}
                    explicit={track.explicit}
                    draggable
                    onPlay={() => playAt(actualIndex)}
                    onRemove={() => removeFromQueue(actualIndex)}
                  />
                </div>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}

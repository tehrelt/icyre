import type { Track } from '@/entities/track/model';

import { initialPlayerState, usePlayerStore } from './playerStore';

const t = (id: string, extra: Partial<Track> = {}): Track => ({
  id,
  title: id,
  artistName: 'Nova Hale',
  albumId: 'alb',
  albumTitle: 'Prism Hours',
  durationSec: 200,
  explicit: false,
  coverUrl: null,
  art: 0,
  liked: false,
  available: true,
  ...extra,
});

const state = () => usePlayerStore.getState();

beforeEach(() => usePlayerStore.setState(initialPlayerState));

describe('player store', () => {
  it('starts a queue and requests a load', () => {
    state().playQueue([t('a'), t('b')], 1, { kind: 'album', id: 'alb' });
    expect(state().currentTrack?.id).toBe('b');
    expect(state().isPlaying).toBe(true);
    expect(state().duration).toBe(200);
    expect(state().loadId).toBe(1);
    expect(state().source).toEqual({ kind: 'album', id: 'alb' });
  });

  it('skips unavailable tracks when starting and advancing', () => {
    state().playQueue([t('a', { available: false }), t('b'), t('c', { available: false }), t('d')]);
    expect(state().currentTrack?.id).toBe('b');
    state().next();
    expect(state().currentTrack?.id).toBe('d');
  });

  it('stops at the end of the queue when repeat is off', () => {
    state().playQueue([t('a'), t('b')], 1);
    state().engineEnded();
    expect(state().isPlaying).toBe(false);
    expect(state().currentTrack?.id).toBe('b');
    expect(state().seekRequest?.at).toBe(0);
  });

  it('wraps with repeat all and restarts with repeat one', () => {
    state().playQueue([t('a'), t('b')], 1);
    state().cycleRepeat(); // all
    state().engineEnded();
    expect(state().currentTrack?.id).toBe('a');

    state().cycleRepeat(); // one
    expect(state().repeatMode).toBe('one');
    const loadId = state().loadId;
    state().engineEnded();
    expect(state().currentTrack?.id).toBe('a');
    expect(state().loadId).toBe(loadId + 1);

    // A manual "next" still moves on under repeat-one.
    state().next();
    expect(state().currentTrack?.id).toBe('b');
  });

  it('previous restarts the track after 3 seconds, otherwise goes back', () => {
    state().playQueue([t('a'), t('b')], 1);
    state().engineTime(10);
    state().previous();
    expect(state().currentTrack?.id).toBe('b');
    expect(state().position).toBe(0);

    state().engineTime(1);
    state().previous();
    expect(state().currentTrack?.id).toBe('a');
  });

  it('shuffle never repeats the current track', () => {
    state().playQueue([t('a'), t('b'), t('c')]);
    state().toggleShuffle();
    for (let i = 0; i < 20; i++) {
      const before = state().currentTrack?.id;
      state().next();
      expect(state().currentTrack?.id).not.toBe(before);
    }
  });

  it('clamps seek and volume, errors stop playback', () => {
    state().playQueue([t('a')]);
    state().seek(999);
    expect(state().seekRequest?.at).toBe(200);
    state().setVolume(140);
    expect(state().volume).toBe(100);
    state().engineError('Stream is not available');
    expect(state().isPlaying).toBe(false);
    expect(state().error).toBe('Stream is not available');
  });

  it('ignores play without a track', () => {
    state().play();
    expect(state().isPlaying).toBe(false);
  });
});

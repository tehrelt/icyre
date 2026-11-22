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

  it('jumps to a track with playAt and toggles when clicking current', () => {
    state().playQueue([t('a'), t('b'), t('c', { available: false })]);
    state().playAt(1);
    expect(state().currentTrack?.id).toBe('b');
    expect(state().isPlaying).toBe(true);

    state().playAt(1);
    expect(state().isPlaying).toBe(false);

    state().playAt(2); // unavailable
    expect(state().currentTrack?.id).toBe('b');
  });

  it('removes tracks from queue and keeps queueIndex consistent', () => {
    state().playQueue([t('a'), t('b'), t('c'), t('d')], 1);
    const loadId = state().loadId;

    // Remove before current: shifts queueIndex without reloading.
    state().removeFromQueue(0);
    expect(state().queue.map((x) => x.id)).toEqual(['b', 'c', 'd']);
    expect(state().queueIndex).toBe(0);
    expect(state().currentTrack?.id).toBe('b');
    expect(state().loadId).toBe(loadId);

    // Remove after current: keeps queueIndex without reloading.
    state().removeFromQueue(2);
    expect(state().queue.map((x) => x.id)).toEqual(['b', 'c']);
    expect(state().queueIndex).toBe(0);
    expect(state().loadId).toBe(loadId);

    // Remove current: advances to next playable track.
    state().removeFromQueue(0);
    expect(state().queue.map((x) => x.id)).toEqual(['c']);
    expect(state().currentTrack?.id).toBe('c');
    expect(state().loadId).toBe(loadId + 1);

    // Remove last track: clears player.
    state().removeFromQueue(0);
    expect(state().queue).toEqual([]);
    expect(state().currentTrack).toBeNull();
    expect(state().isPlaying).toBe(false);
  });

  it('reorders tracks with moveInQueue and tracks current index', () => {
    state().playQueue([t('a'), t('b'), t('c'), t('d')], 1);
    state().moveInQueue(3, 0);
    expect(state().queue.map((x) => x.id)).toEqual(['d', 'a', 'b', 'c']);
    expect(state().queueIndex).toBe(2);
    expect(state().currentTrack?.id).toBe('b');

    state().moveInQueue(2, 0);
    expect(state().queue.map((x) => x.id)).toEqual(['b', 'd', 'a', 'c']);
    expect(state().queueIndex).toBe(0);
  });

  it('enqueues next, appends, clears upcoming, and clears queue', () => {
    state().enqueueNext(t('a'));
    expect(state().currentTrack?.id).toBe('a');

    state().addToQueue(t('c'));
    state().enqueueNext(t('b'));
    expect(state().queue.map((x) => x.id)).toEqual(['a', 'b', 'c']);

    state().clearUpcoming();
    expect(state().queue.map((x) => x.id)).toEqual(['a']);

    state().clearQueue();
    expect(state().queue).toEqual([]);
    expect(state().currentTrack).toBeNull();
    expect(state().isPlaying).toBe(false);
  });

  it('tracks pending output device until engine confirms or rejects', () => {
    state().setOutputDevice('dac-1');
    expect(state().outputDeviceId).toBe('default');
    expect(state().pendingOutputDeviceId).toBe('dac-1');
    expect(state().outputDeviceRequest?.deviceId).toBe('dac-1');

    state().engineOutputDevice('dac-1');
    expect(state().outputDeviceId).toBe('dac-1');
    expect(state().pendingOutputDeviceId).toBeNull();
    expect(state().outputDeviceError).toBeNull();

    state().setOutputDevice('dac-2');
    state().engineOutputDeviceError('Could not switch audio output');
    expect(state().outputDeviceId).toBe('dac-1');
    expect(state().pendingOutputDeviceId).toBeNull();
    expect(state().outputDeviceError).toBe('Could not switch audio output');
  });
});

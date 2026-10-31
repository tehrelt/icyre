import type { Track } from '@/entities/track/model';

import { initialPlayerState, usePlayerStore } from '../model/playerStore';

import { bindMediaSession, bindPlayerHotkeys, bindPlayerPreferences } from './playerExperience';

const track: Track = {
  id: 'track-1',
  title: 'Signal Bloom',
  artistName: 'Nova Hale',
  albumId: 'album-1',
  albumTitle: 'Prism Hours',
  durationSec: 120,
  explicit: false,
  coverUrl: 'https://cdn.example/cover.jpg',
  art: 0,
  liked: false,
  available: true,
};

beforeEach(() => {
  localStorage.clear();
  usePlayerStore.setState(initialPlayerState);
});

describe('player experience', () => {
  it('restores and persists volume preferences', () => {
    localStorage.setItem('icyre.player.preferences', JSON.stringify({ version: 1, volume: 35, muted: true }));
    const dispose = bindPlayerPreferences(usePlayerStore, localStorage);

    expect(usePlayerStore.getState()).toMatchObject({ volume: 35, muted: true });
    usePlayerStore.getState().setVolume(81);
    expect(JSON.parse(localStorage.getItem('icyre.player.preferences')!)).toEqual({ version: 1, volume: 81, muted: false });
    dispose();
  });

  it('controls playback through global hotkeys', () => {
    usePlayerStore.getState().playQueue([track]);
    usePlayerStore.getState().engineTime(20);
    const dispose = bindPlayerHotkeys(usePlayerStore, window);

    window.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', cancelable: true }));
    expect(usePlayerStore.getState().isPlaying).toBe(false);
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', cancelable: true }));
    expect(usePlayerStore.getState().position).toBe(25);
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'm', cancelable: true }));
    expect(usePlayerStore.getState().muted).toBe(true);

    const input = document.createElement('input');
    document.body.append(input);
    input.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true, cancelable: true }));
    expect(usePlayerStore.getState().isPlaying).toBe(false);
    input.remove();
    dispose();
  });

  it('syncs media metadata and hardware controls', () => {
    const handlers = new Map<MediaSessionAction, MediaSessionActionHandler | null>();
    const session = {
      metadata: null,
      playbackState: 'none',
      setActionHandler: vi.fn((action: MediaSessionAction, handler: MediaSessionActionHandler | null) => handlers.set(action, handler)),
      setPositionState: vi.fn(),
    } as unknown as MediaSession;
    const metadata = vi.fn((value: Track) => ({ title: value.title }) as MediaMetadata);
    const dispose = bindMediaSession(usePlayerStore, session, metadata);

    usePlayerStore.getState().playQueue([track]);
    expect(metadata).toHaveBeenCalledWith(track);
    expect(session.playbackState).toBe('playing');
    expect(session.metadata).toEqual({ title: track.title });

    handlers.get('pause')?.({ action: 'pause' });
    expect(usePlayerStore.getState().isPlaying).toBe(false);
    handlers.get('seekto')?.({ action: 'seekto', seekTime: 42 });
    expect(usePlayerStore.getState().position).toBe(42);
    expect(session.setPositionState).toHaveBeenLastCalledWith({ duration: 120, playbackRate: 1, position: 42 });

    dispose();
    expect(session.playbackState).toBe('none');
    expect(session.metadata).toBeNull();
  });

  it('ignores invalid media artwork', () => {
    let metadataInit: MediaMetadataInit | undefined;
    class FakeMediaMetadata {
      constructor(init: MediaMetadataInit) {
        metadataInit = init;
      }
    }
    vi.stubGlobal('MediaMetadata', FakeMediaMetadata);
    const session = {
      metadata: null,
      playbackState: 'none',
      setActionHandler: vi.fn(),
      setPositionState: vi.fn(),
    } as unknown as MediaSession;
    const dispose = bindMediaSession(usePlayerStore, session);

    usePlayerStore.getState().playQueue([{ ...track, coverUrl: 'http://[invalid' }]);
    expect(metadataInit?.artwork).toEqual([]);
    dispose();
    vi.unstubAllGlobals();
  });

  it('retries rejected position updates', () => {
    const setPositionState = vi.fn().mockImplementationOnce(() => {
      throw new Error('loading');
    });
    const session = {
      metadata: null,
      playbackState: 'none',
      setActionHandler: vi.fn(),
      setPositionState,
    } as unknown as MediaSession;
    const dispose = bindMediaSession(usePlayerStore, session, () => ({}) as MediaMetadata);

    usePlayerStore.getState().playQueue([track]);
    usePlayerStore.getState().pause();
    expect(setPositionState).toHaveBeenCalledTimes(2);
    dispose();
  });
});

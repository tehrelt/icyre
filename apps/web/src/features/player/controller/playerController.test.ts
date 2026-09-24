import type { Track } from '@/entities/track/model';

import { BaseEngine } from '../engine/AudioEngine';
import { initialPlayerState, usePlayerStore } from '../model/playerStore';

import { bindPlayer } from './playerController';

class FakeEngine extends BaseEngine {
  calls: string[] = [];
  load(src: string | null, d: number) {
    this.calls.push(`load:${src}:${d}`);
  }
  async play() {
    this.calls.push('play');
  }
  pause() {
    this.calls.push('pause');
  }
  seek(s: number) {
    this.calls.push(`seek:${s}`);
  }
  setVolume(v: number, m: boolean) {
    this.calls.push(`volume:${v}:${m}`);
  }
  fire = this.emit.bind(this);
}

const track = (id: string): Track => ({
  id,
  title: id,
  artistName: 'x',
  albumId: 'a',
  albumTitle: 'a',
  durationSec: 120,
  explicit: false,
  coverUrl: null,
  art: 0,
  liked: false,
  available: true,
});

const flush = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => usePlayerStore.setState(initialPlayerState));

describe('bindPlayer', () => {
  it('loads a signed URL for the current track and plays it', async () => {
    const engine = new FakeEngine();
    const dispose = bindPlayer(usePlayerStore, engine, async (t) => `https://cdn/${t.id}`);

    usePlayerStore.getState().playQueue([track('a')]);
    await flush();
    expect(engine.calls).toEqual(['volume:0.7:false', 'load:https://cdn/a:120', 'play']);

    usePlayerStore.getState().pause();
    usePlayerStore.getState().seek(30);
    usePlayerStore.getState().setMuted(true);
    expect(engine.calls.slice(3)).toEqual(['pause', 'seek:30', 'volume:0.7:true']);
    dispose();
  });

  it('drops a stale stream grant when the track changed meanwhile', async () => {
    const engine = new FakeEngine();
    const pending: Array<() => void> = [];
    const dispose = bindPlayer(usePlayerStore, engine, (t) => new Promise((r) => pending.push(() => r(t.id))));

    usePlayerStore.getState().playQueue([track('a'), track('b')]);
    usePlayerStore.getState().next();
    pending.forEach((resolve) => resolve());
    await flush();
    expect(engine.calls.filter((c) => c.startsWith('load'))).toEqual(['load:b:120']);
    dispose();
  });

  it('feeds engine events back into the store', async () => {
    const engine = new FakeEngine();
    const dispose = bindPlayer(usePlayerStore, engine, async () => 'u');
    usePlayerStore.getState().playQueue([track('a'), track('b')]);
    await flush();

    engine.fire({ type: 'time', position: 42 });
    expect(usePlayerStore.getState().position).toBe(42);
    engine.fire({ type: 'ended' });
    expect(usePlayerStore.getState().currentTrack?.id).toBe('b');
    engine.fire({ type: 'error', message: 'boom' });
    expect(usePlayerStore.getState().error).toBe('boom');
    dispose();
  });

  it('reports stream authorization failures', async () => {
    const engine = new FakeEngine();
    const dispose = bindPlayer(usePlayerStore, engine, () => Promise.reject(new Error('403')));
    usePlayerStore.getState().playQueue([track('a')]);
    await flush();
    expect(usePlayerStore.getState().error).toBe('Could not authorize the stream');
    expect(engine.calls.some((c) => c.startsWith('load'))).toBe(false);
    dispose();
  });
});

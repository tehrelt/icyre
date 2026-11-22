import type { Track } from '@/entities/track/model';

import { BaseEngine } from '../engine/AudioEngine';
import { initialPlayerState, usePlayerStore } from '../model/playerStore';

import { bindPlayer } from './playerController';

class FakeEngine extends BaseEngine {
  calls: string[] = [];
  setOutputDevice?: (deviceId: string) => Promise<void>;
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
    const dispose = bindPlayer(usePlayerStore, engine, async (t) => ({ url: `https://cdn/${t.id}`, expiresAt: null }));

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
    const dispose = bindPlayer(usePlayerStore, engine, (t) => new Promise((r) => pending.push(() => r({ url: t.id, expiresAt: null }))));

    usePlayerStore.getState().playQueue([track('a'), track('b')]);
    usePlayerStore.getState().next();
    pending.forEach((resolve) => resolve());
    await flush();
    expect(engine.calls.filter((c) => c.startsWith('load'))).toEqual(['load:b:120']);
    dispose();
  });

  it('feeds engine events back into the store', async () => {
    const engine = new FakeEngine();
    const dispose = bindPlayer(usePlayerStore, engine, async () => ({ url: 'u', expiresAt: null }));
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

  it('re-authorizes an expired URL and resumes where playback was', async () => {
    const engine = new FakeEngine();
    let n = 0;
    const resolver = vi.fn(async () => ({ url: `https://cdn/a?v=${++n}`, expiresAt: n === 1 ? Date.now() - 1 : Date.now() + 300_000 }));
    const dispose = bindPlayer(usePlayerStore, engine, resolver);
    usePlayerStore.getState().playQueue([track('a')]);
    await flush();
    engine.fire({ type: 'time', position: 75 });

    engine.fire({ type: 'error', message: 'network' });
    await flush();
    expect(usePlayerStore.getState().error).toBeNull();
    expect(engine.calls.slice(-3)).toEqual(['load:https://cdn/a?v=2:120', 'seek:75', 'play']);

    // A fresh URL that fails is a real error: no renewal loop.
    engine.fire({ type: 'error', message: 'This track could not be played' });
    await flush();
    expect(resolver).toHaveBeenCalledTimes(2);
    expect(usePlayerStore.getState().error).toBe('This track could not be played');
    dispose();
  });

  it('confirms output device on resolve and keeps previous device on rejection', async () => {
    const engine = new FakeEngine();
    const setOutputDevice = vi.fn(async (id: string) => {
      if (id === 'bad-dac') throw new DOMException('Device unavailable', 'NotFoundError');
    });
    engine.setOutputDevice = setOutputDevice;
    const dispose = bindPlayer(usePlayerStore, engine, async () => ({ url: 'u', expiresAt: null }));

    usePlayerStore.getState().setOutputDevice('dac-1');
    await flush();
    expect(setOutputDevice).toHaveBeenCalledWith('dac-1');
    expect(usePlayerStore.getState().outputDeviceId).toBe('dac-1');
    expect(usePlayerStore.getState().outputDeviceError).toBeNull();

    usePlayerStore.getState().setOutputDevice('bad-dac');
    await flush();
    expect(setOutputDevice).toHaveBeenCalledWith('bad-dac');
    expect(usePlayerStore.getState().outputDeviceId).toBe('dac-1');
    expect(usePlayerStore.getState().pendingOutputDeviceId).toBeNull();
    expect(usePlayerStore.getState().outputDeviceError).toBe('Could not switch audio output');
    dispose();
  });

  it('serializes rapid output switches so A stays active when A resolves and B rejects', async () => {
    const engine = new FakeEngine();
    const deferred = new Map<string, { resolve: () => void; reject: (err: Error) => void }>();
    const calls: string[] = [];
    engine.setOutputDevice = vi.fn(
      (id: string) =>
        new Promise<void>((resolve, reject) => {
          calls.push(id);
          deferred.set(id, { resolve, reject });
        }),
    );
    const dispose = bindPlayer(usePlayerStore, engine, async () => ({ url: 'u', expiresAt: null }));

    usePlayerStore.getState().setOutputDevice('dac-a');
    usePlayerStore.getState().setOutputDevice('dac-b');
    await flush();

    // B must not start until A settles.
    expect(calls).toEqual(['dac-a']);
    expect(usePlayerStore.getState().outputDeviceId).toBe('default');

    deferred.get('dac-a')!.resolve();
    await flush();

    expect(calls).toEqual(['dac-a', 'dac-b']);
    expect(usePlayerStore.getState().outputDeviceId).toBe('dac-a');

    deferred.get('dac-b')!.reject(new Error('Device disconnected'));
    await flush();

    expect(usePlayerStore.getState().outputDeviceId).toBe('dac-a');
    expect(usePlayerStore.getState().pendingOutputDeviceId).toBeNull();
    expect(usePlayerStore.getState().outputDeviceError).toBe('Could not switch audio output');
    dispose();
  });

  it('restores confirmed output on rebind without re-running failed requests and resets if new engine rejects', async () => {
    const engine1 = new FakeEngine();
    engine1.setOutputDevice = vi.fn(async (id: string) => {
      if (id === 'dac-b') throw new Error('Device disconnected');
    });
    const dispose1 = bindPlayer(usePlayerStore, engine1, async () => ({ url: 'u', expiresAt: null }));

    usePlayerStore.getState().setOutputDevice('dac-a');
    await flush();
    usePlayerStore.getState().setOutputDevice('dac-b');
    await flush();
    expect(usePlayerStore.getState().outputDeviceId).toBe('dac-a');
    expect(usePlayerStore.getState().pendingOutputDeviceId).toBeNull();
    dispose1();

    // Rebind to a new engine starting on default that accepts dac-a.
    const engine2 = new FakeEngine();
    let engine2Sink = 'default';
    const engine2Calls: string[] = [];
    engine2.setOutputDevice = vi.fn(async (id: string) => {
      engine2Calls.push(id);
      engine2Sink = id;
    });
    const dispose2 = bindPlayer(usePlayerStore, engine2, async () => ({ url: 'u', expiresAt: null }));
    await flush();

    expect(engine2Calls).toEqual(['dac-a']);
    expect(engine2Sink).toBe('dac-a');
    expect(usePlayerStore.getState().outputDeviceId).toBe('dac-a');
    dispose2();

    // Rebind to a third engine starting on default that rejects dac-a.
    const engine3 = new FakeEngine();
    const engine3Sink = 'default';
    engine3.setOutputDevice = vi.fn(async () => {
      throw new Error('Unavailable on new engine');
    });
    const dispose3 = bindPlayer(usePlayerStore, engine3, async () => ({ url: 'u', expiresAt: null }));
    await flush();

    expect(engine3Sink).toBe('default');
    expect(usePlayerStore.getState().outputDeviceId).toBe('default');
    expect(usePlayerStore.getState().outputDeviceError).toBe('Could not switch audio output');
    dispose3();
  });

  it('rejects external output selection when engine.setOutputDevice is missing', async () => {
    const engine = new FakeEngine();
    const dispose = bindPlayer(usePlayerStore, engine, async () => ({ url: 'u', expiresAt: null }));

    usePlayerStore.getState().setOutputDevice('dac-1');
    await flush();

    expect(usePlayerStore.getState().outputDeviceId).toBe('default');
    expect(usePlayerStore.getState().pendingOutputDeviceId).toBeNull();
    expect(usePlayerStore.getState().outputDeviceError).toBe('Output switching is not supported in this browser');
    dispose();
  });
});

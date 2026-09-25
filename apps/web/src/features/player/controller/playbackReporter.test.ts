import type { Track } from '@/entities/track/model';

import { BaseEngine } from '../engine/AudioEngine';
import { initialPlayerState, usePlayerStore } from '../model/playerStore';

import { bindPlaybackReporter, type PlaybackReport } from './playbackReporter';

class FakeEngine extends BaseEngine {
  load() {}
  async play() {}
  pause() {}
  seek() {}
  setVolume() {}
  fire = this.emit.bind(this);
}

const track = (id: string, durationSec = 200): Track => ({
  id, title: id, artistName: 'x', albumId: 'alb-a', albumTitle: 'a', durationSec, explicit: false, coverUrl: null, art: 0, liked: false, available: true,
});

/** Plays `seconds` of audio from `from` in 0.25 s ticks. */
const play = (engine: FakeEngine, from: number, seconds: number) => {
  for (let t = from; t <= from + seconds; t += 0.25) engine.fire({ type: 'time', position: t });
};

beforeEach(() => usePlayerStore.setState(initialPlayerState));

describe('bindPlaybackReporter', () => {
  it('reports started and finished with the time actually listened', () => {
    const engine = new FakeEngine();
    const reports: PlaybackReport[] = [];
    const unbind = bindPlaybackReporter(usePlayerStore, engine, async (r) => reports.push(r));

    usePlayerStore.getState().playQueue([track('t1')], 0, { kind: 'album', id: 'alb-a' });
    engine.fire({ type: 'playing' });
    play(engine, 0, 30);
    engine.fire({ type: 'time', position: 150 }); // seek forward: not listened
    play(engine, 150, 50);
    engine.fire({ type: 'ended' });

    expect(reports.map((r) => r.type)).toEqual(['started', 'finished']);
    const [started, finished] = reports;
    expect(started).toMatchObject({ trackId: 't1', source: 'album:alb-a', durationMs: 200000, listenedMs: 0 });
    expect(finished!.playbackId).toBe(started!.playbackId);
    expect(finished!.listenedMs).toBe(80000);
    unbind();
  });

  it('reports a skip when another track replaces a started one', () => {
    const engine = new FakeEngine();
    const reports: PlaybackReport[] = [];
    const unbind = bindPlaybackReporter(usePlayerStore, engine, async (r) => reports.push(r));

    usePlayerStore.getState().playQueue([track('t1'), track('t2')]);
    engine.fire({ type: 'playing' });
    play(engine, 0, 10);
    usePlayerStore.getState().next();
    engine.fire({ type: 'playing' });

    expect(reports.map((r) => `${r.type}:${r.trackId}`)).toEqual(['started:t1', 'skipped:t1', 'started:t2']);
    expect(reports[1]!.listenedMs).toBe(10000);
    expect(reports[1]!.source).toBeUndefined();
    expect(reports[2]!.playbackId).not.toBe(reports[0]!.playbackId);
    unbind();
  });

  it('does not report tracks that never started, and ignores report failures', () => {
    const engine = new FakeEngine();
    const reports: string[] = [];
    const unbind = bindPlaybackReporter(usePlayerStore, engine, async (r) => {
      reports.push(r.type);
      throw new Error('401');
    });
    usePlayerStore.getState().playQueue([track('t1'), track('t2')]);
    usePlayerStore.getState().next(); // t1 never played: no skip
    engine.fire({ type: 'playing' });
    engine.fire({ type: 'ended' });
    expect(reports).toEqual(['started', 'finished']);
    unbind();
  });
});

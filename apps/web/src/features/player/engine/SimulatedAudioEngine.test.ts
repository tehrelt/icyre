import type { EngineEvent } from './AudioEngine';
import { SimulatedAudioEngine } from './SimulatedAudioEngine';

describe('SimulatedAudioEngine', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('advances time while playing and ends the track', async () => {
    const engine = new SimulatedAudioEngine(250);
    const events: EngineEvent[] = [];
    engine.subscribe((e) => events.push(e));

    engine.load(null, 1);
    await engine.play();
    vi.advanceTimersByTime(500);
    engine.pause();
    const lastTime = events.filter((e) => e.type === 'time').at(-1);
    expect(lastTime).toEqual({ type: 'time', position: 0.5 });

    await engine.play();
    vi.advanceTimersByTime(1000);
    expect(events.at(-1)).toEqual({ type: 'ended' });
    engine.destroy();
  });
});

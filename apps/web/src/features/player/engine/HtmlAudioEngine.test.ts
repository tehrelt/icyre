import type { EngineEvent } from './AudioEngine';
import { HtmlAudioEngine } from './HtmlAudioEngine';

function engineWith(playError: DOMException | null) {
  const audio = document.createElement('audio');
  audio.play = () => (playError ? Promise.reject(playError) : Promise.resolve());
  audio.load = () => undefined;
  const engine = new HtmlAudioEngine(audio);
  const events: EngineEvent[] = [];
  engine.subscribe((e) => events.push(e));
  engine.load('https://media.test/tracks/t1/audio/256.aac?sig=x');
  return { engine, events };
}

describe('HtmlAudioEngine.play', () => {
  it.each([
    ['NotAllowedError', 'Playback was blocked by the browser'],
    ['NotSupportedError', 'This track could not be played'],
  ])('maps %s to a message', async (name, message) => {
    const { engine, events } = engineWith(new DOMException('x', name));
    await engine.play();
    expect(events).toContainEqual({ type: 'error', message });
  });

  it('ignores a play() interrupted by a newer load()', async () => {
    const { engine, events } = engineWith(new DOMException('x', 'AbortError'));
    await engine.play();
    expect(events.some((e) => e.type === 'error')).toBe(false);
  });
});

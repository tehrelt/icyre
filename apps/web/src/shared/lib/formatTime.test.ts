import { formatDuration, formatTime } from './formatTime';

describe('formatTime', () => {
  it('formats minutes and hours', () => {
    expect(formatTime(0)).toBe('0:00');
    expect(formatTime(227)).toBe('3:47');
    expect(formatTime(3725)).toBe('1:02:05');
    expect(formatTime(Number.NaN)).toBe('0:00');
  });

  it('formats total durations', () => {
    expect(formatDuration(38 * 60)).toBe('38 min');
    expect(formatDuration(192 * 60)).toBe('3 h 12 min');
  });
});

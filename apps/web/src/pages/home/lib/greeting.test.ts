import { greeting } from './greeting';

describe('greeting', () => {
  it('matches the canvas copy', () => {
    // Friday 20:00
    expect(greeting(new Date(2026, 8, 25, 20, 0), 'Rin Aoki')).toEqual({ kicker: 'Friday evening', title: 'Good evening, Rin' });
  });

  it('handles night and missing names', () => {
    expect(greeting(new Date(2026, 8, 26, 2, 0))).toEqual({ kicker: 'Saturday night', title: 'Good night' });
  });
});

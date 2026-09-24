const WEEKDAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

function dayPart(hour: number): 'morning' | 'afternoon' | 'evening' | 'night' {
  if (hour >= 5 && hour < 12) return 'morning';
  if (hour >= 12 && hour < 18) return 'afternoon';
  if (hour >= 18 && hour < 23) return 'evening';
  return 'night';
}

/** Home header copy: kicker "Friday evening", title "Good evening, Rin". */
export function greeting(now: Date, displayName?: string): { kicker: string; title: string } {
  const part = dayPart(now.getHours());
  const firstName = displayName?.trim().split(/\s+/)[0];
  const salute = part === 'night' ? 'Good night' : `Good ${part}`;
  return {
    kicker: `${WEEKDAYS[now.getDay()]} ${part}`,
    title: firstName ? `${salute}, ${firstName}` : salute,
  };
}

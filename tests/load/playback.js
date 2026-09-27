// Player telemetry load (TASK-038.5): every iteration is one play —
// `started`, then `finished` or `skipped`. The latency of these requests is
// what the listener pays for analytics: an acknowledged Kafka write
// (ANALYTICS_MODE=kafka) or a Catalog lookup plus a ClickHouse insert
// (ANALYTICS_MODE=sync).
import http from 'k6/http';
import { sleep } from 'k6';
import { API, auth, catalogSample, ok, options as build, pick, registerUsers, uuid } from './lib.js';

export const options = build({
  'http_req_duration{kind:playback}': ['p(95)<300'],
});

export function setup() {
  const s = catalogSample(Number(__ENV.ALBUMS || 20));
  return { tracks: s.tracks, tokens: registerUsers(Number(__ENV.USERS || 20)) };
}

const THINK = Number(__ENV.THINK_MS || 200) / 1000;

export default function (s) {
  const token = s.tokens[(__VU - 1) % s.tokens.length];
  const track = pick(s.tracks);
  const playbackId = uuid();
  const params = { ...auth(token), tags: { kind: 'playback', name: 'POST /playback/events' } };
  const report = (type, listenedMs) =>
    http.post(`${API}/playback/events`, JSON.stringify({
      type, playbackId, trackId: track, source: `queue:${playbackId.slice(0, 8)}`, durationMs: 200000, listenedMs,
    }), params);

  ok(report('started', 0), 202);
  sleep(THINK);
  const skipped = Math.random() < 0.3;
  ok(report(skipped ? 'skipped' : 'finished', skipped ? 15000 : 200000), 202);
  sleep(THINK);
}

// Catalog read load through the API Gateway (TASK-038.2 baseline, and the
// workload of the Redis and replica experiments). The mix follows the
// album screen: album, its tracks, a track, an artist, the genre list.
import http from 'k6/http';
import { sleep } from 'k6';
import { API, catalogSample, ok, options as build, pick } from './lib.js';

export const options = build({
  // SLO candidate: Catalog p95 < 300 ms.
  'http_req_duration{kind:catalog}': ['p(95)<300'],
});

export function setup() {
  return catalogSample(Number(__ENV.ALBUMS || 50));
}

const THINK = Number(__ENV.THINK_MS || 100) / 1000;

export default function (s) {
  const r = Math.random();
  let res;
  if (r < 0.35) {
    res = http.get(`${API}/albums/${pick(s.albums)}`, { tags: { kind: 'catalog', name: 'GET /albums/{id}' } });
  } else if (r < 0.65) {
    res = http.get(`${API}/albums/${pick(s.albums)}/tracks`, { tags: { kind: 'catalog', name: 'GET /albums/{id}/tracks' } });
  } else if (r < 0.85) {
    res = http.get(`${API}/tracks/${pick(s.tracks)}`, { tags: { kind: 'catalog', name: 'GET /tracks/{id}' } });
  } else if (r < 0.95) {
    res = http.get(`${API}/artists/${pick(s.artists)}`, { tags: { kind: 'catalog', name: 'GET /artists/{id}' } });
  } else {
    res = http.get(`${API}/genres`, { tags: { kind: 'catalog', name: 'GET /genres' } });
  }
  ok(res);
  sleep(THINK);
}

// Shared helpers of the EPIC-038 k6 scenarios.
import http from 'k6/http';
import { check, fail } from 'k6';

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API = `${BASE_URL}/api/v1`;

// Load profiles (specs/testing/load-testing.md): PROFILE picks one, VUS and
// DURATION override it. A ramp-up of a fifth of the run precedes the plateau.
const PROFILES = {
  smoke: { vus: 5, duration: '20s' },
  '100': { vus: 100, duration: '1m' },
  '1000': { vus: 1000, duration: '2m' },
  '5000': { vus: 5000, duration: '3m' },
  '10000': { vus: 10000, duration: '3m' },
};

function seconds(d) {
  const m = /^(\d+)(s|m)$/.exec(d);
  if (!m) fail(`DURATION must look like 30s or 2m, got ${d}`);
  return Number(m[1]) * (m[2] === 'm' ? 60 : 1);
}

// options builds the k6 options of a scenario with the SLO thresholds.
// Thresholds do not abort the run: they mark the result in the summary.
export function options(thresholds = {}) {
  const base = PROFILES[__ENV.PROFILE || '100'];
  if (!base) fail(`unknown PROFILE ${__ENV.PROFILE}`);
  const vus = Number(__ENV.VUS || base.vus);
  const total = seconds(__ENV.DURATION || base.duration);
  const ramp = Math.max(5, Math.round(total / 5));
  return {
    scenarios: {
      load: {
        executor: 'ramping-vus',
        startVUs: 0,
        stages: [
          { duration: `${ramp}s`, target: vus },
          { duration: `${total - ramp}s`, target: vus },
        ],
        gracefulRampDown: '5s',
      },
    },
    summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
    thresholds: { http_req_failed: ['rate<0.01'], ...thresholds },
    setupTimeout: '5m',
  };
}

export function getJSON(path, params = {}) {
  const res = http.get(`${API}${path}`, params);
  if (res.status !== 200) fail(`GET ${path}: ${res.status} ${res.body}`);
  return res.json();
}

// catalogSample reads up to `albums` albums with their tracks: the IDs the
// virtual users request.
export function catalogSample(albums = 50) {
  const page = getJSON(`/albums?limit=${albums}`);
  const out = { albums: [], tracks: [], readyTracks: [], artists: new Set() };
  for (const a of page.data) {
    out.albums.push(a.id);
    (a.artistIds || []).forEach((id) => out.artists.add(id));
    for (const t of getJSON(`/albums/${a.id}/tracks`).data) {
      out.tracks.push(t.id);
      if (t.status === 'READY') out.readyTracks.push(t.id);
    }
  }
  if (out.albums.length === 0 || out.tracks.length === 0) {
    fail('the catalogue is empty: run `make seed` first');
  }
  return { ...out, artists: [...out.artists] };
}

// registerUsers creates n listeners and returns their access tokens. Setup
// runs once, before the measured load.
export function registerUsers(n) {
  const run = Date.now().toString(36);
  const tokens = [];
  for (let i = 0; i < n; i++) {
    const res = http.post(
      `${API}/auth/register`,
      JSON.stringify({ email: `load-${run}-${i}@load.icyre.test`, password: `load-${run}-${i}-secret` }),
      { headers: { 'Content-Type': 'application/json' } },
    );
    if (res.status !== 201) fail(`register: ${res.status} ${res.body}`);
    tokens.push(res.json('accessToken'));
  }
  return tokens;
}

export function pick(list) {
  return list[Math.floor(Math.random() * list.length)];
}

export function uuid() {
  // RFC 4122 v4 from Math.random: good enough for playback IDs under load.
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    return (c === 'x' ? r : (r & 0x3) | 0x8).toString(16);
  });
}

export function ok(res, status = 200) {
  return check(res, { [`status ${status}`]: (r) => r.status === status });
}

export function auth(token, extra = {}) {
  return { headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json', ...extra } };
}

export { API };

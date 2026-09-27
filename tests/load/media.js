// Audio delivery (TASK-038.6): MODE=direct — Stream Authorization issues a
// signed URL and the bytes come from object storage (MinIO, the CDN origin);
// MODE=proxy — the bytes pass through the gateway and Stream Authorization
// (MEDIA_PROXY_ENABLED=true). Every iteration downloads one whole variant.
import http from 'k6/http';
import { sleep } from 'k6';
import { Trend } from 'k6/metrics';
import { API, auth, catalogSample, ok, options as build, pick, registerUsers } from './lib.js';

const MODE = __ENV.MODE || 'direct';
const QUALITY = __ENV.QUALITY || '128';
const MEDIA_ORIGIN = __ENV.MEDIA_ORIGIN || '';

// Time to the whole file, the "playback can start" moment of a listener.
const audioTime = new Trend('audio_download_ms', true);

export const options = build({
  'http_req_duration{kind:authorize}': ['p(95)<300'],
});

export function setup() {
  if (!['direct', 'proxy'].includes(MODE)) throw new Error(`MODE must be direct or proxy, got ${MODE}`);
  const s = catalogSample(Number(__ENV.ALBUMS || 50));
  const tokens = registerUsers(Number(__ENV.USERS || 20));
  // READY tracks without audio variants (created by tests, not seeded) are
  // not playable: keep only the ones Stream Authorization grants.
  const tracks = s.readyTracks.filter((id) => http.post(`${API}/stream/authorize`,
    JSON.stringify({ trackId: id, quality: QUALITY }),
    { ...auth(tokens[0]), responseCallback: http.expectedStatuses(200, 409) }).status === 200);
  if (tracks.length === 0) throw new Error('no playable tracks: run `make seed-media` first');
  return { tracks, tokens };
}

// signedRequest rewrites the browser-facing origin of a presigned URL to the
// in-network one; the signature covers Host, so the header is kept.
function signedRequest(url) {
  if (!MEDIA_ORIGIN) return { url, params: {} };
  const m = /^(https?:\/\/)([^/]+)(\/.*)$/.exec(url);
  return { url: MEDIA_ORIGIN + m[3], params: { headers: { Host: m[2] } } };
}

export default function (s) {
  const token = s.tokens[(__VU - 1) % s.tokens.length];
  const track = pick(s.tracks);
  let res;
  if (MODE === 'direct') {
    const grant = http.post(`${API}/stream/authorize`, JSON.stringify({ trackId: track, quality: QUALITY }), {
      ...auth(token), tags: { kind: 'authorize', name: 'POST /stream/authorize' },
    });
    if (!ok(grant)) return;
    const { url, params } = signedRequest(grant.json('url'));
    res = http.get(url, { ...params, responseType: 'none', tags: { kind: 'audio', name: 'GET audio (object storage)' } });
    audioTime.add(grant.timings.duration + res.timings.duration);
  } else {
    res = http.get(`${API}/stream/proxy/${track}?quality=${QUALITY}`, {
      ...auth(token), responseType: 'none', tags: { kind: 'audio', name: 'GET /stream/proxy/{id}' },
    });
    audioTime.add(res.timings.duration);
  }
  ok(res);
  sleep(Number(__ENV.THINK_MS || 500) / 1000);
}

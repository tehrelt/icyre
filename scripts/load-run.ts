/**
 * Runs the EPIC-038 load experiments against the local stand and writes a
 * report (tests/load/results/<experiment>-<time>.{json,md}).
 *
 *   bun scripts/load-run.ts <baseline|redis|replicas|analytics|media|all>
 *       [--profile 100] [--vus n] [--duration 1m] [--replicas 3]
 *
 * Each variant reconfigures the stand through the compose variables and the
 * load overlay (deploy/load/compose.yml), runs one k6 scenario (tests/load)
 * in a container and samples meanwhile: CPU and memory of the involved
 * containers (docker stats), PostgreSQL activity (pg_stat_database) and
 * Kafka consumer lag. The stand is put back to its defaults at the end.
 * Needs `make up`, `make seed` (and `make seed-media` for `media`).
 */
import { mkdirSync, writeFileSync } from 'node:fs';

const COMPOSE = ['docker', 'compose', '-f', 'docker-compose.yml', '-f', 'deploy/load/compose.yml'];
const GATEWAY = process.env.LOAD_GATEWAY_URL ?? 'http://localhost:8080';
const RESULTS = 'tests/load/results';

const arg = (name: string) => {
  const i = process.argv.indexOf(`--${name}`);
  return i > 0 ? process.argv[i + 1] : undefined;
};

type Variant = {
  name: string;
  scenario: string;
  // Compose variables of docker-compose.yml (experiment toggles).
  env?: Record<string, string>;
  catalogReplicas?: number;
  // Services recreated with the variant's variables.
  services: string[];
  // Compose services whose CPU and memory are reported.
  watch: string[];
  k6?: Record<string, string>;
  kafkaLag?: boolean;
};

const DEFAULTS = { CATALOG_CACHE_ENABLED: 'false', PLAYBACK_ANALYTICS_MODE: 'kafka', STREAM_MEDIA_PROXY: 'false' };
const replicas = Number(arg('replicas') ?? 3);
const catalogWatch = ['gateway', 'catalog', 'postgres', 'redis'];

const EXPERIMENTS: Record<string, Variant[]> = {
  baseline: [{ name: 'catalog baseline', scenario: 'catalog.js', services: ['catalog'], watch: catalogWatch }],
  redis: [
    { name: 'without Redis', scenario: 'catalog.js', services: ['catalog'], watch: catalogWatch },
    { name: 'with Redis', scenario: 'catalog.js', env: { CATALOG_CACHE_ENABLED: 'true' }, services: ['catalog'], watch: catalogWatch },
  ],
  replicas: [
    { name: 'catalog ×1', scenario: 'catalog.js', catalogReplicas: 1, services: ['catalog'], watch: catalogWatch },
    { name: `catalog ×${replicas}`, scenario: 'catalog.js', catalogReplicas: replicas, services: ['catalog'], watch: catalogWatch },
  ],
  analytics: [
    {
      name: 'Kafka (async)', scenario: 'playback.js', services: ['playback'], kafkaLag: true,
      watch: ['gateway', 'playback', 'kafka', 'analytics', 'history', 'clickhouse', 'catalog'],
    },
    {
      name: 'sync ClickHouse', scenario: 'playback.js', env: { PLAYBACK_ANALYTICS_MODE: 'sync' }, services: ['playback'], kafkaLag: true,
      watch: ['gateway', 'playback', 'kafka', 'analytics', 'history', 'clickhouse', 'catalog'],
    },
  ],
  media: [
    { name: 'object storage (signed URL)', scenario: 'media.js', k6: { MODE: 'direct' }, services: ['stream-auth'], watch: ['gateway', 'stream-auth', 'minio'] },
    {
      name: 'through backend (proxy)', scenario: 'media.js', k6: { MODE: 'proxy' }, env: { STREAM_MEDIA_PROXY: 'true' },
      services: ['stream-auth'], watch: ['gateway', 'stream-auth', 'minio'],
    },
  ],
};

// --- process helpers ---------------------------------------------------------

async function run(cmd: string[], opts: { env?: Record<string, string>; quiet?: boolean } = {}): Promise<string> {
  const p = Bun.spawn(cmd, {
    env: { ...process.env, ...opts.env },
    stdout: opts.quiet ? 'pipe' : 'inherit',
    stderr: opts.quiet ? 'pipe' : 'inherit',
  });
  const out = opts.quiet ? await new Response(p.stdout).text() : '';
  const code = await p.exited;
  if (code !== 0) {
    const err = opts.quiet ? await new Response(p.stderr).text() : '';
    throw new Error(`${cmd.join(' ')} exited with ${code}${err ? `: ${err.trim()}` : ''}`);
  }
  return out;
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

// sampler runs fn every `every` ms without overlapping runs until stopped.
function sampler(every: number, fn: () => Promise<void>) {
  let stopped = false;
  const loop = (async () => {
    while (!stopped) {
      const started = Date.now();
      await fn().catch((e) => console.warn(`sampler: ${e.message}`));
      await sleep(Math.max(0, every - (Date.now() - started)));
    }
  })();
  return async () => {
    stopped = true;
    await loop;
  };
}

// --- stand -------------------------------------------------------------------

async function apply(v: Variant) {
  const env = { ...DEFAULTS, ...v.env };
  console.log(`\n==> ${v.name}: ${JSON.stringify(env)}${v.catalogReplicas ? `, catalog ×${v.catalogReplicas}` : ''}`);
  await run([...COMPOSE, 'up', '-d', '--no-deps', '--no-build', '--scale', `catalog=${v.catalogReplicas ?? 1}`, 'gateway', 'catalog', ...v.services], { env });
  await waitReady();
  // nginx re-resolves upstream names every 10s (resolver valid=10s).
  await sleep(12_000);
}

async function waitReady(timeoutMs = 90_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const res = await fetch(`${GATEWAY}/api/v1/genres`).catch(() => undefined);
    if (res?.ok) return;
    await sleep(2_000);
  }
  throw new Error(`the stand is not ready: ${GATEWAY}/api/v1/genres`);
}

async function restore() {
  console.log('\n==> restoring the stand defaults');
  await run(['docker', 'compose', 'up', '-d', '--no-deps', '--no-build', '--scale', 'catalog=1', 'gateway', 'catalog', 'playback', 'stream-auth'], { env: DEFAULTS });
}

// --- samplers ----------------------------------------------------------------

type Usage = { cpu: number[]; memMiB: number[] };

function parseMiB(s: string): number {
  const m = /^([\d.]+)\s*([KMG]i?B|B)$/.exec(s.trim());
  if (!m) return 0;
  const n = Number(m[1]);
  const unit = m[2]!.replace('i', '');
  return unit === 'GB' ? n * 1024 : unit === 'KB' ? n / 1024 : unit === 'B' ? n / 1024 / 1024 : n;
}

// dockerStats sums CPU % and memory over the replicas of each watched service.
function dockerStats(watch: string[], usage: Record<string, Usage>) {
  return sampler(3_000, async () => {
    const out = await run(['docker', 'stats', '--no-stream', '--format', '{{json .}}'], { quiet: true });
    const cpu: Record<string, number> = {};
    const mem: Record<string, number> = {};
    for (const line of out.split('\n').filter(Boolean)) {
      const s = JSON.parse(line) as { Name: string; CPUPerc: string; MemUsage: string };
      const service = /^icyre-(.+)-\d+$/.exec(s.Name)?.[1];
      if (!service || !watch.includes(service)) continue;
      cpu[service] = (cpu[service] ?? 0) + Number.parseFloat(s.CPUPerc);
      mem[service] = (mem[service] ?? 0) + parseMiB(s.MemUsage.split('/')[0] ?? '');
    }
    for (const service of Object.keys(cpu)) {
      const u = (usage[service] ??= { cpu: [], memMiB: [] });
      u.cpu.push(cpu[service]!);
      u.memMiB.push(mem[service]!);
    }
  });
}

type PgStats = { xact: number; tupReturned: number; tupFetched: number; blksRead: number; blksHit: number };

async function pgStats(): Promise<PgStats> {
  const out = await run([...COMPOSE, 'exec', '-T', 'postgres', 'psql', '-U', 'icyre', '-d', 'icyre', '-Atc',
    "select xact_commit + xact_rollback, tup_returned, tup_fetched, blks_read, blks_hit from pg_stat_database where datname = 'icyre'"], { quiet: true });
  const [xact, tupReturned, tupFetched, blksRead, blksHit] = out.trim().split('|').map(Number) as number[];
  return { xact: xact!, tupReturned: tupReturned!, tupFetched: tupFetched!, blksRead: blksRead!, blksHit: blksHit! };
}

// kafkaLag records the largest total lag seen per consumer group.
function kafkaLag(maxLag: Record<string, number>) {
  return sampler(10_000, async () => {
    const out = await run([...COMPOSE, 'exec', '-T', 'kafka', '/opt/kafka/bin/kafka-consumer-groups.sh',
      '--bootstrap-server', 'localhost:9092', '--describe', '--all-groups'], { quiet: true });
    const total: Record<string, number> = {};
    for (const line of out.split('\n')) {
      const cols = line.trim().split(/\s+/);
      const lag = Number(cols[5]);
      if (cols.length < 6 || cols[0] === 'GROUP' || !Number.isFinite(lag)) continue;
      total[cols[0]!] = (total[cols[0]!] ?? 0) + lag;
    }
    for (const [group, lag] of Object.entries(total)) maxLag[group] = Math.max(maxLag[group] ?? 0, lag);
  });
}

// --- one variant -------------------------------------------------------------

type Trend = Record<string, number>;
type Summary = { metrics: Record<string, Trend & { rate?: number; count?: number; value?: number }> };

type Result = {
  variant: string;
  scenario: string;
  seconds: number;
  rps: number;
  errorRate: number;
  latency: Record<string, Trend>;
  dataReceivedMiBps: number;
  usage: Record<string, { cpuAvg: number; cpuMax: number; memMaxMiB: number }>;
  db: { xactPerSec: number; tuplesReadPerSec: number; blocksReadPerSec: number; cacheHitRatio: number };
  kafkaMaxLag: Record<string, number>;
};

async function measure(v: Variant, stamp: string): Promise<Result> {
  await apply(v);
  const file = `${stamp}-${v.name.replace(/[^a-z0-9]+/gi, '-').toLowerCase()}.json`;
  const k6env: Record<string, string> = { ...v.k6 };
  for (const [flag, key] of [['profile', 'PROFILE'], ['vus', 'VUS'], ['duration', 'DURATION']] as const) {
    const value = arg(flag);
    if (value) k6env[key] = value;
  }

  const usage: Record<string, Usage> = {};
  const maxLag: Record<string, number> = {};
  const before = await pgStats();
  const started = Date.now();
  const stops = [dockerStats(v.watch, usage), ...(v.kafkaLag ? [kafkaLag(maxLag)] : [])];
  try {
    await run([...COMPOSE, 'run', '--rm', '--no-deps', ...Object.entries(k6env).flatMap(([k, val]) => ['-e', `${k}=${val}`]),
      'k6', 'run', '--quiet', '--summary-export', `/results/${file}`, v.scenario]).catch((e) => {
      // k6 exits 99 when a threshold fails: the run is still a measurement.
      if (!String(e.message).includes('exited with 99')) throw e;
      console.warn('thresholds crossed (see the report)');
    });
  } finally {
    for (const stop of stops) await stop();
  }
  const seconds = (Date.now() - started) / 1000;
  const after = await pgStats();

  const summary = (await Bun.file(`${RESULTS}/${file}`).json()) as Summary;
  const m = summary.metrics;
  const latency: Record<string, Trend> = {};
  for (const [name, trend] of Object.entries(m)) {
    if (name === 'http_req_duration' || name.startsWith('http_req_duration{kind:') || name === 'audio_download_ms') {
      latency[name] = { p50: trend.med!, p95: trend['p(95)']!, p99: trend['p(99)']!, avg: trend.avg!, max: trend.max! };
    }
  }
  const blocks = after.blksRead - before.blksRead + (after.blksHit - before.blksHit);
  const avg = (xs: number[]) => (xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : 0);
  return {
    variant: v.name,
    scenario: v.scenario,
    seconds,
    rps: m.http_reqs?.rate ?? 0,
    errorRate: m.http_req_failed?.value ?? 0,
    latency,
    dataReceivedMiBps: (m.data_received?.rate ?? 0) / 1024 / 1024,
    usage: Object.fromEntries(Object.entries(usage).map(([s, u]) => [s, {
      cpuAvg: avg(u.cpu), cpuMax: Math.max(...u.cpu), memMaxMiB: Math.max(...u.memMiB),
    }])),
    db: {
      xactPerSec: (after.xact - before.xact) / seconds,
      tuplesReadPerSec: (after.tupReturned - before.tupReturned + after.tupFetched - before.tupFetched) / seconds,
      blocksReadPerSec: (after.blksRead - before.blksRead) / seconds,
      cacheHitRatio: blocks ? (after.blksHit - before.blksHit) / blocks : 1,
    },
    kafkaMaxLag: maxLag,
  };
}

// --- report ------------------------------------------------------------------

const f = (n: number, d = 1) => (Number.isFinite(n) ? n.toFixed(d) : '—');

function report(experiment: string, results: Result[]): string {
  const lines = [`# Load experiment: ${experiment}`, '', `${new Date().toISOString()} · profile ${arg('profile') ?? '100'}` +
    `${arg('vus') ? ` · ${arg('vus')} VUs` : ''}${arg('duration') ? ` · ${arg('duration')}` : ''}`, ''];
  lines.push('| Variant | RPS | p50, ms | p95, ms | p99, ms | Errors | DB tx/s | DB tuples/s | DB hit ratio |');
  lines.push('|---|---:|---:|---:|---:|---:|---:|---:|---:|');
  for (const r of results) {
    const l = r.latency.http_req_duration ?? {};
    lines.push(`| ${r.variant} | ${f(r.rps)} | ${f(l.p50!)} | ${f(l.p95!)} | ${f(l.p99!)} | ${f(r.errorRate * 100, 2)}% | ` +
      `${f(r.db.xactPerSec)} | ${f(r.db.tuplesReadPerSec, 0)} | ${f(r.db.cacheHitRatio * 100, 2)}% |`);
  }
  lines.push('', '## Latency by request kind', '', '| Variant | Metric | p50, ms | p95, ms | p99, ms | max, ms |', '|---|---|---:|---:|---:|---:|');
  for (const r of results) {
    for (const [name, l] of Object.entries(r.latency)) {
      if (!l.max) continue; // a request kind this variant does not make
      lines.push(`| ${r.variant} | \`${name}\` | ${f(l.p50!)} | ${f(l.p95!)} | ${f(l.p99!)} | ${f(l.max!)} |`);
    }
  }
  lines.push('', '## Resources', '', '| Variant | Service | CPU avg, % | CPU max, % | Memory max, MiB |', '|---|---|---:|---:|---:|');
  for (const r of results) {
    for (const [s, u] of Object.entries(r.usage)) {
      lines.push(`| ${r.variant} | ${s} | ${f(u.cpuAvg)} | ${f(u.cpuMax)} | ${f(u.memMaxMiB, 0)} |`);
    }
  }
  if (results.some((r) => r.dataReceivedMiBps > 1)) {
    lines.push('', '## Bandwidth (received by k6)', '');
    for (const r of results) lines.push(`- ${r.variant}: ${f(r.dataReceivedMiBps, 2)} MiB/s`);
  }
  if (results.some((r) => Object.keys(r.kafkaMaxLag).length > 0)) {
    lines.push('', '## Kafka consumer lag (max during the run)', '', '| Variant | Group | Max lag |', '|---|---|---:|');
    for (const r of results) {
      for (const [g, lag] of Object.entries(r.kafkaMaxLag)) lines.push(`| ${r.variant} | ${g} | ${lag} |`);
    }
  }
  return lines.join('\n') + '\n';
}

// --- main --------------------------------------------------------------------

const experiment = process.argv[2];
const names = experiment === 'all' ? ['baseline', 'redis', 'replicas', 'analytics', 'media'] : [experiment ?? ''];
if (!names.every((n) => EXPERIMENTS[n])) {
  console.error(`usage: bun scripts/load-run.ts <${[...Object.keys(EXPERIMENTS), 'all'].join('|')}> [--profile p] [--vus n] [--duration d]`);
  process.exit(2);
}
if (!Number.isInteger(replicas) || replicas < 2) throw new Error('--replicas must be an integer ≥ 2');

mkdirSync(RESULTS, { recursive: true });
try {
  for (const name of names) {
    const stamp = `${name}-${new Date().toISOString().replace(/[:.]/g, '-')}`;
    const results: Result[] = [];
    for (const v of EXPERIMENTS[name]!) results.push(await measure(v, stamp));
    const md = report(name, results);
    writeFileSync(`${RESULTS}/${stamp}.json`, JSON.stringify(results, null, 2));
    writeFileSync(`${RESULTS}/${stamp}.md`, md);
    console.log(`\n${md}\nsaved ${RESULTS}/${stamp}.md`);
  }
} finally {
  await restore();
}

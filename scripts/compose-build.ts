/**
 * Builds the docker compose images in batches instead of all at once.
 *
 *   bun scripts/compose-build.ts [--batch 4] [service ...]
 *
 * `docker compose build` (and `up --build`) starts every Go build in
 * parallel; a dozen concurrent `go build`s starve each other for CPU and
 * memory and the whole stand takes ~20 minutes. Here the services that have
 * a `build:` section are built `--batch` at a time (COMPOSE_BUILD_BATCH, 4 by
 * default), one `docker compose build` per batch. Services sharing an image
 * (a service and its migrate job) are built once. Named services limit the
 * build to them.
 */
const arg = (name: string) => {
  const i = process.argv.indexOf(`--${name}`);
  return i > 0 ? process.argv[i + 1] : undefined;
};

const batch = Number(arg('batch') ?? process.env.COMPOSE_BUILD_BATCH ?? 4);
if (!Number.isInteger(batch) || batch < 1) throw new Error('--batch must be a positive integer');
const only = process.argv.slice(2).filter((a, i, all) => !a.startsWith('--') && all[i - 1] !== '--batch');

type Service = { build?: unknown; image?: string };
const config = Bun.spawnSync(['docker', 'compose', 'config', '--format', 'json'], { stderr: 'inherit' });
if (config.exitCode !== 0) process.exit(config.exitCode ?? 1);
const services: Record<string, Service> = JSON.parse(config.stdout.toString()).services;

const seen = new Set<string>();
const targets = Object.entries(services)
  .filter(([name, s]) => s.build && (only.length === 0 || only.includes(name)))
  .filter(([name, s]) => {
    const image = s.image ?? name;
    if (seen.has(image)) return false;
    seen.add(image);
    return true;
  })
  .map(([name]) => name)
  .sort();

const unknown = only.filter((name) => !services[name]?.build);
if (unknown.length > 0) throw new Error(`no build section for: ${unknown.join(', ')}`);

const started = performance.now();
for (let i = 0; i < targets.length; i += batch) {
  const group = targets.slice(i, i + batch);
  console.log(`==> build ${i / batch + 1}/${Math.ceil(targets.length / batch)}: ${group.join(' ')}`);
  const build = Bun.spawnSync(['docker', 'compose', 'build', ...group], { stdout: 'inherit', stderr: 'inherit' });
  if (build.exitCode !== 0) process.exit(build.exitCode ?? 1);
}
console.log(`built ${targets.length} images in ${((performance.now() - started) / 1000).toFixed(0)}s`);

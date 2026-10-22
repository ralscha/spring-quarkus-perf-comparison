// Pure workload/configuration functions shared by k6 and the Node regression tests.
export function integer(env, key, fallback, min = 1, max = 100000000) {
  const raw = env[key];
  if (raw === undefined || raw === '') return fallback;
  if (!/^\d+$/.test(raw) || !Number.isSafeInteger(Number(raw)) || Number(raw) < min || Number(raw) > max) {
    throw new Error(`${key} must be an integer between ${min} and ${max}`);
  }
  return Number(raw);
}

export function configuration(env) {
  const rate = integer(env, 'K6_RATE', 100);
  const duration = integer(env, 'K6_DURATION_SECONDS', 120, 20);
  const profile = env.K6_PROFILE || 'mixed';
  if (!['mixed', 'steady'].includes(profile)) throw new Error('K6_PROFILE must be mixed or steady');
  const burst = integer(env, 'K6_BURST_MULTIPLIER', 2, 1, 10);
  // 50% steady, 5% ramp up, 10% burst, 5% ramp down, 30% recovery.
  const durations = [0.5, 0.05, 0.1, 0.05, 0.3].map((part) => part * duration);
  const targets = [rate, rate * burst, rate * burst, rate, rate];
  const stages = profile === 'steady' ? [{ duration: `${duration}s`, target: rate }]
    : durations.map((seconds, i) => ({ duration: `${seconds}s`, target: targets[i] }));
  const offered = duration * rate * (profile === 'steady' ? 1 : 1 + 0.15 * (burst - 1));
  const warmupRamp = integer(env, 'K6_WARMUP_RAMP_SECONDS', 15);
  const warmupHold = integer(env, 'K6_WARMUP_HOLD_SECONDS', 45);
  const warmupStartRate = Math.max(1, Math.floor(rate / 10));
  const warmupOffered = warmupRamp * (warmupStartRate + rate) / 2 + warmupHold * rate;
  const maxCreates = integer(env, 'K6_MAX_CREATES', 10000);
  // Each phase has its own deterministic mix; allow one partial 100-request block per phase.
  const maximumCreates = Math.ceil(offered * 0.05) + Math.ceil(warmupOffered * 0.05) + 10;
  if (maximumCreates > maxCreates) {
    throw new Error('Write budget exceeded: reduce rate/duration or explicitly increase K6_MAX_CREATES');
  }
  const timeout = integer(env, 'K6_TIMEOUT_SECONDS', 5, 1, 60);
  const grace = integer(env, 'K6_GRACE_SECONDS', Math.max(10, timeout + 1), timeout + 1, 120);
  const runId = env.K6_RUN_ID || String(Date.now());
  if (!/^[a-zA-Z0-9_-]{1,100}$/.test(runId)) throw new Error('K6_RUN_ID must be 1-100 alphanumeric, dash or underscore characters');
  const errorRate = Number(env.K6_ERROR_RATE || '0.01');
  if (!Number.isFinite(errorRate) || errorRate <= 0 || errorRate > 1) throw new Error('K6_ERROR_RATE must be in (0, 1]');
  return {
    baseUrl: (env.K6_BASE_URL || 'http://127.0.0.1:8080').replace(/\/$/, ''), rate, duration, profile,
    burst, stages, offered, maxCreates, maximumCreates, timeout, grace, runId,
    vus: integer(env, 'K6_PREALLOCATED_VUS', Math.max(100, rate * burst), 1, 100000),
    warmupRamp, warmupHold, warmupStartRate, warmupOffered,
    seed: integer(env, 'K6_SEED', 42, 0),
    p95: integer(env, 'K6_P95_MS', 300), p99: integer(env, 'K6_P99_MS', 1000), errorRate,
  };
}

function hash(value) {
  let result = value | 0;
  result = Math.imul(result ^ (result >>> 16), 0x21f0aaad);
  result = Math.imul(result ^ (result >>> 15), 0x735a2d97);
  return (result ^ (result >>> 15)) >>> 0;
}

export function operation(index, seed) {
  // A permutation gives an exact 70/20/5/5 mix in every 100 scheduled iterations.
  const bucket = (index * 37 + seed) % 100;
  const random = hash(index + seed * 65537);
  const hot = random % 10 < 8;
  const fruit = hot ? 1 + hash(random) % 100 : 101 + hash(random) % 900;
  const kind = bucket < 70 ? 'detail' : bucket < 90 ? 'list' : bucket < 95 ? 'missing' : 'create';
  return { kind, fruit, offset: (hash(random + 1) % 20) * 50 };
}

export function createName(runId, phase, index) { return `bench-${runId}-${phase}-${index}`; }

const originalNames = ['Apple', 'Pear', 'Banana', 'Orange', 'Strawberry', 'Mango', 'Grape', 'Pineapple', 'Watermelon', 'Kiwi'];
export function fruitName(id) { return id <= 10 ? originalNames[id - 1] : `Fruit${String(id).padStart(4, '0')}`; }

export function validSeededFruit(fruit, id) {
  const prices = fruit?.storePrices;
  return fruit?.id === id && fruit.name === fruitName(id) && Array.isArray(prices) && prices.length === 10
    && new Set(prices.map((price) => price?.store?.id)).size === 10
    && prices.every((price) => {
      const store = price?.store;
      return Number.isFinite(price?.price) && price.price >= 0
        && Number.isInteger(store?.id) && store.id > 0 && store.id <= 100
        && ['name', 'currency'].every((key) => typeof store[key] === 'string' && store[key].length > 0)
        && ['address', 'city', 'country'].every((key) => typeof store.address?.[key] === 'string' && store.address[key].length > 0);
    });
}

export function summary(data, config) {
  const metrics = data.metrics;
  const metric = (name, field, fallback = 0) => metrics[name]?.values?.[field] ?? fallback;
  const selector = '{scenario:measured}';
  const started = metric('measured_started', 'count');
  const completed = metric('measured_completed', 'count');
  const dropped = metric(`dropped_iterations${selector}`, 'count');
  const failures = Object.entries(metrics).flatMap(([name, value]) => Object.entries(value.thresholds || {})
    .filter(([, threshold]) => threshold === false || threshold.ok === false).map(([threshold]) => `${name}: ${threshold}`));
  const required = [
    ['measured_started', 'count'], ['measured_completed', 'count'],
    [`http_req_failed${selector}`, 'rate'], [`checks${selector}`, 'rate'],
    ...['p(50)', 'p(95)', 'p(99)'].map((field) => [`http_req_duration${selector}`, field]),
  ];
  const validMetrics = required.every(([name, field]) => Number.isFinite(metrics[name]?.values?.[field]) && metrics[name].values[field] >= 0);
  if (!validMetrics) failures.push('Missing or invalid measured metrics');
  const complete = started > 0 && completed === started && started + dropped >= config.offered * 0.99;
  const sloPassed = validMetrics && metric(`checks${selector}`, 'rate') >= 0.999
    && metric(`http_req_failed${selector}`, 'rate') < config.errorRate
    && metric(`http_req_duration${selector}`, 'p(95)') < config.p95
    && metric(`http_req_duration${selector}`, 'p(99)') < config.p99;
  const warmupStarted = metric('warmup_started', 'count');
  const warmupCompleted = metric('warmup_completed', 'count');
  const warmupDropped = metric('dropped_iterations{scenario:warmup}', 'count');
  const warmupComplete = [warmupStarted, warmupCompleted, warmupDropped].every(Number.isFinite)
    && warmupStarted > 0 && warmupDropped >= 0 && warmupCompleted === warmupStarted
    && warmupStarted + warmupDropped >= Math.max(1, Math.floor(config.warmupOffered * 0.99));
  const warmupPassed = warmupComplete && warmupDropped === 0
    && metric('checks{scenario:warmup}', 'rate', NaN) === 1
    && metric('http_req_failed{scenario:warmup}', 'rate', NaN) === 0;
  if (!warmupPassed) failures.push('Warmup was incomplete, dropped arrivals or returned incorrect responses');
  return {
    schema_version: 1, config, passed: complete && sloPassed && warmupPassed && failures.length === 0 && dropped === 0,
    failures, complete, requests: completed, offered_requests: config.offered,
    achieved_rps: completed / config.duration, dropped_iterations: dropped,
    error_rate: metric(`http_req_failed${selector}`, 'rate'),
    check_failure_rate: 1 - metric(`checks${selector}`, 'rate', 0),
    p50_ms: metric(`http_req_duration${selector}`, 'p(50)'),
    p95_ms: metric(`http_req_duration${selector}`, 'p(95)'),
    p99_ms: metric(`http_req_duration${selector}`, 'p(99)'),
    warmup: {
      complete: warmupComplete, passed: warmupPassed, requests: warmupCompleted,
      offered_requests: config.warmupOffered, dropped_iterations: warmupDropped,
      error_rate: metric('http_req_failed{scenario:warmup}', 'rate'),
      check_failure_rate: 1 - metric('checks{scenario:warmup}', 'rate'),
    },
    operations: Object.fromEntries(['detail', 'list', 'missing', 'create'].map((name) => [name,
      metrics[`http_req_duration{scenario:measured,operation:${name}}`]?.values || {},
    ])),
  };
}

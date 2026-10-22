import assert from 'node:assert/strict';
import test from 'node:test';
import { configuration, operation, createName, fruitName, validSeededFruit, summary } from './workload.mjs';

function successfulWarmup(config) {
  const count = Math.floor(config.warmupOffered);
  return {
    warmup_started: { values: { count } }, warmup_completed: { values: { count } },
    'checks{scenario:warmup}': { values: { rate: 1 } },
    'http_req_failed{scenario:warmup}': { values: { rate: 0 } },
  };
}

test('the offered rate integrates the burst and write budget', () => {
  const config = configuration({ K6_RATE: '100', K6_RUN_ID: 'test' });
  let previous = config.rate;
  const offered = config.stages.reduce((total, stage) => {
    const count = Number.parseFloat(stage.duration) * (previous + stage.target) / 2;
    previous = stage.target;
    return total + count;
  }, 0);
  assert.ok(Math.abs(offered - config.offered) < 0.000001);
  assert.equal(config.warmupOffered, 15 * (10 + 100) / 2 + 45 * 100);
  assert.ok(configuration({ K6_RATE: '1000' }).maximumCreates <= 10000);
  assert.throws(() => configuration({ K6_RATE: '1000', K6_MAX_CREATES: '8000' }), /Write budget/);
  assert.throws(() => configuration({ K6_RATE: '10000' }), /Write budget/);
  assert.throws(() => configuration({ K6_RATE: '100junk' }), /integer/);
  assert.throws(() => configuration({ K6_TIMEOUT_SECONDS: '20', K6_GRACE_SECONDS: '10' }), /integer/);
  assert.throws(() => configuration({ K6_RUN_ID: 'unsafe/name' }), /K6_RUN_ID/);
});

test('workload has exact mix, hot and cold keys, bounded pages and deterministic requests', () => {
  const counts = {};
  let hot = 0;
  for (let index = 0; index < 10000; index++) {
    const item = operation(index, 42);
    assert.deepEqual(item, operation(index, 42));
    counts[item.kind] = (counts[item.kind] || 0) + 1;
    assert.ok(item.fruit >= 1 && item.fruit <= 1000);
    assert.ok(item.offset >= 0 && item.offset <= 950 && item.offset % 50 === 0);
    if (item.fruit <= 100) hot++;
  }
  assert.deepEqual(counts, { detail: 7000, list: 2000, missing: 500, create: 500 });
  assert.ok(hot > 7700 && hot < 8300);
  assert.equal(fruitName(1), 'Apple');
  assert.equal(fruitName(1000), 'Fruit1000');
  assert.notEqual(createName('run1', 'warmup', 0), createName('run1', 'measured', 0));
  assert.notEqual(createName('run1', 'warmup', 0), createName('run1', 'warmup', 100));
});

test('measured summary excludes warmup and fails incomplete and dropped runs', () => {
  const config = configuration({ K6_RATE: '10', K6_DURATION_SECONDS: '20', K6_PROFILE: 'steady' });
  const metrics = {
    ...successfulWarmup(config),
    measured_started: { values: { count: 200 } }, measured_completed: { values: { count: 200 } },
    'http_req_duration{scenario:measured}': { values: { 'p(50)': 10, 'p(95)': 20, 'p(99)': 30 } },
    http_req_duration: { values: { 'p(95)': 9999 } },
    'checks{scenario:measured}': { values: { rate: 1 } },
    'http_req_failed{scenario:measured}': { values: { rate: 0 } },
  };
  let result = summary({ metrics }, config);
  assert.equal(result.passed, true);
  assert.equal(result.achieved_rps, 10);
  assert.equal(result.p95_ms, 20);
  metrics['dropped_iterations{scenario:measured}'] = { values: { count: 1 } };
  assert.equal(summary({ metrics }, config).passed, false);
  delete metrics['dropped_iterations{scenario:measured}'];
  metrics.measured_completed.values.count = 199;
  assert.equal(summary({ metrics }, config).complete, false);
  metrics.measured_started.values.count = 199;
  metrics['http_req_failed{scenario:measured}'].thresholds = { 'rate<0.01': { ok: false } };
  assert.equal(summary({ metrics }, config).passed, false);
  metrics.measured_started.values.count = 50;
  metrics.measured_completed.values.count = 50;
  assert.equal(summary({ metrics }, config).complete, false);
});

test('missing, nonfinite or failed measured metrics cannot pass without threshold metadata', () => {
  const config = configuration({ K6_RATE: '10', K6_DURATION_SECONDS: '20', K6_PROFILE: 'steady' });
  const metrics = {
    ...successfulWarmup(config),
    measured_started: { values: { count: 200 } }, measured_completed: { values: { count: 200 } },
    'http_req_duration{scenario:measured}': { values: { 'p(50)': 10, 'p(95)': 20, 'p(99)': 30 } },
    'checks{scenario:measured}': { values: { rate: 1 } },
    'http_req_failed{scenario:measured}': { values: { rate: 0 } },
  };
  assert.equal(summary({ metrics }, config).passed, true);
  metrics['checks{scenario:measured}'].values.rate = 0.5;
  assert.equal(summary({ metrics }, config).passed, false);
  metrics['checks{scenario:measured}'].values.rate = 1;
  metrics['http_req_duration{scenario:measured}'].values['p(95)'] = NaN;
  assert.equal(summary({ metrics }, config).passed, false);
  delete metrics['http_req_duration{scenario:measured}'];
  assert.equal(summary({ metrics }, config).passed, false);
});

test('warmup failures invalidate a run without changing measured latency or errors', () => {
  const config = configuration({ K6_RATE: '10', K6_DURATION_SECONDS: '20', K6_PROFILE: 'steady' });
  const metrics = {
    ...successfulWarmup(config),
    measured_started: { values: { count: 200 } }, measured_completed: { values: { count: 200 } },
    'http_req_duration{scenario:measured}': { values: { 'p(50)': 10, 'p(95)': 20, 'p(99)': 30 } },
    'checks{scenario:measured}': { values: { rate: 1 } },
    'http_req_failed{scenario:measured}': { values: { rate: 0 } },
    'http_req_duration{scenario:warmup}': { values: { 'p(95)': 9999 } },
  };
  assert.equal(summary({ metrics }, config).passed, true);
  metrics['http_req_failed{scenario:warmup}'].values.rate = 0.1;
  const failed = summary({ metrics }, config);
  assert.equal(failed.passed, false);
  assert.equal(failed.error_rate, 0);
  assert.equal(failed.p95_ms, 20);
  metrics['http_req_failed{scenario:warmup}'].values.rate = 0;
  metrics['dropped_iterations{scenario:warmup}'] = { values: { count: 1 } };
  assert.equal(summary({ metrics }, config).passed, false);
  delete metrics['dropped_iterations{scenario:warmup}'];
  metrics.warmup_completed.values.count--;
  assert.equal(summary({ metrics }, config).warmup.complete, false);
});

test('seeded response checks require the full price/store graph', () => {
  const fruit = { id: 1, name: 'Apple', storePrices: Array.from({ length: 10 }, (_, i) => ({
    price: 1.99, store: { id: i + 1, name: `Store ${i + 1}`, currency: 'CHF', address: { address: '1 Street', city: 'Zurich', country: 'Switzerland' } },
  })) };
  assert.equal(validSeededFruit(fruit, 1), true);
  assert.equal(validSeededFruit(fruit, 2), false);
  fruit.storePrices[0].store.address = undefined;
  assert.equal(validSeededFruit(fruit, 1), false);
  fruit.storePrices.pop();
  assert.equal(validSeededFruit(fruit, 1), false);
});

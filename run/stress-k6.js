import http from 'k6/http';
import { check } from 'k6';
import execution from 'k6/execution';
import { Counter } from 'k6/metrics';
import { configuration, operation, createName, fruitName, validSeededFruit, summary } from './workload.mjs';

const config = configuration(__ENV);
const started = new Counter('measured_started');
const completed = new Counter('measured_completed');
const warmupStarted = new Counter('warmup_started');
const warmupCompleted = new Counter('warmup_completed');
const measuredTag = 'scenario:measured';

export const options = {
  discardResponseBodies: true,
  // Avoid a time series per fruit name or create request.
  systemTags: ['status', 'method', 'name', 'scenario', 'expected_response', 'error_code'],
  summaryTrendStats: ['avg', 'p(50)', 'p(95)', 'p(99)', 'max'],
  scenarios: {
    warmup: {
      executor: 'ramping-arrival-rate', exec: 'warmup', timeUnit: '1s',
      startRate: config.warmupStartRate,
      preAllocatedVUs: config.vus, maxVUs: config.vus,
      stages: [
        { duration: `${config.warmupRamp}s`, target: config.rate },
        { duration: `${config.warmupHold}s`, target: config.rate },
      ],
      gracefulStop: `${config.grace}s`,
    },
    measured: {
      executor: 'ramping-arrival-rate', exec: 'measured', timeUnit: '1s',
      startTime: `${config.warmupRamp + config.warmupHold + config.grace}s`,
      startRate: config.rate, preAllocatedVUs: config.vus, maxVUs: config.vus,
      stages: config.stages, gracefulStop: `${config.grace}s`,
    },
  },
  thresholds: {
    'checks{scenario:warmup}': ['rate==1'],
    'http_req_failed{scenario:warmup}': ['rate==0'],
    'dropped_iterations{scenario:warmup}': ['count==0'],
    warmup_started: ['count>0'],
    warmup_completed: [`count>=${Math.max(1, Math.floor(config.warmupOffered * 0.99))}`],
    [`checks{${measuredTag}}`]: ['rate>=0.999'],
    [`http_req_failed{${measuredTag}}`]: [`rate<${config.errorRate}`],
    [`http_req_duration{${measuredTag}}`]: [`p(95)<${config.p95}`, `p(99)<${config.p99}`],
    [`dropped_iterations{${measuredTag}}`]: ['count==0'],
    measured_started: ['count>0'],
    measured_completed: ['count>0'],
    // Materialize operation submetrics in the JSON summary without changing SLOs.
    ...Object.fromEntries(['detail', 'list', 'missing', 'create'].map((name) => [
      `http_req_duration{scenario:measured,operation:${name}}`, ['max>=0'],
    ])),
  },
};

export function setup() {
  // Fail before load if seed data or response shape differs between implementations.
  const probe = { responseType: 'text', timeout: `${config.timeout}s` };
  const detail = http.get(`${config.baseUrl}/fruits/Apple`, probe);
  const page = http.get(`${config.baseUrl}/fruits?limit=2&offset=0`, probe);
  if (detail.status !== 200 || !validSeededFruit(detail.json(), 1) || page.status !== 200
      || !Array.isArray(page.json()) || page.json().length !== 2) {
    throw new Error('API preflight failed: initialize run/db.sql and enable bounded pagination');
  }
}

function request(isMeasured) {
  const index = execution.scenario.iterationInTest;
  const item = operation(index, config.seed);
  const inspectBody = item.kind === 'create' || index % 20 === 0;
  const params = {
    tags: { name: item.kind === 'create' ? 'POST /fruits' : `GET /fruits (${item.kind})`, operation: item.kind },
    timeout: `${config.timeout}s`, responseType: inspectBody ? 'text' : 'none',
    responseCallback: http.expectedStatuses(...(item.kind === 'missing' ? [404] : item.kind === 'create' ? [200, 201] : [200])),
  };
  let path;
  let body;
  if (item.kind === 'create') {
    path = '/fruits';
    body = { name: createName(config.runId, isMeasured ? 'measured' : 'warmup', index), description: `Seasonal produce batch ${index}` };
    params.headers = { 'Content-Type': 'application/json' };
  } else if (item.kind === 'list') {
    path = `/fruits?limit=50&offset=${item.offset}`;
  } else {
    path = `/fruits/${item.kind === 'missing' ? `unavailable-${item.fruit}` : fruitName(item.fruit)}`;
  }

  (isMeasured ? started : warmupStarted).add(1);
  const response = item.kind === 'create'
    ? http.post(`${config.baseUrl}${path}`, JSON.stringify(body), params)
    : http.get(`${config.baseUrl}${path}`, params);
  const statusOK = item.kind === 'missing' ? response.status === 404
    : item.kind === 'create' ? [200, 201].includes(response.status) : response.status === 200;
  let payloadOK = statusOK;
  if (statusOK && inspectBody && item.kind !== 'missing') {
    try {
      const payload = response.json();
      payloadOK = item.kind === 'list'
        ? Array.isArray(payload) && payload.length === 50 && payload.every((fruit, offset) => validSeededFruit(fruit, item.offset + offset + 1))
        : body ? payload.id > 0 && payload.name === body.name : validSeededFruit(payload, item.fruit);
    } catch (_) {
      payloadOK = false;
    }
  }
  check(response, { 'expected status and payload': () => statusOK && payloadOK });
  (isMeasured ? completed : warmupCompleted).add(1);
}

export function warmup() { request(false); }
export function measured() { request(true); }

export function handleSummary(data) {
  const result = summary(data, config);
  return {
    [__ENV.K6_RESULT_FILE || 'k6-result.json']: JSON.stringify({ ...result, metrics: data.metrics }, null, 2),
    stdout: `${JSON.stringify(result, null, 2)}\n`,
  };
}

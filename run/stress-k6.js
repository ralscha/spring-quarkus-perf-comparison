import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

function readPositiveIntEnv(name, defaultValue) {
  const rawValue = __ENV[name];

  if (rawValue === undefined || rawValue === '') {
    return defaultValue;
  }

  const parsedValue = Number.parseInt(rawValue, 10);
  return Number.isInteger(parsedValue) && parsedValue > 0 ? parsedValue : defaultValue;
}

const baseUrl = 'http://localhost:8080';
const vus = readPositiveIntEnv('K6_VUS', 100);
const warmupRampUpDurationSeconds = readPositiveIntEnv('K6_WARMUP_RAMP_UP_SECONDS', 20);
const warmupHoldDurationSeconds = readPositiveIntEnv('K6_WARMUP_HOLD_SECONDS', 40);
const warmupRampDownDurationSeconds = readPositiveIntEnv('K6_WARMUP_RAMP_DOWN_SECONDS', 10);
const measuredHoldDurationSeconds = readPositiveIntEnv('K6_MEASURED_HOLD_SECONDS', 20);
const requestTimeoutSeconds = 0;
const gracefulRampDownSeconds = readPositiveIntEnv('K6_GRACEFUL_RAMP_DOWN_SECONDS', 15);
const gracefulStopSeconds = readPositiveIntEnv('K6_GRACEFUL_STOP_SECONDS', 15);

const measuredScenarioName = 'measured_cycle';
const measuredRequestCounter = new Counter('measured_requests');
const requests = [
  { path: '/fruits', name: 'GET /fruits' },
  { path: '/fruits/Pineapple', name: 'GET /fruits/Pineapple' },
  { path: '/fruits/Apple', name: 'GET /fruits/Apple' },
];

function formatSeconds(seconds) {
  return `${seconds}s`;
}

const warmupCycleStartTime = formatSeconds(
  warmupRampUpDurationSeconds
    + warmupHoldDurationSeconds
    + warmupRampDownDurationSeconds
);

function createWarmupStages() {
  return [
    { duration: formatSeconds(warmupRampUpDurationSeconds), target: vus },
    { duration: formatSeconds(warmupHoldDurationSeconds), target: vus },
    { duration: formatSeconds(warmupRampDownDurationSeconds), target: 0 },
  ];
}

export const options = {
  discardResponseBodies: true,
  summaryTrendStats: ['min', 'avg', 'med', 'max', 'p(50)', 'p(95)', 'p(99)'],
  scenarios: {
    warmup_cycle: {
      executor: 'ramping-vus',
      stages: createWarmupStages(),
      gracefulRampDown: formatSeconds(gracefulRampDownSeconds),
      gracefulStop: formatSeconds(gracefulStopSeconds),
      exec: 'warmupCycle',
    },
    measured_cycle: {
      executor: 'constant-vus',
      startTime: warmupCycleStartTime,
      vus,
      duration: formatSeconds(measuredHoldDurationSeconds),
      gracefulStop: formatSeconds(gracefulStopSeconds),
      exec: 'measuredCycle',
    },
  },
  thresholds: {
    [`checks{scenario:${measuredScenarioName}}`]: ['rate==1'],
    [`http_req_failed{scenario:${measuredScenarioName}}`]: ['rate==0'],
    [`http_req_duration{scenario:${measuredScenarioName}}`]: ['max>=0'],
  },
};

function getThresholdResult(metric, thresholdName) {
  const threshold = metric?.thresholds?.[thresholdName];

  if (typeof threshold === 'boolean') {
    return threshold;
  }

  if (threshold && typeof threshold.ok === 'boolean') {
    return threshold.ok;
  }

  return false;
}

function makeRequests(trackMeasuredRequests = false) {
  for (const request of requests) {
    const params = {
      tags: { name: request.name },
    };

    if (requestTimeoutSeconds > 0) {
      params.timeout = formatSeconds(requestTimeoutSeconds);
    }

    const response = http.get(`${baseUrl}${request.path}`, params);

    if (trackMeasuredRequests) {
      measuredRequestCounter.add(1);
    }

    check(response, {
      'status is 200': (res) => res.status === 200,
    });
  }
}

export function warmupCycle() {
  makeRequests();
}

export function measuredCycle() {
  makeRequests(true);
}

export function handleSummary(data) {
  const measuredChecksMetric = data.metrics[`checks{scenario:${measuredScenarioName}}`];
  const measuredHttpFailuresMetric = data.metrics[`http_req_failed{scenario:${measuredScenarioName}}`];
  const measuredDurationMetric = data.metrics[`http_req_duration{scenario:${measuredScenarioName}}`];
  const measuredRequestsMetric = data.metrics.measured_requests;

  const allChecksPassed =
    getThresholdResult(measuredChecksMetric, 'rate==1')
    && getThresholdResult(measuredHttpFailuresMetric, 'rate==0')
    && getThresholdResult(measuredDurationMetric, 'max>=0');

  const measuredRequestCount = measuredRequestsMetric?.values?.count ?? 0;
  const measuredLatencyP50 = Number(measuredDurationMetric?.values?.['p(50)'] ?? 0).toFixed(3);
  const measuredLatencyP95 = Number(measuredDurationMetric?.values?.['p(95)'] ?? 0).toFixed(3);
  const measuredLatencyP99 = Number(measuredDurationMetric?.values?.['p(99)'] ?? 0).toFixed(3);

  return {
    stdout: `all checks passed: ${allChecksPassed ? 'yes' : 'no'}\nmeasured cycle requests: ${measuredRequestCount}\nmeasured cycle latency p50 ms: ${measuredLatencyP50}\nmeasured cycle latency p95 ms: ${measuredLatencyP95}\nmeasured cycle latency p99 ms: ${measuredLatencyP99}\n`,
  };
}

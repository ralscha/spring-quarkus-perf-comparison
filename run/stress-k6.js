import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const baseUrl = 'http://localhost:8080';
const vus = 100;
const rampUpDurationSeconds = 15;
const holdDurationSeconds = 20;
const rampDownDurationSeconds = 15;
const requestTimeoutSeconds = 0;
const gracefulRampDownSeconds = 30;
const gracefulStopSeconds = 30;

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
  rampUpDurationSeconds
    + holdDurationSeconds
    + rampDownDurationSeconds
);

function createStages() {
  return [
    { duration: formatSeconds(rampUpDurationSeconds), target: vus },
    { duration: formatSeconds(holdDurationSeconds), target: vus },
    { duration: formatSeconds(rampDownDurationSeconds), target: 0 },
  ];
}

export const options = {
  discardResponseBodies: true,
  scenarios: {
    warmup_cycle: {
      executor: 'ramping-vus',
      stages: createStages(),
      gracefulRampDown: formatSeconds(gracefulRampDownSeconds),
      gracefulStop: formatSeconds(gracefulStopSeconds),
      exec: 'warmupCycle',
    },
    measured_cycle: {
      executor: 'ramping-vus',
      startTime: warmupCycleStartTime,
      stages: createStages(),
      gracefulRampDown: formatSeconds(gracefulRampDownSeconds),
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

  return {
    stdout: `all checks passed: ${allChecksPassed ? 'yes' : 'no'}\nmeasured cycle requests: ${measuredRequestCount}\n`,
  };
}

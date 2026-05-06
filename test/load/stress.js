import http from 'k6/http';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';

const baseUrl = __ENV.TARGET_BASE_URL || 'http://localhost:9999';
const scenarioName = __ENV.RINHA_SCENARIO || 'burst';
const duration = __ENV.RINHA_DURATION || '20s';
const rate = Number(__ENV.RINHA_RATE || '100');
const preAllocatedVUs = Number(__ENV.RINHA_PRE_ALLOCATED_VUS || '50');
const maxVUs = Number(__ENV.RINHA_MAX_VUS || '200');
const retryOn503 = (__ENV.RETRY_ON_503 || '0') === '1';
const retryDelayMs = Number(__ENV.RETRY_DELAY_MS || '50');

const payloads = new SharedArray('stress-payloads', function () {
  return JSON.parse(open('./payloads.json'));
});

const scenarioOptions = {
  burst: {
    executor: 'constant-arrival-rate',
    rate,
    timeUnit: '1s',
    duration,
    preAllocatedVUs,
    maxVUs,
  },
  ramp: {
    executor: 'ramping-arrival-rate',
    startRate: Math.max(1, Math.floor(rate / 4)),
    timeUnit: '1s',
    preAllocatedVUs,
    maxVUs,
    stages: [
      { duration: '10s', target: Math.max(1, Math.floor(rate / 2)) },
      { duration: '10s', target: rate },
      { duration: '10s', target: Math.max(1, Math.floor(rate / 3)) },
    ],
  },
  soak: {
    executor: 'constant-arrival-rate',
    rate,
    timeUnit: '1s',
    duration,
    preAllocatedVUs,
    maxVUs,
  },
  degrade: {
    executor: 'constant-arrival-rate',
    rate,
    timeUnit: '1s',
    duration,
    preAllocatedVUs,
    maxVUs,
  },
};

export const options = {
  summaryTrendStats: ['avg', 'min', 'med', 'p(95)', 'p(99)', 'max'],
  scenarios: {
    [scenarioName]: scenarioOptions[scenarioName] || scenarioOptions.burst,
  },
  thresholds: {
    http_req_failed: ['rate<0.05'],
    checks: ['rate>0.95'],
  },
};

export default function stressScenario() {
  const iteration = exec.scenario.iterationInTest;
  const payload = payloads[iteration % payloads.length];
  const body = JSON.stringify(payload);
  const requestParams = {
    headers: { 'Content-Type': 'application/json' },
    timeout: '2000ms',
  };

  let res = http.post(`${baseUrl}/fraud-score`, body, requestParams);
  if (retryOn503 && res.status === 503) {
    sleep(retryDelayMs / 1000);
    res = http.post(`${baseUrl}/fraud-score`, body, requestParams);
  }

  check(res, {
    'status is 200': (response) => response.status === 200,
    'body is json': (response) => {
      try {
        JSON.parse(response.body);
        return true;
      } catch {
        return false;
      }
    },
    'approved is boolean': (response) => {
      try {
        return typeof JSON.parse(response.body).approved === 'boolean';
      } catch {
        return false;
      }
    },
    'fraud_score is number': (response) => {
      try {
        return typeof JSON.parse(response.body).fraud_score === 'number';
      } catch {
        return false;
      }
    },
  });
}

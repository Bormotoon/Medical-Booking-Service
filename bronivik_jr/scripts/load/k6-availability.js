import http from 'k6/http';
import { check, sleep } from 'k6';

// Configure target and credentials via env vars
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080'; // HTTP API port
const API_KEY = __ENV.API_KEY || 'set-me';
const API_EXTRA = __ENV.API_EXTRA || 'set-me';

export const options = {
  scenarios: {
    availability_burst: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.RATE || 50), // requests per second
      timeUnit: '1s',
      duration: __ENV.DURATION || '1m',
      preAllocatedVUs: 20,
      maxVUs: 200,
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<400'],
  },
};

export default function () {
  const date = __ENV.DATE || '2025-12-01';
  const item = __ENV.ITEM || 'camera';
  const url = `${BASE_URL}/api/v1/availability/${encodeURIComponent(item)}?date=${date}`;

  const res = http.get(url, {
    headers: {
      'x-api-key': API_KEY,
      'x-api-extra': API_EXTRA,
    },
  });

  check(res, {
    'status is 200 or 404': (r) => r.status === 200 || r.status === 404,
  });

  sleep(0.1);
}

// Example run:
// BASE_URL=http://localhost:8080 API_KEY=your_api_key API_EXTRA=your_api_extra ITEM=camera RATE=100 DURATION=2m k6 run scripts/load/k6-availability.js

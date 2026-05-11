import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate          = new Rate('error_rate');
const createTaskDuration = new Trend('create_task_duration', true);

const BASE_URL        = __ENV.BASE_URL        || 'http://localhost:8080';
const PROJECT_UUID    = __ENV.PROJECT_UUID    || '';
const FEDERATION_UUID = __ENV.FEDERATION_UUID || '';
const COMPANY_UUID    = __ENV.COMPANY_UUID    || '';
const USER_EMAIL      = 'bench@test.com';
const USER_PASSWORD   = 'BenchPass123';

export const options = {
    stages: [
        { duration: '30s', target: 100  },
        { duration: '30s', target: 1000 },
        { duration: '1m',  target: 1000 },
        { duration: '30s', target: 100  },
        { duration: '30s', target: 0    },
    ],
    thresholds: {
        http_req_failed: ['rate<0.15'],
    },
};

export function setup() {
    http.post(
        `${BASE_URL}/api/v1/auth/register`,
        JSON.stringify({ name: 'Bench', lname: 'User', email: USER_EMAIL, password: USER_PASSWORD }),
        { headers: { 'Content-Type': 'application/json' } }
    );

    const loginRes = http.post(
        `${BASE_URL}/api/v1/auth/login`,
        JSON.stringify({ email: USER_EMAIL, password: USER_PASSWORD }),
        { headers: { 'Content-Type': 'application/json' } }
    );

    return { token: loginRes.json('access_token') };
}

export default function (data) {
    const headers = {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${data.token}`,
    };

    const res = http.post(
        `${BASE_URL}/api/v1/tasks`,
        JSON.stringify({
            name:            `Spike task ${__VU}-${__ITER}`,
            project_uuid:    PROJECT_UUID,
            federation_uuid: FEDERATION_UUID,
            company_uuid:    COMPANY_UUID,
            implement_by:    USER_EMAIL,
            priority:        1,
        }),
        { headers, tags: { name: 'create_task_spike' } }
    );

    createTaskDuration.add(res.timings.duration);

    const ok = check(res, { 'status 201': (r) => r.status === 201 });
    errorRate.add(!ok);

    sleep(0.05);
}
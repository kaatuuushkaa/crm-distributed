import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const createTaskDuration = new Trend('create_task_duration', true);
const getTaskDuration    = new Trend('get_task_duration', true);
const errorRate          = new Rate('error_rate');

const BASE_URL        = __ENV.BASE_URL        || 'http://localhost:8080';
const PROJECT_UUID    = __ENV.PROJECT_UUID    || '';
const FEDERATION_UUID = __ENV.FEDERATION_UUID || '';
const COMPANY_UUID    = __ENV.COMPANY_UUID    || '';
const USER_EMAIL      = 'bench@test.com';
const USER_PASSWORD   = 'BenchPass123';

export const options = {
    stages: [
        { duration: '1m', target: 200 },
        { duration: '8m', target: 200 },
        { duration: '1m', target: 0   },
    ],
    thresholds: {
        http_req_duration:      ['p(95)<300'],
        http_req_failed:        ['rate<0.02'],
        'create_task_duration': ['p(99)<800'],
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

    const createRes = http.post(
        `${BASE_URL}/api/v1/tasks`,
        JSON.stringify({
            name:            `Soak task ${__VU}-${__ITER}`,
            project_uuid:    PROJECT_UUID,
            federation_uuid: FEDERATION_UUID,
            company_uuid:    COMPANY_UUID,
            implement_by:    USER_EMAIL,
            priority:        2,
        }),
        { headers, tags: { name: 'create_task_soak' } }
    );

    createTaskDuration.add(createRes.timings.duration);
    const ok = check(createRes, { 'create 201': (r) => r.status === 201 });
    errorRate.add(!ok);

    if (ok) {
        const taskUuid = createRes.json('uuid');

        const getRes = http.get(
            `${BASE_URL}/api/v1/tasks/${taskUuid}`,
            { headers, tags: { name: 'get_task_soak' } }
        );

        getTaskDuration.add(getRes.timings.duration);
        check(getRes, { 'get 200': (r) => r.status === 200 });
    }

    sleep(0.2);
}
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const PROJECT_UUID = __ENV.PROJECT_UUID || '';

export function getToken() {
    const res = http.post(
        `${BASE_URL}/api/v1/auth/login`,
        JSON.stringify({ email: 'bench@test.com', password: 'BenchPass123' }),
        { headers: { 'Content-Type': 'application/json' } }
    );

    check(res, { 'login ok': (r) => r.status === 200 });
    return res.json('access_token');
}

export function createTask(token, name) {
    return http.post(
        `${BASE_URL}/api/v1/tasks`,
        JSON.stringify({
            name: name,
            project_uuid: PROJECT_UUID,
            priority: 5,
        }),
        {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${token}`,
            },
        }
    );
}

export function getTask(token, taskUuid) {
    return http.get(`${BASE_URL}/api/v1/tasks/${taskUuid}`, {
        headers: { Authorization: `Bearer ${token}` },
    });
}
"""
FastAPI аналог task-service для сравнительного бенчмарка.
Реализует те же эндпоинты что и Go task-service:
  POST /auth/register
  POST /auth/login
  POST /tasks
  GET  /tasks/{uuid}
  GET  /health
"""
import os
import uuid
import time
import hashlib
import hmac
import base64
import json
from datetime import datetime, timedelta
from typing import Optional

import asyncpg
import jwt
from fastapi import FastAPI, HTTPException, Depends
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from pydantic import BaseModel
import uvicorn

app = FastAPI(title="CRM FastAPI benchmark")

DB_URL = os.getenv(
    "DATABASE_URL",
    "postgresql://crm:crm_secret@localhost:5432/crm_bench"
)
JWT_SECRET = os.getenv("JWT_SECRET", "super-secret-jwt-key")
JWT_ALGO = "HS256"
ACCESS_TTL = timedelta(minutes=10)

pool: asyncpg.Pool = None

@app.on_event("startup")
async def startup():
    global pool
    pool = await asyncpg.create_pool(DB_URL, min_size=5, max_size=25)
    async with pool.acquire() as conn:
        await conn.execute("""
            CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

            CREATE TABLE IF NOT EXISTS bench_users (
                uuid UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                email TEXT UNIQUE NOT NULL,
                password_hash TEXT NOT NULL,
                created_at TIMESTAMPTZ DEFAULT now()
            );

            CREATE TABLE IF NOT EXISTS bench_projects (
                uuid UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                name TEXT NOT NULL,
                task_id INT NOT NULL DEFAULT 0,
                created_at TIMESTAMPTZ DEFAULT now()
            );

            CREATE TABLE IF NOT EXISTS bench_tasks (
                uuid UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                id INT NOT NULL,
                name TEXT NOT NULL,
                project_uuid UUID NOT NULL,
                created_by TEXT NOT NULL,
                status INT NOT NULL DEFAULT 0,
                priority INT NOT NULL DEFAULT 0,
                created_at TIMESTAMPTZ DEFAULT now()
            );
        """)

@app.on_event("shutdown")
async def shutdown():
    await pool.close()

def hash_password(password: str) -> str:
    """bcrypt-like hash через hashlib для простоты бенчмарка."""
    return hashlib.sha256(password.encode()).hexdigest()

def make_token(user_uuid: str, email: str) -> str:
    payload = {
        "uuid": user_uuid,
        "email": email,
        "exp": datetime.utcnow() + ACCESS_TTL,
        "iat": datetime.utcnow(),
    }
    return jwt.encode(payload, JWT_SECRET, algorithm=JWT_ALGO)

def verify_token(token: str) -> dict:
    try:
        return jwt.decode(token, JWT_SECRET, algorithms=[JWT_ALGO])
    except jwt.PyJWTError:
        raise HTTPException(status_code=401, detail="invalid token")

security = HTTPBearer()

async def current_user(
    creds: HTTPAuthorizationCredentials = Depends(security),
) -> dict:
    return verify_token(creds.credentials)

class RegisterRequest(BaseModel):
    name: str
    email: str
    password: str

class LoginRequest(BaseModel):
    email: str
    password: str

class CreateTaskRequest(BaseModel):
    name: str
    project_uuid: str
    priority: int = 0

@app.get("/healthz")
async def healthz():
    return {"status": "ok"}

@app.post("/api/v1/auth/register", status_code=201)
async def register(req: RegisterRequest):
    async with pool.acquire() as conn:
        existing = await conn.fetchrow(
            "SELECT uuid FROM bench_users WHERE email = $1", req.email
        )
        if existing:
            raise HTTPException(status_code=409, detail="user already exists")

        row = await conn.fetchrow(
            "INSERT INTO bench_users (email, password_hash) VALUES ($1, $2) RETURNING uuid",
            req.email, hash_password(req.password)
        )
        return {"uuid": str(row["uuid"]), "message": "пользователь создан"}

@app.post("/api/v1/auth/login")
async def login(req: LoginRequest):
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            "SELECT uuid, password_hash FROM bench_users WHERE email = $1",
            req.email
        )
        if not row or row["password_hash"] != hash_password(req.password):
            raise HTTPException(status_code=401, detail="invalid credentials")

        token = make_token(str(row["uuid"]), req.email)
        return {
            "access_token": token,
            "user_uuid": str(row["uuid"]),
            "expires_at": (datetime.utcnow() + ACCESS_TTL).isoformat(),
        }

@app.post("/api/v1/tasks", status_code=201)
async def create_task(req: CreateTaskRequest, user: dict = Depends(current_user)):
    async with pool.acquire() as conn:
        async with conn.transaction():
            project = await conn.fetchrow(
                "SELECT uuid, task_id FROM bench_projects WHERE uuid = $1 FOR UPDATE",
                uuid.UUID(req.project_uuid)
            )
            if not project:
                raise HTTPException(status_code=404, detail="project not found")

            new_id = project["task_id"] + 1

            await conn.execute(
                "UPDATE bench_projects SET task_id = $1 WHERE uuid = $2",
                new_id, project["uuid"]
            )

            task_uuid = uuid.uuid4()
            await conn.execute(
                """INSERT INTO bench_tasks (uuid, id, name, project_uuid, created_by, priority)
                   VALUES ($1, $2, $3, $4, $5, $6)""",
                task_uuid, new_id, req.name,
                project["uuid"], user["email"], req.priority
            )

        return {
            "uuid": str(task_uuid),
            "id": new_id,
            "name": req.name,
            "status": 0,
            "priority": req.priority,
        }

@app.get("/api/v1/tasks/{task_uuid}")
async def get_task(task_uuid: str, user: dict = Depends(current_user)):
    async with pool.acquire() as conn:
        row = await conn.fetchrow(
            "SELECT uuid, id, name, status, priority, created_at FROM bench_tasks WHERE uuid = $1",
            uuid.UUID(task_uuid)
        )
        if not row:
            raise HTTPException(status_code=404, detail="task not found")

        return {
            "uuid": str(row["uuid"]),
            "id": row["id"],
            "name": row["name"],
            "status": row["status"],
            "priority": row["priority"],
        }

if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=8000, workers=4)
---
name: python-fastapi
description: FastAPI REST API template principles. SQLAlchemy, Pydantic, Alembic.
---

# FastAPI API Template

## Tech Stack

| Component | Technology |
|-----------|------------|
| Framework | FastAPI |
| Language | Python 3.11+ |
| ORM | SQLAlchemy 2.0 |
| Validation | Pydantic v2 |
| Migrations | Alembic |
| Auth | JWT + passlib |

---

## Directory Structure

```
project-name/
├── alembic/             # Migrations
├── app/
│   ├── main.py          # FastAPI app
│   ├── config.py        # Settings
│   ├── database.py      # DB connection
│   ├── models/          # SQLAlchemy models
│   ├── schemas/         # Pydantic schemas
│   ├── routers/         # API routes
│   ├── services/        # Business logic
│   ├── dependencies/    # DI
│   └── utils/
├── tests/
├── .env.example
└── requirements.txt
```

---

## Key Concepts

| Concept | Description |
|---------|-------------|
| Async | async/await throughout |
| Dependency Injection | FastAPI Depends |
| Pydantic v2 | Validation + serialization |
| SQLAlchemy 2.0 | Async sessions |

---

## API Structure

| Layer | Responsibility |
|-------|---------------|
| Routers | HTTP handling |
| Dependencies | Auth, validation |
| Services | Business logic |
| Models | Database entities |
| Schemas | Request/response |

---

## Setup Steps

1. Create the project virtual environment: `python3 -m venv .venv`.
2. Install dependencies: `./.venv/bin/python3 -m pip install fastapi uvicorn sqlalchemy alembic pydantic`.
3. Create `.env`
4. `alembic upgrade head`
5. `uvicorn app.main:app --reload`

---

## Best Practices

- Use async everywhere
- Pydantic v2 for validation
- SQLAlchemy 2.0 async sessions
- Alembic for migrations
- pytest-asyncio for tests

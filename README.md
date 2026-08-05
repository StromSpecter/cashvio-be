# Cashvio Backend API

## Prerequisites

- Go 1.22+
- PostgreSQL 12+

## Setup

```bash
go mod download
```

## Configuration

All config via `.env`:

```bash
cp .env .env.local
# edit .env.local
```

## Run

```bash
go run cmd/api/main.go
```

App auto-creates database `cashvio` + runs migrations on startup.

API listens on `http://localhost:8080`.

## API Endpoints

All protected routes require `Authorization: Bearer <jwt-token>` header.

| Method   | Endpoint              | Auth  | Description                     |
|----------|-----------------------|-------|---------------------------------|
| POST     | /api/v1/auth/register | No    | Register user                  |
| POST     | /api/v1/auth/login    | No    | Login, returns JWT             |
| GET      | /api/v1/users         | Yes   | List users (paginated)        |
| GET      | /api/v1/users/:id     | Yes   | Get user by ID                |
| PUT      | /api/v1/users/:id     | Yes   | Update user                  |
| DELETE   | /api/v1/users/:id     | Yes   | Delete user                  |
| POST     | /api/v1/cards         | Yes   | Create card (linked to user)  |
| GET      | /api/v1/cards         | Yes   | List user cards (paginated)   |
| GET      | /api/v1/cards/:id     | Yes   | Get card by ID               |
| PUT      | /api/v1/cards/:id     | Yes   | Update card                  |
| DELETE   | /api/v1/cards/:id     | Yes   | Delete card                  |
| POST     | /api/v1/wallets       | Yes   | Create wallet (linked to user)|
| GET      | /api/v1/wallets       | Yes   | List user wallets (paginated) |
| GET      | /api/v1/wallets/:id   | Yes   | Get wallet by ID            |
| PUT      | /api/v1/wallets/:id   | Yes   | Update wallet               |
| DELETE   | /api/v1/wallets/:id   | Yes   | Delete wallet               |
| GET      | /health               | No    | Health check                 |

### Auth

1. `POST /api/v1/auth/register` — create user
2. `POST /api/v1/auth/login` — returns `{ token, expires_in }`
3. Use token in `Authorization: Bearer <token>` for all protected routes

### Pagination, Search & Sort

```
GET /api/v1/cards?limit=10&offset=0&search=bca&sort_by=balance_idr&order=asc
GET /api/v1/wallets?limit=10&offset=0&search=fund&sort_by=name&order=desc
```

| Query Param | Description                            | Default     |
|-------------|----------------------------------------|-------------|
| `limit`     | Page size (max 100)                    | 10          |
| `offset`    | Items to skip                         | 0           |
| `search`    | Case-insensitive substring search     | -           |
| `sort_by`   | Column: `created_at`, `updated_at`, `bank`, `balance_idr`, `number` (cards) / `name`, `balance_idr`, `status`, etc. (wallets) | `created_at` |
| `order`     | `asc` or `desc`                       | `desc`     |

## Project Structure

```
cmd/api/
  └── main.go          # entrypoint + migrations
internal/
  ├── config/          # env config (godotenv.Overload)
  ├── database/        # pgxpool connection + auto-create DB
  ├── model/           # domain models + DTOs
  ├── repository/      # data access layer (SQL queries)
  ├── service/         # business logic (transactions, validation)
  ├── handler/         # HTTP handlers (request/response)
  ├── middleware/      # JWT auth, CORS, recovery
  ├── route/           # gin router setup
  └── util/            # bcrypt, JWT, number masking
```

## Architecture

Layer architecture dengan dependency inversion:

```
Handler -> Service -> Repository -> Database
```

- **Handler**: HTTP request/response, input validation (Gin binding)
- **Service**: Business logic, error handling, masking
- **Repository**: SQL queries, data access

Both `Card` dan `Wallet` scope ke `user_id` dari JWT token.

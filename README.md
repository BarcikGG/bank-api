# Bank API

Учебная банковская система из двух Go-сервисов:

- `services/bank` — HTTP API, PostgreSQL и transactional outbox;
- `services/notifications` — Kafka consumer и worker уведомлений.

Инфраструктура запускается через Docker Compose и включает две базы PostgreSQL и Kafka.

## Запуск

Создайте локальный файл с секретами:

```bash
cp .env.example .env
```

Сгенерируйте отдельное случайное значение для каждой переменной в `.env`:

```bash
openssl rand -hex 32
```

После заполнения `.env` запустите проект:

```bash
docker compose up --build
```

Bank API будет доступен по адресу `http://localhost:8080`, Swagger UI — `http://localhost:8080/swagger/index.html`.

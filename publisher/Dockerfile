FROM python:3.12-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends gcc libpq-dev && rm -rf /var/lib/apt/lists/*
COPY vendor/edgar-valuation /app/vendor/edgar-valuation
COPY publisher /app/publisher
RUN pip install --no-cache-dir -r /app/vendor/edgar-valuation/requirements.txt psycopg[binary] python-dotenv
ENV PYTHONPATH=/app/vendor/edgar-valuation:/app/publisher
WORKDIR /app/publisher
CMD ["python", "-m", "publisher"]

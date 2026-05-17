# Audit Analyzer container image.
#
# Two-stage build keeps the final image small while letting the build stage
# pull large ML wheels (sentence-transformers, torch). The model is NOT
# pre-downloaded here — that adds ~1.5 GB and is a deploy-time choice that
# air-gapped customers want to make against their own model registry.
#
# Build:    docker build -f agents/audit_analyzer/Dockerfile -t clawd/audit-analyzer:0.1.0 .
# Run once: docker run --rm clawd/audit-analyzer:0.1.0 once
# Serve:    docker run -p 9595:9595 clawd/audit-analyzer:0.1.0 serve

FROM python:3.12-slim AS builder

ENV PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIP_NO_CACHE_DIR=1 \
    PYTHONDONTWRITEBYTECODE=1

RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build
COPY agents/audit_analyzer/requirements.txt /build/requirements.txt
RUN pip wheel --wheel-dir /wheels -r /build/requirements.txt

FROM python:3.12-slim

LABEL org.opencontainers.image.title="clawd-audit-analyzer" \
      org.opencontainers.image.description="Clawdlinux autonomous failure clustering for agent traces" \
      org.opencontainers.image.source="https://github.com/Clawdlinux/agentic-operator-core" \
      org.opencontainers.image.licenses="Apache-2.0"

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

RUN groupadd -r analyzer && useradd -r -g analyzer -u 10001 -s /sbin/nologin analyzer

WORKDIR /app
COPY --from=builder /wheels /wheels
COPY agents/audit_analyzer/requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir --no-index --find-links=/wheels -r /app/requirements.txt && \
    rm -rf /wheels

COPY agents/__init__.py /app/agents/__init__.py
COPY agents/observability /app/agents/observability
COPY agents/audit_analyzer /app/agents/audit_analyzer

USER 10001:10001
EXPOSE 9595
ENTRYPOINT ["python", "-m", "agents.audit_analyzer"]
CMD ["once"]

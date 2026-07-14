# Python SDK Game Demo
FROM python:3.12-slim AS build
WORKDIR /src
COPY sdks/python /src/sdks/python
RUN pip install --no-cache-dir /src/sdks/python \
    && cp -r /src/sdks/python/generated /usr/local/lib/python3.12/site-packages/generated

FROM python:3.12-slim
WORKDIR /app
COPY --from=build /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=build /usr/local/bin /usr/local/bin
COPY sdks/python/examples/game_demo.py /app/

CMD ["python", "game_demo.py"]

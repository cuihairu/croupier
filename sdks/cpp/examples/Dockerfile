FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libstdc++6 \
    && rm -rf /var/lib/apt/lists/*
COPY sdks/cpp/build/croupier-game-demo /app/croupier-game-demo
CMD ["/app/croupier-game-demo"]

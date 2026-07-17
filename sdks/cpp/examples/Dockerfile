FROM ubuntu:24.04 AS build
ARG DEBIAN_FRONTEND=noninteractive
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates cmake g++ git ninja-build pkg-config unzip zip curl tar \
    && rm -rf /var/lib/apt/lists/*
COPY sdks/cpp /src/sdks/cpp
COPY proto /src/proto
WORKDIR /src/sdks/cpp
RUN cmake --preset linux-x64-debug-vcpkg \
    && cmake --build --preset linux-x64-debug --target croupier-game-demo

FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libstdc++6 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /src/sdks/cpp/build/linux-x64-debug-vcpkg/bin/croupier-game-demo /app/croupier-game-demo
CMD ["/app/croupier-game-demo"]

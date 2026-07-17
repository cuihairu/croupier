# Java SDK Game Demo
FROM eclipse-temurin:21-jdk AS build
WORKDIR /src

COPY sdks/java /src/sdks/java

WORKDIR /src/sdks/java
RUN chmod +x gradlew || true \
    && ./gradlew --no-daemon jar

RUN mkdir -p /out/classes \
    && javac \
      -cp "build/libs/croupier-java-sdk-0.1.1-SNAPSHOT.jar:examples/game-demo/lib/*" \
      -d /out/classes \
      examples/game-demo/src/main/java/com/croupier/sdk/examples/GameDemo.java \
    && cp build/libs/croupier-java-sdk-0.1.1-SNAPSHOT.jar /out/croupier-java-sdk.jar \
    && cp -r examples/game-demo/lib /out/lib

FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=build /out/croupier-java-sdk.jar /app/lib/croupier-java-sdk.jar
COPY --from=build /out/classes /app/classes
COPY --from=build /out/lib /app/lib

# The game-demo build occasionally bundles the SDK's own classes (io/github/.../wire)
# into target/classes. Since the classpath puts /app/classes before the SDK jar,
# those shadow copies would override the jar and pin stale wire encoding. Strip
# them so the SDK jar is always the source of truth for SDK classes.
RUN rm -rf /app/classes/io

ENV CROUPIER_AGENT_ADDR=agent:19091
ENV CROUPIER_GAME_ID=demo-game
ENV CROUPIER_SERVICE_ID=game-demo-service
ENV CROUPIER_ENV=development

CMD ["java", "-cp", "/app/classes:/app/lib/*", "com.croupier.sdk.examples.GameDemo"]

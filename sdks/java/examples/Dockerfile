# Java SDK Game Demo - 只拷贝预编译产物
FROM eclipse-temurin:21-jre
WORKDIR /app
COPY sdks/java/build/libs/croupier-java-sdk-0.1.1-SNAPSHOT.jar /app/lib/croupier-java-sdk.jar
COPY sdks/java/examples/game-demo/target/classes /app/classes
COPY sdks/java/examples/game-demo/lib /app/lib

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

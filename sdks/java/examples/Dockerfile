# Java SDK Game Demo - 只拷贝预编译产物
FROM eclipse-temurin:21-jre
WORKDIR /app
COPY sdks/java/build/libs/croupier-java-sdk-0.1.1-SNAPSHOT.jar /app/lib/croupier-java-sdk.jar
COPY sdks/java/examples/game-demo/target/classes /app/classes
COPY sdks/java/examples/game-demo/lib /app/lib

ENV CROUPIER_AGENT_ADDR=agent:19091
ENV CROUPIER_GAME_ID=demo-game
ENV CROUPIER_SERVICE_ID=game-demo-service
ENV CROUPIER_ENV=development

CMD ["java", "-cp", "/app/classes:/app/lib/*", "com.croupier.sdk.examples.GameDemo"]

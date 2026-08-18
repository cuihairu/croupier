# C# SDK Game Demo - dotnet 多阶段（本地无 dotnet SDK）
FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
WORKDIR /src
COPY sdks/csharp /src
RUN dotnet publish examples/GameDemo/GameDemo.csproj -c Release -o /out

FROM mcr.microsoft.com/dotnet/runtime:10.0
WORKDIR /app
COPY --from=build /out /app

ENV CROUPIER_AGENT_ADDR=agent:19091
ENV CROUPIER_GAME_ID=default
ENV CROUPIER_SERVICE_ID=game-demo-service
ENV CROUPIER_ENV=dev

CMD ["dotnet", "/app/GameDemo.dll"]

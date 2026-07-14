# JS/TS SDK Game Demo
FROM node:24-slim AS build
RUN corepack enable
WORKDIR /src
COPY sdks/js/package.json sdks/js/pnpm-lock.yaml /src/
RUN pnpm install --frozen-lockfile
COPY sdks/js /src
RUN pnpm build

FROM node:24-slim
RUN corepack enable
WORKDIR /app
COPY --from=build /src/node_modules /app/node_modules
COPY --from=build /src/dist /app/dist
COPY --from=build /src/src /app/src
COPY --from=build /src/package.json /app/
COPY sdks/js/examples/game_demo.ts /app/examples/game_demo.ts
COPY sdks/js/tsconfig.json /app/

CMD ["./node_modules/.bin/ts-node", "--skipProject", "--transpileOnly", "--compilerOptions", "{\"module\":\"CommonJS\",\"moduleResolution\":\"node\"}", "examples/game_demo.ts"]

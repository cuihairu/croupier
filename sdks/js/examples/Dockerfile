# JS/TS SDK Game Demo
# Build locally first: cd sdks/js && pnpm install && pnpm build
# Then: npx esbuild examples/game_demo.ts --bundle --platform=node --format=cjs --outfile=dist/examples/game_demo.js --external:node:*
FROM node:24-slim
WORKDIR /app
COPY sdks/js/dist/examples/game_demo.js /app/game_demo.js
CMD ["node", "game_demo.js"]

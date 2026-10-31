FROM node:22-bookworm-slim AS build
WORKDIR /workspace
COPY package.json ./package.json
COPY apps/api/package.json ./apps/api/package.json
RUN npm install
COPY apps/api ./apps/api
RUN npm run prisma:generate --workspace @vampire-dash/api && npm run build --workspace @vampire-dash/api

FROM node:22-bookworm-slim AS runtime
ENV NODE_ENV=production
WORKDIR /workspace
COPY --from=build /workspace/node_modules ./node_modules
COPY --from=build /workspace/apps/api/package.json ./apps/api/package.json
COPY --from=build /workspace/apps/api/dist ./apps/api/dist
COPY --from=build /workspace/apps/api/prisma ./apps/api/prisma
WORKDIR /workspace/apps/api
EXPOSE 4000
CMD ["node", "dist/server.js"]

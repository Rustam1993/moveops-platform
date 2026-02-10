FROM node:20-alpine AS builder
WORKDIR /app

COPY packages/client ./packages/client
COPY apps/web ./apps/web

WORKDIR /app/packages/client
RUN npm install
RUN npm run gen
RUN npm run build

WORKDIR /app/apps/web
RUN npm install
RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production

COPY --from=builder /app/packages/client ./packages/client
COPY --from=builder /app/apps/web ./apps/web

WORKDIR /app/apps/web
RUN npm install --omit=dev

EXPOSE 3000
CMD ["npm", "run", "start"]

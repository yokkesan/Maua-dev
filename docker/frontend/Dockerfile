FROM node:24-alpine

WORKDIR /app

COPY src/frontend/package*.json ./

RUN npm install

COPY src/frontend/ ./

EXPOSE 5173

CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
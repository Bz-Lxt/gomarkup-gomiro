FROM mcr.microsoft.com/playwright:v1.49.1-noble
ENV TZ=Asia/Shanghai \
    PYTHONUNBUFFERED=1 \
    PYTHONIOENCODING=utf-8 \
    DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends python3 python3-pip \
    && pip3 install --break-system-packages pytest websocket-client \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /work
COPY package.json ./
RUN npm install
COPY . .
CMD ["sleep", "infinity"]

# ------------------------------------------------------------
# Stage 1 — Build Go Binary
# ------------------------------------------------------------
FROM golang:1.25-bookworm AS builder

ARG CACHE_BUSTER=1

RUN apt-get update && apt-get install -y \
    tesseract-ocr \
    tesseract-ocr-eng \
    libtesseract-dev \
    libleptonica-dev \
    pkg-config \
    poppler-utils \
    && rm -rf /var/lib/apt/lists/*

ENV TESSDATA_PREFIX=/usr/share/tesseract-ocr/5/tessdata/

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO enabled so Tesseract/Leptonica works
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o ocr-service main.go


# ------------------------------------------------------------
# Stage 2 — Runtime Image (Python + PaddleOCR + Tesseract libs)
# ------------------------------------------------------------
FROM python:3.10-bookworm


# --- Install OS dependencies for PaddleOCR + Tesseract ----
RUN apt-get update && apt-get install -y --no-install-recommends \
    tesseract-ocr \
    tesseract-ocr-eng \
    libtesseract-dev \
    libleptonica-dev \
    poppler-utils \
    libglib2.0-0 \
    libsm6 \
    libxext6 \
    libxrender1 \
    libgomp1 \
    libopenblas-dev \
    wget \
    && rm -rf /var/lib/apt/lists/*

ENV TESSDATA_PREFIX=/usr/share/tesseract-ocr/5/tessdata/


# ------------------------------------------------------------
# 🔥 FIXED PADDLE INSTALL (No Timeout)
# Using Tsinghua mirror + extended timeout
# ------------------------------------------------------------

RUN pip install --upgrade pip && \
    pip config set global.timeout 500 && \
    pip config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple

# Install Paddle (CPU) + PaddleOCR safely
RUN pip install --no-cache-dir paddlepaddle==2.6.2 paddleocr


# ------------------------------------------------------------
# Copy PaddleOCR Models (your folder)
# ------------------------------------------------------------
RUN mkdir -p /opt/paddleocr/models/en && \
    mkdir -p /opt/paddleocr/models/hi

COPY models/paddle/en/ /opt/paddleocr/models/en/
COPY models/paddle/hi/ /opt/paddleocr/models/hi/

ENV PADDLE_OCR_EN_MODEL_DIR="/opt/paddleocr/models/en"
ENV PADDLE_OCR_HI_MODEL_DIR="/opt/paddleocr/models/hi"


# ------------------------------------------------------------
# Copy Go Binary From Build Stage
# ------------------------------------------------------------
COPY --from=builder /app/ocr-service /usr/local/bin/ocr-service


EXPOSE 8080
CMD ["/usr/local/bin/ocr-service"]

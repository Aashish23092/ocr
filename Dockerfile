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
# Stage 2 — Runtime Image (NO PADDLE OCR HERE)
# ------------------------------------------------------------
FROM python:3.10-bookworm

# Install tesseract + PDF tools
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
# DO NOT INSTALL PADDLE HERE — REMOVED
# ------------------------------------------------------------

COPY --from=builder /app/ocr-service /usr/local/bin/ocr-service

EXPOSE 8080
CMD ["/usr/local/bin/ocr-service"]

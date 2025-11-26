FROM golang:1.25-bookworm

# Cache-buster: forced rebuild every time
ARG CACHE_BUSTER=1

RUN apt-get update && apt-get install -y \
    tesseract-ocr \
    tesseract-ocr-eng \
    libtesseract-dev \
    libleptonica-dev \
    pkg-config \
    poppler-utils \
    && rm -rf /var/lib/apt/lists/*

# Correct language data path for Tesseract 5.x
ENV TESSDATA_PREFIX=/usr/share/tesseract-ocr/5/tessdata/

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o ocr-service main.go

EXPOSE 8080

CMD ["./ocr-service"]

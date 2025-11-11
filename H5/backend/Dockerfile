FROM golang:1.24-alpine

ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /app

COPY . .

RUN if [ ! -f go.mod ]; then go mod init h5-backend; fi
RUN go get github.com/gin-gonic/gin gorm.io/gorm gorm.io/driver/mysql github.com/gorilla/websocket golang.org/x/crypto/bcrypt github.com/google/uuid
RUN go mod tidy
RUN go mod download

RUN go build -o main .

RUN mkdir -p /app/uploads && chmod 777 /app/uploads

CMD ["./main"]

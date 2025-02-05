FROM golang:1.23.6 AS build

RUN apt update -y 
RUN apt install -y ca-certificates iputils-ping net-tools curl dnsutils vim telnet tcpdump  

# COPY go.mod .
# COPY go.sum .
# RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /opt/proxy main.go


# FROM scratch
# COPY --from=build /opt/proxy /opt/proxy
# COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD ["/opt/proxy"]

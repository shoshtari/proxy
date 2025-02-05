package main

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

var (
	proxyAddr     = ":4040"
	user          = "user"
	password      = "password"
	enableSSL      = false
	certFile       = "server.crt"
	keyFile        = "server.key"
	allowedIPs     = []string{"127.0.0.1"}
)

func main() {
	// Load configuration from environment variables
	if tmp := os.Getenv("PROXY_ADDR"); tmp != "" {
		proxyAddr = tmp
	}
	if tmp := os.Getenv("PROXY_USER"); tmp != "" {
		user = tmp
	}
	if tmp := os.Getenv("PROXY_PASSWORD"); tmp != "" {
		password = tmp
	}
	if tmp := os.Getenv("ENABLE_SSL"); tmp == "true" {
		enableSSL = true
	}
	if tmp := os.Getenv("CERT_FILE"); tmp != "" {
		certFile = tmp
	}
	if tmp := os.Getenv("KEY_FILE"); tmp != "" {
		keyFile = tmp
	}
	if tmp := os.Getenv("ALLOWED_IPS"); tmp != "" {
		allowedIPs = strings.Split(tmp, ",")
	}

	var listener net.Listener
	var err error

	if enableSSL {
		log.Println("Starting proxy with SSL termination")
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Fatalf("Failed to load SSL certificate and key: %v", err)
		}

		config := &tls.Config{Certificates: []tls.Certificate{cert}}
		listener, err = tls.Listen("tcp", proxyAddr, config)
	} else {
		log.Println("Starting proxy without SSL termination")
		listener, err = net.Listen("tcp", proxyAddr)
	}

	if err != nil {
		log.Fatalf("Failed to start proxy: %v", err)
	}
	defer listener.Close()

	log.Printf("Proxy server is running on %s", proxyAddr)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept client connection: %v", err)
			continue
		}
		go handleClient(clientConn)
	}
}

func handleClient(clientConn net.Conn) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	log.Printf("New connection from: %s", clientAddr)

	// IP filtering check
	clientIP := strings.Split(clientAddr, ":")[0]
	if !isAllowedIP(clientIP) {
		log.Printf("Connection from %s denied (IP not allowed)", clientIP)
		clientConn.Write([]byte("HTTP/1.1 403 Forbidden

"))
		return
	}

	reader := bufio.NewReader(clientConn)
	request, err := http.ReadRequest(reader)
	if err != nil {
		log.Printf("Failed to read request from %s: %v", clientAddr, err)
		return
	}

	if !authenticate(request) {
		clientConn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required Proxy-Authenticate: Basic realm=\"Proxy\"\r\n\r\n"))
		log.Printf("Authentication failed for %s", clientAddr)
		return
	}

	log.Printf("Request from %s: %s %s", clientAddr, request.Method, request.URL)

	if request.Method == http.MethodConnect {
		handleConnect(clientConn, request)
	} else {
		handleHTTP(clientConn, request)
	}
}

func isAllowedIP(clientIP string) bool {
	for _, allowedIP := range allowedIPs {
		if clientIP == allowedIP {
			return true
		}
	}
	return false
}

func authenticate(request *http.Request) bool {
	auth := request.Header.Get("Proxy-Authorization")
	if auth == "" {
		return false
	}

	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}

	payload, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return false
	}

	credentials := strings.SplitN(string(payload), ":", 2)
	if len(credentials) != 2 {
		return false
	}

	return user == credentials[0] && password == credentials[1]
}

func handleConnect(clientConn net.Conn, request *http.Request) {
	targetAddr := request.URL.Host
	if !strings.Contains(targetAddr, ":") {
		targetAddr += ":443"
	}

	log.Printf("CONNECT request to: %s", targetAddr)

	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Failed to connect to target %s: %v", targetAddr, err)
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer targetConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	go io.Copy(targetConn, clientConn)
	io.Copy(clientConn, targetConn)
}

func handleHTTP(clientConn net.Conn, request *http.Request) {
	targetAddr := request.URL.Host
	if !strings.Contains(targetAddr, ":") {
		targetAddr += ":80"
	}

	log.Printf("HTTP request to: %s", targetAddr)

	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Failed to connect to target %s: %v", targetAddr, err)
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer targetConn.Close()

	err = request.Write(targetConn)
	if err != nil {
		log.Printf("Failed to forward request to %s: %v", targetAddr, err)
		return
	}

	io.Copy(clientConn, targetConn)
}


package main

import (
	"bufio"
	"encoding/base64"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

var proxyAddr = ":4040"
var user = "user"
var password = "password"

func main() {
	if tmp := os.Getenv("PROXY_USER"); tmp != "" {
		proxyAddr = tmp
	}
	if tmp := os.Getenv("PROXY_USER"); tmp != "" {
		user = tmp
	}
	if tmp := os.Getenv("PROXY_PASSWORD"); tmp != "" {
		password = tmp
	}

	listener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		log.Fatalf("Failed to start proxy: %v", err)
	}
	defer listener.Close()

	log.Printf("Proxy server is running on %s\n", proxyAddr)

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
	log.Printf("New connection from: %s\n", clientAddr)

	reader := bufio.NewReader(clientConn)
	request, err := http.ReadRequest(reader)
	if err != nil {
		log.Printf("Failed to read request from %s: %v\n", clientAddr, err)
		return
	}

	if !authenticate(request) {
		clientConn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"Proxy\"\r\n\r\n"))
		log.Printf("Authentication failed for %s\n", clientAddr)
		return
	}

	log.Printf("Request from %s: %s %s\n", clientAddr, request.Method, request.URL)

	if request.Method == http.MethodConnect {
		handleConnect(clientConn, request)
	} else {
		handleHTTP(clientConn, request)
	}
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

	log.Printf("CONNECT request to: %s\n", targetAddr)

	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Failed to connect to target %s: %v\n", targetAddr, err)
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

	log.Printf("HTTP request to: %s\n", targetAddr)

	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Failed to connect to target %s: %v\n", targetAddr, err)
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer targetConn.Close()

	err = request.Write(targetConn)
	if err != nil {
		log.Printf("Failed to forward request to %s: %v\n", targetAddr, err)
		return
	}

	io.Copy(clientConn, targetConn)
}

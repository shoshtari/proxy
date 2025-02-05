package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

func main() {
	// Get the proxy address from environment variables, default to :4040
	proxyAddr := os.Getenv("PROXY_ADDR")
	if proxyAddr == "" {
		proxyAddr = ":4040"
	}

	// Start the proxy server
	listener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		log.Fatalf("Failed to start proxy: %v", err)
	}
	defer listener.Close()

	log.Printf("Proxy server is running on %s\n", proxyAddr)

	for {
		// Accept incoming client connections
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept client connection: %v", err)
			continue
		}

		// Handle each connection in a separate goroutine
		go handleClient(clientConn)
	}
}

func handleClient(clientConn net.Conn) {
	defer clientConn.Close()

	// Log the client connection
	clientAddr := clientConn.RemoteAddr().String()
	log.Printf("New connection from: %s\n", clientAddr)

	// Read the client's request
	reader := bufio.NewReader(clientConn)
	request, err := http.ReadRequest(reader)
	if err != nil {
		log.Printf("Failed to read request from %s: %v\n", clientAddr, err)
		return
	}

	// Log the request details
	log.Printf("Request from %s: %s %s\n", clientAddr, request.Method, request.URL)

	// Check if the request is a CONNECT request (for HTTPS)
	if request.Method == http.MethodConnect {
		handleConnect(clientConn, request)
	} else {
		handleHTTP(clientConn, request)
	}
}

func handleConnect(clientConn net.Conn, request *http.Request) {
	// Extract the target address from the CONNECT request
	targetAddr := request.URL.Host
	if !strings.Contains(targetAddr, ":") {
		targetAddr += ":443" // Default to port 443 for HTTPS
	}

	// Log the CONNECT request
	log.Printf("CONNECT request to: %s\n", targetAddr)

	// Connect to the target server
	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Failed to connect to target %s: %v\n", targetAddr, err)
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer targetConn.Close()

	// Send a 200 Connection Established response to the client
	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Start tunneling data between the client and the target server
	go io.Copy(targetConn, clientConn)
	io.Copy(clientConn, targetConn)
}

func handleHTTP(clientConn net.Conn, request *http.Request) {
	// Forward the HTTP request to the target server
	targetAddr := request.URL.Host
	if !strings.Contains(targetAddr, ":") {
		targetAddr += ":80" // Default to port 80 for HTTP
	}

	// Log the HTTP request
	log.Printf("HTTP request to: %s\n", targetAddr)

	// Connect to the target server
	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Failed to connect to target %s: %v\n", targetAddr, err)
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer targetConn.Close()

	// Forward the request to the target server
	err = request.Write(targetConn)
	if err != nil {
		log.Printf("Failed to forward request to %s: %v\n", targetAddr, err)
		return
	}

	// Forward the response from the target server to the client
	io.Copy(clientConn, targetConn)
}

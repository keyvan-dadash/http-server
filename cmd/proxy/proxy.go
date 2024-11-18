package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	logger := log.Default()
	if len(os.Args) < 2 {
		fmt.Println("Usage: http_server <port> [save_path]")
		os.Exit(1)
	}

	port := os.Args[1]

	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		// TODO: fix this
		panic(err)
	}
	defer listener.Close()

	// Handle ctrl-c
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	shutdown := make(chan struct{})
	go func() {
		<-signalChan
		listener.Close()
		close(shutdown)
	}()

	logger.Println("Starting the server...")
	for {
		select {
		case <-shutdown:
			logger.Println("Shutting down the server...")
			os.Exit(0)
		default:
			client, err := listener.Accept()
			if err != nil {
				logger.Printf("failed to accept client! reason: %v\n", err)
				continue
			}
			go handleClient(client)
		}
	}
}

func handleClient(conn net.Conn) {
	defer conn.Close()

	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		log.Printf("failed to read client request: %v\n", err)
		return
	}

	if req.Method != http.MethodGet {
		handleResponse(
			conn,
			http.StatusText(http.StatusNotImplemented)+"\n",
			http.StatusNotImplemented,
			req,
		)
		return
	}

	resp, err := proxyRequest(req)
	if err != nil {
		handleResponse(
			conn,
			http.StatusText(http.StatusBadGateway)+"\n",
			http.StatusBadGateway,
			req,
		)
		log.Printf("failed to forward request: %v", err)
		return
	}
	defer resp.Body.Close()

	if err := resp.Write(conn); err != nil {
		log.Printf("failed to write response to client: %v", err)
	}
}

func handleResponse(conn net.Conn, body string, statusCode int, originalReq *http.Request) {
	response := http.Response{
		StatusCode: statusCode,
		Proto:      originalReq.Proto,
		ProtoMajor: originalReq.ProtoMajor,
		ProtoMinor: originalReq.ProtoMinor,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	response.Header.Set("Content-Type", "text/plain")
	response.Write(conn)
}

func proxyRequest(request *http.Request) (*http.Response, error) {
	client := &http.Client{}

	url := fmt.Sprintf("http://%s%s", request.Host, request.URL.Path)
	newReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	for header, values := range request.Header {
		for _, value := range values {
			newReq.Header.Add(header, value)
		}
	}

	return client.Do(newReq)
}

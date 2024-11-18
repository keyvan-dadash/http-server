package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

const (
	kMaxNumOfWrokers = 10
)

var (
	savePath = ""
)

type Worker struct {
	internalCh <-chan net.Conn
}

func (w *Worker) RunForever() {
	for conn := range w.internalCh {
		clientHandler(conn)
		conn.Close()
	}
}

func newWorker(internalCh <-chan net.Conn) *Worker {
	return &Worker{
		internalCh: internalCh,
	}
}

func main() {
	logger := log.Default()
	if len(os.Args) < 2 {
		fmt.Println("Usage: http_server <port> [save_path]")
		os.Exit(1)
	}

	port := os.Args[1]
	savePath = "./"
	if len(os.Args) > 2 {
		savePath = os.Args[2]
		if !strings.HasSuffix(savePath, "/") {
			savePath += "/"
		}

		if _, err := os.Stat(savePath); os.IsNotExist(err) {
			fmt.Printf("Error: save path '%s' does not exist\n", savePath)
			os.Exit(1)
		}
	}

	logger.Printf("Save files to %v\n", savePath)

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

	taskCh := make(chan net.Conn)
	wg := sync.WaitGroup{}
	for i := 0; i < kMaxNumOfWrokers; i++ {
		wg.Add(1)
		worker := newWorker(taskCh)
		go func() {
			defer wg.Done()
			worker.RunForever()
		}()
	}

	logger.Println("Starting the server...")
	for {
		select {
		case <-shutdown:
			logger.Println("Shutting down the server...")
			close(taskCh)
			wg.Wait()
			os.Exit(0)
		default:
			client, err := listener.Accept()
			if err != nil {
				logger.Printf("failed to accept client! reason: %v\n", err)
				continue
			}
			taskCh <- client
		}
	}
}

func clientHandler(conn net.Conn) {
	// Parse http request
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil && !errors.Is(err, io.EOF) {
		handleResponse(
			conn,
			http.StatusText(http.StatusBadRequest)+"\n",
			http.StatusBadRequest,
			req,
		)
		return
	}

	// Handle Methods
	switch req.Method {
	case "GET":
		{
			handleGET(conn, req)
		}
	case "POST":
		{
			handlePOST(conn, req)
		}
	default:
		{
			handleResponse(
				conn,
				http.StatusText(http.StatusNotImplemented)+"\n",
				http.StatusNotImplemented,
				req,
			)
		}
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

func handleGET(conn net.Conn, request *http.Request) {
	filePath := savePath + strings.TrimPrefix(request.URL.Path, "/")
	if !isValidExt(filePath) {
		handleResponse(
			conn,
			http.StatusText(http.StatusBadRequest)+"\n",
			http.StatusBadRequest,
			request,
		)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		handleResponse(
			conn,
			http.StatusText(http.StatusNotFound)+"\n",
			http.StatusNotFound,
			request,
		)
		return
	}
	defer file.Close()

	contentType := getContentType(filePath)
	response := http.Response{
		StatusCode: http.StatusOK,
		Proto:      request.Proto,
		ProtoMajor: request.ProtoMajor,
		ProtoMinor: request.ProtoMinor,
		Header:     make(http.Header),
		Body:       file,
	}
	response.Header.Set("Content-Type", contentType)
	response.Write(conn)
}

func handlePOST(conn net.Conn, request *http.Request) {
	err := request.ParseMultipartForm(32 << 20) // 32 MB limit
	if err != nil {
		handleResponse(
			conn,
			http.StatusText(http.StatusBadRequest)+"\n",
			http.StatusBadRequest,
			request,
		)
		return
	}

	file, header, err := request.FormFile("file")
	if err != nil {
		handleResponse(
			conn,
			http.StatusText(http.StatusBadRequest)+"\n",
			http.StatusBadRequest,
			request,
		)
		return
	}
	defer file.Close()

	filePath := filepath.Join(savePath, header.Filename)
	if !isValidExt(filePath) {
		handleResponse(
			conn,
			http.StatusText(http.StatusBadRequest)+"\n",
			http.StatusBadRequest,
			request,
		)
		return
	}

	if _, err := os.Stat(filePath); err == nil {
		handleResponse(
			conn,
			http.StatusText(http.StatusConflict)+"\n",
			http.StatusConflict,
			request,
		)
		return
	}

	outFile, err := os.Create(filePath)
	if err != nil {
		handleResponse(
			conn,
			http.StatusText(http.StatusInternalServerError)+"\n",
			http.StatusInternalServerError,
			request,
		)
		return
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, file)
	if err != nil {
		handleResponse(
			conn,
			http.StatusText(http.StatusInternalServerError)+"\n",
			http.StatusInternalServerError,
			request,
		)
		return
	}

	response := http.Response{
		StatusCode: http.StatusOK,
		Proto:      request.Proto,
		ProtoMajor: request.ProtoMajor,
		ProtoMinor: request.ProtoMinor,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("File uploaded successfully\n")),
	}
	response.Header.Set("Content-Type", "text/plain")
	response.Write(conn)
}

func isValidExt(filePath string) bool {
	switch filepath.Ext(filePath) {
	case ".gif", ".jpg", ".jpeg", ".txt", ".html", ".css":
		{
			return true
		}
	default:
		{
			return false
		}
	}
}

func getContentType(filePath string) string {
	switch filepath.Ext(filePath) {
	case ".html":
		return "text/html"
	case ".gif":
		return "image/gif"
	case ".txt":
		return "text/plain"
	case ".jpeg":
		return "image/jpeg"
	case ".jpg":
		return "image/jpeg"
	case ".css":
		return "text/css"
	default:
		return "application/octet-stream"
	}
}

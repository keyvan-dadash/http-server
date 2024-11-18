package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type mockConn struct {
	io.Reader
	io.Writer
}

func (m *mockConn) Close() error {
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return nil
}

func (m *mockConn) RemoteAddr() net.Addr {
	return nil
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestHandleGET_NetConn(t *testing.T) {
	testFileName := "test.txt"
	testFilePath := filepath.Join(savePath, testFileName)
	content := []byte("Hello, world!")
	if err := os.WriteFile(testFilePath, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Remove(testFilePath)

	reqStr := fmt.Sprintf("GET /%s HTTP/1.1\r\nHost: localhost\r\n\r\n", testFileName)
	conn := &mockConn{
		Reader: strings.NewReader(reqStr),
		Writer: &bytes.Buffer{},
	}

	clientHandler(conn)

	writer := conn.Writer.(*bytes.Buffer)
	resp, err := http.ReadResponse(bufio.NewReader(writer), nil)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != string(content) {
		t.Errorf("expected body %q, got %q", content, body)
	}
}

func TestHandlePOST_NetConn(t *testing.T) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	fileWriter, err := writer.CreateFormFile("file", "upload.txt")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	fileContent := "This is a test file."
	fileWriter.Write([]byte(fileContent))
	writer.Close()

	reqStr := fmt.Sprintf(
		"POST /upload.txt HTTP/1.1\r\nHost: localhost\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n%s",
		writer.FormDataContentType(),
		buffer.Len(),
		buffer.String(),
	)

	conn := &mockConn{
		Reader: strings.NewReader(reqStr),
		Writer: &bytes.Buffer{},
	}

	clientHandler(conn)

	writerResp := conn.Writer.(*bytes.Buffer)
	resp, err := http.ReadResponse(bufio.NewReader(writerResp), nil)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "File uploaded successfully") {
		t.Errorf("unexpected response body: %q", body)
	}

	savedFilePath := filepath.Join(savePath, "upload.txt")
	if _, err := os.Stat(savedFilePath); os.IsNotExist(err) {
		t.Errorf("file was not saved to %q", savedFilePath)
	}
	defer os.Remove(savedFilePath)
}

func TestHandleGET_NotFound_NetConn(t *testing.T) {
	reqStr := "GET /nonexistent.txt HTTP/1.1\r\nHost: localhost\r\n\r\n"
	conn := &mockConn{
		Reader: strings.NewReader(reqStr),
		Writer: &bytes.Buffer{},
	}

	clientHandler(conn)

	writer := conn.Writer.(*bytes.Buffer)
	resp, err := http.ReadResponse(bufio.NewReader(writer), nil)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status Not Found, got %v", resp.Status)
	}
}

func TestHandlePOST_FileExists_NetConn(t *testing.T) {
	existingFileName := "existing.txt"
	existingFilePath := filepath.Join(savePath, existingFileName)
	if err := os.WriteFile(existingFilePath, []byte("Existing content"), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}
	defer os.Remove(existingFilePath)

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	fileWriter, err := writer.CreateFormFile("file", existingFileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	fileWriter.Write([]byte("New content"))
	writer.Close()

	reqStr := fmt.Sprintf(
		"POST /%s HTTP/1.1\r\nHost: localhost\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n%s",
		existingFileName,
		writer.FormDataContentType(),
		buffer.Len(),
		buffer.String(),
	)

	conn := &mockConn{
		Reader: strings.NewReader(reqStr),
		Writer: &bytes.Buffer{},
	}

	clientHandler(conn)

	writerResp := conn.Writer.(*bytes.Buffer)
	resp, err := http.ReadResponse(bufio.NewReader(writerResp), nil)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected status Conflict, got %v", resp.Status)
	}
}

func TestHandlePOST_InvalidExtension_NetConn(t *testing.T) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	fileWriter, err := writer.CreateFormFile("file", "invalid.exe")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	fileContent := "This is a test file with an invalid extension."
	fileWriter.Write([]byte(fileContent))
	writer.Close()

	reqStr := fmt.Sprintf(
		"POST /invalid.exe HTTP/1.1\r\nHost: localhost\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n%s",
		writer.FormDataContentType(),
		buffer.Len(),
		buffer.String(),
	)

	conn := &mockConn{
		Reader: strings.NewReader(reqStr),
		Writer: &bytes.Buffer{},
	}

	clientHandler(conn)

	writerResp := conn.Writer.(*bytes.Buffer)
	resp, err := http.ReadResponse(bufio.NewReader(writerResp), nil)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status Bad Request, got %v", resp.Status)
	}
}

func TestHandleGET_UnsupportedExtension_NetConn(t *testing.T) {
	reqStr := "GET /unsupported.xyz HTTP/1.1\r\nHost: localhost\r\n\r\n"
	conn := &mockConn{
		Reader: strings.NewReader(reqStr),
		Writer: &bytes.Buffer{},
	}

	clientHandler(conn)

	writer := conn.Writer.(*bytes.Buffer)
	resp, err := http.ReadResponse(bufio.NewReader(writer), nil)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status Bad Request, got %v", resp.Status)
	}
}

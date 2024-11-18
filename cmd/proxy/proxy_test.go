package main

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
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

func TestHandleClientRequest_GET_StatusOK(t *testing.T) {
	reqStr := "GET / HTTP/1.1\r\nHost: google.com\r\n\r\n"
	conn := &mockConn{
		Reader: strings.NewReader(reqStr),
		Writer: &bytes.Buffer{},
	}

	handleClient(conn)

	writer := conn.Writer.(*bytes.Buffer)
	resp, err := http.ReadResponse(bufio.NewReader(writer), nil)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}
}

func TestHandleClientRequest_GET_StatusBadGateway(t *testing.T) {
	reqStr := "GET / HTTP/1.1\r\nHost: not-website-not-website.com\r\n\r\n"
	conn := &mockConn{
		Reader: strings.NewReader(reqStr),
		Writer: &bytes.Buffer{},
	}

	handleClient(conn)

	writer := conn.Writer.(*bytes.Buffer)
	resp, err := http.ReadResponse(bufio.NewReader(writer), nil)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected status Bad Gateway, got %v", resp.Status)
	}
}

func TestHandleClientRequest_NotImplemented(t *testing.T) {
	reqStr := "POST / HTTP/1.1\r\nHost: google.com\r\n\r\n"
	conn := &mockConn{
		Reader: strings.NewReader(reqStr),
		Writer: &bytes.Buffer{},
	}

	handleClient(conn)

	writer := conn.Writer.(*bytes.Buffer)
	resp, err := http.ReadResponse(bufio.NewReader(writer), nil)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("expected status Not Implemented, got %v", resp.Status)
	}
}

func TestForwardRequest_InvalidURL(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://invalid_site", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = proxyRequest(req)
	if err == nil {
		t.Errorf("expected error when forwarding request to invalid URL, got none")
	}
}

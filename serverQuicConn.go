package quic_proxy_core

import (
	"bufio"
	"context"
	"crypto/tls"
	"github.com/quic-go/quic-go"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ServerQuicConnectionManager struct {
	QuicConn quic.Connection
	addr     string
	tlsConf  *tls.Config
	mu       sync.Mutex
	lastSent time.Time
	lastRecv time.Time
}

func NewServerQuicConnectionManager(addr string, tlsConf *tls.Config) *ServerQuicConnectionManager {
	return &ServerQuicConnectionManager{
		addr:    addr,
		tlsConf: tlsConf,
	}
}

func (sqcm *ServerQuicConnectionManager) HandleQUICConnection() {
	defer func() {
		if sqcm.QuicConn != nil {
			sqcm.QuicConn.CloseWithError(400, "")
		}
	}()

	for {
		ctx := context.Background()
		stream, err := sqcm.QuicConn.AcceptStream(ctx)
		if err != nil {
			log.Println(err)
			break
		}
		go sqcm.HandleStream(stream)
	}
}

func (sqcm *ServerQuicConnectionManager) HandleStream(stream quic.Stream) {
	defer func() {
		if stream != nil {
			stream.Close()
		}
	}()

	reader := bufio.NewReader(stream)
	data, err := reader.Peek(9)
	if err != nil {
		panic(err)
	}

	if string(data) == "HEARTBEAT" {
		sqcm.mu.Lock()
		sqcm.lastRecv = time.Now()
		sqcm.mu.Unlock()

		// 发送心跳包响应
		_, err = stream.Write([]byte("HEARTBEAT_RESPONSE\n"))
		if err != nil {
			log.Println("Error sending heartbeat response:", err)
		}
		return
	}

	requestLine, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}

	parts := strings.Split(strings.TrimSpace(requestLine), " ")
	if len(parts) != 3 {
		panic("malformed request line: " + requestLine)
	}

	method, requestURI, version := parts[0], parts[1], parts[2]

	handler := NewRequestHandler()

	if method == http.MethodConnect {
		handler.HandleHTTPS(stream, requestURI)
	} else {
		// 创建一个新的 io.Reader，它将包含原始请求行和剩余的数据
		readerWithRequestLine := io.MultiReader(strings.NewReader(requestLine), reader)
		req, err := http.ReadRequest(bufio.NewReader(readerWithRequestLine))
		if err != nil {
			panic(err)
		}
		req.Method, req.RequestURI, req.Proto = method, requestURI, version
		handler.HandleHTTP(stream, req)
	}
}

package quic_proxy_core

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/quic-go/quic-go"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	heartbeatInterval = 5 * time.Second
	heartbeatTimeout  = 10 * time.Second
)

type QuicConnectionManager struct {
	QuicConn quic.Connection
	addr     string
	tlsConf  *tls.Config
	mu       sync.Mutex
	lastSent time.Time
	lastRecv time.Time
}

func NewQuicConnectionManager(addr string, tlsConf *tls.Config) *QuicConnectionManager {
	return &QuicConnectionManager{
		addr:    addr,
		tlsConf: tlsConf,
	}
}

func (qcm *QuicConnectionManager) Connect(ctx context.Context) error {
	qcm.mu.Lock()
	defer qcm.mu.Unlock()

	if qcm.QuicConn != nil {
		return nil
	}

	conn, err := quic.DialAddr(ctx, qcm.addr, qcm.tlsConf, nil)
	go qcm.MonitorConnection()
	if err != nil {
		return err
	}

	qcm.QuicConn = conn
	qcm.lastRecv = time.Now()

	return nil
}

func (qcm *QuicConnectionManager) Reconnect(ctx context.Context) error {
	qcm.mu.Lock()
	defer qcm.mu.Unlock()

	if qcm.QuicConn != nil {
		qcm.QuicConn.CloseWithError(0, "reconnecting")
	}

	conn, err := quic.DialAddr(ctx, qcm.addr, qcm.tlsConf, nil)
	if err != nil {
		return err
	}

	qcm.QuicConn = conn
	qcm.lastRecv = time.Now()
	return nil
}

func (qcm *QuicConnectionManager) OpenStreamSync(ctx context.Context) (quic.Stream, error) {
	qcm.mu.Lock()
	defer qcm.mu.Unlock()

	if qcm.QuicConn == nil {
		return nil, errors.New("connection not established")
	}

	return qcm.QuicConn.OpenStreamSync(ctx)
}

func (qcm *QuicConnectionManager) SendHeartbeat(ctx context.Context) error {
	stream, err := qcm.OpenStreamSync(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	_, err = stream.Write([]byte("HEARTBEAT\n"))

	qcm.mu.Lock()
	qcm.lastSent = time.Now()
	qcm.mu.Unlock()

	return err
}
func (qcm *QuicConnectionManager) CheckTimeout() bool {
	qcm.mu.Lock()
	defer qcm.mu.Unlock()

	return time.Since(qcm.lastRecv) > heartbeatTimeout
}

func (qcm *QuicConnectionManager) MonitorConnection() {
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-heartbeatTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), heartbeatTimeout)
			err := qcm.SendHeartbeat(ctx)
			cancel()
			if err != nil {
				log.Println("Heartbeat failed:", err)
				if qcm.CheckTimeout() {
					log.Println("Reconnecting due to timeout")
					qcm.Reconnect(context.Background())
				}
			}
		}
	}
}

func (qcm *QuicConnectionManager) HandleQUICConnection() {
	defer func() {
		if qcm.QuicConn != nil {
			qcm.QuicConn.CloseWithError(400, "")
		}
	}()

	for {
		ctx := context.Background()
		stream, err := qcm.QuicConn.AcceptStream(ctx)
		if err != nil {
			log.Println(err)
			break
		}
		go qcm.HandleStream(stream)
	}
}

func (qcm *QuicConnectionManager) HandleStream(stream quic.Stream) {
	defer func() {
		if stream != nil {
			stream.Close()
		}
	}()

	reader := bufio.NewReader(stream)
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}

	if requestLine == "HEARTBEAT\n" {
		qcm.mu.Lock()
		qcm.lastRecv = time.Now()
		qcm.mu.Unlock()

		// 发送心跳包响应
		_, err = stream.Write([]byte("HEARTBEAT_RESPONSE\n"))
		if err != nil {
			log.Println("Error sending heartbeat response:", err)
		}
		return
	}

	parts := strings.Split(strings.TrimSpace(requestLine), " ")
	if len(parts) != 3 {
		panic("malformed request line: " + requestLine)
	}

	method, requestURI, version := parts[0], parts[1], parts[2]

	if method == http.MethodConnect {
		handleHTTPS(stream, requestURI)
	} else {
		// 创建一个新的 io.Reader，它将包含原始请求行和剩余的数据
		readerWithRequestLine := io.MultiReader(strings.NewReader(requestLine), reader)
		req, err := http.ReadRequest(bufio.NewReader(readerWithRequestLine))
		if err != nil {
			panic(err)
		}
		req.Method, req.RequestURI, req.Proto = method, requestURI, version
		handleHTTP(stream, req)
	}
}

func handleHTTP(stream quic.Stream, req *http.Request) {
	fmt.Println("远程代理收到请求", "http://"+req.Host+req.URL.String())

	// 构造目标 URL
	target, err := url.Parse("http://" + req.Host + req.URL.String())
	if err != nil {
		panic(err)
	}

	// 代理请求到目标服务器
	proxy := httputil.NewSingleHostReverseProxy(target)
	responseRecorder := httptest.NewRecorder()
	proxy.ServeHTTP(responseRecorder, req)

	// 将响应发送回第一级代理
	responseRecorder.Result().Write(stream)
}

func handleHTTPS(stream quic.Stream, hostAndPort string) {
	fmt.Println("远程代理收到 HTTPS 请求", hostAndPort)

	target := hostAndPort
	proxyConn, err := net.Dial("tcp", target)
	if err != nil {
		fmt.Println("Error dialing target server:", err)
		return
	}
	defer proxyConn.Close()

	go io.Copy(stream, proxyConn)
	io.Copy(proxyConn, stream)
}

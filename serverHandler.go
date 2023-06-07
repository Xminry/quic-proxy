package quic_proxy_core

import (
	"fmt"
	"github.com/quic-go/quic-go"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
)

// Separate RequestHandler struct
type RequestHandler struct{}

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{}
}

func (rh *RequestHandler) HandleHTTP(stream quic.Stream, req *http.Request) {
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

func (rh *RequestHandler) HandleHTTPS(stream quic.Stream, hostAndPort string) {
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

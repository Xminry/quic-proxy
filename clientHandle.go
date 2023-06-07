package quic_proxy_core

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
)

type ProxyHandler struct {
	quicConnMgr *ClientQuicConnectionManager
}

func NewProxyHandler(quicConnMgr *ClientQuicConnectionManager) *ProxyHandler {
	return &ProxyHandler{quicConnMgr: quicConnMgr}
}

func (ph *ProxyHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("本地代理收到 HTTP 请求", r.URL)

	ctx := context.Background()
	proxyStream, err := ph.quicConnMgr.QuicConn.OpenStreamSync(ctx)

	if err != nil {
		log.Println("连接超时断开，重建中")
		var err error
		ph.quicConnMgr.Reconnect(context.Background())
		if err != nil {
			log.Println(err)
		}
		proxyStream, err = ph.quicConnMgr.QuicConn.OpenStreamSync(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}

	defer proxyStream.Close()

	err = r.Write(proxyStream)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	resp, err := http.ReadResponse(bufio.NewReader(proxyStream), r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (ph *ProxyHandler) HandleHTTPS(w http.ResponseWriter, r *http.Request) {
	fmt.Println("本地代理收到 HTTPS 请求", r.Host)

	ctx := context.Background()
	proxyStream, err := ph.quicConnMgr.QuicConn.OpenStreamSync(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer proxyStream.Close()

	// Send the CONNECT request line to the second level proxy
	_, err = proxyStream.Write([]byte(fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", r.Host, r.Host)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	clientConn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	if err != nil {
		return
	}

	go io.Copy(clientConn, proxyStream)
	io.Copy(proxyStream, clientConn)
}

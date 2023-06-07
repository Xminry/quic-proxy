package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"quic-proxy-core"
)

const remoteAddr = "10.114.15.202:7777"

var quicConnMgr *quic_proxy_core.QuicConnectionManager

func main() {
	ctx := context.Background()

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}
	quicConnMgr = quic_proxy_core.NewQuicConnectionManager(remoteAddr, tlsConf)
	err := quicConnMgr.Connect(ctx)
	if err != nil {
		log.Println(err)
	}

	server := http.Server{
		Addr: ":3333",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleHTTPS(w, r)
			} else {
				handleHTTP(w, r)
			}
		}),
	}

	fmt.Println("Starting first level HTTP proxy with HTTPS support at :3333")
	server.ListenAndServe()
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("本地代理收到 HTTP 请求", r.URL)

	ctx := context.Background()
	proxyStream, err := quicConnMgr.QuicConn.OpenStreamSync(ctx)

	if err != nil {
		log.Println("连接超时断开，重建中")
		var err error
		quicConnMgr.Reconnect(context.Background())
		if err != nil {
			log.Println(err)
		}
		proxyStream, err = quicConnMgr.QuicConn.OpenStreamSync(ctx)
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

func handleHTTPS(w http.ResponseWriter, r *http.Request) {
	fmt.Println("本地代理收到 HTTPS 请求", r.Host)

	ctx := context.Background()
	proxyStream, err := quicConnMgr.QuicConn.OpenStreamSync(ctx)
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

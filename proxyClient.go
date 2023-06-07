package quic_proxy_core

import (
	"fmt"
	"net/http"
)

type ProxyClient struct {
	quicConnMgr *ClientQuicConnectionManager
	server      http.Server
}

func NewProxyClient(addr string, quicConnMgr *ClientQuicConnectionManager) *ProxyClient {
	proxyClient := &ProxyClient{
		quicConnMgr: quicConnMgr,
		server: http.Server{
			Addr: addr,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				proxyHandler := NewProxyHandler(quicConnMgr)
				if r.Method == http.MethodConnect {
					proxyHandler.HandleHTTPS(w, r)
				} else {
					proxyHandler.HandleHTTP(w, r)
				}
			}),
		},
	}

	return proxyClient
}

func (p *ProxyClient) Run() {
	fmt.Printf("Starting first level HTTP proxy with HTTPS support at %s\n", p.server.Addr)
	p.server.ListenAndServe()
}

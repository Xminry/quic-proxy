package quic_proxy_core

import (
	"context"
	"fmt"
	"github.com/quic-go/quic-go"
	"log"
)

type ProxyServer struct {
	quicConnMgr *ServerQuicConnectionManager
}

func NewProxyServer(quicConnMgr *ServerQuicConnectionManager) *ProxyServer {
	return &ProxyServer{quicConnMgr: quicConnMgr}
}

func (p *ProxyServer) Serve(addr string) {
	listener, err := quic.ListenAddr(addr, p.quicConnMgr.tlsConf, nil)
	if err != nil {
		log.Println(err)
	}

	fmt.Println("Starting second level HTTP proxy with HTTPS support at :7777")

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Println(err)
		}
		p.quicConnMgr.QuicConn = conn
		go p.quicConnMgr.HandleQUICConnection()
	}
}

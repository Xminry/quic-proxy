package main

import (
	"context"
	"crypto/tls"
	"log"
	qpc "quic-proxy-core"
)

const remoteAddr = "127.0.0.1:7777"

func main() {
	ctx := context.Background()

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}
	quicConnMgr := qpc.NewClientQuicConnectionManager(remoteAddr, tlsConf)
	err := quicConnMgr.Connect(ctx)
	if err != nil {
		log.Println(err)
	}

	proxyClient := qpc.NewProxyClient(":3333", quicConnMgr)
	proxyClient.Run()
}

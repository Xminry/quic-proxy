package quic_proxy_core

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
	"log"
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
	// Enable the proxy with the given address
	err := enableProxy("127.0.0.1:3333")
	if err != nil {
		log.Fatalf("Failed to enable proxy: %s", err)
	}

	fmt.Printf("Starting first level HTTP proxy with HTTPS support at %s\n", p.server.Addr)
	p.server.ListenAndServe()
}

func (p *ProxyClient) Stop() {
	p.quicConnMgr.Stop()

	// Disable the proxy
	err := disableProxy()
	if err != nil {
		log.Fatalf("Failed to disable proxy: %s", err)
	}
}

func enableProxy(proxyAddress string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer k.Close()

	err = k.SetStringValue("ProxyServer", proxyAddress)
	if err != nil {
		return err
	}
	err = k.SetDWordValue("ProxyEnable", 1)
	if err != nil {
		return err
	}

	err = k.SetStringValue("ProxyOverride", "")
	if err != nil {
		return err
	}

	return nil
}

func disableProxy() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer k.Close()

	err = k.SetDWordValue("ProxyEnable", 0)
	if err != nil {
		return err
	}

	return nil
}

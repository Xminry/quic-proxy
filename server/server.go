package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/quic-go/quic-go"
	"log"
	"math/big"
	"quic-proxy-core"
)

const addr = "localhost:7777"

//func handleQUICConnection(conn quic.Connection) {
//	defer conn.CloseWithError(400, "")
//
//	for {
//		stream, err := conn.AcceptStream(context.Background())
//		if err != nil {
//			log.Println(err)
//			break
//		}
//		go handleStream(stream)
//	}
//
//}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-echo-example"},
	}
}

func main() {
	listener, err := quic.ListenAddr(addr, generateTLSConfig(), nil)
	if err != nil {
		log.Println(err)
	}

	fmt.Println("Starting second level HTTP proxy with HTTPS support at :7777")

	quicConnMgr := quic_proxy_core.NewQuicConnectionManager(addr, generateTLSConfig())

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Println(err)
		}
		quicConnMgr.QuicConn = conn
		go quicConnMgr.HandleQUICConnection()
	}
}

//func handleStream(stream quic.Stream) {
//	defer func() {
//		if stream != nil {
//			stream.Close()
//		}
//	}()
//
//	reader := bufio.NewReader(stream)
//	requestLine, err := reader.ReadString('\n')
//	if err != nil {
//		panic(err)
//	}
//
//	parts := strings.Split(strings.TrimSpace(requestLine), " ")
//	if len(parts) != 3 {
//		panic("malformed request line: " + requestLine)
//	}
//
//	method, requestURI, version := parts[0], parts[1], parts[2]
//
//	if method == http.MethodConnect {
//		handleHTTPS(stream, requestURI)
//	} else {
//		// 创建一个新的 io.Reader，它将包含原始请求行和剩余的数据
//		readerWithRequestLine := io.MultiReader(strings.NewReader(requestLine), reader)
//		req, err := http.ReadRequest(bufio.NewReader(readerWithRequestLine))
//		if err != nil {
//			panic(err)
//		}
//		req.Method, req.RequestURI, req.Proto = method, requestURI, version
//		handleHTTP(stream, req)
//	}
//}

//func handleHTTP(stream quic.Stream, req *http.Request) {
//	fmt.Println("远程代理收到请求", "http://"+req.Host+req.URL.String())
//
//	// 构造目标 URL
//	target, err := url.Parse("http://" + req.Host + req.URL.String())
//	if err != nil {
//		panic(err)
//	}
//
//	// 代理请求到目标服务器
//	proxy := httputil.NewSingleHostReverseProxy(target)
//	responseRecorder := httptest.NewRecorder()
//	proxy.ServeHTTP(responseRecorder, req)
//
//	// 将响应发送回第一级代理
//	responseRecorder.Result().Write(stream)
//}
//
//func handleHTTPS(stream quic.Stream, hostAndPort string) {
//	fmt.Println("远程代理收到 HTTPS 请求", hostAndPort)
//
//	target := hostAndPort
//	proxyConn, err := net.Dial("tcp", target)
//	if err != nil {
//		fmt.Println("Error dialing target server:", err)
//		return
//	}
//	defer proxyConn.Close()
//
//	go io.Copy(stream, proxyConn)
//	io.Copy(proxyConn, stream)
//}

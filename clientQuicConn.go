package quic_proxy_core

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/quic-go/quic-go"
	"log"
	"sync"
	"time"
)

const (
	heartbeatInterval = 1 * time.Second
	heartbeatTimeout  = 2 * time.Second
)

type ClientQuicConnectionManager struct {
	QuicConn        Connection
	HeartbeatStream quic.Stream
	addr            string
	tlsConf         *tls.Config
	mu              sync.Mutex
	lastSent        time.Time
	lastRecv        time.Time
	stopChan        chan struct{}
	stopped         bool
}

func NewClientQuicConnectionManager(addr string, tlsConf *tls.Config) *ClientQuicConnectionManager {
	return &ClientQuicConnectionManager{
		addr:     addr,
		tlsConf:  tlsConf,
		stopChan: make(chan struct{}),
		stopped:  false,
	}
}

func (cqcm *ClientQuicConnectionManager) Connect(ctx context.Context) error {
	cqcm.mu.Lock()
	defer cqcm.mu.Unlock()

	if cqcm.QuicConn != nil {
		return nil
	}

	conn, err := quic.DialAddr(ctx, cqcm.addr, cqcm.tlsConf, nil)
	go cqcm.MonitorConnection()
	if err != nil {
		return err
	}

	cqcm.QuicConn = conn
	cqcm.lastRecv = time.Now()

	// 初始化心跳包 stream
	stream, err := cqcm.QuicConn.OpenStreamSync(ctx)
	if err != nil {
		return err
	}
	cqcm.HeartbeatStream = stream

	return nil
}

func (cqcm *ClientQuicConnectionManager) Reconnect(ctx context.Context) error {
	cqcm.mu.Lock()
	defer cqcm.mu.Unlock()

	if cqcm.QuicConn != nil {
		cqcm.QuicConn.CloseWithError(0, "reconnecting")
	}

	conn, err := quic.DialAddr(ctx, cqcm.addr, cqcm.tlsConf, nil)
	if err != nil {
		return err
	}

	cqcm.QuicConn = conn
	cqcm.lastRecv = time.Now()

	// 初始化心跳包 stream
	stream, err := cqcm.QuicConn.OpenStreamSync(ctx)
	if err != nil {
		return err
	}
	cqcm.HeartbeatStream = stream
	return nil
}

func (cqcm *ClientQuicConnectionManager) OpenStreamSync(ctx context.Context) (quic.Stream, error) {
	cqcm.mu.Lock()
	defer cqcm.mu.Unlock()

	if cqcm.QuicConn == nil {
		return nil, errors.New("connection not established")
	}

	return cqcm.QuicConn.OpenStreamSync(ctx)
}

func (cqcm *ClientQuicConnectionManager) SendHeartbeat(ctx context.Context) error {
	if cqcm.HeartbeatStream == nil {
		return errors.New("heartbeat stream not initialized")
	}

	_, err := cqcm.HeartbeatStream.Write([]byte("HEARTBEAT\n"))

	cqcm.mu.Lock()
	cqcm.lastSent = time.Now()
	cqcm.mu.Unlock()

	return err
}

func (cqcm *ClientQuicConnectionManager) CheckTimeout() bool {
	cqcm.mu.Lock()
	defer cqcm.mu.Unlock()

	return time.Since(cqcm.lastRecv) > heartbeatTimeout
}

func (cqcm *ClientQuicConnectionManager) MonitorConnection() {
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-cqcm.stopChan:
			return
		case <-heartbeatTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), heartbeatTimeout)
			err := cqcm.SendHeartbeat(ctx)
			cancel()
			if err != nil {
				log.Println("Heartbeat failed:", err)
				if cqcm.CheckTimeout() {
					log.Println("Reconnecting due to timeout")
					cqcm.Reconnect(context.Background())
				}
			}
		}
	}
}

func (cqcm *ClientQuicConnectionManager) Stop() {
	cqcm.mu.Lock()
	if cqcm.stopped {
		cqcm.mu.Unlock()
		return
	}

	cqcm.stopped = true
	cqcm.mu.Unlock()

	close(cqcm.stopChan)
	if cqcm.QuicConn != nil {
		cqcm.QuicConn.CloseWithError(0, "connection manager stopped")
	}
}

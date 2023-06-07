package main

import (
	"context"
	"crypto/tls"
	"log"
	qpc "quic-proxy-core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	proxyClient       *qpc.ProxyClient
	remoteAddrEntry   *widget.Entry
	proxyEnabledCheck *widget.Check
	connectBtn        *widget.Button
)

func main() {
	a := app.New()
	a.Settings().SetTheme(theme.DarkTheme())

	w := a.NewWindow("Proxy Settings")

	remoteAddrEntry = widget.NewEntry()
	remoteAddrEntry.SetPlaceHolder("127.0.0.1:7777")
	proxyEnabledCheck = widget.NewCheck("Enable Proxy", nil)
	proxyEnabledCheck.SetChecked(true)

	connectBtn = widget.NewButton("Connect", onConnect)

	content := container.NewVBox(
		widget.NewLabel("Remote Address:"),
		remoteAddrEntry,
		proxyEnabledCheck,
		connectBtn,
	)

	//content.SetBorder(true)
	//content.SetBorderWidth(10)
	//content.SetBorderRadius(5)

	w.SetContent(content)
	w.Resize(fyne.NewSize(300, 150))
	w.CenterOnScreen()
	w.ShowAndRun()
}

func onConnect() {
	remoteAddr := remoteAddrEntry.Text
	proxyEnabled := proxyEnabledCheck.Checked

	if proxyEnabled {
		startProxy(remoteAddr)
		connectBtn.SetText("Disconnect")
		proxyEnabledCheck.SetChecked(false)
		connectBtn.OnTapped = onStop
	}
}

func onStop() {
	stopProxy()
	connectBtn.SetText("Connect")
	proxyEnabledCheck.SetChecked(true)
	connectBtn.OnTapped = onConnect
}

func startProxy(remoteAddr string) {
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

	proxyClient = qpc.NewProxyClient(":3333", quicConnMgr)
	go proxyClient.Run()
}

func stopProxy() {
	if proxyClient != nil {
		proxyClient.Stop()
	}
}

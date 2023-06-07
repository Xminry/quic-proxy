package main

import (
	"context"
	"crypto/tls"
	"log"
	qpc "quic-proxy-core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	proxyClient       *qpc.ProxyClient
	remoteAddrEntry   *widget.Entry
	proxyEnabledCheck *widget.Check
	connectBtn        *widget.Button
	statusLabel       *widget.Label
	disconnectBtn     *widget.Button
	mainContainer     *fyne.Container
	formContainer     *fyne.Container
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
	statusLabel = widget.NewLabel("")

	form := &widget.Form{}
	form.Append("Remote Address:", remoteAddrEntry)
	form.Append("", proxyEnabledCheck)
	form.Append("", connectBtn)

	formContainer = container.NewVBox(form)
	mainContainer = container.New(layout.NewCenterLayout(), formContainer)

	content := container.New(layout.NewCenterLayout(), mainContainer)

	w.SetContent(content)
	w.Resize(fyne.NewSize(350, 200))
	w.CenterOnScreen()
	w.ShowAndRun()
}

func onConnect() {
	remoteAddr := remoteAddrEntry.Text
	proxyEnabled := proxyEnabledCheck.Checked

	if proxyEnabled {
		startProxy(remoteAddr)
		connectBtn.Hide()
		proxyEnabledCheck.Hide()
		remoteAddrEntry.Hide()
		statusLabel.SetText("Connecting to " + remoteAddr)
		mainContainer.Objects = nil
		mainContainer.Layout = layout.NewCenterLayout()
		disconnectBtn = widget.NewButton("Disconnect", onStop)
		connectedContainer := container.NewVBox(
			layout.NewSpacer(),
			statusLabel,
			layout.NewSpacer(),
			disconnectBtn,
			layout.NewSpacer(),
		)
		mainContainer.Add(connectedContainer)
	}
}

func onStop() {
	stopProxy()
	statusLabel.SetText("")
	mainContainer.Remove(mainContainer.Objects[0])
	mainContainer.Layout = nil
	mainContainer.Add(formContainer)
	connectBtn.Show()
	proxyEnabledCheck.Show()
	remoteAddrEntry.Show()
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

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startTray() {
	go systray.Run(a.onTrayReady, func() {})
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon())
	systray.SetTooltip("Norfrig Hub")

	mShow := systray.AddMenuItem("Abrir Hub", "Mostrar la ventana")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Salir", "Cerrar Norfrig Hub")

	for {
		select {
		case <-mShow.ClickedCh:
			runtime.WindowShow(a.ctx)
		case <-mQuit.ClickedCh:
			systray.Quit()
			runtime.Quit(a.ctx)
			return
		}
	}
}

// trayIcon genera un círculo azul 32×32 como ícono del tray.
func trayIcon() []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	blue := color.NRGBA{R: 10, G: 132, B: 255, A: 255}
	cx, cy, r := float64(size)/2, float64(size)/2, float64(size)/2-2

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			if math.Sqrt(dx*dx+dy*dy) <= r {
				img.Set(x, y, blue)
			}
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img) //nolint:errcheck
	return buf.Bytes()
}

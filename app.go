package main

import (
	"context"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App gerencia o estado da aplicacao e a comunicacao entre backend e frontend.
type App struct {
	ctx      context.Context
	mu       sync.RWMutex
	sensor1  float64
	sensor2  float64
	ticker   *time.Ticker
	stopChan chan struct{}
}

// NewApp cria uma nova instancia do App.
func NewApp() *App {
	return &App{
		sensor1:  175.0,
		sensor2:  150.0,
		stopChan: make(chan struct{}),
	}
}

// startup eh invocado na inicializacao do Wails.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.iniciarCiclo()
}

// shutdown eh invocado ao fechar a aplicacao.
func (a *App) shutdown(ctx context.Context) {
	if a.ticker != nil {
		a.ticker.Stop()
	}
	close(a.stopChan)
}

// SetTemperature atualiza o valor instantaneo definido pelo knob do sensor.
func (a *App) SetTemperature(sensorID string, value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch sensorID {
	case "s1":
		a.sensor1 = value
	case "s2":
		a.sensor2 = value
	}
}

// GetTemperatures retorna os valores atuais de temperatura dos dois sensores.
func (a *App) GetTemperatures() map[string]float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]float64{
		"s1": a.sensor1,
		"s2": a.sensor2,
	}
}

// iniciarCiclo inicia o ticker com intervalo de tres segundos.
func (a *App) iniciarCiclo() {
	a.ticker = time.NewTicker(3 * time.Second)

	go func() {
		for {
			select {
			case <-a.stopChan:
				return
			case <-a.ticker.C:
				a.mu.RLock()
				t1 := a.sensor1
				t2 := a.sensor2
				a.mu.RUnlock()

				// Emite o evento de ciclo para o frontend com as leituras atuais
				runtime.EventsEmit(a.ctx, "cycle_tick", map[string]interface{}{
					"s1":        t1,
					"s2":        t2,
					"timestamp": time.Now().Unix(),
				})
			}
		}
	}()
}

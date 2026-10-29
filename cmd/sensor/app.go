package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"caldeira-mqtt/pkg/protocol"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App gerencia o estado da aplicacao e a comunicacao entre backend e frontend.
type App struct {
	ctx        context.Context
	mu         sync.RWMutex
	sensor1    float64
	sensor2    float64
	ticker     *time.Ticker
	stopChan   chan struct{}
	mqttClient mqtt.Client
}

// NewApp cria uma nova instancia do App.
func NewApp() *App {
	return &App{
		sensor1:  180.0,
		sensor2:  165.0,
		stopChan: make(chan struct{}),
	}
}

// startup eh invocado na inicializacao do Wails.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.conectarMQTT("tcp://localhost:1883")
	a.iniciarCiclo()
}

// shutdown eh invocado ao fechar a aplicacao.
func (a *App) shutdown(ctx context.Context) {
	if a.ticker != nil {
		a.ticker.Stop()
	}
	close(a.stopChan)

	if a.mqttClient != nil && a.mqttClient.IsConnected() {
		a.mqttClient.Disconnect(250)
	}
}

// conectarMQTT tenta conectar ao broker local de forma assincrona.
func (a *App) conectarMQTT(brokerURL string) {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID("sensor-gui-" + fmt.Sprintf("%d", time.Now().Unix())).
		SetAutoReconnect(true).
		SetKeepAlive(60 * time.Second)

	client := mqtt.NewClient(opts)
	token := client.Connect()

	go func() {
		if token.Wait() && token.Error() != nil {
			log.Printf("[SENSOR] Aviso: Nao foi possivel conectar ao broker MQTT (%s): %v", brokerURL, token.Error())
			return
		}
		a.mqttClient = client
		log.Printf("[SENSOR] Conectado ao broker MQTT em %s", brokerURL)
	}()
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

// publicarLeitura envia a leitura para o broker MQTT se estiver conectado.
func (a *App) publicarLeitura(sensorID string, temp float64, now int64) {
	if a.mqttClient == nil || !a.mqttClient.IsConnected() {
		return
	}

	payload := protocol.TemperatureReading{
		SensorID:    sensorID,
		Timestamp:   now,
		Temperature: temp,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	topic := protocol.TopicSensorPrefix + sensorID
	a.mqttClient.Publish(topic, 0, false, data)
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

				now := time.Now().Unix()

				// Publica telemetria no MQTT para cada sensor
				a.publicarLeitura("sensor_1", t1, now)
				a.publicarLeitura("sensor_2", t2, now)

				// Emite o evento de ciclo para o frontend com as leituras atuais
				runtime.EventsEmit(a.ctx, "cycle_tick", map[string]interface{}{
					"s1":        t1,
					"s2":        t2,
					"timestamp": now,
				})
			}
		}
	}()
}

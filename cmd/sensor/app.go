package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"caldeira-mqtt/pkg/protocol"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	minTempScale = 150.0
	maxTempScale = 210.0
)

// App gerencia o estado dos sensores e a comunicação entre o frontend Wails e o broker MQTT.
type App struct {
	ctx        context.Context
	mu         sync.RWMutex
	sensor1    float64
	sensor2    float64
	ticker     *time.Ticker
	stopChan   chan struct{}
	stopOnce   sync.Once
	mqttClient mqtt.Client
}

// NewApp cria uma nova instância da aplicação com temperaturas iniciais padrão.
func NewApp() *App {
	return &App{
		sensor1:  180.0,
		sensor2:  165.0,
		stopChan: make(chan struct{}),
	}
}

// startup é chamado pelo Wails na abertura da janela.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.conectarMQTT("tcp://localhost:1883")
	a.iniciarCiclo()
}

// shutdown é chamado pelo Wails ao encerrar a janela.
func (a *App) shutdown(ctx context.Context) {
	a.stopOnce.Do(func() {
		if a.ticker != nil {
			a.ticker.Stop()
		}
		close(a.stopChan)
	})

	a.mu.RLock()
	client := a.mqttClient
	a.mu.RUnlock()

	if client != nil && client.IsConnected() {
		client.Disconnect(250)
	}
}

// conectarMQTT inicia a conexão com o broker MQTT de forma assíncrona.
func (a *App) conectarMQTT(brokerURL string) {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(fmt.Sprintf("sensor-gui-%d", time.Now().UnixNano())).
		SetAutoReconnect(true).
		SetKeepAlive(60 * time.Second)

	client := mqtt.NewClient(opts)

	a.mu.Lock()
	a.mqttClient = client
	a.mu.Unlock()

	token := client.Connect()
	go func() {
		if token.Wait() && token.Error() != nil {
			log.Printf("[SENSOR] Aviso: Não foi possível conectar ao broker MQTT (%s): %v", brokerURL, token.Error())
			return
		}
		log.Printf("[SENSOR] Conectado ao broker MQTT em %s", brokerURL)
	}()
}

// SetTemperature atualiza a temperatura instantânea do sensor selecionado pelo usuário.
func (a *App) SetTemperature(sensorID string, value float64) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return
	}

	clamped := math.Max(minTempScale, math.Min(maxTempScale, value))

	a.mu.Lock()
	defer a.mu.Unlock()

	switch sensorID {
	case "s1":
		a.sensor1 = clamped
	case "s2":
		a.sensor2 = clamped
	}
}

// GetTemperatures retorna os valores atuais de temperatura dos sensores.
func (a *App) GetTemperatures() map[string]float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]float64{
		"s1": a.sensor1,
		"s2": a.sensor2,
	}
}

// publicarLeitura serializa e publica a leitura de um sensor no broker MQTT.
func (a *App) publicarLeitura(sensorID string, temp float64, now int64) {
	a.mu.RLock()
	client := a.mqttClient
	a.mu.RUnlock()

	if client == nil || !client.IsConnected() {
		return
	}

	payload := protocol.TemperatureReading{
		SensorID:    sensorID,
		Timestamp:   now,
		Temperature: temp,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[SENSOR] Erro ao serializar leitura do %s: %v", sensorID, err)
		return
	}

	topic := protocol.TopicSensorPrefix + sensorID
	client.Publish(topic, 0, false, data)
}

// iniciarCiclo executa o loop periódico de 3 segundos para envio MQTT e atualização do gráfico. Pra estar em conformidade com os 60 segundos requeridos basta mudar o valor do ticker.
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

				a.publicarLeitura("sensor_1", t1, now)
				a.publicarLeitura("sensor_2", t2, now)

				runtime.EventsEmit(a.ctx, "cycle_tick", map[string]interface{}{
					"s1":        t1,
					"s2":        t2,
					"timestamp": now,
				})
			}
		}
	}()
}

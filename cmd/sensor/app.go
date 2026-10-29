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
	ctx         context.Context
	mu          sync.RWMutex
	sensorCount int
	sensor1     float64
	sensor2     float64
	sensor3     float64
	ticker      *time.Ticker
	stopChan    chan struct{}
	stopOnce    sync.Once
	mqttClient  mqtt.Client
}

// NewApp cria uma nova instância da aplicação com dois sensores ativos por padrão.
func NewApp() *App {
	return &App{
		sensorCount: 2,
		sensor1:     180.0,
		sensor2:     165.0,
		sensor3:     175.0,
		stopChan:    make(chan struct{}),
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

// SetSensorCount ajusta a quantidade de sensores ativos (entre 1 e 3).
func (a *App) SetSensorCount(count int) {
	if count < 1 {
		count = 1
	} else if count > 3 {
		count = 3
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.sensorCount = count
	log.Printf("[SENSOR] Quantidade de sensores ativos alterada para: %d", count)
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
	case "s3":
		a.sensor3 = clamped
	}
}

// GetTemperatures retorna os valores atuais de temperatura dos sensores e a quantidade ativa.
func (a *App) GetTemperatures() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"count": a.sensorCount,
		"s1":    a.sensor1,
		"s2":    a.sensor2,
		"s3":    a.sensor3,
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

// iniciarCiclo executa o loop periódico de 3 segundos para envio MQTT e atualização do gráfico.
func (a *App) iniciarCiclo() {
	a.ticker = time.NewTicker(3 * time.Second)

	go func() {
		for {
			select {
			case <-a.stopChan:
				return
			case <-a.ticker.C:
				a.mu.RLock()
				count := a.sensorCount
				t1 := a.sensor1
				t2 := a.sensor2
				t3 := a.sensor3
				a.mu.RUnlock()

				now := time.Now().Unix()

				if count >= 1 {
					a.publicarLeitura("sensor_1", t1, now)
				}
				if count >= 2 {
					a.publicarLeitura("sensor_2", t2, now)
				}
				if count >= 3 {
					a.publicarLeitura("sensor_3", t3, now)
				}

				runtime.EventsEmit(a.ctx, "cycle_tick", map[string]interface{}{
					"count":     count,
					"s1":        t1,
					"s2":        t2,
					"s3":        t3,
					"timestamp": now,
				})
			}
		}
	}()
}

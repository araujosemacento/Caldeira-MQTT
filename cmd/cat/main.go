package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"caldeira-mqtt/pkg/protocol"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type readingRecord struct {
	reading   protocol.TemperatureReading
	receivedAt time.Time
}

type CATService struct {
	brokerURL    string
	windowSize   time.Duration
	client       mqtt.Client
	mu           sync.Mutex
	readings     []readingRecord
	previousAvg  *float64
}

func NewCATService(brokerURL string, windowSize time.Duration) *CATService {
	return &CATService{
		brokerURL:  brokerURL,
		windowSize: windowSize,
		readings:   make([]readingRecord, 0),
	}
}

func (c *CATService) Start() error {
	opts := mqtt.NewClientOptions().
		AddBroker(c.brokerURL).
		SetClientID("cat-service-" + fmt.Sprintf("%d", time.Now().Unix())).
		SetAutoReconnect(true).
		SetKeepAlive(60 * time.Second)

	c.client = mqtt.NewClient(opts)
	token := c.client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("erro ao conectar ao broker MQTT: %w", token.Error())
	}

	log.Printf("[CAT] Conectado ao broker MQTT em %s", c.brokerURL)
	log.Printf("[CAT] Janela de calculo de media: %s", c.windowSize)

	subToken := c.client.Subscribe(protocol.TopicSensorsWildcard, 1, c.handleMessage)
	if subToken.Wait() && subToken.Error() != nil {
		return fmt.Errorf("erro ao subscrever no topico de sensores: %w", subToken.Error())
	}
	log.Printf("[CAT] Inscrito no topico: %s", protocol.TopicSensorsWildcard)

	// Loop periodico de avaliacao da media
	go c.runAverageLoop()

	return nil
}

func (c *CATService) handleMessage(_ mqtt.Client, msg mqtt.Message) {
	var reading protocol.TemperatureReading
	if err := json.Unmarshal(msg.Payload(), &reading); err != nil {
		log.Printf("[CAT] Erro ao decodificar payload recebido: %v", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.readings = append(c.readings, readingRecord{
		reading:    reading,
		receivedAt: time.Now(),
	})

	log.Printf("[CAT] Leitura recebida de [%s]: %.1f C", reading.SensorID, reading.Temperature)
}

func (c *CATService) runAverageLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.evaluateAverage()
	}
}

func (c *CATService) evaluateAverage() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-c.windowSize)

	// Filtra apenas as leituras dentro da janela deslizante
	valid := make([]readingRecord, 0, len(c.readings))
	sum := 0.0

	for _, r := range c.readings {
		if r.receivedAt.After(cutoff) {
			valid = append(valid, r)
			sum += r.reading.Temperature
		}
	}
	c.readings = valid

	if len(valid) == 0 {
		return
	}

	currentAvg := sum / float64(len(valid))
	log.Printf("[CAT] Media dos ultimos %s (%d leituras): %.2f C", c.windowSize, len(valid), currentAvg)

	// Regra 1: Temperatura Alta (> 200 C)
	if currentAvg > 200.0 {
		c.publishAlarm(protocol.TopicAlarmHighTemp, protocol.AlarmEvent{
			Type:        "temperatura_alta",
			Average:     currentAvg,
			Description: fmt.Sprintf("Media de temperatura critica acima de 200 C: %.2f C", currentAvg),
			Timestamp:   now.Unix(),
		})
	}

	// Regra 2: Variacao Repentina (diferenca >= 5 graus entre medias consecutivas)
	if c.previousAvg != nil {
		diff := math.Abs(currentAvg - *c.previousAvg)
		if diff >= 5.0 {
			c.publishAlarm(protocol.TopicAlarmSuddenChange, protocol.AlarmEvent{
				Type:        "variacao_repentina",
				Average:     currentAvg,
				Description: fmt.Sprintf("Variacao repentina de temperatura detectada: %.2f C (diferenca de %.2f C)", currentAvg, diff),
				Timestamp:   now.Unix(),
			})
		}
	}

	c.previousAvg = &currentAvg
}

func (c *CATService) publishAlarm(topic string, event protocol.AlarmEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[CAT] Erro ao codificar alarme: %v", err)
		return
	}

	token := c.client.Publish(topic, 1, false, data)
	token.Wait()
	log.Printf("[CAT] ALARME DISPARADO no topico [%s]: %s", topic, event.Description)
}

func (c *CATService) Stop() {
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
	}
}

func main() {
	broker := flag.String("broker", "tcp://localhost:1883", "URL do broker MQTT")
	window := flag.Duration("window", 120*time.Second, "Tamanho da janela deslizante do CAT")
	flag.Parse()

	service := NewCATService(*broker, *window)
	if err := service.Start(); err != nil {
		log.Fatalf("[CAT] Falha fatal: %v", err)
	}
	defer service.Stop()

	// Aguarda sinal de interrupcao
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("[CAT] Encerrando servico CAT...")
}


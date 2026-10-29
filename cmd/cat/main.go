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

const maxBufferedReadings = 1000

// readingRecord armazena uma leitura recebida junto ao instante local de chegada.
type readingRecord struct {
	reading    protocol.TemperatureReading
	receivedAt time.Time
}

// CATService calcula a média móvel de temperatura e publica alertas no broker MQTT.
type CATService struct {
	brokerURL   string
	windowSize  time.Duration
	client      mqtt.Client
	mu          sync.Mutex
	readings    []readingRecord
	previousAvg *float64
	stopChan    chan struct{}
	stopOnce    sync.Once
}

// NewCATService inicializa o serviço CAT com a URL do broker e a duração da janela.
func NewCATService(brokerURL string, windowSize time.Duration) *CATService {
	return &CATService{
		brokerURL:  brokerURL,
		windowSize: windowSize,
		readings:   make([]readingRecord, 0),
		stopChan:   make(chan struct{}),
	}
}

// Start conecta ao broker MQTT, registra a inscrição nos sensores e inicia o cálculo periódico.
func (c *CATService) Start() error {
	opts := mqtt.NewClientOptions().
		AddBroker(c.brokerURL).
		SetClientID(fmt.Sprintf("cat-service-%d", time.Now().UnixNano())).
		SetAutoReconnect(true).
		SetKeepAlive(60 * time.Second).
		SetOnConnectHandler(func(client mqtt.Client) {
			subToken := client.Subscribe(protocol.TopicSensorsWildcard, 1, c.handleMessage)
			if subToken.Wait() && subToken.Error() != nil {
				log.Printf("[CAT] Erro ao assinar o tópico %s: %v", protocol.TopicSensorsWildcard, subToken.Error())
				return
			}
			log.Printf("[CAT] Inscrito no tópico: %s", protocol.TopicSensorsWildcard)
		})

	c.client = mqtt.NewClient(opts)
	token := c.client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("erro ao conectar ao broker MQTT: %w", token.Error())
	}

	log.Printf("[CAT] Conectado ao broker MQTT em %s (janela: %s)", c.brokerURL, c.windowSize)

	go c.runAverageLoop()

	return nil
}

// handleMessage processa e valida cada leitura de temperatura recebida dos sensores.
func (c *CATService) handleMessage(_ mqtt.Client, msg mqtt.Message) {
	var reading protocol.TemperatureReading
	if err := json.Unmarshal(msg.Payload(), &reading); err != nil {
		log.Printf("[CAT] Erro ao decodificar mensagem JSON: %v", err)
		return
	}

	if err := reading.Validate(); err != nil {
		log.Printf("[CAT] Leitura descartada (%s): %v", reading.SensorID, err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.readings) >= maxBufferedReadings {
		c.readings = c.readings[1:]
	}

	c.readings = append(c.readings, readingRecord{
		reading:    reading,
		receivedAt: time.Now(),
	})

	log.Printf("[CAT] Leitura recebida de [%s]: %.1f °C", reading.SensorID, reading.Temperature)
}

// runAverageLoop aciona o cálculo da média a cada ciclo de 3 segundos.
func (c *CATService) runAverageLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.evaluateAverage()
		}
	}
}

// evaluateAverage remove leituras expiradas, calcula a média da janela e verifica os limites de alarme.
func (c *CATService) evaluateAverage() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-c.windowSize)

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
	log.Printf("[CAT] Média dos últimos %s (%d leituras): %.2f °C", c.windowSize, len(valid), currentAvg)

	// Regra 1: Temperatura alta (média acima de 200 °C)
	if currentAvg > 200.0 {
		c.publishAlarm(protocol.TopicAlarmHighTemp, protocol.AlarmEvent{
			Type:        "temperatura_alta",
			Average:     currentAvg,
			Description: fmt.Sprintf("Média de temperatura crítica acima de 200 °C: %.2f °C", currentAvg),
			Timestamp:   now.Unix(),
		})
	}

	// Regra 2: Variação repentina (diferença de 5 °C ou mais entre as duas últimas médias)
	if c.previousAvg != nil {
		diff := math.Abs(currentAvg - *c.previousAvg)
		if diff >= 5.0 {
			c.publishAlarm(protocol.TopicAlarmSuddenChange, protocol.AlarmEvent{
				Type:        "variacao_repentina",
				Average:     currentAvg,
				Description: fmt.Sprintf("Variação repentina de temperatura detectada: %.2f °C (diferença de %.2f °C)", currentAvg, diff),
				Timestamp:   now.Unix(),
			})
		}
	}

	c.previousAvg = &currentAvg
}

// publishAlarm envia um evento de alarme para o tópico MQTT correspondente.
func (c *CATService) publishAlarm(topic string, event protocol.AlarmEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[CAT] Erro ao serializar alarme: %v", err)
		return
	}

	token := c.client.Publish(topic, 1, false, data)
	token.Wait()
	if token.Error() != nil {
		log.Printf("[CAT] Erro ao publicar alarme no tópico [%s]: %v", topic, token.Error())
		return
	}
	log.Printf("[CAT] ALARME PUBLICADO no tópico [%s]: %s", topic, event.Description)
}

// Stop encerra a rotina de cálculo e desconecta do broker MQTT.
func (c *CATService) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopChan)
	})
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
	}
}

func main() {
	broker := flag.String("broker", "tcp://localhost:1883", "URL do broker MQTT")
	window := flag.Duration("window", 120*time.Second, "Janela de tempo para o cálculo da média")
	flag.Parse()

	service := NewCATService(*broker, *window)
	if err := service.Start(); err != nil {
		log.Fatalf("[CAT] Falha ao iniciar o serviço: %v", err)
	}
	defer service.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("[CAT] Encerrando serviço CAT...")
}

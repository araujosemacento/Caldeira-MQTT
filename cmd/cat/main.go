package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"caldeira-mqtt/pkg/protocol"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const maxBufferedReadings = 1000

// Códigos de escape ANSI para formatação semântica no terminal.
const (
	ansiReset      = "\033[0m"
	ansiBold       = "\033[1m"
	ansiGray       = "\033[90m"
	ansiCyan       = "\033[36m"
	ansiBoldCyan   = "\033[1;36m"
	ansiMagenta    = "\033[35m"
	ansiBoldMag    = "\033[1;35m"
	ansiYellow     = "\033[33m"
	ansiBoldYellow = "\033[1;33m"
	ansiGreen      = "\033[32m"
	ansiBoldWhite  = "\033[1;97m"
	ansiBoldRed    = "\033[1;91m"
	ansiBgRed      = "\033[41;97m"
)

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

// sensorColor retorna as cores ANSI correspondentes à paleta CMY dos sensores.
func sensorColor(sensorID string) (string, string) {
	switch sensorID {
	case "sensor_1", "s1":
		return ansiCyan, ansiBoldCyan
	case "sensor_2", "s2":
		return ansiMagenta, ansiBoldMag
	case "sensor_3", "s3":
		return ansiYellow, ansiBoldYellow
	default:
		return "", ansiBold
	}
}

// timeStamp retorna o horário atual formatado em cinza neutro.
func timeStamp() string {
	return fmt.Sprintf("%s%s%s", ansiGray, time.Now().Format("15:04:05"), ansiReset)
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
				fmt.Printf("%s %s[CAT]%s Erro ao assinar tópico %s: %v\n", timeStamp(), ansiBoldRed, ansiReset, protocol.TopicSensorsWildcard, subToken.Error())
				return
			}
			fmt.Printf("%s %s[CAT]%s Inscrito no tópico de sensores: %s%s%s\n", timeStamp(), ansiGray, ansiReset, ansiBoldCyan, protocol.TopicSensorsWildcard, ansiReset)
		})

	c.client = mqtt.NewClient(opts)
	token := c.client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("erro ao conectar ao broker MQTT: %w", token.Error())
	}

	fmt.Printf("%s %s[CAT]%s Conectado ao broker MQTT em %s (janela de cálculo: %s)\n", timeStamp(), ansiGray, ansiReset, c.brokerURL, c.windowSize)

	go c.runAverageLoop()

	return nil
}

// handleMessage processa e valida cada leitura de temperatura recebida dos sensores.
func (c *CATService) handleMessage(_ mqtt.Client, msg mqtt.Message) {
	var reading protocol.TemperatureReading
	if err := json.Unmarshal(msg.Payload(), &reading); err != nil {
		fmt.Printf("%s %s[CAT]%s Erro ao decodificar JSON: %v\n", timeStamp(), ansiBoldRed, ansiReset, err)
		return
	}

	if err := reading.Validate(); err != nil {
		fmt.Printf("%s %s[CAT]%s Leitura descartada (%s): %v\n", timeStamp(), ansiBoldRed, ansiReset, reading.SensorID, err)
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

	colNorm, colBold := sensorColor(reading.SensorID)
	fmt.Printf("%s %s[CAT]%s Leitura %s[%s]%s: %s%.1f °C%s\n",
		timeStamp(),
		ansiGray, ansiReset,
		colNorm, reading.SensorID, ansiReset,
		colBold, reading.Temperature, ansiReset,
	)
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

	var statusTag string
	var deltaStr string

	if c.previousAvg != nil {
		diff := currentAvg - *c.previousAvg
		deltaStr = fmt.Sprintf(" (Δ %+.2f °C)", diff)

		if math.Abs(diff) >= 5.0 {
			statusTag = fmt.Sprintf(" %s[SALTO BRUSCO]%s", ansiBoldYellow, ansiReset)
		} else if currentAvg > 200.0 {
			statusTag = fmt.Sprintf(" %s[TEMPERATURA ALTA]%s", ansiBoldRed, ansiReset)
		} else {
			statusTag = fmt.Sprintf(" %s[ESTÁVEL]%s", ansiGreen, ansiReset)
		}
	} else {
		statusTag = fmt.Sprintf(" %s[INICIAL]%s", ansiGreen, ansiReset)
	}

	// Linha compacta da média da janela (mantida sob 65 colunas para evitar quebra de linha)
	fmt.Printf("%s %s[CAT] Média (%d am.): %.2f °C%s%s%s\n",
		timeStamp(),
		ansiBoldWhite,
		len(valid), currentAvg,
		deltaStr, statusTag, ansiReset,
	)

	// Regra 1: Temperatura alta (média acima de 200 °C)
	if currentAvg > 200.0 {
		c.publishAlarm(protocol.TopicAlarmHighTemp, protocol.AlarmEvent{
			Type:        "temperatura_alta",
			Average:     currentAvg,
			Description: fmt.Sprintf("Média em %.2f °C (acima de 200 °C)", currentAvg),
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
				Description: fmt.Sprintf("Salto térmico de %.2f °C", diff),
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
		fmt.Printf("%s %s[CAT]%s Erro ao serializar alarme: %v\n", timeStamp(), ansiBoldRed, ansiReset, err)
		return
	}

	token := c.client.Publish(topic, 1, false, data)
	token.Wait()
	if token.Error() != nil {
		fmt.Printf("%s %s[CAT]%s Erro ao publicar alarme no tópico [%s]: %v\n", timeStamp(), ansiBoldRed, ansiReset, topic, token.Error())
		return
	}

	shortTopic := strings.TrimPrefix(topic, "caldeira/alarmes/")
	if event.Type == "temperatura_alta" {
		fmt.Printf("%s %s[CAT] >>> PUBLICADO [%s]: %s%s\n",
			timeStamp(), ansiBoldRed, shortTopic, event.Description, ansiReset)
	} else {
		fmt.Printf("%s %s[CAT] >>> PUBLICADO [%s]: %s%s\n",
			timeStamp(), ansiBoldYellow, shortTopic, event.Description, ansiReset)
	}
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

	fmt.Printf("\n%s %s[CAT]%s Encerrando serviço CAT...\n", timeStamp(), ansiGray, ansiReset)
}

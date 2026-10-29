package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"caldeira-mqtt/pkg/protocol"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"golang.org/x/term"
)

// TopicRecord armazena os metadados e o último valor registrado de um tópico.
type TopicRecord struct {
	Topic       string
	Count       int
	LastUpdated time.Time
	LastPayload string
}

// Inspector gerencia o estado das mensagens capturadas do broker MQTT.
type Inspector struct {
	mu        sync.RWMutex
	brokerURL string
	topicMask string
	client    mqtt.Client
	records   map[string]*TopicRecord
}

// NewInspector cria uma nova instância do inspetor.
func NewInspector(brokerURL, topicMask string) *Inspector {
	return &Inspector{
		brokerURL: brokerURL,
		topicMask: topicMask,
		records:   make(map[string]*TopicRecord),
	}
}

// Start conecta ao broker e inicia a renderização contínua do painel.
func (ins *Inspector) Start() error {
	opts := mqtt.NewClientOptions().
		AddBroker(ins.brokerURL).
		SetClientID(fmt.Sprintf("inspector-cli-%d", time.Now().UnixNano())).
		SetAutoReconnect(true).
		SetKeepAlive(30 * time.Second).
		SetOnConnectHandler(func(client mqtt.Client) {
			token := client.Subscribe(ins.topicMask, 0, ins.handleMessage)
			if token.Wait() && token.Error() != nil {
				log.Printf("[INSPETOR] Erro ao assinar máscara %s: %v", ins.topicMask, token.Error())
				return
			}
		})

	ins.client = mqtt.NewClient(opts)
	token := ins.client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("falha ao conectar ao broker MQTT (%s): %w", ins.brokerURL, token.Error())
	}

	return nil
}

// Stop desconecta do broker MQTT.
func (ins *Inspector) Stop() {
	if ins.client != nil && ins.client.IsConnected() {
		ins.client.Disconnect(250)
	}
}

// handleMessage atualiza o registro do tópico a cada mensagem recebida.
func (ins *Inspector) handleMessage(_ mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()
	summary := formatPayload(payload)

	ins.mu.Lock()
	defer ins.mu.Unlock()

	rec, exists := ins.records[topic]
	if !exists {
		rec = &TopicRecord{
			Topic: topic,
		}
		ins.records[topic] = rec
	}

	rec.Count++
	rec.LastUpdated = time.Now()
	rec.LastPayload = summary
}

// formatPayload interpreta o JSON das mensagens do sistema ou resume strings brutas.
func formatPayload(raw []byte) string {
	var reading protocol.TemperatureReading
	if err := json.Unmarshal(raw, &reading); err == nil && reading.SensorID != "" {
		return fmt.Sprintf("%.1f °C", reading.Temperature)
	}

	var alarm protocol.AlarmEvent
	if err := json.Unmarshal(raw, &alarm); err == nil && alarm.Type != "" {
		return fmt.Sprintf("%s (Média: %.1f °C)", alarm.Type, alarm.Average)
	}

	s := strings.TrimSpace(string(raw))
	if len(s) > 40 {
		return s[:37] + "..."
	}
	if s == "" {
		return "(vazio)"
	}
	return s
}

// abbreviateTopic encurta dinamicamente o nome do tópico para caber na coluna disponível.
func abbreviateTopic(topic string, maxLen int) string {
	if len(topic) <= maxLen {
		return topic
	}
	trimmed := strings.TrimPrefix(topic, "caldeira/")
	if len(trimmed) <= maxLen {
		return trimmed
	}
	lastSlash := strings.LastIndex(trimmed, "/")
	if lastSlash != -1 && lastSlash < len(trimmed)-1 {
		lastPart := trimmed[lastSlash:]
		if len("..."+lastPart) <= maxLen {
			prevSlash := strings.LastIndex(trimmed[:lastSlash], "/")
			if prevSlash != -1 {
				twoParts := trimmed[prevSlash:]
				if len("..."+twoParts) <= maxLen {
					return "..." + twoParts
				}
			}
			return "..." + lastPart
		}
	}
	if maxLen > 3 {
		return "..." + trimmed[len(trimmed)-(maxLen-3):]
	}
	return topic[:maxLen]
}

// truncateText reduz uma string com reticências para não estourar a largura da coluna.
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen > 3 {
		return s[:maxLen-3] + "..."
	}
	return s[:maxLen]
}

// Render imprime a tabela fixa atualizada no terminal adaptando-se à largura da janela.
func (ins *Inspector) Render() {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		width = 88 // Largura padrão caso a leitura das dimensões do terminal não esteja disponível
	}
	if width < 50 {
		width = 50
	}

	ins.mu.RLock()
	topics := make([]string, 0, len(ins.records))
	for t := range ins.records {
		topics = append(topics, t)
	}
	sort.Strings(topics)

	type row struct {
		topic   string
		count   int
		time    time.Time
		payload string
	}

	rows := make([]row, 0, len(topics))
	for _, t := range topics {
		rec := ins.records[t]
		rows = append(rows, row{
			topic:   rec.Topic,
			count:   rec.Count,
			time:    rec.LastUpdated,
			payload: rec.LastPayload,
		})
	}
	ins.mu.RUnlock()

	// Distribuição dinâmica das colunas com base na largura atual do terminal:
	// Formato: " " + topic + " " + msgs + "  " + elapsed + "  " + payload
	// Espaçamento fixo entre colunas: 1 (início) + 1 + 2 + 2 = 6 caracteres
	const msgsWidth = 5

	timeWidth := 18
	if width < 80 {
		timeWidth = 8 // Em telas estreitas, exibe apenas HH:MM:SS
	}

	fixedWidth := 6 + msgsWidth + timeWidth
	rem := width - fixedWidth
	if rem < 20 {
		rem = 20
	}

	topicWidth := int(float64(rem) * 0.58)
	if topicWidth < 14 {
		topicWidth = 14
	}
	payloadWidth := rem - topicWidth
	if payloadWidth < 8 {
		payloadWidth = 8
	}

	var sb strings.Builder

	// Reposiciona o cursor no topo da tela
	sb.WriteString("\033[H")

	sepBar := strings.Repeat("=", width)
	subBar := strings.Repeat("-", width)

	sb.WriteString(sepBar)
	sb.WriteString("\n")
	title := fmt.Sprintf(" PAINEL DE TÓPICOS DO BROKER MQTT (%s)", ins.brokerURL)
	if len(title) > width-1 {
		title = truncateText(title, width-1)
	}
	sb.WriteString(title)
	sb.WriteString("\n")
	sb.WriteString(sepBar)
	sb.WriteString("\n")

	// Cabeçalho da tabela com larguras calculadas dinamicamente
	header := fmt.Sprintf(" %-*s %*s  %-*s  %-*s\n",
		topicWidth, "TÓPICO ATIVO",
		msgsWidth, "MSGS",
		timeWidth, "ÚLTIMA ATUALIZAÇÃO",
		payloadWidth, "ÚLTIMO VALOR",
	)
	sb.WriteString(header)
	sb.WriteString(subBar)
	sb.WriteString("\n")

	now := time.Now()
	if len(rows) == 0 {
		sb.WriteString(" Aguardando publicações nos tópicos do broker...\n")
	} else {
		for _, r := range rows {
			var elapsed string
			if timeWidth >= 18 {
				diff := int(now.Sub(r.time).Seconds())
				elapsed = fmt.Sprintf("%s (há %ds)", r.time.Format("15:04:05"), diff)
			} else {
				elapsed = r.time.Format("15:04:05")
			}

			abbrTopic := abbreviateTopic(r.topic, topicWidth)
			paddedTopic := fmt.Sprintf("%-*s", topicWidth, abbrTopic)
			colorPre, colorPost := topicColors(r.topic)

			truncPayload := truncateText(r.payload, payloadWidth)
			paddedPayload := fmt.Sprintf("%-*s", payloadWidth, truncPayload)

			sb.WriteString(fmt.Sprintf(" %s%s%s %*d  %-*s  %s\n",
				colorPre, paddedTopic, colorPost,
				msgsWidth, r.count,
				timeWidth, elapsed,
				paddedPayload,
			))
		}
	}

	sb.WriteString(sepBar)
	sb.WriteString("\n")
	sb.WriteString(" Pressione Ctrl+C para encerrar o monitoramento.\n")
	// Limpa quaisquer caracteres ou linhas remanescentes abaixo da tabela
	sb.WriteString("\033[J")

	fmt.Print(sb.String())
}

// topicColors retorna sequências ANSI com base no padrão semântico do tópico.
func topicColors(topic string) (string, string) {
	switch {
	case strings.HasSuffix(topic, "sensor_1"):
		return "\033[36m", "\033[0m" // Ciano
	case strings.HasSuffix(topic, "sensor_2"):
		return "\033[35m", "\033[0m" // Magenta
	case strings.HasSuffix(topic, "sensor_3"):
		return "\033[33m", "\033[0m" // Amarelo
	case strings.HasSuffix(topic, "variacao_repentina"):
		return "\033[1;33m", "\033[0m" // Âmbar em negrito
	case strings.HasSuffix(topic, "temperatura_alta"):
		return "\033[1;31m", "\033[0m" // Vermelho em negrito
	default:
		return "", ""
	}
}

func main() {
	brokerFlag := flag.String("broker", "tcp://localhost:1883", "Endereço do broker MQTT")
	topicFlag := flag.String("topic", "#", "Máscara de tópicos para monitoramento")
	intervalFlag := flag.Duration("interval", 500*time.Millisecond, "Intervalo de atualização da tela")
	flag.Parse()

	inspector := NewInspector(*brokerFlag, *topicFlag)
	if err := inspector.Start(); err != nil {
		log.Fatalf("[INSPETOR] Erro ao iniciar: %v", err)
	}
	defer inspector.Stop()

	// Alterna para o buffer de tela alternativo e oculta o cursor
	// Isso impede que a tabela seja empurrada para o histórico de rolagem do terminal.
	fmt.Print("\033[?1049h\033[?25l")
	defer fmt.Print("\033[?25h\033[?1049l")

	// Captura sinais de interrupção para encerramento gracioso
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(*intervalFlag)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Print("\033[?25h\033[?1049l")
			fmt.Print("[INSPETOR] Monitoramento encerrado pelo usuário.\n")
			return
		case <-ticker.C:
			inspector.Render()
		}
	}
}

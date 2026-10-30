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

// Códigos de escape ANSI para formatação semântica no terminal.
const (
	ansiReset      = "\033[0m"
	ansiBold       = "\033[1m"
	ansiGray       = "\033[90m"
	ansiCyan       = "\033[36m"
	ansiGreen      = "\033[32m"
	ansiBoldYellow = "\033[1;33m"
	ansiBoldRed    = "\033[1;91m"
	ansiBgRed      = "\033[41;97m"
	ansiBgYellow   = "\033[43;30m"
	ansiBoldWhite  = "\033[1;97m"
)

// timeStamp retorna o horário atual formatado em cinza neutro.
func timeStamp() string {
	return fmt.Sprintf("%s%s%s", ansiGray, time.Now().Format("15:04:05"), ansiReset)
}

// App gerencia o estado visual e a conexão MQTT do serviço Alarms.
type App struct {
	ctx       context.Context
	brokerURL string
	mu        sync.RWMutex
	client    mqtt.Client
	state     string
	title     string
	detail    string
	stopChan  chan struct{}
	stopOnce  sync.Once
}

// NewApp inicializa a aplicação com estado regular padrão.
func NewApp(brokerURL string) *App {
	return &App{
		brokerURL: brokerURL,
		state:     "regular",
		title:     "Estado Regular",
		detail:    "Sistema Estável",
		stopChan:  make(chan struct{}),
	}
}

// startup é chamado pelo Wails quando a janela é criada.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.conectarMQTT()
}

// shutdown desconecta o cliente MQTT ao fechar a janela.
func (a *App) shutdown(_ context.Context) {
	a.stopOnce.Do(func() {
		close(a.stopChan)
	})

	a.mu.RLock()
	client := a.client
	a.mu.RUnlock()

	if client != nil && client.IsConnected() {
		client.Disconnect(250)
	}

	fmt.Printf("\n%s %s[ALARMS]%s Encerrando serviço Alarms...\n", timeStamp(), ansiGray, ansiReset)
}

// conectarMQTT inicia a conexão com o broker e registra a subscrição de alarmes.
func (a *App) conectarMQTT() {
	opts := mqtt.NewClientOptions().
		AddBroker(a.brokerURL).
		SetClientID(fmt.Sprintf("alarms-gui-%d", time.Now().UnixNano())).
		SetAutoReconnect(true).
		SetKeepAlive(60 * time.Second).
		SetOnConnectHandler(func(client mqtt.Client) {
			subToken := client.Subscribe(protocol.TopicAlarmsWildcard, 1, a.handleAlarmMessage)
			if subToken.Wait() && subToken.Error() != nil {
				fmt.Printf("%s %s[ALARMS]%s Erro ao assinar tópico de alarmes: %v\n", timeStamp(), ansiBoldRed, ansiReset, subToken.Error())
				return
			}
			fmt.Printf("%s %s[ALARMS]%s Monitorando alarmes no tópico: %s%s%s\n", timeStamp(), ansiGray, ansiReset, ansiCyan, protocol.TopicAlarmsWildcard, ansiReset)
		})

	client := mqtt.NewClient(opts)
	a.mu.Lock()
	a.client = client
	a.mu.Unlock()

	token := client.Connect()
	go func() {
		if token.Wait() && token.Error() != nil {
			log.Printf("[ALARMS] Erro ao conectar ao broker MQTT (%s): %v", a.brokerURL, token.Error())
			return
		}
		fmt.Printf("%s %s[ALARMS]%s Conectado ao broker MQTT em %s\n", timeStamp(), ansiGray, ansiReset, a.brokerURL)
	}()
}

// handleAlarmMessage imprime o cartão ANSI no terminal e atualiza a interface gráfica.
func (a *App) handleAlarmMessage(_ mqtt.Client, msg mqtt.Message) {
	var alarm protocol.AlarmEvent
	if err := json.Unmarshal(msg.Payload(), &alarm); err != nil {
		fmt.Printf("%s %s[ALARMS]%s Mensagem em [%s]: %s\n",
			timeStamp(), ansiBoldWhite, ansiReset, msg.Topic(), string(msg.Payload()))
		return
	}

	horario := time.Unix(alarm.Timestamp, 0).Format("15:04:05")
	const sep = "----------------------------------------------------------"

	var newState, newTitle, newDetail string

	switch alarm.Type {
	case "temperatura_alta":
		newState = "danger"
		newTitle = "Temperatura Alta"
		newDetail = fmt.Sprintf("Média crítica: %.2f °C", alarm.Average)

		fmt.Println()
		fmt.Printf("%s%s%s\n", ansiBoldRed, sep, ansiReset)
		fmt.Printf(" %s TEMPERATURA ALTA (> 200 °C) %s             %s%s%s\n",
			ansiBgRed, ansiReset, ansiBoldWhite, horario, ansiReset)
		fmt.Printf(" Média Crítica: %s%.2f °C%s  |  Limite excedido (> 200 °C)\n",
			ansiBoldRed, alarm.Average, ansiReset)
		fmt.Printf("%s%s%s\n", ansiBoldRed, sep, ansiReset)

	case "variacao_repentina":
		newState = "warning"
		newTitle = "Variação Repentina"
		newDetail = alarm.Description

		fmt.Println()
		fmt.Printf("%s%s%s\n", ansiBoldYellow, sep, ansiReset)
		fmt.Printf(" %s VARIAÇÃO REPENTINA %s                    %s%s%s\n",
			ansiBgYellow, ansiReset, ansiBoldWhite, horario, ansiReset)
		fmt.Printf(" Média do Ciclo: %s%.2f °C%s  |  %s\n",
			ansiBoldYellow, alarm.Average, ansiReset, alarm.Description)
		fmt.Printf("%s%s%s\n", ansiBoldYellow, sep, ansiReset)

	default:
		newState = "warning"
		newTitle = "Alarme Detectado"
		newDetail = alarm.Description

		fmt.Println()
		fmt.Printf("%s%s%s\n", ansiBoldWhite, sep, ansiReset)
		fmt.Printf(" %s ALARME [%s] %s   %s%s%s\n",
			ansiBoldWhite, alarm.Type, ansiReset, ansiBoldWhite, horario, ansiReset)
		fmt.Printf(" Média: %.2f °C  |  %s\n", alarm.Average, alarm.Description)
		fmt.Printf("%s%s%s\n", ansiBoldWhite, sep, ansiReset)
	}

	a.mu.Lock()
	a.state = newState
	a.title = newTitle
	a.detail = newDetail
	a.mu.Unlock()

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "alarm_event", map[string]interface{}{
			"type":        alarm.Type,
			"average":     alarm.Average,
			"description": newDetail,
			"time":        horario,
		})
	}
}

// ResetAlarm restaura o estado para regular via interface ou chamada remota.
func (a *App) ResetAlarm() {
	a.mu.Lock()
	a.state = "regular"
	a.title = "Estado Regular"
	a.detail = "Sistema Estável"
	a.mu.Unlock()

	fmt.Printf("%s %s[ALARMS]%s %sAlarme reconhecido pelo operador. Estado restaurado para REGULAR.%s\n",
		timeStamp(), ansiGray, ansiReset, ansiGreen, ansiReset)

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "alarm_reset", nil)
	}
}

// GetState retorna o estado visual atual para sincronização inicial.
func (a *App) GetState() map[string]string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]string{
		"state":  a.state,
		"title":  a.title,
		"detail": a.detail,
	}
}


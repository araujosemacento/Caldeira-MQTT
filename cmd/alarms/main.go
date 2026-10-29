package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"caldeira-mqtt/pkg/protocol"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Códigos de escape ANSI para formatação semântica no terminal.
const (
	ansiReset      = "\033[0m"
	ansiBold       = "\033[1m"
	ansiGray       = "\033[90m"
	ansiCyan       = "\033[36m"
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

// handleAlarmMessage decodifica e exibe no console os alertas emitidos pelo serviço CAT.
func handleAlarmMessage(_ mqtt.Client, msg mqtt.Message) {
	var alarm protocol.AlarmEvent
	if err := json.Unmarshal(msg.Payload(), &alarm); err != nil {
		fmt.Printf("%s %s[ALARMS]%s Alerta recebido em [%s] (texto bruto): %s\n",
			timeStamp(), ansiBoldWhite, ansiReset, msg.Topic(), string(msg.Payload()))
		return
	}

	horario := time.Unix(alarm.Timestamp, 0).Format("15:04:05")
	const sep = "----------------------------------------------------------"

	switch alarm.Type {
	case "temperatura_alta":
		fmt.Println()
		fmt.Printf("%s%s%s\n", ansiBoldRed, sep, ansiReset)
		fmt.Printf(" %s TEMPERATURA ALTA %s             %s%s%s\n",
			ansiBgRed, ansiReset, ansiBoldWhite, horario, ansiReset)
		fmt.Printf(" Média Crítica: %s%.2f °C%s  |  Limite excedido\n",
			ansiBoldRed, alarm.Average, ansiReset)
		fmt.Printf("%s%s%s\n", ansiBoldRed, sep, ansiReset)
	case "variacao_repentina":
		fmt.Println()
		fmt.Printf("%s%s%s\n", ansiBoldYellow, sep, ansiReset)
		fmt.Printf(" %s VARIAÇÃO REPENTINA %s                    %s%s%s\n",
			ansiBgYellow, ansiReset, ansiBoldWhite, horario, ansiReset)
		fmt.Printf(" Média do Ciclo: %s%.2f °C%s  |  %s\n",
			ansiBoldYellow, alarm.Average, ansiReset, alarm.Description)
		fmt.Printf("%s%s%s\n", ansiBoldYellow, sep, ansiReset)
	default:
		fmt.Println()
		fmt.Printf("%s%s%s\n", ansiBoldWhite, sep, ansiReset)
		fmt.Printf(" %s ALARME [%s] %s   %s%s%s\n",
			ansiBoldWhite, alarm.Type, ansiReset, ansiBoldWhite, horario, ansiReset)
		fmt.Printf(" Média: %.2f °C  |  %s\n", alarm.Average, alarm.Description)
		fmt.Printf("%s%s%s\n", ansiBoldWhite, sep, ansiReset)
	}
}

func main() {
	broker := flag.String("broker", "tcp://localhost:1883", "URL do broker MQTT")
	flag.Parse()

	opts := mqtt.NewClientOptions().
		AddBroker(*broker).
		SetClientID(fmt.Sprintf("alarms-service-%d", time.Now().UnixNano())).
		SetAutoReconnect(true).
		SetKeepAlive(60 * time.Second).
		SetOnConnectHandler(func(client mqtt.Client) {
			subToken := client.Subscribe(protocol.TopicAlarmsWildcard, 1, handleAlarmMessage)
			if subToken.Wait() && subToken.Error() != nil {
				fmt.Printf("%s %s[ALARMS]%s Erro ao assinar tópico de alarmes: %v\n", timeStamp(), ansiBoldRed, ansiReset, subToken.Error())
				return
			}
			fmt.Printf("%s %s[ALARMS]%s Monitorando alarmes no tópico: %s%s%s\n", timeStamp(), ansiGray, ansiReset, ansiCyan, protocol.TopicAlarmsWildcard, ansiReset)
		})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		log.Fatalf("[ALARMS] Erro ao conectar ao broker MQTT: %v", token.Error())
	}
	defer client.Disconnect(250)

	fmt.Printf("%s %s[ALARMS]%s Conectado ao broker MQTT em %s\n", timeStamp(), ansiGray, ansiReset, *broker)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Printf("\n%s %s[ALARMS]%s Encerrando serviço Alarms...\n", timeStamp(), ansiGray, ansiReset)
}

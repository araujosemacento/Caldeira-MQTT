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

// handleAlarmMessage decodifica e exibe no console os alertas emitidos pelo serviço CAT.
func handleAlarmMessage(_ mqtt.Client, msg mqtt.Message) {
	var alarm protocol.AlarmEvent
	if err := json.Unmarshal(msg.Payload(), &alarm); err != nil {
		log.Printf("[ALARMS] Alerta recebido em [%s] (texto bruto): %s", msg.Topic(), string(msg.Payload()))
		return
	}

	horario := time.Unix(alarm.Timestamp, 0).Format("15:04:05")

	fmt.Println("==================================================")
	fmt.Printf("[ALARME DISPARADO] Horário: %s\n", horario)
	fmt.Printf("Tópico: %s\n", msg.Topic())
	fmt.Printf("Tipo: %s\n", alarm.Type)
	fmt.Printf("Média Calculada: %.2f °C\n", alarm.Average)
	fmt.Printf("Descrição: %s\n", alarm.Description)
	fmt.Println("==================================================")
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
				log.Printf("[ALARMS] Erro ao assinar o tópico de alarmes: %v", subToken.Error())
				return
			}
			log.Printf("[ALARMS] Monitorando alarmes no tópico: %s", protocol.TopicAlarmsWildcard)
		})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		log.Fatalf("[ALARMS] Erro ao conectar ao broker MQTT: %v", token.Error())
	}
	defer client.Disconnect(250)

	log.Printf("[ALARMS] Conectado ao broker MQTT em %s", *broker)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("[ALARMS] Encerrando serviço Alarms...")
}

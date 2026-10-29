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

func handleAlarmMessage(_ mqtt.Client, msg mqtt.Message) {
	var alarm protocol.AlarmEvent
	if err := json.Unmarshal(msg.Payload(), &alarm); err != nil {
		log.Printf("[ALARMS] Alerta recebido em [%s] (formato texto puro): %s", msg.Topic(), string(msg.Payload()))
		return
	}

	tempoFormatado := time.Unix(alarm.Timestamp, 0).Format("15:04:05")

	fmt.Println("==================================================")
	fmt.Printf("[ALARME DISPARADO] Horario: %s\n", tempoFormatado)
	fmt.Printf("Topico: %s\n", msg.Topic())
	fmt.Printf("Tipo: %s\n", alarm.Type)
	fmt.Printf("Media Registrada: %.2f C\n", alarm.Average)
	fmt.Printf("Descricao: %s\n", alarm.Description)
	fmt.Println("==================================================")
}

func main() {
	broker := flag.String("broker", "tcp://localhost:1883", "URL do broker MQTT")
	flag.Parse()

	opts := mqtt.NewClientOptions().
		AddBroker(*broker).
		SetClientID("alarms-service-" + fmt.Sprintf("%d", time.Now().Unix())).
		SetAutoReconnect(true).
		SetKeepAlive(60 * time.Second)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		log.Fatalf("[ALARMS] Erro ao conectar ao broker MQTT: %v", token.Error())
	}
	defer client.Disconnect(250)

	log.Printf("[ALARMS] Conectado ao broker MQTT em %s", *broker)

	subToken := client.Subscribe(protocol.TopicAlarmsWildcard, 1, handleAlarmMessage)
	if subToken.Wait() && subToken.Error() != nil {
		log.Fatalf("[ALARMS] Erro ao subscrever no topico de alarmes: %v", subToken.Error())
	}

	log.Printf("[ALARMS] Monitorando alarmes em: %s", protocol.TopicAlarmsWildcard)

	// Aguarda sinal de interrupcao
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("[ALARMS] Encerrando servico Alarms...")
}


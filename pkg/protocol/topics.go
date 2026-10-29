package protocol

// Tópicos MQTT utilizados na comunicação entre sensores, CAT e Alarms.
const (
	// TopicSensorsWildcard é assinado pelo serviço CAT para receber leituras de todos os sensores.
	TopicSensorsWildcard = "caldeira/sensores/temperatura/+"

	// TopicSensorPrefix é o prefixo base para publicação individual de cada sensor.
	TopicSensorPrefix = "caldeira/sensores/temperatura/"

	// TopicAlarmSuddenChange recebe alertas quando a média varia em 5 °C ou mais entre ciclos.
	TopicAlarmSuddenChange = "caldeira/alarmes/variacao_repentina"

	// TopicAlarmHighTemp recebe alertas quando a média ultrapassa 200 °C.
	TopicAlarmHighTemp = "caldeira/alarmes/temperatura_alta"

	// TopicAlarmsWildcard é assinado pelo serviço Alarms para monitorar todos os alertas.
	TopicAlarmsWildcard = "caldeira/alarmes/#"
)

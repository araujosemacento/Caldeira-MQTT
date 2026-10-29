package protocol

// Constantes de topicos MQTT utilizados no sistema de monitoramento da caldeira.
const (
	// TopicSensorsWildcard eh o topico assinado pelo CAT para receber leituras de qualquer sensor.
	TopicSensorsWildcard = "caldeira/sensores/temperatura/+"

	// TopicSensorPrefix eh o prefixo base para o topico de envio de cada sensor individual.
	TopicSensorPrefix = "caldeira/sensores/temperatura/"

	// TopicAlarmSuddenChange eh o topico em que o CAT publica avisos de variacao repentina (>= 5 graus).
	TopicAlarmSuddenChange = "caldeira/alarmes/variacao_repentina"

	// TopicAlarmHighTemp eh o topico em que o CAT publica avisos de temperatura alta (> 200 graus).
	TopicAlarmHighTemp = "caldeira/alarmes/temperatura_alta"

	// TopicAlarmsWildcard eh o topico assinado pelo servico Alarms para ouvir todos os alarmes.
	TopicAlarmsWildcard = "caldeira/alarmes/#"
)


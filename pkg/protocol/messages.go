package protocol

// TemperatureReading encapsula a telemetria enviada pelos sensores simulados.
type TemperatureReading struct {
	SensorID    string  `json:"sensor_id"`
	Timestamp   int64   `json:"timestamp"`
	Temperature float64 `json:"temperatura"`
}

// AlarmEvent encapsula a notificacao de alarme gerada pelo servico CAT.
type AlarmEvent struct {
	Type        string  `json:"tipo"`
	Average     float64 `json:"media_calculada"`
	Description string  `json:"descricao"`
	Timestamp   int64   `json:"timestamp"`
}


package protocol

import (
	"errors"
	"math"
	"strings"
)

// Limites físicos aceitáveis para validação das leituras recebidas via rede.
const (
	MinValidTemperature = -50.0
	MaxValidTemperature = 1000.0
	MaxSensorIDLength   = 64
)

// TemperatureReading representa a leitura das temperatura enviada por um sensor.
type TemperatureReading struct {
	SensorID    string  `json:"sensor_id"`
	Timestamp   int64   `json:"timestamp"`
	Temperature float64 `json:"temperatura"`
}

// Validate verifica se os dados da leitura são íntegros e estão dentro de limites válidos.
func (r TemperatureReading) Validate() error {
	id := strings.TrimSpace(r.SensorID)
	if id == "" || len(id) > MaxSensorIDLength {
		return errors.New("identificador de sensor inválido")
	}
	if math.IsNaN(r.Temperature) || math.IsInf(r.Temperature, 0) {
		return errors.New("valor de temperatura numérico inválido (NaN ou Inf)")
	}
	if r.Temperature < MinValidTemperature || r.Temperature > MaxValidTemperature {
		return errors.New("temperatura fora dos limites operacionais aceitáveis")
	}
	if r.Timestamp <= 0 {
		return errors.New("timestamp inválido")
	}
	return nil
}

// AlarmEvent representa uma notificação de alarme emitida pelo serviço CAT.
type AlarmEvent struct {
	Type        string  `json:"tipo"`
	Average     float64 `json:"media_calculada"`
	Description string  `json:"descricao"`
	Timestamp   int64   `json:"timestamp"`
}

# Caldeira-MQTT

Sistema distribuído de monitoramento térmico industrial baseado no protocolo MQTT, desenvolvido para a atividade AT03 da disciplina de Sistemas Distribuídos.

O ecossistema é formado por cinco módulos: o broker MQTT (Mosquitto), o simulador de sensores, o serviço CAT de cálculo de média, o serviço Alarms e um inspetor de tópicos do broker.

---

## 1. Execução via Binários Pré-compilados (Releases)

Esta é a forma mais rápida de executar o projeto, não exige compiladores ou dependências de desenvolvimento instaladas.

1. Baixe o pacote da versão mais recente na aba [**Releases**](https://github.com/araujosemacento/Caldeira-MQTT/releases/) do repositório:
   - Linux: `caldeira-mqtt-linux-amd64-<tag>.tar.gz`
   - Windows: `caldeira-mqtt-windows-amd64-<tag>.zip`
2. Extraia o conteúdo do pacote para uma pasta de sua preferência e abra o terminal (Bash no Linux ou PowerShell/CMD no Windows) dentro dela.

### 1.1. Inicialização do Broker MQTT

#### Opção A: Utilizando Docker ou Podman (Recomendado)

Se você já possui o Docker ou o Podman instalado na máquina, execute no terminal da pasta extraída:

```bash
# Com Podman:
podman compose up -d

# Ou com Docker:
docker compose up -d
```

O contêiner subirá automaticamente utilizando o arquivo `docker-compose.yml` e as configurações de `config/mosquitto/mosquitto.conf` já inclusos no pacote.

#### Opção B: Sem Gerenciador de Contêineres (Instalação Nativa do Mosquitto)

Caso você não utilize contêineres, é possível instalar e executar o Mosquitto diretamente no sistema operacional como um serviço local:

- **Fedora / RHEL:**

  ```bash
  sudo dnf install -y mosquitto
  mosquitto -c config/mosquitto/mosquitto.conf
  # ou inicie como serviço do sistema:
  # sudo systemctl start mosquitto
  ```

- **Ubuntu / Debian:**

  ```bash
  sudo apt update && sudo apt install -y mosquitto
  mosquitto -c config/mosquitto/mosquitto.conf
  # ou inicie como serviço do sistema:
  # sudo systemctl start mosquitto
  ```

- **Windows:**
  1. Baixe o instalador oficial de 64 bits em [mosquitto.org/download](https://mosquitto.org/download/).
  2. Conclua a instalação padrão (que registra o Mosquitto como serviço do Windows rodando na porta 1883).
  3. Caso prefira executar manualmente pelo terminal apontando para a configuração do pacote:

     ```powershell
     mosquitto -c config\mosquitto\mosquitto.conf
     ```

### 1.2. Execução dos Módulos

Com o broker ativo na porta `127.0.0.1:1883`, abra quatro terminais na pasta extraída e inicie os módulos na seguinte ordem:

#### No Linux

```bash
# Terminal 1: Inspetor de Tópicos (TUI)
./caldeira-inspector

# Terminal 2: Serviço CAT (Processamento da média)
./caldeira-cat

# Terminal 3: Serviço Alarms (GUI)
./caldeira-alarms

# Terminal 4: Simulador de Sensores
./caldeira-sensor
```

#### No Windows

```powershell
# Terminal 1: Inspetor de Tópicos (TUI)
.\caldeira-inspector.exe

# Terminal 2: Serviço CAT (Processamento da média)
.\caldeira-cat.exe

# Terminal 3: Serviço Alarms (GUI)
.\caldeira-alarms.exe

# Terminal 4: Simulador de Sensores
.\caldeira-sensor.exe
```

---

## 2. Execução a partir do Código-Fonte (Clone do Repositório)

Utilize esta abordagem caso queira inspecionar o código, modificar os parâmetros ou compilar uma nova versão.

### 2.1. Clonar o Repositório

```bash
git clone https://github.com/araujosemacento/Caldeira-MQTT.git
cd Caldeira-MQTT
```

### 2.2. Pré-requisitos de Desenvolvimento

- **Go:** Versão 1.25 ou superior.
- **Wails CLI:** Caso deseje compilar ou rodar as GUIs com atualizações em tempo real:

  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

- **Bibliotecas C/GTK (apenas para build do Wails no Linux):**
  - Fedora: `sudo dnf install -y gtk3-devel webkit2gtk4.1-devel`
  - Ubuntu/Debian: `sudo apt install -y libgtk-3-dev libwebkit2gtk-4.1-dev`

### 2.3. Inicializar o Broker MQTT

Suba o broker pela raiz do repositório conforme orientado na seção 1.1 (`podman compose up -d` ou `docker compose up -d`).

### 2.4. Executar os Serviços

Abra quatro terminais na raiz do projeto clonado:

1. **Terminal 1: Inspetor de Tópicos**

   ```bash
   go run ./cmd/inspector
   ```

2. **Terminal 2: Serviço CAT**

   ```bash
   go run ./cmd/cat
   ```

   *É possível alterar a janela de cálculo móvel via flag, ex: `go run ./cmd/cat -window 60s`.*

3. **Terminal 3: Serviço Alarms (GUI)**

   ```bash
   cd cmd/alarms && wails dev
   # ou compile o binário com:
   # cd cmd/alarms && wails build && ./build/bin/caldeira-alarms
   ```

4. **Terminal 4: Simulador de Sensores (GUI)**

   ```bash
   cd cmd/sensor && wails dev
   # ou compile o binário com:
   # cd cmd/sensor && wails build && ./build/bin/caldeira-sensor
   ```

---

## 3. Estrutura do Projeto

```text
.
├── .github/
│   └── workflows/
│       └── release.yml          # Workflow de compilação e release
├── cmd/
│   ├── alarms/                  # Serviço e interface de alarmes
│   │   ├── frontend/            # Frontend (HTML, CSS e JS)
│   │   ├── app.go               # Estado e conexão MQTT
│   │   ├── main.go              # Ponto de entrada Wails
│   │   └── wails.json           # Configuração do Wails
│   ├── cat/                     # Serviço de cálculo de média temporal
│   │   └── main.go              # Lógica de processamento e alertas
│   ├── inspector/               # Inspetor de tópicos do broker
│   │   └── main.go              # Interface TUI de monitoramento
│   └── sensor/                  # Simulador de sensores de temperatura
│       ├── frontend/            # Gráfico cartesiano interativo
│       ├── app.go               # Publicação periódica de leituras
│       ├── main.go              # Ponto de entrada Wails
│       └── wails.json           # Configuração do Wails
├── config/
│   └── mosquitto/
│       └── mosquitto.conf       # Configuração do broker Mosquitto
├── pkg/
│   └── protocol/
│       ├── messages.go          # Estruturas de dados JSON
│       └── topics.go            # Constantes de tópicos MQTT
├── docker-compose.yml           # Definição do contêiner Mosquitto
├── go.mod                       # Módulo Go e dependências
└── README.md                    # Documentação do projeto
```

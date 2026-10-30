const alarmWindow = document.getElementById("alarmWindow");
const iconBox = document.getElementById("iconBox");
const statusTitle = document.getElementById("statusTitle");
const statusDetail = document.getElementById("statusDetail");
const btnReset = document.getElementById("btnReset");

// SVGs oficiais vetorizados
const icons = {
  regular: `
    <svg class="status-icon" viewBox="0 0 1200 1200">
      <path fill="currentColor" d="M600 0C268.629 0 0 268.629 0 600s268.629 600 600 600s600-268.629 600-600S931.371 0 600 0m-18.823 205.884c27.494.001 49.881 7.415 67.163 22.339c17.281 14.926 25.928 34.609 25.928 58.96c0 24.353-8.646 43.962-25.928 58.887c-17.282 14.926-39.669 22.413-67.163 22.412c-27.494.001-49.882-7.486-67.163-22.412c-16.496-14.925-24.756-34.534-24.756-58.887c0-24.351 8.26-44.034 24.756-58.96c17.281-14.925 39.669-22.338 67.163-22.339m85.986 262.72l23.584 27.1c-12.569 32.993-18.896 69.509-18.896 109.57v288.721c0 10.212.846 17.239 2.417 21.167c2.356 3.928 7.452 7.864 15.308 11.792c14.14 8.641 34.908 16.9 62.402 24.756l-8.203 42.407c-67.557-7.07-115.109-10.547-142.603-10.547s-74.587 3.477-141.357 10.547l-9.448-42.407c28.279-6.284 49.121-14.157 62.476-23.584c11.783-7.069 17.651-18.42 17.651-34.131v-294.58c0-27.493-10.65-41.235-31.86-41.235l-41.235 8.203l-9.375-47.095c28.279-11.782 65.567-23.206 111.914-34.204c46.345-10.998 82.087-16.48 107.225-16.48" />
    </svg>
  `,
  warning: `
    <svg class="status-icon" viewBox="0 0 1200 1200">
      <path fill="currentColor" d="M600 0C268.629 0 0 268.629 0 600s268.629 600 600 600s600-268.629 600-600S931.371 0 600 0M381.958 118.213h436.084l-93.75 660.571H475.708zm220.386 705.542c71.262 0 129.053 57.79 129.053 129.053s-57.791 128.979-129.053 128.979s-129.053-57.717-129.053-128.979s57.791-129.053 129.053-129.053" />
    </svg>
  `,
  danger: `
    <svg class="status-icon" viewBox="0 0 1200 1200">
      <path fill="currentColor" d="M381.64 1200C135.779 1061.434 71.049 930.278 108.057 751.148c27.321-132.271 116.782-239.886 125.36-371.903c38.215 69.544 54.183 119.691 58.453 192.364C413.413 422.695 493.731 216.546 498.487 0c0 0 316.575 186.01 337.348 466.98c27.253-57.913 40.972-149.892 13.719-209.504c81.757 59.615 560.293 588.838-64.818 942.524c117.527-228.838 30.32-537.611-173.739-680.218c13.628 61.319-10.265 290.021-100.542 390.515c25.014-167.916-23.8-238.918-23.8-238.918s-16.754 94.054-81.758 189.065c-59.36 86.762-100.49 178.847-23.257 339.556" />
    </svg>
  `
};

function applyState(state, title, detail) {
  alarmWindow.className = "alarm-window state-" + state;
  iconBox.innerHTML = icons[state] || icons.regular;
  statusTitle.textContent = title;
  statusDetail.textContent = detail;
}

// Ação de reconhecimento/reset
btnReset.addEventListener("click", () => {
  if (window.go?.main?.App?.ResetAlarm) {
    window.go.main.App.ResetAlarm();
  } else {
    applyState("regular", "Estado Regular", "Sistema Estável");
  }
});

// Eventos enviados pelo backend Go via Wails
if (window.runtime?.EventsOn) {
  window.runtime.EventsOn("alarm_event", (data) => {
    if (data.type === "temperatura_alta") {
      const avg = typeof data.average === "number" ? data.average.toFixed(2) : data.average;
      applyState("danger", "Temperatura Alta", "Média crítica: " + avg + " °C");
    } else if (data.type === "variacao_repentina") {
      const desc = data.description || ("Média: " + (typeof data.average === "number" ? data.average.toFixed(2) : data.average) + " °C");
      applyState("warning", "Variação Repentina", desc);
    } else {
      applyState("warning", "Alarme Detectado", data.type);
    }
  });

  window.runtime.EventsOn("alarm_reset", () => {
    applyState("regular", "Estado Regular", "Sistema Estável");
  });
}

// Sincroniza estado inicial ao abrir a janela
if (window.go?.main?.App?.GetState) {
  window.go.main.App.GetState().then((res) => {
    if (res && res.state) {
      applyState(res.state, res.title, res.detail);
    }
  }).catch(() => { });
}


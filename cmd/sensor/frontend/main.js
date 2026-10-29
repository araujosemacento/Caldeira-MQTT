// Limites da escala de temperatura e coordenadas correspondentes no SVG
const Y_MIN = 150;
const Y_MAX = 210;
const Y_BOTTOM = 270; // Posição correspondente a 150 °C
const Y_TOP = 30;     // Posição correspondente a 210 °C
const X_NOW = 500;
const X_START = 50;

// Quantidade de sensores ativos (mínimo: 1, máximo: 3)
let sensorCount = 2;

let s1 = 180;
let s2 = 165;
let s3 = 175;

let hist1 = [162, 166, 170, 174, 176, 178, 180];
let hist2 = [154, 156, 158, 160, 162, 164, 165];
let hist3 = [170, 171, 172, 173, 174, 174, 175];

const btnAdd = document.getElementById("btn-add");
const btnRemove = document.getElementById("btn-remove");

const knob1 = document.getElementById("knob-s1");
const knob2 = document.getElementById("knob-s2");
const knob3 = document.getElementById("knob-s3");

const path1 = document.getElementById("path-s1");
const path2 = document.getElementById("path-s2");
const path3 = document.getElementById("path-s3");

const chart = document.getElementById("chart");

// Converte temperatura (°C) para coordenada vertical no SVG
function toY(temp) {
  const t = Math.max(Y_MIN, Math.min(Y_MAX, temp));
  const ratio = (t - Y_MIN) / (Y_MAX - Y_MIN);
  return Y_BOTTOM - ratio * (Y_BOTTOM - Y_TOP);
}

// Converte coordenada vertical do SVG para temperatura (°C)
function toTemp(y) {
  const clamped = Math.max(Y_TOP, Math.min(Y_BOTTOM, y));
  const ratio = (Y_BOTTOM - clamped) / (Y_BOTTOM - Y_TOP);
  return Math.round(Y_MIN + ratio * (Y_MAX - Y_MIN));
}

// Atualiza a visibilidade das curvas, controles e estado dos botões
function updateSensorVisibility() {
  path1.style.display = sensorCount >= 1 ? "block" : "none";
  knob1.style.display = sensorCount >= 1 ? "block" : "none";

  path2.style.display = sensorCount >= 2 ? "block" : "none";
  knob2.style.display = sensorCount >= 2 ? "block" : "none";

  path3.style.display = sensorCount >= 3 ? "block" : "none";
  knob3.style.display = sensorCount >= 3 ? "block" : "none";

  btnAdd.disabled = sensorCount >= 3;
  btnRemove.disabled = sensorCount <= 1;
}

// Redesenha as linhas de histórico e atualiza a posição dos controles ativos
function draw() {
  if (sensorCount >= 1) {
    const step1 = (X_NOW - X_START) / hist1.length;
    let d1 = "";
    hist1.forEach((t, i) => {
      d1 += (i === 0 ? `M ${X_START + i * step1} ${toY(t)}` : ` L ${X_START + i * step1} ${toY(t)}`);
    });
    d1 += ` L ${X_NOW} ${toY(s1)}`;
    path1.setAttribute("d", d1);
    knob1.setAttribute("cy", toY(s1));
  }

  if (sensorCount >= 2) {
    const step2 = (X_NOW - X_START) / hist2.length;
    let d2 = "";
    hist2.forEach((t, i) => {
      d2 += (i === 0 ? `M ${X_START + i * step2} ${toY(t)}` : ` L ${X_START + i * step2} ${toY(t)}`);
    });
    d2 += ` L ${X_NOW} ${toY(s2)}`;
    path2.setAttribute("d", d2);
    knob2.setAttribute("cy", toY(s2));
  }

  if (sensorCount >= 3) {
    const step3 = (X_NOW - X_START) / hist3.length;
    let d3 = "";
    hist3.forEach((t, i) => {
      d3 += (i === 0 ? `M ${X_START + i * step3} ${toY(t)}` : ` L ${X_START + i * step3} ${toY(t)}`);
    });
    d3 += ` L ${X_NOW} ${toY(s3)}`;
    path3.setAttribute("d", d3);
    knob3.setAttribute("cy", toY(s3));
  }
}

// Ações dos botões de adicionar e remover sensores
btnAdd.addEventListener("click", () => {
  if (sensorCount < 3) {
    sensorCount++;
    if (window.go?.main?.App?.SetSensorCount) {
      window.go.main.App.SetSensorCount(sensorCount);
    }
    updateSensorVisibility();
    draw();
  }
});

btnRemove.addEventListener("click", () => {
  if (sensorCount > 1) {
    sensorCount--;
    if (window.go?.main?.App?.SetSensorCount) {
      window.go.main.App.SetSensorCount(sensorCount);
    }
    updateSensorVisibility();
    draw();
  }
});

let dragging = null;

function onDown(sensor, e) {
  if (sensor > sensorCount) return;
  dragging = sensor;
  e.target.setPointerCapture(e.pointerId);
}

function onMove(e) {
  if (!dragging) return;
  const rect = chart.getBoundingClientRect();
  const y = (e.clientY - rect.top) * (300 / rect.height);
  const temp = toTemp(y);

  if (dragging === 1) {
    s1 = temp;
    if (window.go?.main?.App?.SetTemperature) window.go.main.App.SetTemperature("s1", s1);
  } else if (dragging === 2) {
    s2 = temp;
    if (window.go?.main?.App?.SetTemperature) window.go.main.App.SetTemperature("s2", s2);
  } else if (dragging === 3) {
    s3 = temp;
    if (window.go?.main?.App?.SetTemperature) window.go.main.App.SetTemperature("s3", s3);
  }
  draw();
}

function onUp() {
  dragging = null;
}

knob1.addEventListener("pointerdown", (e) => onDown(1, e));
knob2.addEventListener("pointerdown", (e) => onDown(2, e));
knob3.addEventListener("pointerdown", (e) => onDown(3, e));

chart.addEventListener("pointermove", onMove);
chart.addEventListener("pointerup", onUp);
chart.addEventListener("pointercancel", onUp);

// Avança o histórico a cada ciclo de leitura
function tick(data) {
  const t1 = typeof data?.s1 === "number" ? data.s1 : s1;
  const t2 = typeof data?.s2 === "number" ? data.s2 : s2;
  const t3 = typeof data?.s3 === "number" ? data.s3 : s3;

  if (sensorCount >= 1) {
    hist1.push(t1);
    if (hist1.length > 14) hist1.shift();
  }
  if (sensorCount >= 2) {
    hist2.push(t2);
    if (hist2.length > 14) hist2.shift();
  }
  if (sensorCount >= 3) {
    hist3.push(t3);
    if (hist3.length > 14) hist3.shift();
  }

  draw();
}

if (window.runtime?.EventsOn) {
  window.runtime.EventsOn("cycle_tick", (data) => tick(data));
} else {
  setInterval(tick, 3000);
}

updateSensorVisibility();
draw();

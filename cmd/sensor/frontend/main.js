// Limites da escala de temperatura e coordenadas correspondentes no SVG
const Y_MIN = 150;
const Y_MAX = 210;
const Y_BOTTOM = 270; // Posição correspondente a 150 °C
const Y_TOP = 30;     // Posição correspondente a 210 °C
const X_NOW = 500;
const X_START = 50;

let s1 = 180;
let s2 = 165;

let hist1 = [162, 166, 170, 174, 176, 178, 180];
let hist2 = [154, 156, 158, 160, 162, 164, 165];

const knob1 = document.getElementById("knob-s1");
const knob2 = document.getElementById("knob-s2");
const path1 = document.getElementById("path-s1");
const path2 = document.getElementById("path-s2");
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

// Redesenha as linhas de histórico e atualiza a posição dos controles
function draw() {
  const step = (X_NOW - X_START) / hist1.length;

  let d1 = "";
  hist1.forEach((t, i) => {
    d1 += (i === 0 ? `M ${X_START + i * step} ${toY(t)}` : ` L ${X_START + i * step} ${toY(t)}`);
  });
  d1 += ` L ${X_NOW} ${toY(s1)}`;
  path1.setAttribute("d", d1);

  let d2 = "";
  hist2.forEach((t, i) => {
    d2 += (i === 0 ? `M ${X_START + i * step} ${toY(t)}` : ` L ${X_START + i * step} ${toY(t)}`);
  });
  d2 += ` L ${X_NOW} ${toY(s2)}`;
  path2.setAttribute("d", d2);

  knob1.setAttribute("cy", toY(s1));
  knob2.setAttribute("cy", toY(s2));
}

let dragging = null;

function onDown(sensor, e) {
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
  } else {
    s2 = temp;
    if (window.go?.main?.App?.SetTemperature) window.go.main.App.SetTemperature("s2", s2);
  }
  draw();
}

function onUp() {
  dragging = null;
}

knob1.addEventListener("pointerdown", (e) => onDown(1, e));
knob2.addEventListener("pointerdown", (e) => onDown(2, e));
chart.addEventListener("pointermove", onMove);
chart.addEventListener("pointerup", onUp);
chart.addEventListener("pointercancel", onUp);

// Avança o histórico a cada ciclo de leitura
function tick(t1, t2) {
  hist1.push(typeof t1 === "number" ? t1 : s1);
  hist2.push(typeof t2 === "number" ? t2 : s2);

  if (hist1.length > 14) hist1.shift();
  if (hist2.length > 14) hist2.shift();

  draw();
}

if (window.runtime?.EventsOn) {
  window.runtime.EventsOn("cycle_tick", (data) => tick(data.s1, data.s2));
} else {
  setInterval(tick, 3000);
}

draw();

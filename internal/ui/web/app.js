const $ = (id) => document.getElementById(id);

let state = null;
let menuOpen = false;
let proxyMenuOpen = false;
let lastStratListKey = "";
let lastGameStratListKey = "";
let lastMenuKey = "";
let enterTimer = 0;
let statusAnimTimer = 0;
let lastStatusTitle = "";
let lastStatusSub = "";

const statusMap = {
  running: ["Работает", "on"],
  starting: ["Запускается", "on"],
  stopping: ["Останавливается", "off"],
  stopped: ["Выключен", "off"],
  missing: ["Не установлен", "miss"],
  unknown: ["Неизвестно", "miss"],
};

async function call(name, ...args) {
  if (typeof window[name] !== "function") {
    throw new Error("bridge " + name + " is missing");
  }
  return window[name](...args);
}

function setText(el, value) {
  const next = value == null ? "" : String(value);
  if (el.textContent !== next) el.textContent = next;
}

function setClass(el, value) {
  if (el.className !== value) el.className = value;
}

function toggle(el, cls, on) {
  el.classList.toggle(cls, Boolean(on));
}

let refreshing = false;

async function refresh() {
  if (refreshing) return;
  refreshing = true;
  try {
    state = await call("getState");
    render();
  } catch (e) {
    setText($("hint"), String(e));
    toggle($("hint"), "err", true);
    reportError(e);
  } finally {
    refreshing = false;
  }
}

function animateStatus(title, cls, sub) {
  const titleEl = $("statusTitle");
  const subEl = $("statusSub");
  const titleChanged = title !== lastStatusTitle;
  const subChanged = sub !== lastStatusSub;

  setClass($("statusDot"), "dot " + cls);

  if (!titleChanged) {
    setClass(titleEl, "status-title " + cls);
  } else if (!lastStatusTitle) {
    setText(titleEl, title);
    setClass(titleEl, "status-title " + cls);
    titleEl.classList.add("swap-in");
  } else {
    clearTimeout(statusAnimTimer);
    titleEl.classList.remove("swap-in");
    titleEl.classList.add("swap-out");
    statusAnimTimer = setTimeout(() => {
      setText(titleEl, title);
      setClass(titleEl, "status-title " + cls + " swap-in");
    }, 160);
  }

  if (subChanged) {
    setText(subEl, sub);
    subEl.classList.remove("swap");
    void subEl.offsetWidth;
    subEl.classList.add("swap");
  }

  lastStatusTitle = title;
  lastStatusSub = sub;
}

function render() {
  if (!state) return;
  const svc = state.service || {};
  const key = svc.status || (state.installed ? "stopped" : "missing");
  const [title, cls] = statusMap[key] || statusMap.unknown;
  const running = key === "running" || key === "starting";
  const busy = Boolean(state.busy);
  const sub = svc.strategy || (state.installed ? "готова к включению" : "ожидание сборки");

  animateStatus(title, cls, sub);

  setText($("versionLabel"), state.versionLabel || "Актуальная");
  const shown = state.followLatest
    ? (state.latestVersion || state.localVersion)
    : (state.targetVersion || state.localVersion);
  setText($("versionNum"), shown || "запрос к GitHub…");

  const showProgress = busy && (state.progress || 0) > 0;
  toggle($("progressWrap"), "hidden", !showProgress);
  const width = Math.round((state.progress || 0) * 100) + "%";
  if ($("progressBar").style.width !== width) $("progressBar").style.width = width;

  setText($("hint"), state.error || state.message || "");
  toggle($("hint"), "err", Boolean(state.error));

  document.querySelectorAll("#gameFilter button").forEach((b) => {
    toggle(b, "active", b.dataset.mode === (state.gameFilter?.mode || "off"));
    b.disabled = !state.installed || busy;
  });

  const dnsID = state.dnsProfile || "cloudflare";
  document.querySelectorAll("#dnsProfile button").forEach((b) => {
    toggle(b, "active", b.dataset.id === dnsID);
    b.disabled = false;
  });

  renderServiceBoosts(busy);
  renderGeoProxy();

  renderMenu();
  renderStrategies(svc);
  renderGameStrategies(busy);

  const power = $("btnPower");
  power.disabled = busy || !state.installed;
  setText(power, running ? "Выключить" : "Включить");
  setClass(power, "power " + (running ? "off" : "on"));
  $("btnRemove").disabled = busy || key === "missing";
  toggle($("btnRemove"), "hidden", !state.installed || key === "missing");
  $("versionBtn").disabled = busy;
  const tg = $("tgBoost");
  const tgOn = state.telegramWebBoost !== false;
  toggle(tg, "on", tgOn);
  tg.setAttribute("aria-pressed", tgOn ? "true" : "false");
  tg.disabled = busy;
  const gb = $("gameBoost");
  const gbOn = state.gameBoost !== false;
  toggle(gb, "on", gbOn);
  gb.setAttribute("aria-pressed", gbOn ? "true" : "false");
  gb.disabled = busy;
  toggle($("gameStratCard"), "dimmed", !gbOn);
  toggle($("versionBtn"), "open", menuOpen);
  $("proxyBtn").disabled = busy;
  toggle($("proxyBtn"), "open", proxyMenuOpen);
}

function renderGeoProxy() {
  const proxy = state.geoProxy || {};
  setText($("proxyName"), proxy.name || "GeoHide");
  setText($("proxyIP"), proxy.ip || "");
  toggle($("proxyMenu"), "hidden", !proxyMenuOpen);
}

function formatPing(item) {
  if (item.latencyMs == null || item.latencyMs < 0) return "нет";
  return item.latencyMs + " мс";
}

async function openProxyMenu() {
  proxyMenuOpen = true;
  toggle($("proxyMenu"), "hidden", false);
  toggle($("proxyBtn"), "open", true);
  const list = $("proxyMenuList");
  const selected = state?.geoProxy?.ip || "";
  list.innerHTML = `<button type="button" class="checking" disabled><span>Проверка…</span><span class="ping">…</span></button>`;
  try {
    const items = await call("pingGeoProxies");
    list.innerHTML = (items || [])
      .map((item) => {
        const cls = [
          item.ok ? "ok" : "bad",
          item.ip === selected ? "active" : "",
        ]
          .filter(Boolean)
          .join(" ");
        return `<button type="button" class="${cls}" data-ip="${escapeAttr(item.ip)}"><span><strong>${escapeHtml(item.name)}</strong><small class="meta">${escapeHtml(item.ip)}</small></span><span class="ping">${escapeHtml(formatPing(item))}</span></button>`;
      })
      .join("");
    list.querySelectorAll("button[data-ip]").forEach((btn) => {
      btn.onclick = async (e) => {
        e.stopPropagation();
        closeProxyMenu();
        try {
          state = await call("setGeoProxy", btn.dataset.ip);
          render();
        } catch (err) {
          setText($("hint"), String(err));
          toggle($("hint"), "err", true);
          await refresh();
        }
      };
    });
  } catch (err) {
    list.innerHTML = `<button type="button" class="bad" disabled><span>${escapeHtml(String(err))}</span><span class="ping">нет</span></button>`;
  }
}

function closeProxyMenu() {
  if (!proxyMenuOpen) return;
  proxyMenuOpen = false;
  toggle($("proxyMenu"), "hidden", true);
  toggle($("proxyBtn"), "open", false);
}

function renderServiceBoosts(busy) {
  const box = $("serviceBoosts");
  if (!box) return;
  const items = state.serviceBoosts || [];
  const key = items.map((i) => i.id + ":" + (i.enabled ? "1" : "0")).join("|") + "|" + (busy ? "1" : "0");
  if (box.dataset.key === key) {
    box.querySelectorAll(".toggle-row").forEach((btn) => {
      btn.disabled = busy;
    });
    return;
  }
  box.dataset.key = key;
  if (!items.length) {
    box.innerHTML = "";
    return;
  }
  box.innerHTML = items
    .map(
      (item) =>
        `<button type="button" class="toggle-row ${item.enabled ? "on" : ""}" data-id="${escapeAttr(item.id)}" aria-pressed="${item.enabled ? "true" : "false"}"${busy ? " disabled" : ""}><span class="toggle-label">${escapeHtml(item.title)}</span><span class="switch" aria-hidden="true"><i></i></span></button>`
    )
    .join("");
  box.querySelectorAll(".toggle-row").forEach((btn) => {
    btn.onclick = async (e) => {
      e.stopPropagation();
      const next = !btn.classList.contains("on");
      toggle(btn, "on", next);
      btn.setAttribute("aria-pressed", next ? "true" : "false");
      try {
        state = await call("setServiceBoost", btn.dataset.id, next);
        render();
      } catch (err) {
        setText($("hint"), String(err));
        toggle($("hint"), "err", true);
        await refresh();
      }
    };
  });
}

function renderMenu() {
  const menu = $("versionMenu");
  const list = $("versionMenuList") || menu;
  toggle(menu, "hidden", !menuOpen);
  const releases = state.releases || [];
  const latest = state.latestVersion || "";
  const key = [state.channel, state.followLatest, latest, JSON.stringify(releases)].join("|");
  if (key === lastMenuKey) {
    list.querySelectorAll("button").forEach((btn) => {
      const active = btn.dataset.channel === "latest" ? state.followLatest : !state.followLatest && state.channel === btn.dataset.channel;
      toggle(btn, "active", active);
    });
    return;
  }
  lastMenuKey = key;
  const items = [
    { channel: "latest", title: "Актуальная", meta: latest, active: state.followLatest },
    ...releases.map((r) => ({
      channel: r.version,
      title: r.version,
      meta: r.publishedAt || "",
      active: !state.followLatest && state.channel === r.version,
    })),
  ];
  list.innerHTML = items
    .map(
      (item) =>
        `<button type="button" class="${item.active ? "active" : ""}" data-channel="${escapeAttr(item.channel)}"><span>${escapeHtml(item.title)}</span><span class="meta">${escapeHtml(item.meta)}</span></button>`
    )
    .join("");
  list.querySelectorAll("button").forEach((btn) => {
    btn.onclick = () => chooseVersion(btn.dataset.channel);
  });
}

function syncChipClasses(svc) {
  const running = svc.status === "running" || svc.status === "starting";
  $("strategies").querySelectorAll(".chip[data-name]").forEach((el) => {
    toggle(el, "selected", el.dataset.name === state.selected);
    toggle(el, "current", running && el.dataset.name === svc.strategy);
  });
}

function renderStrategies(svc) {
  const box = $("strategies");
  const items = state.strategies || [];
  const listKey = [state.installed, items.map((s) => s.name).join("\n")].join("|");

  if (listKey === lastStratListKey) {
    syncChipClasses(svc);
    return;
  }
  lastStratListKey = listKey;

  if (!state.installed) {
    box.innerHTML = `<span class="chip empty">Сборка ещё не загружена</span>`;
    return;
  }
  if (!items.length) {
    box.innerHTML = `<span class="chip empty">Нет стратегий</span>`;
    return;
  }

  box.innerHTML = items
    .map((s, i) => {
      const label = s.shortName || s.name;
      return `<button type="button" class="chip" data-name="${escapeAttr(s.name)}" style="animation-delay:${Math.min(i, 12) * 18}ms">${escapeHtml(label)}</button>`;
    })
    .join("");
  box.querySelectorAll(".chip[data-name]").forEach((el) => {
    el.onclick = () => select(el.dataset.name);
  });
  syncChipClasses(svc);

  box.classList.add("entering");
  clearTimeout(enterTimer);
  enterTimer = setTimeout(() => box.classList.remove("entering"), 450);
}

let selectSeq = 0;

async function select(name) {
  if (!state || state.selected === name) return;
  state.selected = name;
  syncChipClasses(state.service || {});
  const seq = ++selectSeq;
  try {
    const next = await call("selectStrategy", name);
    if (seq !== selectSeq) return;
    state = next;
    render();
  } catch (e) {
    if (seq !== selectSeq) return;
    setText($("hint"), String(e));
    toggle($("hint"), "err", true);
  }
}

function renderGameStrategies(busy) {
  const box = $("gameStrategies");
  const items = state.gameStrategies || [];
  const selected = new Set(state.selectedGames || []);
  const selKey = [...selected].sort().join(",");
  const listKey = items.map((s) => s.id + ":" + s.name).join("|") + "|" + selKey + "|" + (busy ? "1" : "0");
  if (listKey === lastGameStratListKey) {
    box.querySelectorAll(".chip[data-id]").forEach((el) => {
      toggle(el, "selected", selected.has(el.dataset.id));
      el.disabled = busy;
    });
    return;
  }
  lastGameStratListKey = listKey;
  if (!items.length) {
    box.innerHTML = `<span class="chip empty">Нет игровых стратегий</span>`;
    return;
  }
  box.innerHTML = items
    .map((s, i) => {
      const cls = selected.has(s.id) ? "chip selected" : "chip";
      return `<button type="button" class="${cls}" data-id="${escapeAttr(s.id)}" style="animation-delay:${Math.min(i, 12) * 18}ms"${busy ? " disabled" : ""}>${escapeHtml(s.name)}</button>`;
    })
    .join("");
  box.querySelectorAll(".chip[data-id]").forEach((el) => {
    el.onclick = () => selectGame(el.dataset.id);
  });
}

let selectGameSeq = 0;

async function selectGame(id) {
  if (!state) return;
  const selected = new Set(state.selectedGames || []);
  if (selected.has(id)) selected.delete(id);
  else selected.add(id);
  state.selectedGames = [...selected];
  renderGameStrategies(Boolean(state.busy));
  const seq = ++selectGameSeq;
  try {
    const next = await call("selectGameStrategy", id);
    if (seq !== selectGameSeq) return;
    state = next;
    render();
  } catch (e) {
    if (seq !== selectGameSeq) return;
    setText($("hint"), String(e));
    toggle($("hint"), "err", true);
    await refresh();
  }
}

async function chooseVersion(channel) {
  menuOpen = false;
  toggle($("versionMenu"), "hidden", true);
  toggle($("versionBtn"), "open", false);
  await run(() => call("selectVersion", channel));
}

async function run(fn) {
  try {
    state = await fn();
    render();
  } catch (e) {
    setText($("hint"), String(e));
    toggle($("hint"), "err", true);
    await refresh();
  }
}

function escapeHtml(s) {
  return String(s)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function escapeAttr(s) {
  return escapeHtml(s).replaceAll('"', "&quot;");
}

$("versionBtn").onclick = async (e) => {
  e.stopPropagation();
  menuOpen = !menuOpen;
  toggle($("versionMenu"), "hidden", !menuOpen);
  toggle($("versionBtn"), "open", menuOpen);
  if (!menuOpen) return;
  try {
    if (typeof loadReleases === "function") {
      state = await loadReleases();
    }
  } catch (_) {}
  renderMenu();
};

async function openExternal(url) {
  try {
    await call("openURL", url);
  } catch (err) {
    setText($("hint"), String(err));
    toggle($("hint"), "err", true);
  }
}

$("byLink").onclick = async (e) => {
  e.preventDefault();
  e.stopPropagation();
  await openExternal($("byLink").href);
};

document.addEventListener("click", () => {
  closeMenu();
  closeProxyMenu();
});

$("versionMenu").addEventListener("click", (e) => e.stopPropagation());
$("proxyMenu").addEventListener("click", (e) => e.stopPropagation());

$("proxyBtn").onclick = async (e) => {
  e.stopPropagation();
  closeMenu();
  if (proxyMenuOpen) {
    closeProxyMenu();
    return;
  }
  await openProxyMenu();
};

$("btnPower").onclick = () => {
  const running = state?.service?.status === "running" || state?.service?.status === "starting";
  run(() => call(running ? "stopZapret" : "startZapret"));
};

$("btnRemove").onclick = () => {
  run(() => call("removeZapret"));
};

$("btnExtra").onclick = (e) => {
  e.stopPropagation();
  closeMenu();
  closeProxyMenu();
  toggle($("overlayExtra"), "hidden", false);
};

$("btnHelp").onclick = (e) => {
  e.stopPropagation();
  closeMenu();
  closeProxyMenu();
  toggle($("overlayHelp"), "hidden", false);
};

$("tgBoost").onclick = async (e) => {
  e.stopPropagation();
  closeMenu();
  const next = !($("tgBoost").classList.contains("on"));
  toggle($("tgBoost"), "on", next);
  $("tgBoost").setAttribute("aria-pressed", next ? "true" : "false");
  try {
    state = await call("setTelegramWebBoost", next);
    render();
  } catch (err) {
    setText($("hint"), String(err));
    toggle($("hint"), "err", true);
    await refresh();
  }
};

$("gameBoost").onclick = async (e) => {
  e.stopPropagation();
  closeMenu();
  const next = !($("gameBoost").classList.contains("on"));
  toggle($("gameBoost"), "on", next);
  $("gameBoost").setAttribute("aria-pressed", next ? "true" : "false");
  toggle($("gameStratCard"), "dimmed", !next);
  try {
    state = await call("setGameBoost", next);
    render();
  } catch (err) {
    setText($("hint"), String(err));
    toggle($("hint"), "err", true);
    await refresh();
  }
};

$("closeExtra").onclick = () => toggle($("overlayExtra"), "hidden", true);
$("overlayExtra").onclick = (e) => {
  if (e.target === $("overlayExtra")) toggle($("overlayExtra"), "hidden", true);
};

$("btnShowLogs").onclick = async (e) => {
  e.stopPropagation();
  toggle($("overlayExtra"), "hidden", true);
  toggle($("overlayLogs"), "hidden", false);
  await renderLogs();
};

$("closeLogs").onclick = () => toggle($("overlayLogs"), "hidden", true);
$("overlayLogs").onclick = (e) => {
  if (e.target === $("overlayLogs")) toggle($("overlayLogs"), "hidden", true);
};
$("btnClearLogs").onclick = async (e) => {
  e.stopPropagation();
  try {
    await call("clearLogs");
    await renderLogs();
  } catch (err) {
    setText($("hint"), String(err));
    toggle($("hint"), "err", true);
    reportError(err);
  }
};

$("closeHelp").onclick = () => toggle($("overlayHelp"), "hidden", true);
$("overlayHelp").onclick = (e) => {
  if (e.target === $("overlayHelp")) toggle($("overlayHelp"), "hidden", true);
};

$("btnHelpTelegram").onclick = async (e) => {
  e.stopPropagation();
  await openExternal("https://t.me/SizLovesSlugs");
};

document.querySelectorAll(".help-link").forEach((el) => {
  el.onclick = async (e) => {
    e.preventDefault();
    e.stopPropagation();
    await openExternal(el.dataset.url || el.href);
  };
});

document.addEventListener("keydown", (e) => {
  if (e.key !== "Escape") return;
  if (proxyMenuOpen) {
    closeProxyMenu();
    return;
  }
  if (!$("overlayLogs").classList.contains("hidden")) {
    toggle($("overlayLogs"), "hidden", true);
    return;
  }
  if (!$("overlayHelp").classList.contains("hidden")) {
    toggle($("overlayHelp"), "hidden", true);
    return;
  }
  if (!$("overlayExtra").classList.contains("hidden")) {
    toggle($("overlayExtra"), "hidden", true);
  }
});

function closeMenu() {
  if (!menuOpen) return;
  menuOpen = false;
  toggle($("versionMenu"), "hidden", true);
  toggle($("versionBtn"), "open", false);
}

function formatLogTime(value) {
  const raw = String(value || "");
  const m = raw.match(/^(\d{4})-(\d{2})-(\d{2}) (\d{2}:\d{2}:\d{2})$/);
  if (!m) return raw;
  return m[3] + "." + m[2] + "." + m[1] + " · " + m[4];
}

async function copyText(text) {
  const value = String(text || "");
  if (!value) return false;
  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(value);
      return true;
    }
  } catch (_) {}
  const area = document.createElement("textarea");
  area.value = value;
  area.setAttribute("readonly", "");
  area.style.position = "fixed";
  area.style.opacity = "0";
  document.body.appendChild(area);
  area.select();
  let ok = false;
  try {
    ok = document.execCommand("copy");
  } catch (_) {}
  document.body.removeChild(area);
  return ok;
}

async function renderLogs() {
  const box = $("logList");
  box.innerHTML = `<p class="log-empty">Загрузка…</p>`;
  try {
    const data = await call("getLogs");
    const entries = data && data.entries ? data.entries : [];
    const path = data && data.path ? data.path : "";
    setText($("logsPath"), path);
    if (!entries.length) {
      box.innerHTML = `<p class="log-empty">Ошибок пока нет</p>`;
      return;
    }
    box.innerHTML = entries
      .map((item, i) => {
        const time = formatLogTime(item.time);
        const msg = item.message || "";
        const full = time + "\n" + msg;
        return `<article class="log-item" style="animation-delay:${Math.min(i, 10) * 24}ms"><p class="log-time">${escapeHtml(time)}</p><p class="log-msg">${escapeHtml(msg)}</p><button type="button" class="log-copy" data-copy="${escapeAttr(full)}" title="Скопировать" aria-label="Скопировать">⧉</button></article>`;
      })
      .join("");
    box.querySelectorAll(".log-copy").forEach((btn) => {
      btn.onclick = async (e) => {
        e.stopPropagation();
        const ok = await copyText(btn.dataset.copy || "");
        if (!ok) return;
        btn.classList.add("done");
        btn.textContent = "✓";
        setTimeout(() => {
          btn.classList.remove("done");
          btn.textContent = "⧉";
        }, 1200);
      };
    });
  } catch (e) {
    box.innerHTML = `<p class="log-empty">${escapeHtml(String(e))}</p>`;
    reportError(e);
  }
}

let lastReported = "";

function reportError(err) {
  const msg = String(err || "").trim();
  if (!msg || msg === lastReported) return;
  lastReported = msg;
  try {
    if (typeof logError === "function") logError(msg);
  } catch (_) {}
}

document.querySelectorAll("#gameFilter button").forEach((b) => {
  b.onclick = async () => {
    document.querySelectorAll("#gameFilter button").forEach((x) => toggle(x, "active", x === b));
    try {
      state = await call("setGameFilter", b.dataset.mode);
      render();
    } catch (e) {
      setText($("hint"), String(e));
      toggle($("hint"), "err", true);
    }
  };
});

document.querySelectorAll("#dnsProfile button").forEach((b) => {
  b.onclick = async () => {
    document.querySelectorAll("#dnsProfile button").forEach((x) => toggle(x, "active", x === b));
    try {
      state = await call("setDNSProfile", b.dataset.id);
      render();
    } catch (e) {
      setText($("hint"), String(e));
      toggle($("hint"), "err", true);
      await refresh();
    }
  };
});

refresh();
setInterval(() => {
  if (!document.hidden) refresh();
}, 2500);

(function watchAppUpdate() {
  let shown = false;
  async function tick() {
    if (shown) return;
    try {
      const st = await call("getState");
      if (st && st.appUpdateReady && !st.busy) {
        shown = true;
        state = st;
        const el = $("overlayAppUpdate");
        if (el) el.classList.remove("hidden");
        setTimeout(async () => {
          try {
            await call("applyAppUpdate");
          } catch (_) {
            if (el) el.classList.add("hidden");
          }
        }, 2000);
        return;
      }
    } catch (_) {}
    setTimeout(tick, 1200);
  }
  setTimeout(tick, 5000);
})();

const loginView = document.querySelector("#loginView");
const appView = document.querySelector("#appView");
const loginForm = document.querySelector("#loginForm");
const loginError = document.querySelector("#loginError");
const createForm = document.querySelector("#createForm");
const createGroupForm = document.querySelector("#createGroupForm");
const serviceList = document.querySelector("#serviceList");
const serviceGroupList = document.querySelector("#serviceGroupList");
const serviceGroupMembers = document.querySelector("#serviceGroupMembers");
const oneTimeCode = document.querySelector("#oneTimeCode");
const toast = document.querySelector("#toast");
const template = document.querySelector("#serviceTemplate");
const groupTemplate = document.querySelector("#serviceGroupTemplate");

let servicesCache = [];

loginForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  loginError.textContent = "";
  const form = new FormData(loginForm);
  try {
    await api("/api/admin/login", {
      method: "POST",
      body: {
        username: form.get("username"),
        password: form.get("password"),
      },
    });
    loginForm.reset();
    await showApp();
  } catch (error) {
    loginError.textContent = error.message;
  }
});

document.querySelector("#logoutBtn").addEventListener("click", async () => {
  await api("/api/admin/logout", { method: "POST" }).catch(() => {});
  appView.classList.add("hidden");
  loginView.classList.remove("hidden");
});

document.querySelector("#reloadBtn").addEventListener("click", loadServices);
document.querySelector("#reloadGroupsBtn").addEventListener("click", loadServiceGroups);

createForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(createForm);
  try {
    const result = await api("/api/admin/services", {
      method: "POST",
      body: {
        serviceName: form.get("serviceName"),
        serviceUrl: form.get("serviceUrl"),
        authorizationCode: form.get("authorizationCode") || "",
        qps: numberValue(form.get("qps")),
        qpm: numberValue(form.get("qpm")),
      },
    });
    createForm.reset();
    createForm.qps.value = 0;
    createForm.qpm.value = 0;
    showOneTimeCode(result.authorizationCode);
    showToast("服务注册成功");
    await loadServices();
  } catch (error) {
    showToast(error.message);
  }
});

createGroupForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(createGroupForm);
  try {
    const result = await api("/api/admin/service-groups", {
      method: "POST",
      body: {
        serviceGroupName: form.get("serviceGroupName"),
        authorizationCode: form.get("authorizationCode") || "",
        serviceIds: checkedServiceIds(serviceGroupMembers),
      },
    });
    createGroupForm.reset();
    renderMemberCheckboxes(serviceGroupMembers, servicesCache, []);
    showOneTimeCode(result.authorizationCode);
    showToast("服务组创建成功");
    await loadServiceGroups();
  } catch (error) {
    showToast(error.message);
  }
});

oneTimeCode.querySelector("[data-copy-code]").addEventListener("click", () => {
  copyText(oneTimeCode.querySelector("code").textContent);
});

async function showApp() {
  loginView.classList.add("hidden");
  appView.classList.remove("hidden");
  await loadServices();
  await loadServiceGroups();
}

async function loadServices() {
  try {
    const result = await api("/api/admin/services");
    servicesCache = result.services || [];
    renderServices(servicesCache);
    renderMemberCheckboxes(serviceGroupMembers, servicesCache, checkedServiceIds(serviceGroupMembers));
  } catch (error) {
    if (error.status === 401) {
      appView.classList.add("hidden");
      loginView.classList.remove("hidden");
      return;
    }
    showToast(error.message);
  }
}

async function loadServiceGroups() {
  try {
    const result = await api("/api/admin/service-groups");
    renderServiceGroups(result.serviceGroups || []);
  } catch (error) {
    if (error.status === 401) {
      appView.classList.add("hidden");
      loginView.classList.remove("hidden");
      return;
    }
    showToast(error.message);
  }
}

function renderServices(services) {
  serviceList.textContent = "";
  if (services.length === 0) {
    const empty = document.createElement("p");
    empty.className = "empty";
    empty.textContent = "暂无服务";
    serviceList.appendChild(empty);
    return;
  }
  for (const service of services) {
    const node = template.content.firstElementChild.cloneNode(true);
    const form = node.querySelector(".edit-form");
    form.id.value = service.id;
    form.serviceName.value = service.serviceName;
    form.serviceUrl.value = service.serviceUrl;
    form.qps.value = service.qps;
    form.qpm.value = service.qpm;
    node.querySelector("[data-code-masked]").textContent = service.authorizationCodeMasked;
    updateTokenView(node, service);

    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      try {
        await api(`/api/admin/services/${service.id}`, {
          method: "PUT",
          body: {
            serviceName: form.serviceName.value,
            serviceUrl: form.serviceUrl.value,
            qps: numberValue(form.qps.value),
            qpm: numberValue(form.qpm.value),
          },
        });
        showToast("服务已保存");
        await loadServices();
      } catch (error) {
        showToast(error.message);
      }
    });

    node.querySelector("[data-refresh-token]").addEventListener("click", async () => {
      try {
        const tokens = await api(`/api/admin/services/${service.id}/tokens/refresh`, { method: "POST" });
        updateTokenView(node, tokens);
        showToast("Token 已刷新");
      } catch (error) {
        showToast(error.message);
      }
    });
    node.querySelector("[data-copy-access]").addEventListener("click", () => copyText(node.querySelector("[data-access]").value));
    node.querySelector("[data-copy-refresh]").addEventListener("click", () => copyText(node.querySelector("[data-refresh]").value));
    node.querySelector("[data-delete-service]").addEventListener("click", async () => {
      if (!confirm(`确定删除服务「${service.serviceName}」吗？删除后会同步移除它在服务组中的成员关系。`)) {
        return;
      }
      try {
        await api(`/api/admin/services/${service.id}`, { method: "DELETE" });
        showToast("服务已删除");
        await loadServices();
        await loadServiceGroups();
      } catch (error) {
        showToast(error.message);
      }
    });

    serviceList.appendChild(node);
  }
}

function renderServiceGroups(groups) {
  serviceGroupList.textContent = "";
  if (groups.length === 0) {
    const empty = document.createElement("p");
    empty.className = "empty";
    empty.textContent = "暂无服务组";
    serviceGroupList.appendChild(empty);
    return;
  }
  for (const group of groups) {
    const node = groupTemplate.content.firstElementChild.cloneNode(true);
    const form = node.querySelector(".group-edit-form");
    const members = node.querySelector("[data-group-members]");
    form.id.value = group.id;
    form.serviceGroupName.value = group.serviceGroupName;
    form.serviceGroupUrl.value = group.serviceGroupUrl;
    node.querySelector("[data-group-code-masked]").textContent = group.authorizationCodeMasked;
    renderMemberCheckboxes(members, servicesCache, group.serviceIds || []);
    updateGroupTokenView(node, group);

    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      try {
        await api(`/api/admin/service-groups/${group.id}`, {
          method: "PUT",
          body: {
            serviceGroupName: form.serviceGroupName.value,
            serviceIds: checkedServiceIds(members),
          },
        });
        showToast("服务组已保存");
        await loadServiceGroups();
      } catch (error) {
        showToast(error.message);
      }
    });

    node.querySelector("[data-refresh-group-token]").addEventListener("click", async () => {
      try {
        const tokens = await api(`/api/admin/service-groups/${group.id}/tokens/refresh`, { method: "POST" });
        updateGroupTokenView(node, tokens);
        showToast("服务组密钥已刷新");
      } catch (error) {
        showToast(error.message);
      }
    });
    node.querySelector("[data-copy-group-access]").addEventListener("click", () => copyText(node.querySelector("[data-group-access]").value));

    serviceGroupList.appendChild(node);
  }
}

function updateTokenView(node, data) {
  node.querySelector("[data-access]").value = data.accessToken || "";
  node.querySelector("[data-refresh]").value = data.refreshToken || "";
  node.querySelector("[data-access-exp]").textContent = expiresText("Access", data.accessTokenExpiresAt, data.accessTokenExpiresAtLocal);
  node.querySelector("[data-refresh-exp]").textContent = expiresText("Refresh", data.refreshTokenExpiresAt, data.refreshTokenExpiresAtLocal);
}

function updateGroupTokenView(node, data) {
  node.querySelector("[data-group-access]").value = data.accessToken || "";
  node.querySelector("[data-group-access-exp]").textContent = expiresText("Access", data.accessTokenExpiresAt, data.accessTokenExpiresAtLocal);
}

function renderMemberCheckboxes(container, services, selectedIDs) {
  const selected = new Set((selectedIDs || []).map(String));
  container.textContent = "";
  if (services.length === 0) {
    const empty = document.createElement("span");
    empty.className = "member-empty";
    empty.textContent = "暂无可选服务";
    container.appendChild(empty);
    return;
  }
  for (const service of services) {
    const label = document.createElement("label");
    const input = document.createElement("input");
    const text = document.createElement("span");
    input.type = "checkbox";
    input.value = service.id;
    input.checked = selected.has(String(service.id));
    text.textContent = service.serviceName;
    label.append(input, text);
    container.appendChild(label);
  }
}

function checkedServiceIds(container) {
  return [...container.querySelectorAll("input[type='checkbox']:checked")].map((input) => Number(input.value));
}

function expiresText(label, ts, local) {
  if (!ts) return `${label} 尚未生成`;
  return `${label} 过期时间：${ts} / ${local || ""}`;
}

function showOneTimeCode(code) {
  oneTimeCode.classList.remove("hidden");
  oneTimeCode.querySelector("code").textContent = code;
}

async function api(path, options = {}) {
  const init = {
    method: options.method || "GET",
    credentials: "same-origin",
    headers: {},
  };
  if (options.body !== undefined) {
    init.headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(options.body);
  }
  const response = await fetch(path, init);
  const text = await response.text();
  const data = text ? JSON.parse(text) : {};
  if (!response.ok) {
    const error = new Error(data.error || `请求失败：${response.status}`);
    error.status = response.status;
    throw error;
  }
  return data;
}

function numberValue(value) {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}

async function copyText(text) {
  if (!text) {
    showToast("没有可复制的内容");
    return;
  }
  await navigator.clipboard.writeText(text);
  showToast("已复制");
}

let toastTimer = null;
function showToast(message) {
  toast.textContent = message;
  toast.classList.remove("hidden");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toast.classList.add("hidden"), 2600);
}

loadServices();

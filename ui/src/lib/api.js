// Client for the go-via REST API.
//
// The UI is a static export served from the same origin as the API, so every
// path here is absolute from the root rather than relative to the app's
// basePath (/web). The session cookie is HttpOnly, so it cannot be read from
// JavaScript — it rides along automatically and the only way to know whether a
// session is live is to ask, which is what session() does.

export class ApiError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

async function request(method, path, body) {
  const opts = {
    method,
    // Same origin, but be explicit: without this the session cookie is not
    // sent on some fetch configurations.
    credentials: "same-origin",
    headers: {},
  };

  if (body !== undefined) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }

  const res = await fetch(`/v1${path}`, opts);

  if (res.status === 204) return null;

  let payload = null;
  const text = await res.text();
  if (text) {
    try {
      payload = JSON.parse(text);
    } catch {
      payload = text;
    }
  }

  if (!res.ok) {
    const message =
      payload?.error_message ||
      (typeof payload === "string" && payload) ||
      res.statusText;
    throw new ApiError(res.status, message);
  }

  return payload;
}

export const get = (path) => request("GET", path);
export const post = (path, body) => request("POST", path, body);
export const patch = (path, body) => request("PATCH", path, body);
export const del = (path) => request("DELETE", path);

// Auth

export const login = (username, password) =>
  post("/login", { username, password });
export const logout = () => post("/logout");
export const session = () => get("/session");

// Resources

export const listHosts = () => get("/hosts");
export const createHost = (host) => post("/hosts", host);
export const updateHost = (id, host) => patch(`/hosts/${id}`, host);
export const deleteHost = (id) => del(`/hosts/${id}`);

export const listGroups = () => get("/groups");
export const createGroup = (group) => post("/groups", group);
export const updateGroup = (id, group) => patch(`/groups/${id}`, group);
export const deleteGroup = (id) => del(`/groups/${id}`);

export const listImages = () => get("/images");
export const deleteImage = (id) => del(`/images/${id}`);

export const listUsers = () => get("/users");
export const createUser = (user) => post("/users", user);
export const updateUser = (id, user) => patch(`/users/${id}`, user);
export const deleteUser = (id) => del(`/users/${id}`);

export const version = () => get("/version");

// uploadImage posts a multipart form, which the JSON helpers above cannot do.
// onProgress is called with a percentage so a multi-gigabyte ISO does not look
// like a hung browser.
export function uploadImage({ file, description, hash, onProgress }) {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    form.append("file[]", file);
    if (description) form.append("description", description);
    if (hash) form.append("hash", hash);

    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/v1/images");
    xhr.withCredentials = true;

    xhr.upload.onprogress = (e) => {
      if (onProgress && e.lengthComputable) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(xhr.responseText ? JSON.parse(xhr.responseText) : null);
        return;
      }
      let message = xhr.statusText;
      try {
        message = JSON.parse(xhr.responseText).error_message || message;
      } catch {
        /* keep statusText */
      }
      reject(new ApiError(xhr.status, message));
    };

    xhr.onerror = () => reject(new ApiError(0, "upload failed"));
    xhr.send(form);
  });
}

// logSocket opens the live log stream. The session cookie is sent with the
// handshake, so no token handling is needed here.
export function logSocket() {
  const scheme = window.location.protocol === "https:" ? "wss" : "ws";
  return new WebSocket(`${scheme}://${window.location.host}/v1/log`);
}

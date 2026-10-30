let listenerCount = 0;
let inputRemoved = false;
let content = null;
let cursor = null;
let input = null;
let clipboardText = "clipboard seed";
const notifications = [];
let nextFrameID = 1;
const animationFrames = new Map();

class Node {
  constructor(tag, text = "") {
    this.tagName = tag.toUpperCase();
    this.textContent = text;
    this.style = {};
    this.children = [];
    this.listeners = new Map();
    this.clientWidth = tag === "pre" ? 800 : 0;
    this.clientHeight = tag === "pre" ? 400 : 0;
    this.scrollLeft = 0;
    this.scrollTop = 0;
    this.parentNode = null;
  }

  addEventListener(name, fn) {
    const list = this.listeners.get(name) || [];
    list.push(fn);
    this.listeners.set(name, list);
    listenerCount++;
  }

  removeEventListener(name, fn) {
    const list = this.listeners.get(name) || [];
    const at = list.indexOf(fn);
    if (at >= 0) {
      list.splice(at, 1);
      listenerCount--;
    }
  }

  __emit(name, values = {}) {
    const event = Object.assign({
      key: "",
      data: "",
      shiftKey: false,
      altKey: false,
      ctrlKey: false,
      metaKey: false,
      button: 0,
      buttons: 0,
      clientX: 0,
      clientY: 0,
      deltaY: 0,
      deltaMode: 0,
      keyCode: 0,
      isComposing: false,
      modifierState: {},
      defaultPrevented: false,
      getModifierState(name) { return !!this.modifierState[name]; },
      preventDefault() { this.defaultPrevented = true; },
    }, values);
    if (name === "paste") {
      event.clipboardData = { getData: () => values.text || "" };
    }
    for (const fn of [...(this.listeners.get(name) || [])]) fn(event);
    return event;
  }

  appendChild(child) {
    child.parentNode = this;
    this.children.push(child);
    return child;
  }

  removeChild(child) {
    const at = this.children.indexOf(child);
    if (at >= 0) this.children.splice(at, 1);
    child.parentNode = null;
    return child;
  }

  replaceChildren(...children) {
    this.children = [];
    for (const child of children) {
      const items = child.tagName === "#FRAGMENT" ? child.children : [child];
      for (const item of items) this.appendChild(item);
    }
    if (this["data-flatte-content"] !== undefined) content = this;
  }

  remove() {
    if (this.parentNode) this.parentNode.removeChild(this);
    if (this === input) inputRemoved = true;
  }

  focus() { this.__emit("focus"); }
  blur() { this.__emit("blur"); }

  getBoundingClientRect() {
    if (this.tagName === "PRE") return { left: 0, top: 0, width: this.clientWidth, height: this.clientHeight };
    return { left: 0, top: 0, width: [...String(this.textContent)].length * 8, height: 16 };
  }
}

const surface = new Node("pre");
surface["data-flatte"] = "";
let mounted = true;

const document = {
  title: "original",
  head: new Node("head"),
  getElementById() { return null; },
  querySelector(selector) { return selector === "[data-flatte]" && mounted ? surface : null; },
  createElement(tag) {
    const node = new Node(tag);
    if (tag === "textarea") input = node;
    return node;
  },
  createTextNode(text) { return new Node("#text", text); },
  createDocumentFragment() { return new Node("#fragment"); },
};

const windowListeners = new Map();
globalThis.addEventListener = (name, fn) => {
  const list = windowListeners.get(name) || [];
  list.push(fn);
  windowListeners.set(name, list);
  listenerCount++;
};
globalThis.removeEventListener = (name, fn) => {
  const list = windowListeners.get(name) || [];
  const at = list.indexOf(fn);
  if (at >= 0) {
    list.splice(at, 1);
    listenerCount--;
  }
};

globalThis.document = document;
globalThis.location = { hash: "" };
globalThis.history = { replaceState(_state, _title, hash) { globalThis.location.hash = hash; } };
globalThis.innerWidth = 800;
globalThis.innerHeight = 400;
globalThis.getComputedStyle = (node) => ({
  position: node === surface ? "relative" : "static",
  color: "rgb(255, 255, 255)",
  backgroundColor: "rgb(0, 0, 0)",
  getPropertyValue(name) {
    return ({
      "font-size": "13px",
      "line-height": "16px",
      "padding-left": "8px",
      "padding-right": "8px",
      "padding-top": "8px",
      "padding-bottom": "8px",
    })[name] || "0px";
  },
});
globalThis.ResizeObserver = class {
  constructor(fn) { this.fn = fn; }
  observe() {}
  disconnect() {}
};
globalThis.requestAnimationFrame = (fn) => {
  const id = nextFrameID++;
  animationFrames.set(id, fn);
  return id;
};
globalThis.cancelAnimationFrame = (id) => animationFrames.delete(id);
Object.defineProperty(globalThis, "navigator", { configurable: true, value: {
  clipboard: {
    writeText(text) { clipboardText = text; return Promise.resolve(); },
    readText() { return Promise.resolve(clipboardText); },
  },
} });
class BrowserNotification {
  static permission = "granted";
  constructor(title, options = {}) { notifications.push({ title, body: options.body || "" }); }
}
globalThis.Notification = BrowserNotification;

function nodeText(node) {
  if (node.tagName === "#TEXT") return node.textContent;
  if (node.children.length === 0) return String(node.textContent || "");
  return node.children.map(nodeText).join("");
}

globalThis.__flatteTest = {
  surface,
  setMounted(value) { mounted = value; },
  emitWindow(name, values) {
    for (const fn of [...(windowListeners.get(name) || [])]) fn(values || {});
  },
  flushAnimationFrame() {
    const pending = [...animationFrames.entries()];
    animationFrames.clear();
    for (const [, fn] of pending) fn(Date.now());
  },
  setNotificationPermission(value) { BrowserNotification.permission = value; },
};
Object.defineProperties(globalThis.__flatteTest, {
  input: { get: () => input },
  text: { get: () => content ? nodeText(content) : "" },
  title: { get: () => document.title },
  cursorVisible: {
    get: () => {
      cursor = cursor || surface.children.find((node) => node.ariaHidden === true || node.ariaHidden === "true");
      return !!cursor && cursor.style.display !== "none";
    },
  },
  inputRemoved: { get: () => inputRemoved },
  listenerCount: { get: () => listenerCount },
  clipboardText: { get: () => clipboardText },
  notifications: { get: () => notifications },
  pendingAnimationFrames: { get: () => animationFrames.size },
});

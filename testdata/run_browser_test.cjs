const fs = require("node:fs");
const os = require("node:os");

const [wasmExec, testWasm, mode = "test"] = process.argv.slice(2);
if (!wasmExec || !testWasm || !["test", "smoke", "landing"].includes(mode)) {
	throw new Error("usage: node run_browser_test.cjs WASM_EXEC WASM [test|smoke|landing]");
}

require(wasmExec);
require("./browser_dom_shim.cjs");

const go = new Go();
let browserPassed = false;
go.argv = mode === "test" ? [testWasm, "-test.run=TestBrowserRun", "-test.v"] : [testWasm];
// Go's js/wasm runtime caps argv+env size; pass only what the test binary
// needs rather than the (often large) CI environment.
go.env = { TMPDIR: os.tmpdir() };
// wasm_exec.js only logs a non-zero exit code by default. Surface it as the
// process status so a failing test fails CI.
if (mode === "test") {
	go.exit = (code) => process.exit(code);
} else {
	go.exit = (code) => {
		go.exitCode = code;
		go.exited = true;
		if (go._resolveExitPromise) go._resolveExitPromise();
	};
}

// If Go parks on a JS callback that never comes, Node drains its event loop
// without the runtime exiting; that must not look like success.
process.on("exit", (code) => {
  if (code === 0 && !go.exited && !browserPassed) {
    console.error("test binary never exited (stuck waiting on the event loop)");
    process.exitCode = 1;
  }
});

function smokeEvents() {
	const dom = globalThis.__flatteTest;
	const input = dom.input;
	const surface = dom.surface;
	input.__emit("keydown", { key: "s", metaKey: true });
	input.__emit("keydown", { key: "s", ctrlKey: true });
	input.__emit("paste", { text: "paste" });
	surface.__emit("pointerdown", { button: 0, clientX: 16, clientY: 8 });
	surface.__emit("pointerdown", { button: 2, clientX: 16, clientY: 8 });
	surface.__emit("pointerdown", { button: 0, clientX: 24, clientY: 24 });
	surface.__emit("wheel", { deltaY: 16, clientX: 24, clientY: 8 });
	surface.__emit("wheel", { deltaY: 16, clientX: 24, clientY: 8 });
	dom.flushAnimationFrame();
}

function smokeReady() {
	const text = globalThis.__flatteTest.text;
	return [
		"layout=true",
		"focus=true",
		"graphical=false",
		"commands=command-save,control-save",
		"chosen=open:smoke",
		"paste=paste",
		"clicks=1",
		"wheels=2",
		"clipboard=browser clipboard",
		"media=true",
	].every((part) => text.includes(part)) &&
		globalThis.__flatteTest.notifications.length === 1 &&
		globalThis.__flatteTest.clipboardText === "browser clipboard";
}

function landingReady() {
	const text = globalThis.__flatteTest.text;
	return text.includes("Game") && text.includes("App") && text.includes("Flatte") &&
		text.includes("Layout") && text.includes("UI") && text.includes("SNAKE");
}

function startLanding() {
	const input = globalThis.__flatteTest.input;
	if (!input) {
		setTimeout(startLanding, 1);
		return;
	}
	input.__emit("keydown", { key: "Enter" });
}

WebAssembly.instantiate(fs.readFileSync(testWasm), go.importObject)
	.then(({ instance }) => {
		const run = go.run(instance);
		if (mode === "test") return run;
		run.catch((error) => {
			console.error(error);
			process.exit(1);
		});
		if (mode === "smoke") setTimeout(smokeEvents, 0);
		if (mode === "landing") setTimeout(startLanding, 0);
		const deadline = Date.now() + 10000;
		const poll = () => {
			const ready = mode === "smoke" ? smokeReady() : landingReady();
			if (ready) {
				browserPassed = true;
				process.exit(0);
				return;
			}
			if (Date.now() >= deadline) {
				console.error(`browser smoke timed out: ${globalThis.__flatteTest.text}`);
				process.exit(1);
				return;
			}
			setTimeout(poll, 1);
		};
		poll();
		return undefined;
	})
	.catch((error) => {
		console.error(error);
		process.exit(1);
	});

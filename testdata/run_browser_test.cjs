const fs = require("node:fs");
const os = require("node:os");

const [wasmExec, testWasm, mode = "test"] = process.argv.slice(2);
if (!wasmExec || !testWasm || !["test", "smoke", "conformance", "landing"].includes(mode)) {
	throw new Error("usage: node run_browser_test.cjs WASM_EXEC WASM [test|smoke|conformance|landing]");
}

require(wasmExec);
require("./browser_dom_shim.cjs");

const go = new Go();
let browserPassed = false;
go.argv = mode === "test" ? [testWasm, "-test.run=^(TestBrowser(Run|HostConformance)|TestRunSelectsHostAndOwnsSessionLifecycle)$", "-test.v"] : [testWasm];
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

let conformanceStep = 0;
function conformanceEvents() {
	const dom = globalThis.__flatteTest;
	const input = dom.input;
	if (!input) return;
	const text = dom.text;
	const key = (name, values = {}) => {
		if (name.length === 1 && !values.ctrlKey && !values.altKey && !values.metaKey) {
			input.value = name;
			input.__emit("input", {});
			return;
		}
		input.__emit("keydown", { key: name, ...values });
	};
	switch (conformanceStep) {
	case 0:
		if (text.includes("async=true") && text.includes("order=init,resize")) {
			key("a");
			conformanceStep++;
		}
		break;
	case 1:
		if (text.includes("Choose a result")) {
			key("Tab");
			key("Enter");
			conformanceStep++;
		}
		break;
	case 2:
		if (text.includes("alert=true/1/")) {
			key("e");
			conformanceStep++;
		}
		break;
	case 3:
		if (text.includes("exec=true/unsupported")) {
			key("c");
			conformanceStep++;
		}
		break;
	case 4:
		if (text.includes("clipboard=host conformance clipboard")) {
			key("c", { ctrlKey: true });
			conformanceStep++;
		}
		break;
	case 5:
		if (text.includes("ctrlc=1")) {
			key("q");
			conformanceStep++;
		}
	}
}

function conformanceReady() {
	const dom = globalThis.__flatteTest;
	const text = dom.text;
	return [
		"async=true",
		"alert=true/1/",
		"exec=true/unsupported",
		"clipboard=host conformance clipboard",
		"ctrlc=1",
		"order=init,resize",
		"fxcaps=exec:false,alert:true,clipboard:true,mouse:true,quit:false",
		"ctxcaps=exec:false,alert:true,clipboard:true,mouse:true,quit:false",
	].every((part) => text.includes(part)) && conformanceStep === 6 && dom.hostExit === true && dom.inputRemoved;
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
		if (mode === "conformance") setTimeout(conformanceEvents, 0);
		if (mode === "landing") setTimeout(startLanding, 0);
		const deadline = Date.now() + 10000;
		const poll = () => {
			if (mode === "conformance") conformanceEvents();
			const ready = mode === "smoke" ? smokeReady() : mode === "conformance" ? conformanceReady() : landingReady();
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

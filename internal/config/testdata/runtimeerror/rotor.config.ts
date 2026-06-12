import { defineConfig } from "rotor/config";

function explode(): never {
	throw new Error("boom from config");
}

export default defineConfig({
	deploy: { environments: { dev: { universeId: explode() } } },
});

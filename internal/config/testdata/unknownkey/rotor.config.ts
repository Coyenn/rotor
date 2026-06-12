import { defineConfig } from "rotor/config";

export default defineConfig({
	assets: {
		paths: ["assets/**/*.png"],
		creator: { type: "user", id: 1 },
	},
	// Unknown sections are tolerated with a warning (forward compatibility).
	// @ts-expect-error -- intentionally not part of the Config type yet.
	analytics: { enabled: true },
});

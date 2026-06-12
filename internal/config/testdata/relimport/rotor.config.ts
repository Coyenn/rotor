import { defineConfig } from "rotor/config";
import { CREATOR, universeFor } from "./shared";

export default defineConfig({
	assets: {
		paths: ["assets/**/*.png"],
		output: { luau: "src/shared/assets.luau", types: "src/shared/assets.d.ts" },
		creator: CREATOR,
	},
	deploy: {
		environments: {
			dev: {
				universeId: universeFor("dev"),
				places: { start: { file: "build/game.rbxl", placeId: 2 } },
			},
		},
	},
});

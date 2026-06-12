import { defineConfig } from "rotor/config";

export default defineConfig({
	assets: {
		paths: ["assets/**/*.png", "assets/**/*.ogg"],
		output: { luau: "src/shared/assets.luau", types: "src/shared/assets.d.ts" },
		creator: { type: "group", id: 12345 },
	},
	deploy: {
		environments: {
			dev: {
				universeId: 111,
				places: { start: { file: "build/game.rbxl", placeId: 222 } },
				payments: "free",
			},
			prod: {
				universeId: 333,
				places: {
					start: { file: "build/game.rbxl", placeId: 444 },
					lobby: { file: "build/lobby.rbxl", placeId: 555 },
				},
				experience: {
					name: "My Game",
					description: "The best game",
					playability: "public",
				},
				payments: "paid",
				badges: {
					winner: {
						name: "Winner!",
						description: "You won",
						icon: "assets/badge.png",
					},
				},
			},
		},
	},
});

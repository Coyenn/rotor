import { defineConfig } from "@rotor-rbx/rotor";

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
					start: {
						file: "build/game.rbxl",
						placeId: 444,
						name: "Start Place",
						maxPlayers: 30,
						versionType: "saved",
					},
					lobby: { file: "build/lobby.rbxl", placeId: 555 },
				},
				experience: {
					name: "My Game",
					description: "The best game",
					playability: "public",
					privateServers: { price: 100 },
				},
				payments: "paid",
				badges: {
					winner: {
						name: "Winner!",
						description: "You won",
						icon: "assets/badge.png",
					},
				},
				gamepasses: {
					vip: { name: "VIP", description: "VIP perks", price: 250, icon: "assets/vip.png" },
				},
				icon: "assets/icon.png",
				thumbnails: ["assets/thumb1.png", "assets/thumb2.png"],
				products: {
					coins: { name: "100 Coins", description: "A pile of coins", price: 25 },
				},
				socials: {
					discord: { title: "Join us", url: "https://discord.gg/x", type: "discord" },
				},
			},
		},
	},
});

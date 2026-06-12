// Plain CommonJS config: no defineConfig, direct module.exports assignment.
module.exports = {
	deploy: {
		environments: {
			dev: {
				universeId: 42,
				places: { start: { file: "build/game.rbxl", placeId: 43 } },
			},
		},
	},
};

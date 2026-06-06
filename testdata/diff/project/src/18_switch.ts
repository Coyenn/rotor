function describe(n: number) {
	switch (n) {
		case 0:
			return "zero";
		case 1:
		case 2:
			return "small";
		case 3: {
			const label = "three";
			return label;
		}
		default:
			return "big";
	}
}
let mode = "idle";
switch (mode) {
	case "idle":
		mode = "running";
		break;
	default:
		mode = "unknown";
}
print(describe(0), describe(2), describe(3), describe(9), mode);

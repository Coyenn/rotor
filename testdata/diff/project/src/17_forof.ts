const items = [10, 20, 30];
let total = 0;
for (const item of items) {
	total += item;
}
for (const [i, v] of [[1, 2], [3, 4]]) {
	total += i + v;
}
const words = ["a", "b"];
let joined = "";
for (const w of words) {
	joined += w;
	if (joined !== "") {
		break;
	}
}
print(total, joined);

const fns: Array<() => number> = [];
for (let i = 0; i < 3; i++) {
	fns[i] = () => i;
}
const gns: Array<() => number> = [];
for (let k = 0; k !== 3; k++) {
	gns[k] = () => k;
}
let calls = 0;
for (let j = 0; j !== 5; ) {
	j = j + 2;
	calls += 1;
}
print(fns[0](), fns[1](), fns[2](), gns[0](), gns[1](), gns[2](), calls);

let counter = 0;
const arr = [1, 2, 3];
const obj = { a: 1, b: 2 };
const r1 = (counter++ > 0 && arr[counter] === 2) || obj.a === 1;
const r2 = arr[counter++] + arr[counter++];
let s = "x";
const r3 = `${s} ${(s += "y")} ${s}`;
const cond = counter > 2 ? arr[0] : obj.b;
if ((counter += 1) > 3 && (counter -= 1) < 10) {
	print("both sides ran");
}
print(r1, r2, r3, cond, counter);

const t = true;
const f = false;
const n = 5;
const s = "hi";
let maybe: string | undefined = undefined;
print(t && f, t || f, !t);
print(n && s);
print(maybe ?? "default");
maybe = "set";
print(maybe ?? "default");
if (n) {
	print("n truthy");
}
if (s) {
	print("s truthy");
}

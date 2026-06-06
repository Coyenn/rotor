export const constant = 42;
export let mutable = "start";
mutable = "changed";
const internal = constant + 1;
print(internal, mutable);

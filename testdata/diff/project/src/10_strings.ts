const name = "world";
const n = 3;
const plain = `no substitution`;
const multi = `a ${name} b ${n} c`;
const tableInTemplate = `${[1, 2]}`;
const concat = "x" + "y" + "z";
print(plain, multi, tableInTemplate, concat);

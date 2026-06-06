const limit = 10;
let total = 0;
for (let i = 0; i < limit; i++) {
	total += i;
}
for (let i = 10; i > 0; i -= 2) {
	total += 1;
	if (i === 4) {
		continue;
	}
	if (i === 2) {
		break;
	}
}
let count = 0;
while (count < 3) {
	count += 1;
}
do {
	count -= 1;
} while (count > 0);
if (total > 40) {
	print("big", total);
} else if (total > 20) {
	print("medium");
} else {
	print("small");
}
{
	const scoped = 1;
	print(scoped);
}
print(total, count);

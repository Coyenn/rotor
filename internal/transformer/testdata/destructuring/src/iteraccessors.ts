declare const pairsIter: IterableFunction<LuaTuple<[string, number]>>;
const [p1, p2] = pairsIter;
print(p1[0], p2[1]);
declare const nums: IterableFunction<number>;
const [, second] = nums;
print(second);
declare const tags: Set<string>;
const [, t2] = tags;
print(t2);
// A non-literal $git field cannot be inlined at compile time. Cast through
// unknown so the checker accepts the (otherwise-rejected) dynamic argument and
// the rotor diagnostic — not a type error — is what surfaces.
declare const field: string;
print($git(field as unknown as "sha"));

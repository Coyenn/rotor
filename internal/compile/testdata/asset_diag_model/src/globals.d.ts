declare function print(...params: Array<unknown>): void;

// noLib fundamental stubs (see asset_model/src/globals.d.ts). `$asset` is
// provided by the compiler's synthetic __rotor_asset.d.ts injection.
interface Array<T> {}
interface Boolean {}
interface CallableFunction {}
interface Function {}
interface IArguments {}
interface NewableFunction {}
interface Number {}
interface Promise<T> {}
interface PromiseConstructor {}
declare var Promise: PromiseConstructor;
interface Object {}
interface RegExp {}
interface String {}

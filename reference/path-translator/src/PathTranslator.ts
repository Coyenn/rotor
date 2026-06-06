import path from "path";

import {
	D_EXT,
	DTS_EXT,
	INDEX_NAME,
	INIT_NAME,
	LUA_EXT,
	LUAU_EXT,
	TRANSFORMED_EXT,
	TS_EXT,
	TSX_EXT,
} from "./constants";
import { assert } from "./util/assert";

class PathInfo {
	private constructor(
		public dirName: string,
		public fileName: string,
		public exts: Array<string>,
	) {}

	public static from(filePath: string) {
		const dirName = path.dirname(filePath);
		const parts = filePath.slice(dirName.length + path.sep.length).split(".");
		const fileName = parts.shift();
		const exts = parts.map(v => "." + v);
		assert(fileName !== undefined);
		return new PathInfo(dirName, fileName, exts);
	}

	public extsPeek(depth = 0): string | undefined {
		return this.exts[this.exts.length - (depth + 1)];
	}

	public join(): string {
		return path.join(this.dirName, [this.fileName, ...this.exts].join(""));
	}
}

export class PathTranslator {
	constructor(
		public readonly rootDir: string,
		public readonly outDir: string,
		public readonly buildInfoOutputPath: string | undefined,
		public readonly declaration: boolean,
		public readonly useLuauExtension = false,
	) {}

	private getLuauExt() {
		return this.useLuauExtension ? LUAU_EXT : LUA_EXT;
	}

	private makeRelativeFactory(from = this.rootDir, to = this.outDir) {
		return (pathInfo: PathInfo) => path.join(to, path.relative(from, pathInfo.join()));
	}

	/**
	 * Maps an input path to an output path
	 * - `.ts(x)` && !`.d.ts(x)` -> `.lua(u)`
	 * 	- `index` -> `init`
	 * - `src/*` -> `out/*`
	 */
	public getOutputPath(filePath: string) {
		const makeRelative = this.makeRelativeFactory();
		const pathInfo = PathInfo.from(filePath);

		if ((pathInfo.extsPeek() === TS_EXT || pathInfo.extsPeek() === TSX_EXT) && pathInfo.extsPeek(1) !== D_EXT) {
			pathInfo.exts.pop(); // pop .ts(x)

			// index -> init
			if (pathInfo.fileName === INDEX_NAME) {
				pathInfo.fileName = INIT_NAME;
			}

			pathInfo.exts.push(this.getLuauExt());
		}

		return makeRelative(pathInfo);
	}

	/**
	 * Maps an input path to an output .d.ts path
	 * - `.ts(x)` && !`.d.ts(x)` -> `.d.ts`
	 * - `src/*` -> `out/*`
	 */
	public getOutputDeclarationPath(filePath: string) {
		const makeRelative = this.makeRelativeFactory();
		const pathInfo = PathInfo.from(filePath);

		if ((pathInfo.extsPeek() === TS_EXT || pathInfo.extsPeek() === TSX_EXT) && pathInfo.extsPeek(1) !== D_EXT) {
			pathInfo.exts.pop(); // pop .ts(x)
			pathInfo.exts.push(DTS_EXT);
		}

		return makeRelative(pathInfo);
	}

	/**
	 * Maps an input path to an output .transformed.ts(x) path
	 * - `.ts(x)` -> `.transformed.ts(x)`
	 * - `src/*` -> `out/*`
	 */
	public getOutputTransformedPath(filePath: string) {
		const makeRelative = this.makeRelativeFactory();
		const pathInfo = PathInfo.from(filePath);

		if (pathInfo.extsPeek(1) === D_EXT) {
			// Transformers currently never get a chance to transform .d.ts files
			// But case is covered anyways
			pathInfo.exts.splice(pathInfo.exts.length - 2, 0, TRANSFORMED_EXT);
		} else {
			// splice with deleteCount 0 = insert at index, shift up further elements
			pathInfo.exts.splice(pathInfo.exts.length - 1, 0, TRANSFORMED_EXT);
		}

		return makeRelative(pathInfo);
	}

	/**
	 * Maps an output path to possible import paths
	 * - `.lua(u)` -> `.ts(x)`
	 * 	- `init` -> `index`
	 * - `out/*` -> `src/*`
	 */
	public getInputPaths(filePath: string) {
		const makeRelative = this.makeRelativeFactory(this.outDir, this.rootDir);
		const possiblePaths = new Array<string>();
		const pathInfo = PathInfo.from(filePath);

		// index.*.lua(u) cannot come from a .ts file
		if (pathInfo.extsPeek() === this.getLuauExt() && pathInfo.fileName !== INDEX_NAME) {
			pathInfo.exts.pop(); // pop .lua(u)

			// ts
			pathInfo.exts.push(TS_EXT);
			possiblePaths.push(makeRelative(pathInfo));
			pathInfo.exts.pop();

			// tsx
			pathInfo.exts.push(TSX_EXT);
			possiblePaths.push(makeRelative(pathInfo));
			pathInfo.exts.pop();

			// init -> index
			if (pathInfo.fileName === INIT_NAME) {
				const originalFileName = pathInfo.fileName;
				pathInfo.fileName = INDEX_NAME;

				// index.*.ts
				pathInfo.exts.push(TS_EXT);
				possiblePaths.push(makeRelative(pathInfo));
				pathInfo.exts.pop();

				// index.*.tsx
				pathInfo.exts.push(TSX_EXT);
				possiblePaths.push(makeRelative(pathInfo));
				pathInfo.exts.pop();

				pathInfo.fileName = originalFileName;
			}

			pathInfo.exts.push(this.getLuauExt());
		}

		if (this.declaration) {
			if ((pathInfo.extsPeek() === TS_EXT || pathInfo.extsPeek() === TSX_EXT) && pathInfo.extsPeek(1) === D_EXT) {
				const tsExt = pathInfo.exts.pop(); // pop .ts(x)
				assert(tsExt);
				pathInfo.exts.pop(); // pop .d

				// .ts
				pathInfo.exts.push(TS_EXT);
				possiblePaths.push(makeRelative(pathInfo));
				pathInfo.exts.pop();

				// .tsx
				pathInfo.exts.push(TSX_EXT);
				possiblePaths.push(makeRelative(pathInfo));
				pathInfo.exts.pop();

				pathInfo.exts.push(D_EXT);
				pathInfo.exts.push(tsExt);
			}
		}

		possiblePaths.push(makeRelative(pathInfo));
		return possiblePaths;
	}

	/**
	 * Maps a src path to an import path.
	 * Import paths are passed to RojoResolver and used virtually to resolve RbxPaths.
	 * Because of this, the import path may not actually exist.
	 * - `.d.ts(x)` -> `.ts(x)` -> `.lua(u)`
	 * 	- `index` -> `init`
	 */
	public getImportPath(filePath: string, isNodeModule = false) {
		const makeRelative = this.makeRelativeFactory();
		const pathInfo = PathInfo.from(filePath);

		if (pathInfo.extsPeek() === TS_EXT || pathInfo.extsPeek() === TSX_EXT) {
			pathInfo.exts.pop(); // pop .ts(x)
			if (pathInfo.extsPeek() === D_EXT) {
				pathInfo.exts.pop(); // pop .d
			}

			// index -> init
			if (pathInfo.fileName === INDEX_NAME) {
				pathInfo.fileName = INIT_NAME;
			}

			pathInfo.exts.push(this.getLuauExt()); // push .lua(u)
		}

		// inside of node_modules, we assume compiled file is sibling of filePath
		// outside, we check relative to outDir
		return isNodeModule ? pathInfo.join() : makeRelative(pathInfo);
	}
}

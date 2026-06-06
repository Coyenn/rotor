const brackets = "contains ]] double brackets";
const bothQuotes = `has "double" and 'single'`;
const newlines = "line1\nline2";
const tabs = "col1\tcol2";
const braces = `template with ${"literal"} and {braces}`;
const nested = `outer ${`inner ${1 + 1}`} end`;
const unicode = "héllo wörld";
print(brackets, bothQuotes, newlines, tabs, braces, nested, unicode);

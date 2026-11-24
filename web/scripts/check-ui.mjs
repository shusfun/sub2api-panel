import {readdirSync, readFileSync} from "node:fs";
import {createRequire} from "node:module";
import {join} from "node:path";
import {fileURLToPath} from "node:url";
import ts from "typescript";

// Use the CSS parser that Vite resolves under pnpm.
const require = createRequire(import.meta.url);
const postcss = createRequire(require.resolve("vite/package.json"))("postcss");
const config = JSON.parse(readFileSync(new URL("../ui-static.config.json", import.meta.url), "utf8"));
const root = fileURLToPath(new URL("../", import.meta.url));
const files = config.sources.flatMap(dir => readdirSync(join(root, dir), {recursive: true})
  .map(file => (dir + "/" + file).replaceAll("\\", "/"))
  .filter(file => /\.(css|tsx?)$/.test(file)));
const sources = files.map(file => ({file, text: readFileSync(join(root, file), "utf8")}));
const css = sources.filter(source => source.file.endsWith(".css"))
  .map(source => ({...source, ast: postcss.parse(source.text, {from: source.file})}));
const tokens = new Set();
for (const source of css) source.ast.walkDecls(/^--(?:weir|panel)-/, declaration => tokens.add(declaration.prop));

const findings = [];
function report(file, line, message) {
  findings.push(file + ":" + line + " " + message);
}

const rawHtmlControls = new Set(["button", "details", "fieldset", "dialog", "table", "select", "textarea"]);
const MAX_FEATURE_LINES = 600;

for (const {file, text} of sources) {
  for (const match of text.matchAll(/var\(\s*(--(?:weir|panel)-[\w-]+)/g)) {
    if (!tokens.has(match[1])) report(file, text.slice(0, match.index).split("\n").length, "未定义 token " + match[1]);
  }
  if (file.endsWith(".tsx") && /<select[\s>]/.test(text)) {
    report(file, 1, "禁止使用原生 <select>，请统一使用基于 Base UI 的自定义 Select 组件");
  }

  // 检查 src/ui/ 严禁反向导入 features/ 或 app/
  if (file.startsWith("src/ui/") && /\.(tsx?)$/.test(file)) {
    const ast = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, file.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS);
    const checkUiImport = node => {
      if ((ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) && node.moduleSpecifier && ts.isStringLiteral(node.moduleSpecifier)) {
        const spec = node.moduleSpecifier.text;
        if (/^@\/(?:features|app)(?:\/|$)/.test(spec) || /(?:^|\/)(?:features|app)(?:\/|$)/.test(spec)) {
          const line = ast.getLineAndCharacterOfPosition(node.getStart(ast)).line + 1;
          report(file, line, `src/ui/ 禁止反向导入 ${spec}`);
        }
      }
      ts.forEachChild(node, checkUiImport);
    };
    checkUiImport(ast);
  }

  // 检查 src/features/ 下单个 .tsx 文件行数 <= 600
  if (file.startsWith("src/features/") && file.endsWith(".tsx")) {
    const lineCount = text.split(/\r?\n/).length;
    if (lineCount > MAX_FEATURE_LINES) {
      report(file, lineCount, `业务组件文件共 ${lineCount} 行，超过上限 ${MAX_FEATURE_LINES} 行，请按职责拆分子文件`);
    }
  }

  if (!file.endsWith(".tsx") || file.startsWith("src/ui/")) continue;
  const ast = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const fail = (node, message) => report(file, ast.getLineAndCharacterOfPosition(node.getStart(ast)).line + 1, message);

  function checkClasses(node) {
    if (ts.isStringLiteralLike(node) || ts.isTemplateLiteralToken(node)) {
      for (const name of node.text.split(/\s+/)) {
        if (/\[[^\]]+\]/.test(name)) fail(node, "页面样式使用了任意值或任意选择器 " + name);
      }
    }
    ts.forEachChild(node, checkClasses);
  }

  function getAttrString(attrs, attrName) {
    for (const prop of attrs.properties) {
      if (!ts.isJsxAttribute(prop) || prop.name.getText(ast) !== attrName || !prop.initializer) continue;
      if (ts.isStringLiteral(prop.initializer)) return prop.initializer.text;
      if (ts.isJsxExpression(prop.initializer) && prop.initializer.expression && ts.isStringLiteralLike(prop.initializer.expression)) {
        return prop.initializer.expression.text;
      }
    }
    return undefined;
  }

  function visit(node) {
    if (ts.isImportSpecifier(node) && node.name.text === "EyeOff") {
      fail(node, "禁止导入 EyeOff 双图标硬切，请使用 PasswordInput 或 EyeMorphIcon");
    }
    if (ts.isJsxAttribute(node)) {
      if (node.name.getText(ast) === "style") fail(node, "页面内联样式应由公共组件解释");
      if (node.name.getText(ast) === "className" && node.initializer) checkClasses(node.initializer);
    }
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      const tag = node.tagName.getText(ast);
      if (tag === "style") fail(node, "页面不能注入样式表");
      if (rawHtmlControls.has(tag)) {
        fail(node, `页面禁止直接使用原生 <${tag}>，请使用 src/ui/ 公共组件`);
      }
      if (tag === "input") {
        fail(node, "页面禁止直接使用原生 <input>，请使用 Input / PasswordInput / FilePicker / Checkbox");
      }
      if (tag === "Input") {
        const inputType = getAttrString(node.attributes, "type");
        if (inputType === "password") fail(node, "禁止 <Input type=\"password\">，请统一使用 PasswordInput");
        if (inputType === "file") fail(node, "禁止 <Input type=\"file\">，请统一使用 FilePicker");
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(ast);
}

const protectedSelectors = [
  /\.weir-(?:button|hold-button|input|input-shell|field-control|switch|checkbox|segmented|tabs|savebar|file-picker|tag-box|toast)(?:\W|$)/,
  /\.swift-(?:row|group|disclosure|choice-card|choice-grid)(?:\W|$)/,
  /\.(?:form-(?:actions|section|grid|fields)|overlay|entity-card|stage-(?:shell|stack|panel))(?:\W|$)/,
  /\[data-slot\s*=\s*['"]?(?:table|button|input|input-shell|select|form)(?:[-'"\]]|$)/,
];

for (const {file, ast} of css) {
  ast.walkDecls(decl => {
    // 检查硬编码偶数 px（跳过 0px、1px、2px 细描边/焦点与负偏移或媒体查询外值）
    for (const m of decl.value.matchAll(/(?<![\d.-])(\d+)px\b/g)) {
      const n = Number(m[1]);
      if (n >= 4 && n % 2 === 0) {
        report(file, decl.source.start.line, `样式使用了偶数像素值 ${n}px（请改用奇数刻度或语义变量）`);
      }
    }
  });
  if (file.startsWith("src/ui/styles/")) continue;
  ast.walkRules(rule => {
    if (protectedSelectors.some(pattern => pattern.test(rule.selector))) {
      report(file, rule.source.start.line, "应用样式覆盖公共控件 " + rule.selector);
    }
  });
}

for (const contract of config.layoutContracts ?? []) {
  const source = css.find(source => source.file === contract.file);
  const declarations = new Map();
  let line = 1;
  source?.ast.walkRules(rule => {
    if (!rule.selectors.includes(contract.selector)) return;
    line = rule.source.start.line;
    rule.walkDecls(declaration => declarations.set(declaration.prop, declaration.value));
  });
  for (const [property, value] of Object.entries(contract.declarations)) {
    if (declarations.get(property) !== value) {
      report(contract.file, line, `布局约束 ${contract.selector} 需要 ${property}: ${value}`);
    }
  }
}

if (findings.length) {
  console.error(findings.join("\n"));
  process.exitCode = 1;
} else {
  console.log("界面静态检查通过，已扫描 " + sources.length + " 个源码文件。");
}

use anyhow::{anyhow, Context, Result};
use serde::Serialize;
use std::{env, fs, path::Path};
use tree_sitter::{Language, Node, Parser};

#[derive(Debug, Serialize)]
struct SymbolTable {
    file: String,
    language: String,
    symbols: Vec<Symbol>,
}

#[derive(Debug, Serialize)]
struct DependencyGraph {
    file: String,
    language: String,
    dependencies: Vec<Dependency>,
}

#[derive(Debug, Serialize)]
struct Dependency {
    target: String,
    target_symbol: String,
    kind: String,
    confidence: f64,
    line: usize,
}

#[derive(Debug, Serialize)]
struct Symbol {
    name: String,
    kind: String,
    start_line: usize,
    end_line: usize,
}

fn main() -> Result<()> {
    let mut args = env::args().skip(1);
    let command = args.next().unwrap_or_else(|| "help".to_string());
    match command.as_str() {
        "symbols" => {
            let file = args
                .next()
                .ok_or_else(|| anyhow!("usage: pane-analyze symbols <file>"))?;
            let table = analyze_file(&file)?;
            println!("{}", serde_json::to_string_pretty(&table)?);
            Ok(())
        }
        "deps" | "dependencies" => {
            let file = args
                .next()
                .ok_or_else(|| anyhow!("usage: pane-analyze deps <file>"))?;
            let graph = analyze_dependencies(&file)?;
            println!("{}", serde_json::to_string_pretty(&graph)?);
            Ok(())
        }
        _ => Err(anyhow!("usage: pane-analyze symbols <file> | deps <file>")),
    }
}

fn analyze_file(file: &str) -> Result<SymbolTable> {
    let source = fs::read_to_string(file).with_context(|| format!("read {file}"))?;
    let path = Path::new(file);
    let language = detect_language(path).ok_or_else(|| anyhow!("unsupported file type: {file}"))?;
    let tree_language = tree_sitter_language(language)?;

    let mut parser = Parser::new();
    parser.set_language(&tree_language)?;
    let tree = parser
        .parse(&source, None)
        .ok_or_else(|| anyhow!("parse failed: {file}"))?;

    let mut symbols = Vec::new();
    collect_symbols(tree.root_node(), &source, language, &mut symbols);
    Ok(SymbolTable {
        file: file.to_string(),
        language: language.to_string(),
        symbols,
    })
}

fn analyze_dependencies(file: &str) -> Result<DependencyGraph> {
    let source = fs::read_to_string(file).with_context(|| format!("read {file}"))?;
    let path = Path::new(file);
    let language = detect_language(path).ok_or_else(|| anyhow!("unsupported file type: {file}"))?;
    let tree_language = tree_sitter_language(language)?;

    let mut parser = Parser::new();
    parser.set_language(&tree_language)?;
    let tree = parser
        .parse(&source, None)
        .ok_or_else(|| anyhow!("parse failed: {file}"))?;

    let mut dependencies = Vec::new();
    collect_dependencies(tree.root_node(), &source, language, &mut dependencies);
    dependencies.sort_by(|a, b| (a.line, &a.target).cmp(&(b.line, &b.target)));
    dependencies.dedup_by(|a, b| {
        a.target == b.target && a.target_symbol == b.target_symbol && a.kind == b.kind
    });
    Ok(DependencyGraph {
        file: file.to_string(),
        language: language.to_string(),
        dependencies,
    })
}

fn detect_language(path: &Path) -> Option<&'static str> {
    match path.extension()?.to_str()? {
        "go" => Some("go"),
        "py" => Some("python"),
        "rs" => Some("rust"),
        "ts" => Some("typescript"),
        "tsx" => Some("tsx"),
        _ => None,
    }
}

fn tree_sitter_language(language: &str) -> Result<Language> {
    match language {
        "go" => Ok(tree_sitter_go::LANGUAGE.into()),
        "python" => Ok(tree_sitter_python::LANGUAGE.into()),
        "rust" => Ok(tree_sitter_rust::LANGUAGE.into()),
        "typescript" => Ok(tree_sitter_typescript::LANGUAGE_TYPESCRIPT.into()),
        "tsx" => Ok(tree_sitter_typescript::LANGUAGE_TSX.into()),
        _ => Err(anyhow!("unsupported language: {language}")),
    }
}

fn collect_symbols(node: Node, source: &str, language: &str, symbols: &mut Vec<Symbol>) {
    if let Some(kind) = symbol_kind(node.kind(), language) {
        if let Some(name) = symbol_name(node, source) {
            let start = node.start_position().row + 1;
            let end = node.end_position().row + 1;
            symbols.push(Symbol {
                name,
                kind: kind.to_string(),
                start_line: start,
                end_line: end,
            });
        }
    }

    let mut cursor = node.walk();
    for child in node.children(&mut cursor) {
        collect_symbols(child, source, language, symbols);
    }
}

fn symbol_kind(node_kind: &str, language: &str) -> Option<&'static str> {
    match (language, node_kind) {
        ("go", "function_declaration") | ("go", "method_declaration") => Some("function"),
        ("go", "type_declaration") => Some("type"),
        ("python", "function_definition") => Some("function"),
        ("python", "class_definition") => Some("class"),
        ("rust", "function_item") => Some("function"),
        ("rust", "struct_item") => Some("struct"),
        ("rust", "enum_item") => Some("enum"),
        ("rust", "trait_item") => Some("trait"),
        ("typescript" | "tsx", "function_declaration") => Some("function"),
        ("typescript" | "tsx", "method_definition") => Some("method"),
        ("typescript" | "tsx", "class_declaration") => Some("class"),
        _ => None,
    }
}

fn symbol_name(node: Node, source: &str) -> Option<String> {
    if let Some(name) = node.child_by_field_name("name") {
        return node_text(name, source);
    }
    find_identifier_descendant(node, source)
}

fn find_identifier_descendant(node: Node, source: &str) -> Option<String> {
    let mut cursor = node.walk();
    for child in node.children(&mut cursor) {
        if matches!(
            child.kind(),
            "identifier" | "type_identifier" | "field_identifier"
        ) {
            return node_text(child, source);
        }
        if let Some(value) = find_identifier_descendant(child, source) {
            return Some(value);
        }
    }
    None
}

fn collect_dependencies(
    node: Node,
    source: &str,
    language: &str,
    dependencies: &mut Vec<Dependency>,
) {
    if is_dependency_node(node.kind(), language) {
        if let Some(mut parsed) = parse_dependency(node, source, language) {
            parsed.line = node.start_position().row + 1;
            dependencies.push(parsed);
        }
    }

    let mut cursor = node.walk();
    for child in node.children(&mut cursor) {
        collect_dependencies(child, source, language, dependencies);
    }
}

fn is_dependency_node(node_kind: &str, language: &str) -> bool {
    matches!(
        (language, node_kind),
        ("go", "import_spec")
            | ("python", "import_statement")
            | ("python", "import_from_statement")
            | ("rust", "use_declaration")
            | ("typescript" | "tsx", "import_statement")
            | ("typescript" | "tsx", "call_expression")
    )
}

fn parse_dependency(node: Node, source: &str, language: &str) -> Option<Dependency> {
    let text = node_text(node, source)?.trim().to_string();
    let line = node.start_position().row + 1;
    match language {
        "go" => quoted_value(&text).map(|target| Dependency {
            target,
            target_symbol: String::new(),
            kind: "import".to_string(),
            confidence: 0.9,
            line,
        }),
        "python" => parse_python_dependency(&text, line),
        "rust" => Some(Dependency {
            target: text
                .trim_start_matches("use")
                .trim()
                .trim_end_matches(';')
                .to_string(),
            target_symbol: String::new(),
            kind: "use".to_string(),
            confidence: 0.8,
            line,
        }),
        "typescript" | "tsx" => parse_typescript_dependency(&text, line),
        _ => None,
    }
}

fn parse_python_dependency(text: &str, line: usize) -> Option<Dependency> {
    if let Some(rest) = text.strip_prefix("from ") {
        let (module, symbol) = rest.split_once(" import ")?;
        return Some(Dependency {
            target: module.trim().to_string(),
            target_symbol: symbol.split(',').next().unwrap_or("").trim().to_string(),
            kind: "import".to_string(),
            confidence: 0.85,
            line,
        });
    }
    if let Some(rest) = text.strip_prefix("import ") {
        return Some(Dependency {
            target: rest.split(',').next().unwrap_or("").trim().to_string(),
            target_symbol: String::new(),
            kind: "import".to_string(),
            confidence: 0.8,
            line,
        });
    }
    None
}

fn parse_typescript_dependency(text: &str, line: usize) -> Option<Dependency> {
    if text.starts_with("import") {
        return quoted_value(text).map(|target| Dependency {
            target,
            target_symbol: imported_ts_symbol(text),
            kind: "import".to_string(),
            confidence: 0.85,
            line,
        });
    }
    if text.contains("require") {
        return quoted_value(text).map(|target| Dependency {
            target,
            target_symbol: String::new(),
            kind: "require".to_string(),
            confidence: 0.75,
            line,
        });
    }
    None
}

fn imported_ts_symbol(text: &str) -> String {
    if let Some(start) = text.find('{') {
        if let Some(end) = text[start + 1..].find('}') {
            return text[start + 1..start + 1 + end]
                .split(',')
                .next()
                .unwrap_or("")
                .trim()
                .to_string();
        }
    }
    String::new()
}

fn quoted_value(text: &str) -> Option<String> {
    for quote in ['\"', '\''] {
        if let Some(start) = text.find(quote) {
            let rest = &text[start + 1..];
            if let Some(end) = rest.find(quote) {
                return Some(rest[..end].to_string());
            }
        }
    }
    None
}

fn node_text(node: Node, source: &str) -> Option<String> {
    node.utf8_text(source.as_bytes())
        .ok()
        .map(|value| value.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detects_go_symbols() {
        let source = "package main\n\nfunc Hello() {}\ntype Person struct{}\n";
        let language = tree_sitter_language("go").unwrap();
        let mut parser = Parser::new();
        parser.set_language(&language).unwrap();
        let tree = parser.parse(source, None).unwrap();
        let mut symbols = Vec::new();
        collect_symbols(tree.root_node(), source, "go", &mut symbols);
        assert!(symbols
            .iter()
            .any(|s| s.name == "Hello" && s.kind == "function"));
        assert!(symbols
            .iter()
            .any(|s| s.name == "Person" && s.kind == "type"));
    }

    #[test]
    fn detects_python_symbols() {
        let source = "class Greeter:\n    def hello(self):\n        pass\n";
        let language = tree_sitter_language("python").unwrap();
        let mut parser = Parser::new();
        parser.set_language(&language).unwrap();
        let tree = parser.parse(source, None).unwrap();
        let mut symbols = Vec::new();
        collect_symbols(tree.root_node(), source, "python", &mut symbols);
        assert!(symbols
            .iter()
            .any(|s| s.name == "Greeter" && s.kind == "class"));
        assert!(symbols
            .iter()
            .any(|s| s.name == "hello" && s.kind == "function"));
    }

    #[test]
    fn detects_go_dependencies() {
        let source = "package main\n\nimport \"fmt\"\nimport alias \"net/http\"\n";
        let language = tree_sitter_language("go").unwrap();
        let mut parser = Parser::new();
        parser.set_language(&language).unwrap();
        let tree = parser.parse(source, None).unwrap();
        let mut dependencies = Vec::new();
        collect_dependencies(tree.root_node(), source, "go", &mut dependencies);
        assert!(dependencies.iter().any(|d| d.target == "fmt"));
        assert!(dependencies.iter().any(|d| d.target == "net/http"));
    }

    #[test]
    fn detects_python_from_import_dependencies() {
        let source = "from auth.crypto import validate_token\nimport os\n";
        let language = tree_sitter_language("python").unwrap();
        let mut parser = Parser::new();
        parser.set_language(&language).unwrap();
        let tree = parser.parse(source, None).unwrap();
        let mut dependencies = Vec::new();
        collect_dependencies(tree.root_node(), source, "python", &mut dependencies);
        assert!(dependencies
            .iter()
            .any(|d| d.target == "auth.crypto" && d.target_symbol == "validate_token"));
        assert!(dependencies.iter().any(|d| d.target == "os"));
    }
}

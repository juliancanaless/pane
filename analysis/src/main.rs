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
        _ => Err(anyhow!("usage: pane-analyze symbols <file>")),
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
}

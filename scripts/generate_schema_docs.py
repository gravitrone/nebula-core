#!/usr/bin/env python3
"""Generate schema documentation from SQLAlchemy models.

Produces docs/SCHEMA.md with a Mermaid ER diagram and per-table
column reference. Regenerate with: make docs-schema
"""

import sys
from pathlib import Path

# Add src to path so we can import models
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "server" / "src"))

from nebula_models.base import Base


def get_column_type(col) -> str:
    """Safely extract a column type string."""

    try:
        return str(col.type)
    except Exception:
        return "unknown"


def generate_mermaid_er() -> str:
    """Create a Mermaid ER diagram from SQLAlchemy metadata."""

    lines = ["erDiagram"]

    tables = sorted(Base.metadata.tables.values(), key=lambda t: t.name)
    relationships = []

    for table in tables:
        lines.append(f"    {table.name} {{")
        for col in table.columns:
            col_type = get_column_type(col).replace("(", "").replace(")", "").replace(",", "")
            pk_marker = " PK" if col.primary_key else ""
            fk_marker = " FK" if col.foreign_keys else ""
            lines.append(f"        {col_type} {col.name}{pk_marker}{fk_marker}")
        lines.append("    }")

        for fk in table.foreign_keys:
            target_table = fk.column.table.name
            relationships.append(
                f'    {table.name} }}o--|| {target_table} : "{fk.parent.name}"'
            )

    lines.extend(relationships)
    return "\n".join(lines)


def generate_table_docs() -> str:
    """Generate markdown tables for each database table."""

    sections = []
    tables = sorted(Base.metadata.tables.values(), key=lambda t: t.name)

    for table in tables:
        sections.append(f"### {table.name}\n")
        sections.append("| Column | Type | Nullable | Key | Description |")
        sections.append("|--------|------|----------|-----|-------------|")

        for col in table.columns:
            col_type = get_column_type(col)
            nullable = "yes" if col.nullable else "no"

            key = ""
            if col.primary_key:
                key = "PK"
            elif col.foreign_keys:
                fk = next(iter(col.foreign_keys))
                key = f"FK -> {fk.column.table.name}.{fk.column.name}"

            desc = col.doc or ""
            sections.append(f"| {col.name} | {col_type} | {nullable} | {key} | {desc} |")

        sections.append("")

    return "\n".join(sections)


def main() -> None:
    """Generate docs/SCHEMA.md."""

    # Force all models to register with Base.metadata
    import nebula_models  # noqa: F401

    output = []
    output.append("# Database Schema")
    output.append("")
    output.append("> Auto-generated from SQLAlchemy models. Do not edit manually.")
    output.append("> Regenerate with: `make docs-schema`")
    output.append("")
    output.append("## Entity Relationship Diagram")
    output.append("")
    output.append("```mermaid")
    output.append(generate_mermaid_er())
    output.append("```")
    output.append("")
    output.append("## Tables")
    output.append("")
    output.append(generate_table_docs())

    docs_dir = Path(__file__).resolve().parents[1] / "docs"
    docs_dir.mkdir(exist_ok=True)
    schema_path = docs_dir / "SCHEMA.md"
    schema_path.write_text("\n".join(output), encoding="utf-8")
    print(f"Schema docs written to {schema_path}")


if __name__ == "__main__":
    main()

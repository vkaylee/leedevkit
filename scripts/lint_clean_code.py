#!/usr/bin/env python3
import sys
import re
from pathlib import Path
from typing import Any


def extract_structs(content: str) -> list[dict[str, Any]]:
    structs = []
    # Find all occurrences of 'struct <name>'
    for m in re.finditer(r"\bstruct\s+(\w+)", content):
        name = m.group(1)
        idx = m.end()
        line_no = content[: m.start()].count("\n") + 1

        # Check if it's a tuple struct (ends with ';' and has '(' before it)
        # E.g. struct Foo(pub Bar, Baz);
        semi_idx = content.find(";", idx)
        paren_idx = content.find("(", idx)
        open_brace_idx = content.find("{", idx)

        # Determine if it's a tuple struct
        is_tuple_struct = (
            paren_idx != -1
            and (open_brace_idx == -1 or paren_idx < open_brace_idx)
            and (semi_idx == -1 or paren_idx < semi_idx)
        )

        if is_tuple_struct:
            # Extract content inside paren
            # Find matching closing paren
            paren_count = 1
            body_chars = []
            for i in range(paren_idx + 1, len(content)):
                char = content[i]
                if char == "(":
                    paren_count += 1
                elif char == ")":
                    paren_count -= 1
                    if paren_count == 0:
                        break
                body_chars.append(char)
            body = "".join(body_chars)
            structs.append(
                {"name": name, "body": body, "line_no": line_no, "type": "tuple"}
            )
            continue

        # Otherwise it should be a normal struct with curly braces
        if open_brace_idx == -1:
            # E.g. unit struct like: struct Foo;
            continue

        if semi_idx != -1 and semi_idx < open_brace_idx:
            # E.g. struct Foo; before some curly brace
            continue

        # Extract content inside braces
        brace_count = 1
        body_chars = []
        body_end_idx = -1
        for i in range(open_brace_idx + 1, len(content)):
            char = content[i]
            if char == "{":
                brace_count += 1
            elif char == "}":
                brace_count -= 1
                if brace_count == 0:
                    body_end_idx = i
                    break
            body_chars.append(char)

        if body_end_idx != -1:
            body = "".join(body_chars)
            structs.append(
                {"name": name, "body": body, "line_no": line_no, "type": "normal"}
            )
    return structs


def check_private_fields(filename: str, structs: list[dict[str, Any]]) -> int:
    errors = 0
    for s in structs:
        body = s["body"]
        name = s["name"]
        line_no = s["line_no"]

        if s["type"] == "normal":
            for offset, line in enumerate(body.splitlines()):
                clean_line = re.sub(r"//.*$", "", line).strip()
                clean_line = re.sub(r"/\*.*?\*/", "", clean_line).strip()
                # Check if 'pub' exists before colon (field definition)
                # E.g. pub name: Type
                # or pub(crate) name: Type
                if re.search(r"\bpub\b[^:]*:", clean_line):
                    print(
                        f"❌ ERROR: Struct '{name}' in {filename}:{line_no + offset} has a public field."
                    )
                    print(f"   Line: {line.strip()}")
                    errors += 1
        elif s["type"] == "tuple":
            # Strip comments and check if 'pub' exists anywhere in the tuple fields
            clean_body = re.sub(r"//.*$", "", body)
            clean_body = re.sub(r"/\*.*?\*/", "", clean_body)
            # Tuple fields are separated by commas.
            # E.g. (pub i32, String)
            fields = clean_body.split(",")
            for field in fields:
                if re.search(r"\bpub\b", field):
                    print(
                        f"❌ ERROR: Tuple struct '{name}' in {filename}:{line_no} has a public field."
                    )
                    print(f"   Field: {field.strip()}")
                    errors += 1
    return errors


def check_pure_domain_separation(filename: str, content: str) -> int:
    errors = 0
    forbidden_patterns = [
        (r"\bPgConnection\b", "PgConnection (Database Connection)"),
        (r"\bTenantAuthToken\b", "TenantAuthToken (Database/Tenant Session Context)"),
        (r"\bdiesel::", "diesel framework reference"),
        (r"\bsqlx::", "sqlx framework reference"),
        (r"\bcrate::repositories\b", "crate::repositories import"),
    ]

    for pattern, description in forbidden_patterns:
        # Search line-by-line to give precise line numbers
        for idx, line in enumerate(content.splitlines()):
            # Strip comments on this line
            clean_line = re.sub(r"//.*$", "", line).strip()
            clean_line = re.sub(r"/\*.*?\*/", "", clean_line).strip()

            if re.search(pattern, clean_line):
                print(
                    f"❌ ERROR: Pure Domain file {filename}:{idx + 1} references {description}."
                )
                print(f"   Line: {line.strip()}")
                errors += 1

    return errors


def main() -> None:
    print("🔍 Running Clean Code & Architecture Linter...")
    domain_dir = Path("apiserver/src/domain")
    if not domain_dir.exists():
        print(f"❌ ERROR: Directory {domain_dir} not found.")
        sys.exit(1)
        return

    errors = 0

    # Scan all .rs files in domain/ recursively
    files = list(domain_dir.glob("**/*.rs"))

    for filepath in sorted(files, key=str):
        filename = filepath.relative_to(domain_dir.parent.parent.parent)

        # Skip mod.rs as it usually only exposes modules
        if filepath.name == "mod.rs":
            continue

        with filepath.open("r", encoding="utf-8") as f:
            content = f.read()

        # 1. Check Private Fields
        structs = extract_structs(content)
        errors += check_private_fields(str(filename), structs)

        # 2. Check Pure Domain separation (no DB connections / repos)
        errors += check_pure_domain_separation(str(filename), content)

    print(f"\nScan complete. Clean code violations found: {errors}")
    if errors > 0:
        sys.exit(1)
        return
    print("✅ All domain logic verified for clean code compliance.")
    sys.exit(0)


if __name__ == "__main__":
    main()

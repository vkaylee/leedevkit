#!/usr/bin/env python3
from pathlib import Path
import re
import sys
from typing import Any


def extract_methods(file_content: str) -> list[dict[str, Any]]:
    methods = []
    text = file_content
    fn_indices = [m.start() for m in re.finditer(r"\b(async\s+)?fn\s+(\w+)", text)]

    for idx in fn_indices:
        line_no = text[:idx].count("\n") + 1
        name_match = re.match(r"\b(?:async\s+)?fn\s+(\w+)", text[idx:])
        name = name_match.group(1) if name_match else ""

        open_brace_idx = text.find("{", idx)
        if open_brace_idx == -1:
            continue

        signature = text[idx:open_brace_idx].strip()

        brace_count = 1
        body_chars = []
        body_end_idx = -1
        for i in range(open_brace_idx + 1, len(text)):
            char = text[i]
            if char == "{":
                brace_count += 1
            elif char == "}":
                brace_count -= 1
                if brace_count == 0:
                    body_end_idx = i
                    break
            body_chars.append(char)

        if body_end_idx == -1:
            continue

        body = "".join(body_chars)
        methods.append(
            {"name": name, "signature": signature, "body": body, "line_no": line_no}
        )

    return methods


def find_files_to_scan(repo_dir: Path, worker_dir: Path) -> list[Path]:
    files_to_scan = []

    for p in repo_dir.rglob("*.rs"):
        if p.is_file():
            if p.name == "mod.rs":
                continue
            is_sub_dir = p.parent != repo_dir
            if is_sub_dir or p.name.startswith("diesel_"):
                files_to_scan.append(p)

    if worker_dir.exists():
        for p in worker_dir.rglob("*.rs"):
            if p.is_file():
                files_to_scan.append(p)
    else:
        print(f"⚠️ WARNING: Directory {worker_dir} not found.")

    return files_to_scan


def main() -> None:
    print("🔍 Running Multi-Tenancy Isolation Linter...")
    repo_dir = Path("apiserver/src/repositories")
    worker_dir = Path("apiserver/src/worker")

    if not repo_dir.exists():
        print(f"❌ ERROR: Directory {repo_dir} not found.")
        sys.exit(1)
        return

    files_to_scan = find_files_to_scan(repo_dir, worker_dir)

    errors = 0
    warnings = 0

    # Exclude system repositories that don't operate on workspace/tenant level
    system_repos = {
        "diesel_system_repository.rs",
        "diesel_system_db_cluster_repository.rs",
        "diesel_outbox_job_repository.rs",
    }

    for filepath in sorted(files_to_scan, key=lambda p: p.name):
        filename = filepath.name
        if filename in system_repos:
            continue

        with filepath.open("r", encoding="utf-8") as f:
            content = f.read()

        methods = extract_methods(content)
        for m in methods:
            sig = m["signature"]
            body = m["body"]

            is_tenant_scoped = "TenantAuthToken" in sig

            if is_tenant_scoped:
                has_workspace_ref = "workspace_id" in body or "auth_token" in body

                tables = re.findall(r"\b([a-z0-9_]+)::table\b", body)
                queries_exist = (
                    len(tables) > 0
                    or "insert_into" in body
                    or "update(" in body
                    or "delete(" in body
                )

                if queries_exist and not has_workspace_ref:
                    print(
                        f"❌ ERROR: Tenant-scoped method '{m['name']}' in {filename}:{m['line_no']} performs database operations but does not reference workspace_id or auth_token."
                    )
                    print(f"   Signature: {sig}")
                    errors += 1
                elif not has_workspace_ref:
                    print(
                        f"⚠️ WARNING: Tenant-scoped method '{m['name']}' in {filename}:{m['line_no']} takes TenantAuthToken but does not reference workspace_id or auth_token."
                    )
                    warnings += 1

                if queries_exist and "workspace_id" not in body:
                    print(
                        f"❌ ERROR: Query in method '{m['name']}' in {filename}:{m['line_no']} does not check 'workspace_id'."
                    )
                    print(f"   Signature: {sig}")
                    errors += 1

    print(f"\nScan complete. Errors found: {errors}, Warnings found: {warnings}")
    if errors > 0:
        sys.exit(1)
        return
    print("✅ All repository queries verified for multi-tenancy isolation.")
    sys.exit(0)


if __name__ == "__main__":
    main()

#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURE="$(mktemp -d)"
PY_CACHE="$(mktemp -d)"
trap 'rm -rf "$FIXTURE" "$PY_CACHE"' EXIT

mkdir -p "$FIXTURE/src" "$FIXTURE/locales/en"
printf '%s\n' '<!doctype html><html><head><title>Fixture</title></head><body><main>Hello</main></body></html>' > "$FIXTURE/index.html"
printf '%s\n' 'export const fixture = "ok";' > "$FIXTURE/src/app.ts"
printf '%s\n' '{"title":"Fixture"}' > "$FIXTURE/locales/en/common.json"
printf '%s\n' '{"scripts":{"test":"true"}}' > "$FIXTURE/package.json"

mapfile -t documented < <(grep -rhoE '\.agent/skills[^`|[:space:]]+\.py' "$ROOT/.agent/skills" --include='SKILL.md' | sort -u)
test "${#documented[@]}" -gt 0
for relative in "${documented[@]}"; do
    test -f "$ROOT/$relative" || { printf 'Missing documented skill utility: %s\n' "$relative" >&2; exit 1; }
done

while IFS= read -r script; do
    python3 -I -X "pycache_prefix=$PY_CACHE" -m py_compile "$script"
done < <(find "$ROOT/.agent/skills" -type f -name '*.py' | sort)

run_skill() {
    local relative="$1"; shift
    local errors="$FIXTURE/$(basename "$relative").stderr"
    local code=0
    python3 -I -B "$ROOT/$relative" "$@" >/dev/null 2>"$errors" || code=$?
    case "$code" in 0|1) ;; *) cat "$errors" >&2; return "$code" ;; esac
    if grep -qE 'Traceback \(|ModuleNotFoundError|ImportError|SyntaxError' "$errors"; then cat "$errors" >&2; return 1; fi
}

run_skill .agent/skills/api-patterns/scripts/api_validator.py "$FIXTURE"
run_skill .agent/skills/database-design/scripts/schema_validator.py "$FIXTURE"
run_skill .agent/skills/frontend-design/scripts/accessibility_checker.py "$FIXTURE"
run_skill .agent/skills/frontend-design/scripts/ux_audit.py "$FIXTURE" --json
run_skill .agent/skills/geo-fundamentals/scripts/geo_checker.py "$FIXTURE"
run_skill .agent/skills/i18n-localization/scripts/i18n_checker.py "$FIXTURE"
run_skill .agent/skills/lint-and-validate/scripts/lint_runner.py "$FIXTURE"
run_skill .agent/skills/lint-and-validate/scripts/type_coverage.py "$FIXTURE"
run_skill .agent/skills/mobile-design/scripts/mobile_audit.py "$FIXTURE" --json
run_skill .agent/skills/nextjs-react-expert/scripts/react_performance_checker.py "$FIXTURE"
run_skill .agent/skills/seo-fundamentals/scripts/seo_checker.py "$FIXTURE"
run_skill .agent/skills/testing-patterns/scripts/test_runner.py "$FIXTURE"
run_skill .agent/skills/vulnerability-scanner/scripts/security_scan.py "$FIXTURE" --output summary

printf '%s\n' "Skill utility smoke checks passed"

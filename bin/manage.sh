#!/bin/bash
# LeeDevKit Enterprise Infrastructure Manager
export DEVKIT_HOME="${DEVKIT_HOME:-$HOME/.leedevkit/current}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_BIN=$("$SCRIPT_DIR/scripts/_ensure-venv.sh" 2>/dev/null | tail -n 1)
if [ $# -eq 0 ]; then
    "$PYTHON_BIN" "$SCRIPT_DIR/scripts/_orchestrator.py" manage --help
    exit 0
fi
exec "$PYTHON_BIN" "$SCRIPT_DIR/scripts/_orchestrator.py" manage "$@"

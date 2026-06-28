#!/bin/bash
# LeeDevKit Enterprise Test Proxy
# Ensures venv is ready and delegates to the Python Orchestrator.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Ensure venv and get python path
PYTHON_BIN=$("$SCRIPT_DIR/scripts/_ensure-venv.sh" | tail -n 1)

if [ $# -eq 0 ]; then
    "$PYTHON_BIN" "$SCRIPT_DIR/scripts/_orchestrator.py" test --help
    exit 0
fi

export SKIP_SAFE_RUN=1 # Orchestrator handles safe-run internally

# Run orchestrator in background so we can trap signals
"$PYTHON_BIN" "$SCRIPT_DIR/scripts/_orchestrator.py" test "$@" &
PID=$!

cleanup_trap() {
    # Forward SIGTERM to Python process
    kill -TERM "$PID" 2>/dev/null
    wait "$PID" 2>/dev/null
    
    if [ -t 1 ]; then
        stty sane 2>/dev/null || true
    fi
    exit 143
}

trap cleanup_trap TERM INT HUP

wait "$PID"
EXIT_CODE=$?

# Restore PTY state to prevent hang
if [ -t 1 ]; then
    stty sane 2>/dev/null || true
fi

exit $EXIT_CODE

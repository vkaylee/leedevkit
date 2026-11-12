# Implementation Plan: `--pattern` lọc tên test bằng regex

## Context và quyết định

Audit phát hiện help và runner không cùng contract: Rust dùng substring, Web unit nhận positional filter, infra và Rust/Web coverage bỏ qua pattern. Test chủ yếu mock command; shell quoting chưa ngăn option injection.

Người dùng đã chọn:
- Regex theo tên test, dùng regex engine native từng runner.
- Hỗ trợ infra qua pytest collection hook nhỏ, không thêm dependency.
- Zero-match phải fail; matched-but-skipped vẫn tính là matched.

Không pattern/chuỗi rỗng giữ hành vi cũ; repeated flag lấy giá trị cuối; không trim regex. Reject control characters trước lifecycle; Unicode, quotes, slash/backslash và leading dash phải là dữ liệu an toàn. Invalid regex không được fallback chạy unfiltered.

Run có pattern luôn partial/full_regression=false. Focused coverage informational, không áp threshold toàn suite; unfiltered giữ nguyên gate, gồm infra 80%. `--lint-only --pattern` báo usage error. `all` áp cùng regex cho mỗi phase test bắt buộc; preflight capability tất cả target trước khi chạy.

## Thiết kế

Tái sử dụng `CliParser`, `TestHandler.run_phase`, `_safe_pattern`, `build_compose_exec`, `run_parallel_ordered`, executor/log flow hiện có.

- Rust: xác minh nextest regex expression `-E test(/…/)`; escape DSL riêng với shell quoting. Unit/integration/coverage cùng filter; filtered no-tests error.
- Web: chỉ nhận diện script/runner đã biết từ manifest; dùng name-filter native đã kiểm chứng, không positional file filter hoặc bypass project script. Unknown custom script fail rõ. Playwright dùng `--grep=<regex>`.
- Go: `-run=<regex>` với `-json` khi filtered; host stdlib kiểm tra events `run`/`skip` có `Test`, không tính package-only events. Giữ runner failure, timeout, signals, streaming/logs. Không đòi Python trong Go container.
- Infra: module mới `scripts/_pytest_pattern.py`, load rõ bằng `-p`; Python re match node ID; gọi pytest deselection hook. Cả hai lượt coverage dùng cùng filter; zero selected giữ exit failure.
- Không dùng Python re validate regex Rust/Go/JS. Docs ghi khác biệt engine, qualified name, Go subtest `/`, pytest node ID và Playwright full title.

## Dependency và tasks

Luồng: CLI/preflight → TestHandler → runner modules → executor → partial/failure report. Infra tách qua pytest hook. Go match-check phụ thuộc JSON/log executor.

### T1 — Rust regex từ CLI đến runner (Medium, tối đa 5 files)

Files: `_cli_parser.py`, `_test_handler.py`, `_test_modules.py`, `test_test_modules.py`, `test_handlers.py` dưới `scripts/` và `scripts/tests/`.

Acceptance:
- Boundary bảo toàn absent/empty/repeated, reject controls/lint-only trước lifecycle.
- Unit/integration/coverage nhận regex-name; shell và DSL escaping an toàn với slash, backslash, quotes, leading dash.
- Invalid regex/zero-match fail; unfiltered không đổi.

Verification: regression argv/handler tests; fixture Rust thật có auth/oauth/login, anchors, alternation, invalid/no-match. Native syntax chưa chứng minh thì task blocked, không đoán encoder.

### T2 — Infra regex collection (Medium)

Files: `_pytest_pattern.py` mới, `_test_handler.py`, `test_handlers.py`, `test_pytest_pattern.py` mới.

Acceptance:
- Match node IDs gồm class/parametrize; invalid/no-match fail, selected skip không bị coi zero-match.
- Hai lượt pytest dùng cùng filter; focused coverage không áp gate toàn project, unfiltered giữ 80%.
- Không sửa checkout hoặc đưa production hook vào fixture guard `scripts/tests/conftest.py`.

Verification: subprocess pytest thật trong tmp_path, assert test chọn/loại, exit và deselection; handler assert hai coverage commands.

### Checkpoint 1

T1/T2 pass; review qualified-name, DSL/option safety, zero-match mỗi phase và focused coverage trước nhánh tiếp theo.

### T3 — Web regex unit/E2E/coverage (Medium)

Files: `_test_modules.py`, config module hiện có nếu cần, `_test_handler.py`, hai test files modules/handlers.

Acceptance:
- Chỉ dùng Bun/Vitest/Jest command shape đã xác minh; unknown script fail trước test, không bypass script.
- Name-filter unit/coverage và Playwright grep chọn đúng tên, không filename; filtered zero-match fail.
- Preflight capability toàn bộ target của all trước lifecycle đầu tiên.

Verification: native fixtures có tên test khác filename, cả coverage; unknown script không gây test side effects. Không suy luận Vitest chỉ từ `bun run test`.

### T4 — Go regex và zero-match (Medium)

Files: `_test_modules.py`, `_test_utils.py`, `test_test_modules.py`, `test_test_utils.py`.

Acceptance:
- Unit/coverage dùng safe attached regex và JSON; no-tests/package-only fail; matched skip/subtest đúng.
- Host checker giữ failure/timeout/signals/logs/live output; malformed/missing evidence không báo pass.
- Unfiltered Go và runner khác không đổi.

Verification: JSON stream fixtures cho cache/skip/subtest/multiple packages/failure/malformed; real Go fixture hai packages và coverage.

### Checkpoint 2

T3/T4 pass; không silent-ignore ở nhánh test/coverage; all preflight an toàn; leading dash/quotes/slash/backslash qua từng encoder.

### T5 — Cross-runner regression matrix (Small/Medium)

Files: `test_test_modules.py`, `test_handlers.py`, thêm `test_pattern_runners.py` nếu không có chỗ tái dùng.

Acceptance:
- Phủ absent/empty/repeated, whitespace/Unicode, anchors/alternation, quote/slash/backslash/leading dash/control/invalid/no-match/failure.
- Native fixture assert cả selected và excluded; sửa test newline hiện chỉ truyền "test".
- JSON partial khi filtered pass, failed khi zero-match/failure; không false full regression.

Verification: focused pytest + available native runtime/container smoke checks. Missing toolchain ghi blocked, không gộp skip thành pass hoặc tự tải/cài.

### T6 — Help, migration và final gate (Medium)

Files: `_cli_parser.py`, `README.md`, `CHANGELOG.md`, `tasks/plan.md`, `tasks/todo.md`.

Acceptance:
- Docs nêu regex engine/name differences, empty/repeated, all, unknown scripts, no-match, partial và focused coverage.
- Release note nêu Rust substring/Web positional chuyển sang regex tên, không thêm compatibility mode.
- Evidence command/pass/fail/blocked rõ; không sửa pin runtime để làm xanh test.

Verification: `./leedevkit --help`, `./leedevkit test --help`, rồi `./leedevkit test all --json --quiet` khi compatible runtime sẵn có.

## Evidence và verification

Baseline audit: `PYTHONPATH=scripts .venv/bin/pytest scripts/tests/test_test_modules.py scripts/tests/test_handlers.py -q -k pattern --basetemp=/tmp/leedevkit-pattern-audit-289273d9`: 23 passed, 98 deselected. Đây không phải native matching E2E.

Wrapper `./leedevkit test infra --unit-only --json --quiet` bị chặn trước test vì project pin 0.8.7 không có runtime tương thích. Không tự sửa TOML, doctor repair, tải runtime hoặc packaging.

Implementation: focused pytest từng task, basetemp mới riêng từng run; tests regression fail trước fix/pass sau fix. Final dùng configured `test all` JSON và log paths. Native tests blocked phải nêu rõ; direct pytest không thay full regression gate. Graph stale nên source/coverage-check lại paths liên quan khi sửa. Packaging gate chỉ áp nếu thật sự sửa allowlist/artifact/install.

Không thêm dependency, framework adapter, general resolver, cache, report schema hay generic executor refactor. Không commit/push nếu chưa được yêu cầu.

Canonical approved plan: `/home/elt1541/.claude/plans/drifting-munching-widget.md`. Chỉ lưu tài liệu kế hoạch; chưa bắt đầu implementation.

---

## Kế hoạch trước đó — `leedevkit test` Run Isolation (giữ nguyên nội dung)

### Decision

No separate parallel runner is needed. Every invocation of `./leedevkit test <target>` already receives a unique Docker Compose project name in `scripts/_orchestrator.py`:

```python
suffix = uuid.uuid4().hex[:8]
project_name = f"leedevkit-test-{suffix}"
```

The value is applied to both `COMPOSE_PROJECT_NAME` and `PODMAN_COMPOSE_PROJECT_NAME` before test execution. Docker Compose therefore isolates containers and networks for each invocation, including runs that overlap in time.

SQLite test databases remain container-internal. Compose tmpfs/ephemeral mounts discard them when `compose down --remove-orphans -v` removes the run's containers. No host DB directory or extra cleanup layer is required.

`QUIZCLASS_VOLUME_NAMESPACE` may remain shared for Go/Bun cache reuse. Override it only when full volume isolation is required. Host ports still require unique allocation by the repository's existing Compose/test contract when a test exposes ports externally.

### Verification

```bash
python3 -m pytest scripts/tests/test_orchestrator.py scripts/tests/test_lifecycle.py -q
```

No change required in the quizclass repository.

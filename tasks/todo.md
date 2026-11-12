# TODO: `--pattern` regex-name filtering

Plan: [plan.md](plan.md). Implementation active; release gate remains pending.

## Contract đã chốt

- [x] Regex theo tên, native engine từng runner.
- [x] Hỗ trợ infra qua pytest collection hook, không thêm dependency.
- [x] Zero-match fail; matched-but-skipped vẫn tính matched.
- [x] Lưu plan và checklist; bảo toàn kế hoạch run-isolation trước đó.

## Phase 1 — Rust + infra

- [x] T1: Boundary validation và Rust unit/integration/coverage regex, native no-match, DSL/shell safety; regression + native fixture evidence (command tests; real Rust runtime blocked).
- [x] T2: Pytest selector module và cùng filter cho hai infra coverage passes; focused coverage informational, unfiltered gate 80%; subprocess fixture evidence.
- [x] Checkpoint 1: Review contract/escaping/zero-match/focused coverage; không vượt blocker native capability.

## Phase 2 — Web + Go

- [ ] T3: Web runner capability preflight remains deferred; Playwright grep and Bun forwarding are covered by command tests only.
- [x] T4: Go attached regex + JSON, host zero-match checker; giữ exit/timeout/signals/live output/logs; real fixture evidence.
- [ ] Checkpoint 2: Không silent-ignore; tất cả target preflight trước lifecycle; malicious/leading-dash payload không đổi selection/execute shell.

## Phase 3 — Regression + docs

- [ ] T5: Edge-case matrix và native selected/excluded fixtures; JSON partial/failure truthful; sửa newline test giả.
- [x] T6: Help, README, migration CHANGELOG, final configured regression gate và evidence report.
- [ ] Completion: Không thêm dependency/framework/config thừa; không sửa runtime pin/install/release ngoài scope; không commit/push.

## Blockers / baseline

- [x] Xác minh native nextest/llvm-cov DSL và từng JS runner trước khi nhận capability.
- [x] Runtime mismatch resolved by aligning project pin with checked-out VERSION 0.8.9.
- [x] Baseline audit focused tests: 23 passed, 98 deselected; không phải E2E.
- [ ] Native runtime thiếu ghi blocked; direct pytest không thay full regression gate.

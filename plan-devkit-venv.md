# Tách runtime LeeDevKit khỏi môi trường kiểm thử

Status: Completed

## Goal

Giảm chi phí `leedevkit init` và loại bỏ việc cài dependency kiểm thử vào môi trường runtime của từng dự án, đồng thời giữ CLI, Claude hooks và MCP hoạt động ổn định.

## Quyết định

Không dùng `.venv` root của dự án: runtime LeeDevKit cần được tự chứa, ổn định với dependency của dự án. Giữ `.leedevkit/.venv`, nhưng chỉ cài hai package third-party cần ở runtime: `PyYAML` (eager import từ `_compose_up.py`) và `psutil` (eager import từ `_test_utils.py`, được orchestrator nạp cho mọi command). Các tool `pytest`, `ruff`, `mypy` và coverage chỉ cài khi chạy `leedevkit test`. Python 3.11+ là baseline đã được bootstrap contract xác nhận; TOML có stdlib `tomllib`.

## Ràng buộc và interfaces

- Tạo `scripts/requirements-runtime.lock` pin `PyYAML==6.0.3` và `psutil==7.2.2`; Python 3.11+ đã là bootstrap baseline, TOML có stdlib `tomllib`. Giữ `requirements.lock` làm full test/dev lock và SBOM nguồn.
- Mở rộng `_ensure-venv.sh` nhận profile `runtime` mặc định hoặc `test`; profile test có fingerprint riêng và nâng cấp environment theo full lock. Runtime lock giúp init mới không cài toolchain test; cài tool test trong một venv đã dùng test trước đó không bị giả định tự gỡ.
- Chọn profile theo lệnh trong `bin/leedevkit`: `test` dùng test profile, lệnh còn lại dùng runtime profile; `init` vẫn tạo isolated venv và chỉ cài hai runtime libraries.
- Giữ `.leedevkit/.venv` cho self-contained runtime, Claude hook/MCP và không phụ thuộc `.venv` root của dự án.
- Gói runtime lock vào artifact và verify manifest; SBOM tiếp tục phản ánh full test/dev lock.
## Tasks

- [x] Kiểm kê imports: runtime command eager-loads `PyYAML` via `_compose_up.py` and `psutil` via `_test_utils.py`; Claude router/MCP themselves use stdlib. `tomllib` comes with Python 3.11+.
- [x] Tách runtime lock khỏi test/dev lock; `init` cài `pip`, `PyYAML`, `psutil`, còn `test` bootstrap nâng lên full lock.
- [x] Giữ hook/MCP interpreter riêng và guard; không đổi các command path hoặc interpreter contract.
- [x] Cập nhật bootstrap, doctor, update/init tests và README/runtime docs; doctor reporting phải tiếp tục phản ánh runtime venv hợp lệ.
- [x] Thêm contract tests: runtime profile không cài pytest/ruff/mypy; test profile cài full lock, gồm chuyển runtime→test; Claude hook/MCP vẫn chạy bằng isolated interpreter.

## Verify (cuối cùng)

- [x] `./leedevkit test all --json` — 859 passed, 1 skipped; production coverage 82.91%.
- [x] Build và acceptance artifact — passed.
- [x] Smoke artifact fast paths — `version` returned `0.8.2`; runtime lock included.

## Done When

- `init` không cài toolchain kiểm thử/lint nếu người dùng không gọi lệnh test.
- Claude hook/MCP chạy được sau init mà không phụ thuộc `.venv` root project.
- Lệnh `leedevkit test` bootstrap đúng dependencies theo lock; doctor/update tiếp tục repair và bảo toàn trạng thái.
- Full regression và release acceptance đạt.

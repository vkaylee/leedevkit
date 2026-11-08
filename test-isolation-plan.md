# Cô lập side effect khi chạy test

Status: Approved

## Goal

Đảm bảo test `init`/harness chỉ ghi vào project fixture hoặc workspace tạm, không bao giờ sửa `CLAUDE.md`, `.agent`, `.leedevkit` hay config của checkout thật; generated context vẫn dùng path ổn định.

## Constraints and interfaces

- Giữ `./leedevkit test all` và release acceptance hoạt động; không bắt buộc Docker/Podman cho unit tests.
- Không đổi contract runtime path: source checkout dùng `templates/...` và `.agent/...`; installed project dùng `.leedevkit/...`.
- Test fixture phải độc lập với thứ tự chạy; không dựa vào module cache hoặc `cwd` còn sót từ test trước.
- Container/workspace tạm chỉ dùng cho integration/release acceptance; mọi cleanup phải tự động.
- Không che giấu lỗi bằng skip/retry; test phải bắt được việc ghi nhầm checkout.

## Tasks

- [x] Xác định project-root stateful globals (`_bootstrap.PROJECT_ROOT`, `_devkit_config._DEVKIT_ROOT`, các handler imports) và thiết kế helper reset/resolve theo fixture; không thay đổi public CLI contract → verify bằng test đổi `cwd` giữa hai project.
- [x] Chuyển các test `init`/bootstrap/harness có side effect sang subprocess hoặc fixture-bound invocation; assert mọi output (`CLAUDE.md`, `.agent`, `.leedevkit`, `leedevkit.toml`) nằm dưới fixture root → verify bằng sentinel trong checkout.
- [x] Thêm guard test: chạy full init suite không làm thay đổi checksum/nội dung sentinel của checkout; generated instruction files không chứa `/tmp/`, pytest temp path hoặc absolute fixture path.
- [x] Giữ và mở rộng `base_context` contract cho source checkout, installed runtime và external staged runtime; kiểm tra các harness (`claude`, `codex`, `gemini`, `omp`, `pi`) dùng path ổn định.
- [x] Chỉ đưa container/workspace isolation vào acceptance/integration nếu boundary cần filesystem/process thực; không containerize unit suite mặc định.
- [x] Cập nhật tài liệu test/developer workflow về fixture root, subprocess boundary và cleanup; không ghi path môi trường tạm vào tracked files.

## Verify (last)

- [x] `.venv/bin/pytest scripts/tests/test_harness_engine.py scripts/tests/test_init_per_project.py scripts/tests/test_rust_container.py -q`
- [x] `./leedevkit test all --json`
- [x] Release acceptance not required: no packaging or release-artifact changes.
- [x] `git diff --check`; kiểm tra sentinel/diff sau test.

## Done When

- [x] Test init không thể sửa checkout thật mà vẫn pass.
- [x] Hai project fixtures chạy liên tiếp không dùng nhầm root/cache.
- [x] `CLAUDE.md` generated không chứa `/tmp/...` hoặc absolute pytest path.
- [x] Full regression đạt; release acceptance không áp dụng vì không đổi packaging/artifact.

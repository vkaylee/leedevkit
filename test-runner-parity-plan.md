# Kế hoạch hoàn thiện test runner

Status: Superseded — yêu cầu mới bỏ gate parity; tiếp tục kiểm thử pattern và logging.

Ghi chú: Nội dung bên dưới là kế hoạch cũ, không còn là điều kiện hoàn tất hoặc phát hành.

Cập nhật: `--pattern` hỗ trợ regex Go không chứa `/` với gate `go-test` mặc định; gate tùy biến và các flag không tương thích bị từ chối. Run lọc báo partial; test Go bị skip được tính là đã chọn, không chứng minh assertion đã chạy. Test dùng Compose fixture và `go test` thật; chưa xác minh qua Compose engine thật. Compose hiện buffer output chưa giới hạn. Logging custom gate và `--quiet` vẫn chưa hoàn thiện. Go suite, `go vet` đạt; coverage tổng 81,8%.

## Mục tiêu và giới hạn

Khôi phục và kiểm thử hợp đồng logging, `--quiet`, `--pattern` của `leedevkit test`; đối chiếu hành vi cũ có bằng chứng. Giữ `leedevkit.toml` và dữ liệu người dùng; không giảm ngưỡng coverage Go 80%, không bypass gate. Không coi 100% statement coverage là bằng chứng bao phủ mọi hành vi. Không thay đổi hợp đồng công khai khi chưa đánh giá tương thích.

## Công việc

- [ ] **Xác định hợp đồng:** đối chiếu code và test Go hiện tại với tài liệu, changelog và suite Python trong Git history; lập bảng hành vi và test thiếu cho từng target, human/JSON/JSONL. Ghi rõ hành vi không đủ bằng chứng, không tự nhận parity.
- [ ] **Logging:** viết test tái hiện trước khi sửa; kiểm tra output live có prefix đúng, stdout/stderr, log theo task, output không có newline cuối, tác vụ song song, failure, timeout và lỗi ghi log. Xác minh đường dẫn log trong báo cáo và không ghi ngoài fixture tạm; không lộ secrets qua diagnostics.
- [ ] **`--quiet`:** kiểm tra và sửa theo hợp đồng: tắt live task output nhưng giữ log đầy đủ; xác minh human, `--json`, `--json-stream`, thành công/lỗi/timeout, stdout machine-readable không lẫn output và exit code không đổi.
- [ ] **`--pattern`:** xác định ngữ nghĩa regex từng target theo lịch sử; test match, không match, regex sai, matched-but-skipped, ký tự shell, kết hợp `--lint-only` và flags khác. Triển khai lọc an toàn bằng argv, không chạy shell từ input; target không hỗ trợ phải từ chối rõ, không báo thành công giả. Filtered run phải báo `partial`, `full_regression=false`.
- [ ] **Tích hợp:** dùng fixture và subprocess thực tế kiểm tra CLI end-to-end, log trên filesystem, JSON/JSONL và exit code; đối chiếu suite cũ hoặc fixture hợp đồng có nguồn. Bao phủ các nhánh thành công, lỗi, timeout và các kết hợp flag quan trọng, không claim bao phủ mọi input có thể.
- [ ] **Gate và tài liệu:** chỉ thay `partial` thành full regression khi chứng minh toàn bộ hành vi cần thiết của suite cũ, không chỉ ba mục trên; nếu thiếu bằng chứng, giữ `partial`, exit 2 và ghi blocker. Cập nhật hướng dẫn, bảng parity và kế hoạch release theo kết quả thực tế.
- [ ] **Xác minh cuối:** chạy focused tests, `go test ./...`, `go vet ./...`, `gofmt`, phép đo `go test -coverpkg=./...`, `leedevkit test all --json` và `git diff --check`; ghi lệnh và kết quả thực tế. Kiểm tra Linux và macOS theo phạm vi hỗ trợ; nếu thiếu môi trường, ghi blocked. Review bảo mật độc lập nếu thay đổi bề mặt thực thi lệnh.

## Bằng chứng hợp đồng ban đầu

- Git `d2b9fb0:scripts/_test_modules.py` và `scripts/_test_utils.py`: Go dùng `go test -json -run`; cần event test `run` hoặc `skip` để xác nhận có lựa chọn, không tính package event. Rust, Web và pytest có dialect và cơ chế lọc riêng.
- Git `d2b9fb0:scripts/_test_handler.py`: `--json` và `--json-stream` cũng tắt live task output; filtered/focused run là `partial`. Hợp đồng hiện hành tại `templates/CLAUDE.base.md:18` yêu cầu prefix task và log đầy đủ ngay cả khi quiet.
- Go hiện tại: đã thêm log từng task cho infra và kiểm tra cờ không hợp lệ; custom gate chưa ghi log, `--pattern` còn bị từ chối, chưa chứng minh parity. Patch thử nghiệm lọc Go bị giữ ngoài checkout vì chưa chứng minh an toàn và đúng trong đường Compose thực tế. Cần xác minh từng đường thực thi thay vì suy từ mã lịch sử.
- Kiểm thử Go suite qua Docker và coverage tổng đạt 80,0%; `go vet` đạt. JSONL infra đã sửa để kết quả `partial`, exit 2 được xác lập trước event cuối, có regression test. Gate CLI `test all --json-stream --quiet` xuất 3 JSONL event hợp lệ, event cuối `partial`, exit 2, `full_regression=false`; chưa chạy macOS. Coverage không chứng minh parity hoặc hoàn thành logging/quiet/pattern.

## Điều kiện hoàn tất

Từng hành vi và nhánh quan trọng trong bảng hợp đồng có test đạt; coverage Go tổng ≥80%; không có lỗi chưa xử lý hoặc báo thành công sai. Parity và release readiness chỉ được xác nhận khi đủ bằng chứng riêng; các blocker release khác vẫn giữ nguyên.

Phương án ít việc hơn: giữ `--pattern` bị từ chối và gate `partial`; an toàn nhưng không hoàn thiện hợp đồng.

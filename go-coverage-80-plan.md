# Kế hoạch đạt coverage Go 80%

Status: Coverage target met — regression parity remains open

## Mục tiêu và ràng buộc

Đạt tối thiểu 80% **statement coverage tổng** bằng đúng phép đo `go test -coverpkg=./... -coverprofile=... ./...` của gate `infra`. Baseline: 68,6%; kết quả hiện tại: 80,0%. Không giảm ngưỡng, loại package, viết test chỉ để chạy dòng lệnh, hoặc thay phép đo để đạt số. Giữ nguyên thay đổi sẵn có trong `leedevkit.toml` và dữ liệu người dùng.

80% Go không chứng minh tương đương suite Python đã xóa. Gate có thể vẫn trả `partial`, exit 2 sau khi đạt coverage; parity và các blocker release khác thuộc `global-only-release-readiness-plan.md`, không tự coi là hoàn tất.

## Công việc

- [x] **Lập baseline:** `go test -coverpkg=./... -coverprofile=... ./...` đạt; profile `/tmp/leedevkit-coverage-report/cover.out` cho 68,6%, 1055/3358 statement chưa phủ. Ưu tiên `cmd/leedevkit/main.go` (380), `internal/runtime/release.go` (132), `internal/skills/skills.go` (55), `internal/debug/launch.go` (53), `internal/runtime/init.go` (36). Số lượng tính theo block duy nhất trong profile hợp nhất.
- [ ] **CLI và test runner:** bổ sung test đầu vào sai, kết quả/exit code, lựa chọn target và báo cáo JSON; kiểm tra quiet, log, timeout và các flag theo hợp đồng thực tế. `--pattern` tiếp tục fail-closed nếu chưa triển khai đúng. Test từng package và đo lại sau mỗi nhóm.
- [ ] **Runtime và dữ liệu:** test đường thành công/thất bại của update, manifest, init và skills theo kết quả baseline; xác nhận lỗi không đổi release đang chạy, lock hoặc dữ liệu project. Dùng fixture tạm, local server; không chạm runtime thật.
- [ ] **Lifecycle và orchestration:** test scheduler, Compose/process, build và cleanup ở các nhánh chưa phủ có rủi ro; kiểm tra timeout, lỗi lệnh và không xóa tài nguyên không thuộc quyền sở hữu. Dùng tiến trình/fixture thật khi cần chứng minh biên hệ thống.
- [x] **Các package còn thiếu:** tiếp tục chọn từ profile theo số statement chưa phủ và hành vi quan trọng cho tới khi tổng đạt ít nhất 80%. Phép đo `go test -coverpkg=./...` gần nhất đạt 80,0% (2688/3360 statement). Đã sửa TOML bỏ qua trường gate `snake_case`, pattern coverage bị shell diễn giải và lỗi teardown bị báo thành công; có regression test. CLI thực tế báo Infrastructure `passed`, tổng `partial`, exit 2, `full_regression=false` do parity chưa xác minh.
- [ ] **Sửa lỗi tìm được:** trước mỗi sửa hành vi, thêm regression test tái hiện lỗi và xác nhận test fail; sửa tối thiểu rồi chạy lại test. Không thay đổi public contract ngoài phạm vi mà không đánh giá tương thích.
- [x] **Xác minh cuối:** `go test ./...`, `go vet ./...`, `gofmt`, `git diff --check` đạt; `go test -coverpkg=./...` đạt 80,0%. CLI thực tế: Infrastructure passed, tổng partial, exit 2, full_regression=false. CI được cấu hình gọi cùng gate, chưa chạy CI mới; parity chưa chứng minh.

## Điều kiện hoàn tất

- Coverage tổng thực đo ≥ 80% với phép đo hiện tại; toàn bộ test và các kiểm tra Go đạt.
- Test mới xác minh kết quả và tác dụng phụ thực tế, không skip hoặc nới assertion để qua gate.
- Báo đúng `partial`/`full_regression=false` nếu parity chưa được chứng minh; đạt 80% không đồng nghĩa đủ điều kiện release.

Phương án ít việc hơn: giữ 68,6% và gate thất bại; không đáp ứng mục tiêu 80%.

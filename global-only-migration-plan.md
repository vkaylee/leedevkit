# Chuyển LeeDevKit sang global-only

Status: Approved — implementation in progress; not ready for release

## Mục tiêu và phạm vi

Một bản LeeDevKit cài theo user/môi trường, được mọi project gọi qua `leedevkit` trên PATH; không cài runtime, launcher hoặc pin phiên bản runtime trong từng project. Chấp nhận các máy và CI có thể dùng phiên bản khác nhau. Release vẫn có phiên bản, checksum và provenance; không nhầm bỏ pin theo project với bỏ định danh hoặc kiểm tra release. Project tiếp tục sở hữu cấu hình và dữ liệu của mình.

## Công việc

1. **Chốt contract và quyết định kiến trúc.** Ghi ADR về global-only, asset global so với dữ liệu project, `init`/`update`, cách xử lý repo cũ, trách nhiệm CI và rollback. Giữ `leedevkit.lock`: file này khóa commit của skills, không phải phiên bản runtime (`internal/skills/skills.go:144`). Xác nhận owner và tiêu chí loại bỏ đường cũ. → Kiểm tra: ADR nêu phương án thay thế, bảo mật, hệ quả và migration.
2. **Thiết kế cài đặt, tài nguyên và chọn project.** Chuẩn hóa một đường cài/update global theo user; CLI tìm asset từ bản cài global và tìm project từ thư mục làm việc, không từ vị trí binary. Cài/update theo transaction, kiểm tra nguồn và tính toàn vẹn, giữ bản cũ nếu thất bại. → Kiểm tra: hai project khác nhau dùng cùng bản cài; chạy từ thư mục con, ngoài project và đường dẫn có khoảng trắng.
3. **Chuyển `init` và dự án cũ.** `init` chỉ tạo/cập nhật cấu hình và tài nguyên project cần thiết; không tải runtime riêng. Xây dựng hướng dẫn migration có thể lặp lại, chỉ rõ xử lý launcher, runtime, lockfile và tham chiếu cũ; không tự xóa hoặc ghi đè dữ liệu người dùng. Cảnh báo khi dự án còn dùng launcher cũ để tránh nhầm bản thực thi. → Kiểm tra: fixture project mới/cũ, file đã sửa, chạy lại, gián đoạn và phục hồi.
4. **Chuyển các lệnh và tích hợp.** Thay giả định runtime project-local trong các lệnh, CI, script, template, cấu hình AI, tài liệu và quy tắc; dùng `leedevkit` trên PATH. Bảo toàn asset, cấu hình, skills và nội dung tùy biến thuộc project. → Kiểm tra: tìm mọi tham chiếu cũ và phân loại theo ý nghĩa; chạy các luồng `init`, `doctor`, `skills`, `test`, `update` trên fixture.
5. **Đóng gói và vòng đời release.** Điều chỉnh artifact, manifest, checksum, SBOM và release acceptance cho bản cài global; bỏ đường bootstrap local chỉ sau khi có hướng dẫn chuyển đổi và kiểm tra consumers. Ghi breaking change và cách khôi phục bản trước, không sửa release đã phát hành. → Kiểm tra: cài từ local mirror, chạy offline sau cài, update thành công/thất bại và khôi phục; không ghi vào repo ngoài tài nguyên project được yêu cầu.
6. **Kiểm thử và nghiệm thu cuối.** Thêm regression tests cho lựa chọn project, assets, migration an toàn, PATH, cạnh tranh update, lỗi download/checksum và không phá bản cài cũ; chạy full configured suite và packaging acceptance trong môi trường cô lập. Xác nhận CI mới không phụ thuộc runtime project-local. → Kiểm tra: `./leedevkit test all --json` và `scripts/build-release-go.sh /tmp/dist` theo contract hiện hành hoặc lệnh thay thế được cập nhật; báo rõ mọi check blocked, skipped hay partial.

## Trạng thái triển khai

- Đã tích hợp bản nháp init, skills, doctor, global Go Compose, installer, template và CI checksum; installer offline tests và Go suite trong Docker đạt. Init chọn template theo `go.mod`/`Cargo.toml`, đọc asset từ executable và không tạo runtime project-local.
- `update` đã nối lại: kiểm tra checksum/manifest, khóa chung installer, stage và kích hoạt atomic; test fixture bao gồm upgrade, checksum sai/thiếu, lock và bảo toàn project. Acceptance artifact thật qua HTTP loopback đã đạt sau khi sửa thiếu SBOM; còn cần failure injection cho activation trước release.
- Review bảo mật phát hiện installer shell chưa giới hạn kích thước download/giải nén; chưa xử lý an toàn, chặn release. Còn thiếu hướng dẫn migration chi tiết và failure injection activation. Go CLI hiện không hỗ trợ Rust test target; Rust Compose mới được kiểm tra config, không khẳng định Rust test hoạt động.
- Artifact từ source hiện tại đã build thành công; checksum, cài từ local mirror, `version` và `init` trong project cô lập đạt. Update qua HTTP loopback từ binary cài thật sang artifact mới, rồi rollback symlink trong fixture Linux đều đạt; fixture bản cũ dùng cùng binary với thư mục version khác, chưa chứng minh tương thích binary release cũ hoặc rollback trên macOS.
- Full configured suite `leedevkit test all --json` trong bản sao cô lập thất bại với `Go test path does not support target "all"` vì cấu hình project dùng target `infra`; gọi trực tiếp `test infra --json` cũng thất bại với `test target "infra" has no configured services`. Cần sửa và chạy lại. Source wrapper cũng không chạy `test --help` vì thiếu binary Go.

## Rollout và dừng

Phát hành thay đổi như breaking release theo chính sách versioning; thông báo trước cách migrate và rollback. Không tự xóa runtime, launcher, lockfile hoặc dữ liệu trong project cũ. Trong kế hoạch này, giữ `leedevkit.lock` cho skills; không coi nó là runtime lockfile. Dừng triển khai nếu mất dữ liệu tùy biến, không xác định được asset, cài/update hỏng làm mất bản chạy được, hoặc CI downstream chưa có đường chuyển đổi. Rollback bằng bản global trước và khôi phục tệp project từ backup/Git; xác minh fixture trước khi công bố. Chỉ loại bỏ hỗ trợ cũ khi consumers trong phạm vi đã được kiểm kê và migration được xác nhận; không giả định đã chuyển hết các repo bên ngoài.

## Ghi chú khảo sát

- `README.md:5` và `README.md:30` mô tả CLI global nhưng runtime pin theo project; `README.md:44` lại khuyến nghị bootstrap local.
- `install.sh:9` đã cài theo user; `internal/runtime/init.go:32` và `internal/runtime/release.go:53` vẫn init/update runtime riêng trong project. `cmd/leedevkit/main.go:383` tìm project từ CWD.
- `cmd/leedevkit/skills.go:10` và `internal/skills/skills.go:67` dùng skills trong `.leedevkit` của project; update hiện bảo toàn `skills.d` và `.venv` (`internal/runtime/release.go:97`). Phải xác định nơi lưu dữ liệu đó và migrate an toàn, không chỉ đổi lookup.
- CI vẫn kiểm tra bootstrap và launcher (`.github/workflows/quality.yml:26`) và dựa vào release artifact (`.github/workflows/release.yml:56`); phải chuyển gate cùng packaging.
- `.agent/rules/devkit-internals.md:36` yêu cầu packaging acceptance cho thay đổi cài đặt/release; cập nhật gate thay vì xóa bằng chứng tương đương.
- Baseline: `./leedevkit --help` hoạt động; `./leedevkit test --help` trả exit 2 với `Go-only mode requires executable leedevkit-go binary`. Cần khôi phục môi trường kiểm thử trước implementation; chưa chạy test suite.
- Giữ nguyên thay đổi sẵn có trong `leedevkit.toml`.

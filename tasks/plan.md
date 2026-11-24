# Kế hoạch: Test output có cấu trúc cho AI agents

## Mục tiêu

Hoàn thiện thay đổi đang có trong working tree: trích xuất failure theo test case và compact chính xác với runner được hỗ trợ, không mất stderr lifecycle, giữ hợp đồng CLI và MCP.

## Quyết định

- Ưu tiên output có cấu trúc của runner; không suy đoán runner chỉ từ ngôn ngữ hoặc lệnh `bun run`.
- Không sửa hành vi lệnh test tùy biến hay thêm flag không tương thích. Xác định adapter theo cấu hình và khả năng runner; khi chưa xác định được, giữ log đầy đủ trên disk, báo rõ không thể lọc chính xác.
- Giữ schema JSON version 1 và các trường hiện có; thêm metadata về mức hỗ trợ nếu cần. Không bịa file, dòng hoặc test name.
- Fallback không được tuyên bố compact chính xác; không gửi nguyên log lớn vào context agent. Giữ diagnostic, đường dẫn log và hướng dẫn rerun.

## Công việc

### 1. Khóa hợp đồng và kiểm chứng runner

- Xác định gate Go mặc định, gate tùy biến, script web và runner được dự án hỗ trợ. Kiểm tra tài liệu phiên bản thực tế của Go, Vitest và Bun trước khi chọn reporter/flag.
- Xác định hành vi khi không nhận diện được runner, lỗi build/setup, không có failure record, hoặc reporter lỗi; giữ exit code và diagnostic.
- Tiêu chí: fixture độc lập cho các trường hợp trên; contract JSON, compact và `--json-stream` rõ ràng; không thay đổi command tùy biến ngoài cấu hình được hỗ trợ.
- Kiểm tra: focused tests cho scheduler, gates và CLI.
- Phụ thuộc: không. Phạm vi dự kiến: `internal/gates/`, `cmd/leedevkit/`, test fixtures.

### 2. Go structured output

- Dùng `go test -json` cho gate Go nhận diện chắc chắn, không thay đổi gate custom. Gom theo package/test, xử lý test lồng nhau, package/build failure, output xen kẽ và stderr.
- Xuất `failures` theo test thất bại, không lặp record package và test; giữ stack và message khi có; compact không in output test PASS.
- Tiêu chí: fixture nhiều package/test pass/fail, lỗi compile/setup, log lớn, ANSI và exit code; JSON-stream kết thúc với structured summary tương đương.
- Kiểm tra: focused Go tests và integration fixture qua CLI.
- Phụ thuộc: 1. Phạm vi: runner Go, report parser, tests.

### 3. Web structured output

- Khám phá runner thực tế và cấu hình adapter/report file theo tài liệu runner; hỗ trợ Vitest trước, Bun test khi xác thực được output và cách gọi. Không tự giả định script web dùng Vitest.
- Chuẩn hóa test name, file, line, message, stack; không ghi đè reporter người dùng hoặc làm thay đổi exit code. Xử lý reporter thiếu, lỗi và nhiều test failures.
- Tiêu chí: fixture output thực từ từng runner được hỗ trợ; integration test script giả lập hoặc runner thật; compact loại PASS mà vẫn giữ lỗi setup.
- Kiểm tra: focused web runner tests; chạy target web cấu hình thực tế nếu môi trường cung cấp.
- Phụ thuộc: 1; có thể làm song song với 2 sau khi chốt contract. Phạm vi: web runner, adapter, tests.

### 4. Compact và fallback

- Dùng cùng failure records cho `--json`, `--failed-only`, `--compact` và MCP; không in log của phase failed nguyên khối khi có structured result.
- Với output chưa hỗ trợ, chỉ in diagnostic ngắn cùng đường dẫn log, và đánh dấu không thể lọc chính xác; log gốc vẫn giữ trên disk. Với build/lifecycle failure, trả stderr cần thiết và không nuốt lỗi.
- Tiêu chí: kiểm thử pass/fail/blocked, nhiều failed checks infra, JSON cuối cùng, phối hợp `--json-stream`, log 300 KB+, stderr runtime và log không hỗ trợ.
- Kiểm tra: focused CLI và lifecycle tests.
- Phụ thuộc: 2 và 3.

### 5. Discovery, MCP và kiểm thử hồi quy

- Đối chiếu `targets --json` với target và phase thực sự chạy; MCP `list_targets` dùng cùng nguồn. Kiểm tra MCP `initialize`, notifications, tool schema, error handling, run_test flags và output stdout không nhiễm log.
- Rà lỗi sẵn có khi alias dependency chọn nhầm service; chỉ sửa nếu xác nhận liên quan đến tính đúng của target được yêu cầu, kèm regression test.
- Tiêu chí: fixture target api/web/all/infra; MCP client stdio thực tế gọi đủ ba tool; không thay đổi code hay config ngoài phạm vi cần thiết.
- Kiểm tra: `go test ./...` theo CI, sau đó build binary và `LEEDEVKIT_GO_BINARY=<binary> ./leedevkit test all --json`; chạy target web/Go tương ứng nếu manifest có cấu hình. Báo rõ môi trường hoặc runner không có.
- Phụ thuộc: 4 cho kiểm thử cuối; kiểm tra discovery độc lập có thể làm sớm.

## Checkpoints

- Sau 1: xác nhận contract, phiên bản runner và fallback trước khi đổi lệnh test.
- Sau 2–3: test từng adapter và tích hợp CLI độc lập.
- Sau 4–5: chạy full regression cấu hình, kiểm tra MCP stdio, review diff và báo giới hạn còn lại.

## Rủi ro

- Reporter khác phiên bản hoặc custom script không hỗ trợ flag: chỉ kích hoạt adapter khi xác định được khả năng; fallback có trạng thái rõ ràng.
- Log xen kẽ, test lồng nhau hoặc output lớn: fixture đa package, nhiều test, giới hạn bộ nhớ hợp lý và không mất diagnostic.
- Working tree đã có thay đổi: giữ nguyên và hoàn thiện; không reset, commit hoặc ghi đè thay đổi không liên quan.

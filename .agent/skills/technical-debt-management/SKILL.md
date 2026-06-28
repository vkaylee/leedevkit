---
name: technical-debt-management
description: Phân tích, đánh giá và xử lý Nợ Kỹ Thuật (Technical Debt) một cách có hệ thống. Dùng khi user yêu cầu refactor, tối ưu code cũ, hoặc dọn dẹp các đoạn code/kiến trúc không còn phù hợp.
---

# 🧹 Technical Debt Management Skill

## 🎯 Mục đích (Purpose)
Skill này cung cấp phương pháp luận để xác định, định lượng, và giải quyết Technical Debt một cách an toàn mà không làm gãy (break) các tính năng hiện tại. Đảm bảo codebase luôn duy trì ở trạng thái "Enterprise Grade".

## 🧭 Nguyên tắc cốt lõi (Core Principles)

### 1. Phân loại Nợ Kỹ Thuật
Trước khi tiến hành sửa đổi, bắt buộc phải phân loại khoản "nợ" đang đối mặt:
- **Code Debt:** Code phức tạp, lặp lặp (vi phạm DRY), function quá dài, magic numbers, hardcoded strings (cần thay bằng i18n).
- **Architecture Debt:** Các component phụ thuộc chéo (tight coupling), vi phạm SOLID, thiếu lớp Interface/Repository abstraction.
- **Testing Debt:** Phủ test dưới mức 80%, thiếu các test E2E hoặc có các test "flaky" (thiếu tính ổn định).
- **Documentation/UI Debt:** Comment không còn đúng với logic hiện tại, UI sử dụng các HTML elements gốc thay vì Design System (vi phạm tiêu chuẩn Premium UI).

### 2. Định lượng & Ưu tiên (Prioritization)
Không phải khoản nợ nào cũng cần trả ngay lập tức. Đánh giá theo ma trận:
- **High Impact / Low Effort (Làm ngay):** Tách hàm nhỏ, xóa code thừa, chuyển hardcode sang i18n, nâng cấp các component UI cơ bản.
- **High Impact / High Effort (Cần lên kế hoạch):** Thay đổi State Management, chuyển đổi DB schema. (Bắt buộc dùng `/plan` và xin ý kiến user).
- **Low Impact:** Ghi nhận lại bằng các thẻ TODO có gắn issue number.

### 3. Quy trình "Safe Refactoring" (Bắt buộc)
Khi xử lý nợ kỹ thuật, bạn phải tuân thủ nghiêm ngặt 3 bước:
1. **Bảo vệ (Protect):** Không bao giờ refactor code không có test. Xác nhận test đang Passed (`./run-tests.sh --api`) HOẶC chủ động viết test bổ sung trước khi sửa đổi.
2. **Cải tạo (Refactor):** Tổ chức lại cấu trúc code nhưng TUYỆT ĐỐI KHÔNG làm thay đổi hành vi/logic nghiệp vụ hiện tại.
3. **Kiểm chứng (Verify):** Xác minh lại bằng hệ thống type-checker (`./scripts/_npm.sh run typecheck`), linter, và chạy toàn bộ test suite. Kết quả phải trả về 0 errors, 0 warnings.

### 4. Enterprise Grade Enforcement
Quá trình dọn dẹp Nợ Kỹ Thuật phải luôn hướng về tiêu chuẩn được định nghĩa tại `GEMINI.md`:
- Mọi UI components sau khi dọn dẹp phải tuân thủ Premium Standard (e.g., dùng Combobox/Radix, có Error Boundary).
- Mọi APIs sau khi refactor phải đảm bảo Type-safety, không có lỗi N+1 Query.

## 📋 Lời nhắc khi giao tiếp với User
Khi kích hoạt Skill này, AI cần tóm tắt nhanh tình trạng của đoạn code:
`"Phát hiện [X] vấn đề nợ kỹ thuật (Hardcode/Coupling/Missing Tests). Đang tiến hành Safe Refactoring..."`

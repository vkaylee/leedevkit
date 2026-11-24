# Checklist: Test output cho AI agents

- [ ] Kiểm chứng runner thực tế, định nghĩa contract và fallback không nhận diện.
- [ ] Thêm adapter Go structured, test nhiều package/test, lỗi build và JSON-stream. Đã kiểm tra fixture nhiều package, compiler thật và JSON-stream qua CLI; còn giới hạn log lớn và stderr không nhận diện.
- [ ] Thêm adapter Vitest theo reporter chính thức và kiểm thử thực tế. Repo không có package.json/Vitest; script `bun run test` có thể dùng runner tùy biến, hiện dùng fallback an toàn.
- [ ] Xác thực Bun test; Bun 1.4.4 hỗ trợ JUnit, nhưng chưa có cấu hình xác định script dùng Bun test để bật reporter an toàn.
- [ ] Hoàn thiện compact, blocked/lifecycle stderr và fallback ngắn có log path.
- [x] Xác minh targets discovery và cả ba MCP tool qua stdio. Đã bổ sung direct service và sửa alias chọn nhầm; MCP stdio thực tế gọi đủ ba tool.
- [ ] Review diff, chạy Go tests và `./leedevkit test all --json`; báo các target hoặc runner chưa chạy được.

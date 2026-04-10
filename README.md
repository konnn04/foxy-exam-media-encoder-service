# Media Encoder Service (Go + FFmpeg)

Dịch vụ vi xử lý video hậu kỳ cho hệ thống Foxy Exam. Chạy trên cổng `8097`.

## Kiến trúc

```
Laravel (exam-sys)                   Go Service (media-encoder)
     │                                     │
     │  POST /api/stitch {files...}        │
     ├────────────────────────────────────►│  → FFmpeg re-encode + concat
     │                                     │
     │  POST /api/snapshot {file, offset}  │
     ├────────────────────────────────────►│  → FFmpeg -ss -vframes 1
     │                                     │
     │  POST /api/clip {file, start, dur}  │
     ├────────────────────────────────────►│  → FFmpeg clip + watermark
     │                                     │
     │  ◄── JSON {success, output_file}    │
     └─────────────────────────────────────┘
```

## Cài đặt & Chạy

### Local Development
```bash
# Yêu cầu: Go >= 1.22, FFmpeg
cd media-encoder-service

# Tải dependencies (chỉ cần chạy 1 lần)
go mod tidy

# Gen Swagger docs (nếu có update endpoint)
swag init

# Chạy service
go run .
# → 🚀 Media Encoder Service listening on :8097
```

### Docker
```bash
docker-compose up -d --build
```
> **Lưu ý**: Container `media-encoder` đã được khóa RAM (`mem_limit: 2g`) để đảm bảo các tiến trình FFmpeg không gây ra Out-Of-Memory (OOM) cho host.

## Swagger API Docs

Dịch vụ này đã tích hợp Swagger UI để review và test trực tiếp các endpoint.
Sau khi chạy ứng dụng (cổng 8097), truy cập vào:
**👉 `http://localhost:8097/swagger/index.html`**

## API Endpoints

Tất cả API (trừ `/health` và `/swagger`) yêu cầu header:
```
Authorization: Bearer KLTN-hehehehe
```

| Method | Path | Mô tả |
|--------|------|--------|
| GET | `/health` | Health check (public) |
| GET | `/swagger/*any` | Giao diện API Docs (public) |
| POST | `/api/ffmpeg/run` | Chạy lệnh FFmpeg tùy ý |
| POST | `/api/ffmpeg/probe` | Lấy duration (ms) của file |
| POST | `/api/stitch` | Ghép nhiều chunks → 1 MP4 |
| POST | `/api/snapshot` | Trích JPEG tại offset |
| POST | `/api/clip` | Cắt MP4 clip + watermark |
| GET | `/api/encoder` | Phát hiện GPU encoder |

## Ví dụ API (Trích xuất)

### Extract clip with watermark
```bash
curl -X POST http://localhost:8097/api/clip \
  -H "Authorization: Bearer KLTN-hehehehe" \
  -H "Content-Type: application/json" \
  -d '{
    "source_file": "/data/storage/exam_recordings/1/10/camera_final.mp4",
    "start_sec": 40.0,
    "duration_sec": 15,
    "watermark": "multiple_faces - 14:30:00 09/04/2026",
    "output_file": "/data/storage/violation_clips/1/10/violation_5_cam.mp4"
  }'
```

## Tích hợp CI/CD (GitHub Actions)
Service được trang bị GitHub Actions workflow (`.github/workflows/golang.yml`).
Mỗi khi có PR push lên folder `media-encoder-service`, pipeline sẽ tự động chạy:
1. Setup Go 1.22
2. Tải dependencies & Cache
3. `go vet ./...` rà soát lỗi tĩnh.
4. Xây dựng (`go build`) và `go test` đảm bảo ứng dụng không crash.

## Biến môi trường

| Biến | Mặc định | Mô tả |
|------|----------|--------|
| `API_KEY` hoặc `MEDIA_ENCODER_SERVICE_API_KEY` | *(bắt buộc)* | API key xác thực |
| `PORT` | `8097` | Cổng HTTP |

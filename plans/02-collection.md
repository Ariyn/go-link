# 계획 02 - 컬렉션 설정 (links)

Status: done

## 목표
- Admin UI에서 `links` 컬렉션 생성
- 필드 제약 및 인덱스 규칙 적용

## 작업
- `links` 컬렉션 생성
- 필드 추가:
  - slug (text, required, unique)
  - target_url (url 또는 text, required)
  - enabled (bool, 기본 true)
  - hits (number, 기본 0)
  - last_hit_at (date)
- slug에 유니크 인덱스 보장

# 계획 04 - 리다이렉트 정책

Status: done

## 목표
- 안전한 target URL 처리 강제
- 올바른 헤더와 상태 코드 반환

## 작업
- target_url 스킴이 http 또는 https인지 검증
- 다른 스킴은 차단
- 302와 Location 헤더로 응답
- `Cache-Control: no-store` 추가
